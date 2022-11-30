package applier

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hornwind/filter-chain/internal/models"
	ipt "github.com/hornwind/filter-chain/internal/models/firewall/iptables"
	"github.com/hornwind/filter-chain/pkg/config"
	"github.com/hornwind/filter-chain/pkg/ipset"
	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
	utilexec "k8s.io/utils/exec"
)

const (
	namePrefix             = "fc"
	reconciliationInterval = 30 * time.Second
	iptTable               = "filter"
	iptChain               = "ipset-filter"
	iptChainDefaultPolicy  = "DROP"
	iptMatchSetTemplate    = "-m set --match-set %s src -j %s"
)

type Applier struct {
	mu             sync.RWMutex
	fnCancelRunCTX context.CancelFunc
	config         config.Config
	storage        models.Repository
	fw             models.Firewall
	set            ipset.Interface
	liveSets       map[string]interface{}
}

func NewApplier(localCTX context.CancelFunc, config config.Config, storage models.Repository) (*Applier, error) {
	exec := utilexec.New()
	fw, err := ipt.NewIptables()
	if err != nil {
		return nil, err
	}

	applier := &Applier{
		mu:             sync.RWMutex{},
		fnCancelRunCTX: localCTX,
		config:         config,
		storage:        storage,
		set:            ipset.New(exec),
		fw:             fw,
		liveSets:       make(map[string]interface{}, 1),
	}
	return applier, nil
}

func (a *Applier) Run(ctx context.Context) {
	ticker := time.NewTicker(reconciliationInterval)
	defer ticker.Stop()
	localCTX, cancel := context.WithCancel(ctx)
	a.fnCancelRunCTX = cancel

	// time.Sleep(15 * time.Second)
	log.Debug("Applier started")

	for {
		select {
		case <-localCTX.Done():
			log.Debug("Applier ctx:", localCTX.Err())
			return
		case <-ticker.C:
			if err := a.refreshLiveSets(); err != nil {
				log.Error(err)
				return
			}
			if err := a.reconcile(); err != nil {
				log.Error(err)
				return
			}
		}
	}

}

func (a *Applier) refreshLiveSets() error {
	sets, err := a.set.ListSets()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(sets) == 0 {
		// flush liveSets
		a.liveSets = make(map[string]interface{}, 1)
		return nil
	}
	// flush and refill liveSets
	a.mu.Lock()
	a.liveSets = make(map[string]interface{}, 1)
	for _, v := range sets {
		a.liveSets[v] = nil
	}
	a.mu.Unlock()
	return nil
}

func (a *Applier) ipsetCreateOrUpdate(name string, entries []string) error {
	if len(entries) != 0 {
		newSet := &ipset.IPSet{
			Name: name,
		}
		if _, ok := a.liveSets[name]; ok {
			// if ipset with name `n` exists
			temp := strings.Join([]string{name, "temp"}, "-")
			newSet.Name = temp
			// create tmp ipset
			if err := a.set.RestoreSet(entries, newSet, true); err != nil {
				return err
			}
			// and swap with existing set
			if err := a.set.SwapSets(temp, name); err != nil {
				return err
			}
			// Flush tmp set
			if err := a.set.FlushSet(newSet.Name); err != nil {
				return err
			}
			// And delete it
			if err := a.set.DestroySet(newSet.Name); err != nil {
				return err
			}
		} else {
			// else create it
			if err := a.set.RestoreSet(entries, newSet, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Applier) reconcile() error {
	pos := 1
	var muPos sync.RWMutex
	type rule struct {
		bucket string
		name   string
		rule   []string
	}
	ruleChan := make(chan rule)
	defer close(ruleChan)

	go func(data <-chan rule) {
		log.Debug("Start iptables filling goroutine loop")
		for item := range data {
			// tr := strings.Split(item.rule, " ")
			// log.Debug(tr)
			muPos.Lock()
			if _, err := a.fw.EnsureRule(pos, iptTable, iptChain, item.rule...); err != nil {
				log.Error("Chain filling goroutime failed: ", err)
				return
			}
			if item.bucket != "" {
				log.Debug("Set bucket ", item.bucket, " is applied")
				if err := a.storage.SetIpsetApplied(item.bucket); err != nil {
					log.Error(fmt.Sprintf("Could not set %s applied: %v", item.name, err))
				}
			}
			pos++
			muPos.Unlock()
		}
	}(ruleChan)

	if _, err := a.fw.EnsureChain(iptTable, iptChain, strings.ToUpper(iptChainDefaultPolicy)); err != nil {
		return err
	}

	if len(a.config.AllowNetworkList) != 0 {
		name := strings.Join([]string{namePrefix, "allow", "networks"}, "-")
		if err := a.ipsetCreateOrUpdate(name, a.config.AllowNetworkList); err != nil {
			return err
		}
		rule := &rule{
			bucket: "",
			name:   name,
			rule:   []string{"-m", "set", "--match-set", name, "src", "-j", "ACCEPT"},
		}
		ruleChan <- *rule
	}

	countryApplier := func(name, ruleVerb string) error {
		if ok, err := a.storage.GetIpsetAppliedStatus(name); err != nil {
			// if err skip step
			return err
		} else {
			// if applied skip step
			if ok {
				return nil
			}
			// create set if not applied
			if !ok {
				n := strings.Join([]string{namePrefix, name}, "-")
				if err := a.ipsetCreateOrUpdate(n, a.config.AllowNetworkList); err != nil {
					return err
				}
				rule := &rule{
					bucket: name,
					name:   n,
					rule:   []string{"-m", "set", "--match-set", n, "src", "-j", strings.ToUpper(ruleVerb)},
				}
				ruleChan <- *rule
			}
		}
		return nil
	}

	if len(a.config.CountryAllowList) != 0 {
		for _, i := range a.config.CountryAllowList {
			if err := countryApplier(i, "ACCEPT"); err != nil {
				log.Error(err)
				return err
			}
		}
	}

	if len(a.config.CountryDenyList) != 0 {
		for _, i := range a.config.CountryDenyList {
			if err := countryApplier(i, "DROP"); err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return nil
}
