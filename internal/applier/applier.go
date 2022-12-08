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

var (
	netAllowIpsetName = strings.Join([]string{namePrefix, "allow", "networks"}, "-")
	netAllowRule      = []string{"-m", "set", "--match-set", netAllowIpsetName, "src", "-j", "ACCEPT"}
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
	localCTX, cancel := context.WithCancel(ctx)
	a.fnCancelRunCTX = cancel
	go a.runApplier(localCTX)
	go a.runCleanup(localCTX)
}

func (a *Applier) runApplier(ctx context.Context) {
	ticker := time.NewTicker(reconciliationInterval)
	defer ticker.Stop()

	// time.Sleep(15 * time.Second)
	log.Debug("Applier started")

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Applier ctx: %v", ctx.Err())
			return
		case <-ticker.C:
			if err := a.refreshLiveSets(); err != nil {
				log.Warn(err)
			}
			if err := a.reconcile(); err != nil {
				log.Warn(err)
				a.fnCancelRunCTX()
			}
		}
	}
}

func (a *Applier) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(reconciliationInterval)
	defer ticker.Stop()

	// time.Sleep(15 * time.Second)
	log.Debug("Cleanup started")

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Cleanup ctx: %v", ctx.Err())
			return
		case <-ticker.C:
			if err := a.refreshLiveSets(); err != nil {
				log.Warn(err)
			}
			if err := a.markBucketsForDeletion(); err != nil {
				log.Warn(err)
			}
			if err := a.cleanupCountryResources(); err != nil {
				log.Warn(err)
				a.fnCancelRunCTX()
			}
			if err := a.cleanupNetworks(); err != nil {
				log.Warn(err)
				a.fnCancelRunCTX()
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
		a.mu.Lock()
		a.liveSets = make(map[string]interface{}, 1)
		a.mu.Unlock()
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
		a.mu.RLock()
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
		a.mu.RUnlock()
	}
	return nil
}

func (a *Applier) reconcile() error {
	pos := 1
	var muPos sync.RWMutex
	type rule struct {
		bucket string
		rule   []string
	}
	ruleChan := make(chan rule)
	defer close(ruleChan)

	go func(data <-chan rule) {
		log.Debug("Start iptables filling goroutine loop")
		for item := range data {
			muPos.Lock()
			if _, err := a.fw.EnsureRule(pos, iptTable, iptChain, item.rule...); err != nil {
				log.Errorf("Chain filling goroutime failed: %v", err)
				return
			}
			if item.bucket != "" {
				log.Debugf("Set bucket %s is applied", item.bucket)
				if err := a.storage.StoreRule(item.bucket, "rule", item.rule); err != nil {
					log.Errorf("Could not store rule %s in bucket %s", item.rule, item.bucket)
				}
				if err := a.storage.SetBoolKV(item.bucket, "applied", true); err != nil {
					log.Errorf("Could not set %s applied: %v", item.bucket, err)
				}
			}
			pos++
			muPos.Unlock()
		}
	}(ruleChan)

	if _, err := a.fw.EnsureChain(iptTable, iptChain, strings.ToUpper(iptChainDefaultPolicy)); err != nil {
		return err
	}

	if len(a.config.AllowNetworkList) > 0 {
		if err := a.ipsetCreateOrUpdate(netAllowIpsetName, a.config.AllowNetworkList); err != nil {
			return err
		}
		rule := &rule{
			bucket: "",
			rule:   netAllowRule,
		}
		ruleChan <- *rule
	}

	countryApplier := func(bucket, ruleVerb string) error {
		if ok, err := a.storage.GetBoolKV(bucket, "applied"); err != nil {
			// if err skip step
			return err
		} else {
			// if applied skip step
			if ok {
				return nil
			}
			// create set if not applied
			if !ok {
				ipsetName := strings.Join([]string{namePrefix, bucket}, "-")
				entries, err := a.storage.GetIpsetResources(bucket)
				if err != nil {
					log.Warnf("Could not get ipset resource from db: %v", err)
					return err
				}
				if err := a.ipsetCreateOrUpdate(ipsetName, entries.Ipv4); err != nil {
					log.Warnf("Could not create ipset: %v", err)
					return err
				}
				if err := a.storage.SetStringKV(bucket, "ipset", ipsetName); err != nil {
					log.Warnf("Could not store ipset name %s to bucket %s: %v", ipsetName, bucket, err)
					return err
				}
				rule := &rule{
					bucket: bucket,
					rule:   []string{"-m", "set", "--match-set", ipsetName, "src", "-j", strings.ToUpper(ruleVerb)},
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

func (a *Applier) makeCountriesMap() map[string]interface{} {
	c := make(map[string]interface{}, 1)
	if len(a.config.CountryAllowList) > 0 {
		for _, v := range a.config.CountryAllowList {
			c[strings.ToUpper(v)] = nil
		}
	}
	if len(a.config.CountryDenyList) > 0 {
		for _, v := range a.config.CountryDenyList {
			c[strings.ToUpper(v)] = nil
		}
	}
	return c
}

func (a *Applier) markBucketsForDeletion() error {
	log.Debug("Start marking buckets")
	countries := a.makeCountriesMap()
	log.Debugf("Mark func receive countries: %v", countries)
	buckets, err := a.storage.ListBuckets()
	log.Debugf("Mark func receive buckets: %v", buckets)
	if err != nil {
		return err
	}
	// edge cases
	if len(countries) == 0 {
		log.Warn("Unable to run cleanup because not found countries in config")
		return nil
	}
	if len(buckets) == 0 {
		log.Warn("Unable to run cleanup because not found buckets in db")
		return nil
	}

	for v := range buckets {
		if _, ok := countries[strings.ToUpper(v)]; !ok {
			log.Debugf("Bucket %s not in %v", strings.ToUpper(v), countries)
			if err := a.storage.SetBoolKV(v, "deletion_mark", true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Applier) cleanupCountryResources() error {
	log.Debug("Start cleanup")
	buckets, err := a.storage.ListBucketsForDeletion()
	if err != nil {
		return err
	}

	for _, bucket := range buckets {
		ipset, err := a.storage.GetStringKV(bucket, "ipset")
		if err != nil {
			return err
		}
		rule, err := a.storage.GetRule(bucket, "rule")
		if err != nil {
			return err
		}
		if ipset == "" || len(rule) == 0 {
			err := fmt.Errorf("cleanup failed. ipset: %v, rule: %v", ipset, rule)
			return err
		}
		a.mu.RLock()
		if _, ok := a.liveSets[ipset]; ok {
			log.Debugf("Delete rule %v", rule)
			if err := a.fw.DeleteRule(iptTable, iptChain, rule...); err != nil {
				a.mu.RUnlock()
				return err
			}
			log.Debugf("%s in %v", ipset, a.liveSets)
			log.Debugf("Flush set %s", ipset)
			if err := a.set.FlushSet(ipset); err != nil {
				a.mu.RUnlock()
				return err
			}
			log.Debugf("Destroy set %s", ipset)
			if err := a.set.DestroySet(ipset); err != nil {
				a.mu.RUnlock()
				return err
			}
			log.Debugf("Delete bucket %s", bucket)
			if err := a.storage.DeleteBucket(bucket); err != nil {
				a.mu.RUnlock()
				return err
			}
		}
		if _, ok := a.liveSets[ipset]; !ok {
			log.Debugf("Delete bucket %s", bucket)
			if err := a.storage.DeleteBucket(bucket); err != nil {
				a.mu.RUnlock()
				return err
			}
		}
		a.mu.RUnlock()
	}
	return nil
}

func (a *Applier) cleanupNetworks() error {
	a.mu.RLock()
	if _, ok := a.liveSets[netAllowIpsetName]; ok {
		log.Debugf("Delete rule %v", netAllowRule)
		if err := a.fw.DeleteRule(iptTable, iptChain, netAllowRule...); err != nil {
			a.mu.RUnlock()
			return err
		}
		log.Debugf("%s in %v", netAllowIpsetName, a.liveSets)
		log.Debugf("Flush set %s", netAllowIpsetName)
		if err := a.set.FlushSet(netAllowIpsetName); err != nil {
			a.mu.RUnlock()
			return err
		}
		log.Debugf("Destroy set %s", netAllowIpsetName)
		if err := a.set.DestroySet(netAllowIpsetName); err != nil {
			a.mu.RUnlock()
			return err
		}
	}
	a.mu.RUnlock()
	return nil
}
