package iptm

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/Azure/azure-container-networking/npm/util"
)

var (
	chainSaveFormat = ":%s - [0:0]"
)

// Iptable holds a table contents
type Iptable struct {
	Chains    map[string]*IptableChain
	TableName string
}

// IptableChain holds information about the chain
type IptableChain struct {
	Chain string
	// chain save format in bytes
	// :AZURE-NPM - [0:0] -> in bytes
	Data []byte
	//Rules []IptableRule
	Rules [][]byte
}

// IptableRule holds a ipt rule
type IptableRule struct {
	MatchSets []RuleMatchType
	Target    TargetType
	Protocol  string
	SourceIP  string
	DestIP    string
}

// RuleMatchType match set rules
type RuleMatchType struct {
	Type      string
	Operator  string
	Value     string
	Direction string
	Options   []string
}

// TargetType target type for rules
type TargetType struct {
	Target string
	Value  string
}

var (
	DefaultChains = map[string][]string{
		"filter": []string{"INPUT", "FORWARD", "OUTPUT"},
	}
)

func NewIptable(tableName string, buffer *bytes.Buffer) *Iptable {
	return &Iptable{
		Chains:    GetChainLines(tableName, buffer.Bytes()),
		TableName: tableName,
	}
}

func (t *Iptable) Validate() error {
	for _, chain := range DefaultChains[t.TableName] {
		if _, ok := t.Chains[chain]; !ok {
			return fmt.Errorf("error: %s table does not contain %s of default chains %+v", t.TableName, chain, DefaultChains[t.TableName])
		}
	}
	return nil
}

func (t *Iptable) CheckNpmBaseChains() error {
	if t.TableName != "filter" {
		return fmt.Errorf("error: only filter table should contains NPM chains not %s table", t.TableName)
	}

	iptablesAzureChainList := getAllNpmChains()
	for _, chain := range iptablesAzureChainList {
		if _, ok := t.Chains[chain]; !ok {
			return fmt.Errorf("error: %s table does not contain default chains %+v", t.TableName, DefaultChains[t.TableName])
		}
	}

	return nil
}

func (t *Iptable) getMissingChains() []string {
	var (
		missingChainsList = []string{}
		allNpmChains      = getChainsWithTable(t.TableName)
	)

	for _, chainName := range allNpmChains {
		if _, ok := t.Chains[chainName]; !ok {
			missingChainsList = append(missingChainsList, chainName)
		}
	}

	return missingChainsList
}

func (t *Iptable) InitializeChains() error {
	missingChains := t.getMissingChains()
	if len(missingChains) == 0 {
		missingChains = getChainsWithTable(t.TableName)
	}

	npmDefaultFilterTable := getFilterDefaultChainObjects()

	for _, chainName := range missingChains {
		t.Chains[chainName] = npmDefaultFilterTable[chainName]
	}

	return nil
}

// BulkUpdateIPtables will handle updating of iptables
// the IPtEntries are expected to start with -A or -I
func (t *Iptable) BulkUpdateIPtables(toAddEntries []*IptEntry, toDeleteEntries []*IptEntry) error {
	// TODO add metric timers
	for _, addIptEntry := range toAddEntries {
		err := t.Chains[addIptEntry.Chain].addIptableEntry(addIptEntry)
		if err != nil {
			metrics.SendErrorLogAndMetric(util.IptmID, "Error: In chain %s failed to add iptable rule %+v",
				addIptEntry.Chain,
				addIptEntry,
			)
			continue
		}
		metrics.NumIPTableRules.Inc()
	}

	for _, delIptEntry := range toDeleteEntries {
		err := t.Chains[delIptEntry.Chain].deleteIptableEntry(delIptEntry)
		if err != nil {
			metrics.SendErrorLogAndMetric(util.IptmID, "Error: In chain %s failed to delete iptable rule %+v",
				delIptEntry.Chain,
				delIptEntry,
			)
			continue
		}
		metrics.NumIPTableRules.Dec()
	}
	return nil
}

func NewIptableChain(chain string) *IptableChain {
	data := MakeChainLine(chain)
	return &IptableChain{
		Chain: chain,
		Data:  data,
		Rules: [][]byte{},
	}
}

// appendRule will take in a string rule and adds
// at the bottom of rules. This helps in adding jump targets
// at the bottom of the chain
func (c *IptableChain) appendRule(rule []string) {
	c.Rules = append(c.Rules, getByteSliceFromRule(rule))
}

// insertRule will take in a string rule and adds it
// at the top of the rules.
func (c *IptableChain) insertRule(rule []string) {
	tempSlice := [][]byte{getByteSliceFromRule(rule)}
	c.Rules = append(tempSlice, c.Rules...)
}

// deleteRule will take in a string rule and will
// compare existing rules and deletes if found
func (c *IptableChain) deleteRule(rule []string) error {
	byteRule := getByteSliceFromRule(rule)

	for i, chainRule := range c.Rules {
		if reflect.DeepEqual(byteRule, chainRule) {
			c.Rules = append(c.Rules[:i], c.Rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("error: In Chains %s could not find rule %+v", c.Chain, rule)
}

// addIptableEntry will accept IptEntry from existing IPTM
// and will add the entry at approppriate place
func (c *IptableChain) addIptableEntry(entry *IptEntry) error {
	log.Logf("Adding iptables entry: %+v.", entry)
	rule := entry.getAppendRule()

	if entry.IsJumpEntry {
		c.appendRule(rule)
	} else {
		c.insertRule(rule)
	}

	return nil
}

// deleteIptableEntry will accept IptEntry from existing IPTM
// and will delete the entry
func (c *IptableChain) deleteIptableEntry(entry *IptEntry) error {
	log.Logf("Deleting iptables entry: %+v.", entry)
	rule := entry.getAppendRule()

	return c.deleteRule(rule)
}
