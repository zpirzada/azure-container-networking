package iptm

// Iptable holds a table contents
type Iptable struct {
	Chains    map[string]IptableChain
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

func (t *Iptable) CompareNpmChains() {
	return
}
