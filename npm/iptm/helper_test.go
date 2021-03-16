package iptm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/azure-container-networking/npm/util"
)

func TestGetAllChainsAndRules(t *testing.T) {
	allChainsandRules := getAllChainsAndRules()

	parentNpmRulesCount := 6

	if len(allChainsandRules[0]) > 3 {
		t.Fatalf("TestGetAllChainsAndRules failed @ INGRESS target check")
	}

	if len(allChainsandRules[1]) > 3 {
		t.Fatalf("TestGetAllChainsAndRules failed @ EGRESS target check")
	}

	for i, rule := range allChainsandRules {
		if i == parentNpmRulesCount {
			break
		}
		// make sure the ordering is correct
		// first 7 rules should be parent chain rules
		if rule[0] != util.IptablesAzureChain {
			t.Fatalf("TestGetAllChainsAndRules failed @ AzureNpmChain rule count check")
		}
	}
}

func TestGetChainObjects(t *testing.T) {
	iptableobj := getChainObjects()

	if len(iptableobj.Chains) < 6 {
		t.Errorf("TestGetChainObjects failed @ INGRESS target check")
	}

	allChainsandRules := getAllChainsAndRules()

	rulesByteArray := []string{}

	for _, chain := range getAllNpmChains() {
		if strings.Contains(chain, "DROP") {
			continue
		}
		val, ok := iptableobj.Chains[chain]
		if !ok {
			t.Fatalf("TestGetChainObjects failed @ Objects array does not contain %s chain data", chain)
		}

		for _, rule := range val.Rules {
			rulesByteArray = append(rulesByteArray, string(rule))
		}
	}

	if len(rulesByteArray) != len(allChainsandRules) {
		t.Fatalf("TestGetChainObjects failed @ failed at length check for the byte and string chains")
	}

	expectedOrderArray := []string{}
	for _, rule := range allChainsandRules {
		expectedOrderArray = append(expectedOrderArray, convertRuleListToString(rule))
	}

	for i := 0; i < len(rulesByteArray); i++ {
		fmt.Println(rulesByteArray[i])

	}
	for i := 0; i < len(rulesByteArray); i++ {
		fmt.Println(expectedOrderArray[i])

	}

	// If any of the below two cases fail it is because of ordering difference between helper
	// functions and IptablesAzureChainList
	for i := 0; i < len(rulesByteArray); i++ {
		if rulesByteArray[i] != expectedOrderArray[i] {
			t.Errorf("Index: %d", i)
			t.Errorf("byteArray: %+v", rulesByteArray[i])
			t.Errorf("stringArray: %+v", expectedOrderArray[i])
			t.Fatalf("TestGetChainObjects failed @ failed at order equal check for the byte and string chains")

		}

	}

	if !reflect.DeepEqual(rulesByteArray, expectedOrderArray) {
		t.Errorf("byteArray: %+v", rulesByteArray)
		t.Errorf("stringArray: %+v", expectedOrderArray)
		t.Fatalf("TestGetChainObjects failed @ failed at order equal check for the byte and string chains")
	}

}
