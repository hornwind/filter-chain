package ipset

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
	utilexec "k8s.io/utils/exec"
)

var _ = Interface(&runner{})

type Interface interface {
	// FlushSet deletes all entries from a named set.
	FlushSet(set string) error
	// DestroySet deletes a named set.
	DestroySet(set string) error
	// CreateSet creates a new set.  It will ignore error when the set already exists if ignoreExistErr=true.
	CreateSet(set *IPSet, ignoreExistErr bool) error
	// SwapSets swaps two sets
	SwapSets(tmpSet string, set string) error
	// AddEntry adds a new entry to the named set.  It will ignore error when the entry already exists if ignoreExistErr=true.
	AddEntry(entry string, set *IPSet, ignoreExistErr bool) error
	// RestoreSet creates a new set with list of entries.  It will ignore error when the entry already exists if ignoreExistErr=true.
	RestoreSet(entry []string, set *IPSet, ignoreExistErr bool) error
	// DelEntry deletes one entry from the named set
	DelEntry(entry string, set string) error
	// Test test if an entry exists in the named set
	TestEntry(entry string, set string) (bool, error)
	// ListEntries lists all the entries from a named set
	ListEntries(set string) ([]string, error)
	// ListSets list all set names from kernel
	ListSets() ([]string, error)
}

// IPSet implements an Interface to a set.
type IPSet struct {
	// Name is the set name.
	Name string
	// SetType specifies the ipset type.
	SetType Type
	// HashFamily specifies the protocol family of the IP addresses to be stored in the set.
	// The default is inet, i.e IPv4.  If users want to use IPv6, they should specify inet6.
	HashFamily string
	// HashSize specifies the hash table size of ipset.
	HashSize int
	// MaxElem specifies the max element number of ipset.
	MaxElem int
	// comment message for ipset
	Comment string
}

// IPSetCmd represents the ipset util. We use ipset command for ipset execute.
const IPSetCmd = "ipset"

var EntryMemberPattern = "(?m)^(.*\n)*Members:\n"

type runner struct {
	exec utilexec.Interface
}

// setIPSetDefaults sets some IPSet fields if not present to their default values.
func (set *IPSet) setIPSetDefaults() {
	// Setting default values if not present
	if set.HashSize == 0 {
		set.HashSize = 1024
	}
	if set.MaxElem == 0 {
		set.MaxElem = 65536
	}
	// Default protocol is IPv4
	if set.HashFamily == "" {
		set.HashFamily = ProtocolFamilyIPV4
	}
	// Default ipset type is "hash:net"
	if len(set.SetType) == 0 {
		set.SetType = HashNet
	}
}

// New returns a new Interface which will exec ipset.
func New(exec utilexec.Interface) Interface {
	return &runner{
		exec: exec,
	}
}

// ListSets list all set names from kernel
func (runner *runner) ListSets() ([]string, error) {
	out, err := runner.exec.Command(IPSetCmd, "list", "-n").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing all sets, error: %v", err)
	}
	return strings.Split(string(out), "\n"), nil
}

// DestroySet is used to destroy a named set.
func (runner *runner) DestroySet(set string) error {
	if out, err := runner.exec.Command(IPSetCmd, "destroy", set).CombinedOutput(); err != nil {
		return fmt.Errorf("error destroying set %s, error: %v(%s)", set, err, out)
	}
	return nil
}

// CreateSet creates a new set, it will ignore error when the set already exists if ignoreExistErr=true.
func (runner *runner) CreateSet(set *IPSet, ignoreExistErr bool) error {
	// sets some IPSet fields if not present to their default values.
	set.setIPSetDefaults()

	// Validate ipset before creating
	if err := set.Validate(); err != nil {
		return err
	}
	return runner.createSet(set, ignoreExistErr)
}

// If ignoreExistErr is set to true, then the -exist option of ipset will be specified, ipset ignores the error
// otherwise raised when the same set (setname and create parameters are identical) already exists.
func (runner *runner) createSet(set *IPSet, ignoreExistErr bool) error {
	args := []string{"create", set.Name, string(set.SetType)}
	if set.SetType == HashIP || set.SetType == HashNet {
		args = append(args,
			"family", set.HashFamily,
			"hashsize", strconv.Itoa(set.HashSize),
			"maxelem", strconv.Itoa(set.MaxElem),
		)
	}
	if ignoreExistErr {
		args = append(args, "-exist")
	}
	if _, err := runner.exec.Command(IPSetCmd, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("error creating ipset %s, error: %v", set.Name, err)
	}
	return nil
}

// Validate checks if a given ipset is valid or not.
func (set *IPSet) Validate() error {
	// Check if protocol is valid for `HashIPPort`, `HashIPPortIP` and `HashIPPortNet` type set.
	if set.SetType == HashIPPort || set.SetType == HashIPPortIP || set.SetType == HashIPPortNet {
		if err := validateHashFamily(set.HashFamily); err != nil {
			return err
		}
	}
	// check set type
	if err := validateIPSetType(set.SetType); err != nil {
		return err
	}
	// check hash size value of ipset
	if set.HashSize <= 0 {
		return fmt.Errorf("invalid HashSize: %d", set.HashSize)
	}
	// check max elem value of ipset
	if set.MaxElem <= 0 {
		return fmt.Errorf("invalid MaxElem %d", set.MaxElem)
	}

	return nil
}

// checks if given hash family is supported in ipset
func validateHashFamily(family string) error {
	if family == ProtocolFamilyIPV4 || family == ProtocolFamilyIPV6 {
		return nil
	}
	return fmt.Errorf("unsupported HashFamily %q", family)
}

// checks if the given ipset type is valid.
func validateIPSetType(set Type) error {
	for _, valid := range ValidIPSetTypes {
		if set == valid {
			return nil
		}
	}
	return fmt.Errorf("unsupported SetType: %q", set)
}

// AddEntry adds a new entry to the named set.
// If the -exist option is specified, ipset ignores the error otherwise raised when
// the same set (setname and create parameters are identical) already exists.
func (runner *runner) AddEntry(entry string, set *IPSet, ignoreExistErr bool) error {
	args := []string{"add", set.Name, entry}
	if ignoreExistErr {
		args = append(args, "-exist")
	}
	if out, err := runner.exec.Command(IPSetCmd, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("error adding entry %s, error: %v (%s)", entry, err, out)
	}
	return nil
}

// RestoreSet build and create new ipset with ipset resore command
// If the -exist option is specified, ipset ignores the error otherwise raised when
// the same set (setname and create parameters are identical) already exists.
func (runner *runner) RestoreSet(entries []string, set *IPSet, ignoreExistErr bool) error {
	ipset := new(strings.Builder)
	ipset.WriteString(fmt.Sprintln(
		"create", set.Name, string(set.SetType),
		"family", set.HashFamily,
		"hashsize", strconv.Itoa(set.HashSize),
		"maxelem", strconv.Itoa(set.MaxElem),
	))
	for _, cidr := range entries {
		if cidr == "0.0.0.0/0" {
			log.Info("We will replace 0.0.0.0/0 as it's an ipset limitation")
			ipset.WriteString(fmt.Sprintln("add", set.Name, "0.0.0.0/1"))
			ipset.WriteString(fmt.Sprintln("add", set.Name, "128.0.0.0/1"))
			continue
		}
		ipset.WriteString(fmt.Sprintln("add", set.Name, cidr))
	}

	args := []string{"-exist", "restore"}
	cmd := runner.exec.Command(IPSetCmd, args...)
	cmd.SetStdin(strings.NewReader(ipset.String()))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error restore %s error: %v (%s)", set.Name, err, out)
	}
	return nil
}

func (runner *runner) SwapSets(tmpSet string, set string) error {
	if _, err := runner.exec.Command(IPSetCmd, "swap", tmpSet, set).CombinedOutput(); err != nil {
		return fmt.Errorf("error swap %s and %s set, error: %v", set, tmpSet, err)
	}
	return nil
}

// DelEntry is used to delete the specified entry from the set.
func (runner *runner) DelEntry(entry string, set string) error {
	if out, err := runner.exec.Command(IPSetCmd, "del", set, entry).CombinedOutput(); err != nil {
		return fmt.Errorf("error deleting entry %s: from set: %s, error: %v (%s)", entry, set, err, out)
	}
	return nil
}

// TestEntry is used to check whether the specified entry is in the set or not.
func (runner *runner) TestEntry(entry string, set string) (bool, error) {
	if out, err := runner.exec.Command(IPSetCmd, "test", set, entry).CombinedOutput(); err == nil {
		reg, e := regexp.Compile("is NOT in set " + set)
		if e == nil && reg.MatchString(string(out)) {
			return false, nil
		} else if e == nil {
			return true, nil
		} else {
			return false, fmt.Errorf("error testing entry: %s, error: %v", entry, e)
		}
	} else {
		return false, fmt.Errorf("error testing entry %s: %v (%s)", entry, err, out)
	}
}

// ListEntries lists all the entries from a named set.
func (runner *runner) ListEntries(set string) ([]string, error) {
	if len(set) == 0 {
		return nil, fmt.Errorf("set name can't be nil")
	}
	out, err := runner.exec.Command(IPSetCmd, "list", set).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing set: %s, error: %v", set, err)
	}
	memberMatcher := regexp.MustCompile(EntryMemberPattern)
	list := memberMatcher.ReplaceAllString(string(out[:]), "")
	strs := strings.Split(list, "\n")
	results := make([]string, 0)
	for i := range strs {
		if len(strs[i]) > 0 {
			results = append(results, strs[i])
		}
	}
	return results, nil
}

// FlushSet deletes all entries from a named set.
func (runner *runner) FlushSet(set string) error {
	if _, err := runner.exec.Command(IPSetCmd, "flush", set).CombinedOutput(); err != nil {
		return fmt.Errorf("error flushing set: %s, error: %v", set, err)
	}
	return nil
}
