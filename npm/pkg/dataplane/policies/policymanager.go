package policies

import (
	"github.com/Azure/azure-container-networking/common"
	"k8s.io/klog"
)

// PolicyManagerMode will be used in windows to decide if
// SetPolicies should be used or not
type PolicyManagerMode string

const (
	// IPSetPolicyMode will references IPSets in policies
	IPSetPolicyMode PolicyManagerMode = "IPSet"
	// IPPolicyMode will replace ipset names with their value IPs in policies
	IPPolicyMode PolicyManagerMode = "IP"
)

type PolicyMap struct {
	cache map[string]*NPMNetworkPolicy
}

type PolicyManager struct {
	policyMap *PolicyMap
	ioShim    *common.IOShim
	*PolicyManagerCfg
}

func NewPolicyManager(ioShim *common.IOShim) *PolicyManager {
	return &PolicyManager{
		policyMap: &PolicyMap{
			cache: make(map[string]*NPMNetworkPolicy),
		},
		ioShim: ioShim,
	}
}

type PolicyManagerCfg struct {
	Mode PolicyManagerMode
}

func (pMgr *PolicyManager) Reset() error {
	return pMgr.reset()
}

func (pMgr *PolicyManager) PolicyExists(name string) bool {
	_, ok := pMgr.policyMap.cache[name]
	return ok
}

func (pMgr *PolicyManager) GetPolicy(name string) (*NPMNetworkPolicy, bool) {
	policy, ok := pMgr.policyMap.cache[name]
	return policy, ok
}

func (pMgr *PolicyManager) AddPolicy(policy *NPMNetworkPolicy, endpointList map[string]string) error {
	if len(policy.ACLs) == 0 {
		klog.Infof("[DataPlane] No ACLs in policy %s to apply", policy.Name)
		return nil
	}
	// Call actual dataplane function to apply changes
	err := pMgr.addPolicy(policy, endpointList)
	if err != nil {
		return err
	}

	pMgr.policyMap.cache[policy.Name] = policy
	return nil
}

func (pMgr *PolicyManager) RemovePolicy(name string, endpointList map[string]string) error {
	policy, ok := pMgr.GetPolicy(name)
	if !ok {
		return nil
	}

	if len(policy.ACLs) == 0 {
		klog.Infof("[DataPlane] No ACLs in policy %s to remove", policy.Name)
		return nil
	}
	// Call actual dataplane function to apply changes
	err := pMgr.removePolicy(policy, endpointList)
	if err != nil {
		return err
	}

	delete(pMgr.policyMap.cache, name)

	return nil
}
