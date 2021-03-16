package iptm

import (
	"bytes"
	"fmt"
)

// Iptable holds a table contents
type Iptable struct {
	Chains    map[string]*IptableChain
	TableName string
}

// IptableChain holds information about the chain
type IptableChain struct {
	Chain string
	Data  []byte
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

func (t *Iptable) CompareNpmChains() {
	return
}

func NewIptable(tableName string, buffer *bytes.Buffer) *Iptable {
	return &Iptable{
		Chains:    GetChainLines(tableName, buffer.Bytes()),
		TableName: tableName,
	}
}

func (t *Iptable) Validate() error {
	for _, chain := range DefaultChains[t.TableName] {
		if _, ok := t.Chains[chain]; !ok {
			return fmt.Errorf("Error: %s table does not contain %s of default chains %+v", t.TableName, chain, DefaultChains[t.TableName])
		}
	}
	return nil
}

func (t *Iptable) CheckNpmBaseChains() error {
	if t.TableName != "filter" {
		return fmt.Errorf("Error: only filter table should contains NPM chains not %s table", t.TableName)
	}

	iptablesAzureChainList := getAllNpmChains()
	for _, chain := range iptablesAzureChainList {
		if _, ok := t.Chains[chain]; !ok {
			return fmt.Errorf("Error: %s table does not contain default chains %+v", t.TableName, DefaultChains[t.TableName])
		}
	}

	return nil
}

func NewIptableChain(chain string) *IptableChain {
	return &IptableChain{
		Chain: chain,
		Data:  []byte{},
		Rules: [][]byte{},
	}
}

func (c *IptableChain) Append(rule []byte) error {
	c.Rules = append(c.Rules, rule)
	return nil
}

func (c *IptableChain) Insert(rule []byte) error {
	tempSlice := [][]byte{rule}
	c.Rules = append(tempSlice, c.Rules...)
	return nil
}
