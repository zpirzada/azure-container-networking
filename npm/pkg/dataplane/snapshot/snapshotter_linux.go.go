package snapshot

import (
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/npm/util"
	"k8s.io/klog"
)

const defaultlockWaitTimeInSeconds = "60"

func getACLRules(ioShim *common.IOShim) string {
	iptablesListCommand := ioShim.Exec.Command(util.Iptables,
		util.IptablesWaitFlag, defaultlockWaitTimeInSeconds, util.IptablesTableFlag, util.IptablesFilterTable,
		util.IptablesNumericFlag, util.IptablesListFlag,
	)
	output, err := iptablesListCommand.CombinedOutput()
	if err != nil {
		klog.Errorf("failed to get iptables rules for snapshot: %v", err)
		return ""
	}
	return string(output)
}

func getIPSets(ioShim *common.IOShim) string {
	output, err := ioShim.Exec.Command(util.Ipset, util.IPsetCheckListFlag).CombinedOutput()
	if err != nil {
		klog.Errorf("failed to get ipsets for snapshot: %v", err)
		return ""
	}
	return string(output)
}
