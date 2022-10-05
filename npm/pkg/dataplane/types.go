package dataplane

import (
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/ipsets"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/policies"
	"github.com/Azure/azure-container-networking/npm/util"
	"k8s.io/klog"
)

const applyDataplaneMaxBatches = 999

type GenericDataplane interface {
	BootupDataplane() error
	RunPeriodicTasks()
	GetAllIPSets() map[string]string
	GetIPSet(setName string) *ipsets.IPSet
	CreateIPSets(setMetadatas []*ipsets.IPSetMetadata)
	DeleteIPSet(setMetadata *ipsets.IPSetMetadata, deleteOption util.DeleteOption)
	AddToSets(setMetadatas []*ipsets.IPSetMetadata, podMetadata *PodMetadata) error
	RemoveFromSets(setMetadatas []*ipsets.IPSetMetadata, podMetadata *PodMetadata) error
	AddToLists(listMetadatas []*ipsets.IPSetMetadata, setMetadatas []*ipsets.IPSetMetadata) error
	RemoveFromList(listMetadata *ipsets.IPSetMetadata, setMetadatas []*ipsets.IPSetMetadata) error
	ApplyDataPlane() error
	// LockDataPlane must be called before making calls to modify IPSets/Lists, including ApplyDataPlane.
	// For the Dataplane implementation, it should currently NOT be called for Add/Update/RemovePolicy.
	LockDataPlane()
	// UnlockDataPlane must be called when the consumer is done with the Lock.
	UnlockDataPlane()
	// GetAllPolicies is deprecated and only used in the goalstateprocessor, which is deprecated
	GetAllPolicies() []string
	AddPolicy(policies *policies.NPMNetworkPolicy) error
	RemovePolicy(PolicyKey string) error
	UpdatePolicy(policies *policies.NPMNetworkPolicy) error
}

// UpdateNPMPod pod controller will populate and send this datastructure to dataplane
// to update the dataplane with the latest pod information
// this helps in calculating if any update needs to have policies applied or removed
type updateNPMPod struct {
	*PodMetadata
	IPSetsToAdd    []string
	IPSetsToRemove []string
}

// PodMetadata is what is passed to dataplane to specify pod ipset
// todo definitely requires further optimization between the intersection
// of types, PodMetadata, NpmPod and corev1.pod
type PodMetadata struct {
	PodKey   string
	PodIP    string
	NodeName string
}

func NewPodMetadata(podKey, podIP, nodeName string) *PodMetadata {
	return &PodMetadata{
		PodKey:   podKey,
		PodIP:    podIP,
		NodeName: nodeName,
	}
}

func newUpdateNPMPod(podMetadata *PodMetadata) *updateNPMPod {
	return &updateNPMPod{
		PodMetadata:    podMetadata,
		IPSetsToAdd:    make([]string, 0),
		IPSetsToRemove: make([]string, 0),
	}
}

func (npmPod *updateNPMPod) updateIPSetsToAdd(setNames []*ipsets.IPSetMetadata) {
	for _, set := range setNames {
		npmPod.IPSetsToAdd = append(npmPod.IPSetsToAdd, set.GetPrefixName())
	}
}

func (npmPod *updateNPMPod) updateIPSetsToRemove(setNames []*ipsets.IPSetMetadata) {
	for _, set := range setNames {
		npmPod.IPSetsToRemove = append(npmPod.IPSetsToRemove, set.GetPrefixName())
	}
}

type batchHelper struct {
	numBatches        int
	inBackground      bool
	justAppliedSignal chan struct{}
}

func newBatchHelper(inBackground bool) *batchHelper {
	var signal chan struct{}
	if inBackground {
		// FIXME not sure if we need a large buffer to prevent deadlock
		// buffer of 1 works for local UTs
		signal = make(chan struct{}, 123456)
	}
	return &batchHelper{
		numBatches:        0,
		inBackground:      inBackground,
		justAppliedSignal: signal,
	}
}

func (bh *batchHelper) incrementBatches() {
	bh.numBatches += 1
	klog.Infof("[DataPlane] incremented number of batches for ApplyDataPlane to %d", bh.numBatches)
}

func (bh *batchHelper) resetBatches() {
	bh.numBatches = 0
	klog.Info("[DataPlane] reset number of batches for ApplyDataPlane")
}

func (bh *batchHelper) maxBatches() int {
	if bh.inBackground {
		return applyDataplaneMaxBatches
	}
	return 1
}

func (bh *batchHelper) shouldApply() bool {
	return bh.numBatches >= bh.maxBatches()
}

func (bh *batchHelper) markAsApplied() {
	if bh.inBackground {
		klog.Infof("[DataPlane] marking batchHelper as applied")
		bh.justAppliedSignal <- struct{}{}
	}
}
