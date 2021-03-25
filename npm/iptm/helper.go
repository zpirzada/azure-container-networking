package iptm

import (
	"fmt"

	"github.com/Azure/azure-container-networking/npm/util"
)

var (
	IptablesAzureChainMap = map[string]func() [][]string{
		util.IptablesAzureChain:             getAzureNPMChainRules,
		util.IptablesAzureAcceptChain:       getAzureNPMAcceptChainRules,
		util.IptablesAzureIngressChain:      getAzureNPMIngressChainRules,
		util.IptablesAzureIngressPortChain:  getAzureNPMIngressPortChainRules,
		util.IptablesAzureIngressFromChain:  getAzureNPMIngressFromChainRules,
		util.IptablesAzureEgressChain:       getAzureNPMEgressChainRules,
		util.IptablesAzureEgressPortChain:   getAzureNPMEgressPortChainRules,
		util.IptablesAzureEgressToChain:     getAzureNPMEgressToChainRules,
		util.IptablesAzureIngressDropsChain: func() [][]string { return nil },
		util.IptablesAzureEgressDropsChain:  func() [][]string { return nil },
	}
)

//getChainsRulesWithTable will return chains and rules for a given tablename
func getChainsRulesWithTable(tablename string) [][]string {

	switch tablename {
	case "filter":
		return getAllChainsAndRules()
	default:
	}

	return [][]string{}

}

//getChainsRulesWithTable will return chains and rules for a given tablename
func getChainsWithTable(tablename string) []string {

	switch tablename {
	case "filter":
		return getAllNpmChains()
	default:
	}

	return []string{}

}

func getAllNpmChains() []string {
	listOfChains := []string{}
	for chain, _ := range IptablesAzureChainMap {
		listOfChains = append(listOfChains, chain)
	}
	return listOfChains
}

// GetAllChainsAndRules returns all NPM chains and rules
func getAllChainsAndRules() [][]string {
	chainsAndRules := [][]string{}
	for _, fn := range IptablesAzureChainMap {
		tempRules := fn()
		chainsAndRules = append(chainsAndRules, tempRules...)
	}

	return chainsAndRules
}

func getChainsFromRule(rule []string) string {
	return rule[0]
}

func getFilterDefaultChainObjects() map[string]*IptableChain {
	var (
		chains = make(map[string]*IptableChain)
	)
	allRules := getAllChainsAndRules()

	for _, rule := range allRules {
		chain := getChainsFromRule(rule)
		val, ok := chains[chain]
		if !ok {
			val = NewIptableChain(chain)
		}

		val.Rules = append(val.Rules, convertRuleListToByte(rule))
		chains[chain] = val

	}

	return chains
}

func convertRuleListToByte(rule []string) []byte {
	return []byte(convertRuleListToString(rule))
}

func convertRuleListToString(rule []string) string {
	returnString := "-A"
	for _, word := range rule {
		returnString = fmt.Sprintf("%s %s", returnString, word)
	}

	return returnString
}

func convertRuleStringToByte(rule string) []byte {
	return []byte(rule)
}

// getAzureNPMChainRules returns all rules for AZURE-NPM chain
func getAzureNPMChainRules() [][]string {
	// Note: make sure 0th index is prent chain for logging
	return [][]string{
		{
			util.IptablesAzureChain,
			util.IptablesJumpFlag,
			util.IptablesAzureIngressChain,
		},
		{
			util.IptablesAzureChain,
			util.IptablesJumpFlag,
			util.IptablesAzureEgressChain,
		},
		{
			util.IptablesAzureChain,
			util.IptablesJumpFlag,
			util.IptablesAzureAcceptChain,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureAcceptMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("ACCEPT-on-INGRESS-and-EGRESS-mark-%s", util.IptablesAzureAcceptMarkHex),
		},
		{
			util.IptablesAzureChain,
			util.IptablesJumpFlag,
			util.IptablesAzureAcceptChain,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureIngressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("ACCEPT-on-INGRESS-mark-%s", util.IptablesAzureIngressMarkHex),
		},
		{
			util.IptablesAzureChain,
			util.IptablesJumpFlag,
			util.IptablesAzureAcceptChain,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureEgressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("ACCEPT-on-EGRESS-mark-%s", util.IptablesAzureEgressMarkHex),
		},
		{
			util.IptablesAzureChain,
			util.IptablesModuleFlag,
			util.IptablesStateModuleFlag,
			util.IptablesStateFlag,
			util.IptablesRelatedState + "," + util.IptablesEstablishedState,
			util.IptablesJumpFlag,
			util.IptablesAccept,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("ACCEPT-on-connection-state"),
		},
	}
}

// getAzureNPMAcceptChainRules clears all marks and accepts packets
func getAzureNPMAcceptChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureAcceptChain,
			util.IptablesJumpFlag,
			util.IptablesMark,
			util.IptablesSetMarkFlag,
			util.IptablesAzureClearMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("Clear-AZURE-NPM-MARKS"),
		},
		{
			util.IptablesAzureAcceptChain,
			util.IptablesJumpFlag,
			util.IptablesAccept,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("ACCEPT-All-packets"),
		},
	}
}

// getAzureNPMIngressChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMIngressChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureIngressChain,
			util.IptablesJumpFlag,
			util.IptablesAzureIngressPortChain,
		},
		{
			util.IptablesAzureIngressChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureIngressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-INGRESS-mark-%s", util.IptablesAzureIngressMarkHex),
		},
		{
			util.IptablesAzureIngressChain,
			util.IptablesJumpFlag,
			util.IptablesAzureIngressDropsChain,
		},
	}
}

// getAzureNPMIngressPortChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMIngressPortChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureIngressPortChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureIngressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-INGRESS-mark-%s", util.IptablesAzureIngressMarkHex),
		},
	}
}

// getAzureNPMIngressFromChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMIngressFromChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureIngressFromChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureIngressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-INGRESS-mark-%s", util.IptablesAzureIngressMarkHex),
		},
	}
}

// getAzureNPMEgressChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMEgressChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureEgressChain,
			util.IptablesJumpFlag,
			util.IptablesAzureEgressPortChain,
		},
		{
			util.IptablesAzureEgressChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureAcceptMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-and-INGRESS-mark-%s", util.IptablesAzureAcceptMarkHex),
		},
		{
			util.IptablesAzureEgressChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureEgressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-mark-%s", util.IptablesAzureEgressMarkHex),
		},
		{
			util.IptablesAzureEgressChain,
			util.IptablesJumpFlag,
			util.IptablesAzureEgressDropsChain,
		},
	}
}

// getAzureNPMEgressPortChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMEgressPortChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureEgressPortChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureAcceptMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-and-INGRESS-mark-%s", util.IptablesAzureAcceptMarkHex),
		},
		{
			util.IptablesAzureEgressPortChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureEgressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-mark-%s", util.IptablesAzureEgressMarkHex),
		},
	}
}

// getAzureNPMEgressToChainRules returns rules for AZURE-NPM-INGRESS-PORT
func getAzureNPMEgressToChainRules() [][]string {
	return [][]string{
		{
			util.IptablesAzureEgressToChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureAcceptMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-and-INGRESS-mark-%s", util.IptablesAzureAcceptMarkHex),
		},
		{
			util.IptablesAzureEgressToChain,
			util.IptablesJumpFlag,
			util.IptablesReturn,
			util.IptablesModuleFlag,
			util.IptablesMarkVerb,
			util.IptablesMarkFlag,
			util.IptablesAzureEgressMarkHex,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			fmt.Sprintf("RETURN-on-EGRESS-mark-%s", util.IptablesAzureEgressMarkHex),
		},
	}
}
