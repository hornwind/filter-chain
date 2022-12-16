package iptables

import (
	"sync"
	"time"

	iptables "github.com/coreos/go-iptables/iptables"
	"github.com/hornwind/filter-chain/internal/models"
	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
)

type Iptables struct {
	mu       sync.RWMutex
	iptables iptables.IPTables
}

var _ models.Firewall = (*Iptables)(nil)

func NewIptables() (*Iptables, error) {
	i, err := iptables.New()
	log.Debug(i)
	ipt := &Iptables{
		mu:       sync.RWMutex{},
		iptables: *i,
	}
	return ipt, err
}

// EnsureChain checks if the specified chain exists and, if not, creates it.  If the chain existed, return true.
func (ipt *Iptables) EnsureChain(table, chain, policy string) (bool, error) {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()
	if ok, err := ipt.iptables.ChainExists(table, chain); err != nil {
		log.Error(err)
		return false, err
	} else {
		if ok {
			return true, nil
		}
	}
	if err := ipt.iptables.NewChain(table, chain); err != nil {
		log.Errorf("Create chain %s in table %s failed: %v", chain, table, err)
		return false, err
	}
	return true, nil
}

// DeleteRule checks if the specified rule is present and, if so, deletes it.
func (ipt *Iptables) DeleteChain(table, chain string) error {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()
	if ok, err := ipt.iptables.ChainExists(table, chain); err != nil {
		log.Error(err)
		return err
	} else {
		if ok {
			if err := ipt.iptables.ClearAndDeleteChain(table, chain); err != nil {
				log.Errorf("Delete chain %s in table %s failed: %v", chain, table, err)
				return err
			}
		}
	}
	return nil
}

// EnsureRule checks if the specified rule is present and, if not, creates it.  If the rule existed, return true.
func (ipt *Iptables) EnsureRule(pos int, table, chain string, rulespec ...string) (bool, error) {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()
	log.Debugf("Check iptables rule %v", rulespec)
	if ok, err := ipt.iptables.Exists(table, chain, rulespec...); err != nil {
		log.Error(err)
		return false, err
	} else {
		if ok {
			return true, nil
		}
	}
	if err := ipt.iptables.Insert(table, chain, pos, rulespec...); err != nil {
		log.Errorf("Insert rule into chain %s in table %s failed: %v", chain, table, err)
		return false, err
	}
	return true, nil
}

// DeleteRule checks if the specified rule is present and, if so, deletes it.
func (ipt *Iptables) DeleteRule(table, chain string, rulespec ...string) error {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()
	// not so fast. Wait for the kernel to release resources
	defer time.Sleep(1 * time.Second)
	if err := ipt.iptables.DeleteIfExists(table, chain, rulespec...); err != nil {
		log.Errorf("Delete rule in chain %s failed: %v", chain, err)
		return err
	}
	return nil
}

// CheckRule checks if the specified rule is present and return bool status and err.
func (ipt *Iptables) CheckRule(table, chain string, rulespec ...string) (bool, error) {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()

	status, err := ipt.iptables.Exists(table, chain, rulespec...)
	if err != nil {
		log.Errorf("Check rule in chain %s failed: %v", chain, err)
		return status, err
	}
	return status, nil
}
