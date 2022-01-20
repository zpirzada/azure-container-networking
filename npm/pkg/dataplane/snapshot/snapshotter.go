package snapshot

import (
	"strings"

	"github.com/Azure/azure-container-networking/common"
	"k8s.io/klog"
)

type Snapshotter interface {
	// Record records an error message and notifies the snapshotter
	// to log an error message containing a snapshot of the dataplane.
	Record(errorMessage string)
}

// ImmediateSnapshotter logs an error message with a snapshot right away.
type ImmediateSnapshotter struct {
	ioShim *common.IOShim
}

// PeriodicSnapshotter compiles error messages,
// then logs an error summary with a snapshot when told to do so.
type PeriodicSnapshotter struct {
	ioShim        *common.IOShim
	errorMessages []string
}

const snapshotFormat = `taking a snapshot due to errors: [%s]
BEGIN-SNAPSHOT-OF-CURRENT-DATAPLANE
BEGIN-CURRENT-ACL-RULES:
%s
END-CURRENT-ACL-RULES
BEGIN-CURRENT-IPSETS:
%s
END-CURRENT-IPSETS
END-SNAPSHOT-OF-CURRENT-DATAPLANE`

func NewImmediateSnapshotter(ioShim *common.IOShim) *ImmediateSnapshotter {
	return &ImmediateSnapshotter{
		ioShim: ioShim,
	}
}

func NewPeriodicSnapshotter(ioShim *common.IOShim) *PeriodicSnapshotter {
	return &PeriodicSnapshotter{
		ioShim:        ioShim,
		errorMessages: make([]string, 0),
	}
}

func (iSnap *ImmediateSnapshotter) Record(errorMessage string) {
	logSnapshot(iSnap.ioShim, []string{errorMessage})
}

func (pSnap *PeriodicSnapshotter) Record(errorMessage string) {
	pSnap.errorMessages = append(pSnap.errorMessages, errorMessage)
}

// CaptureIfNeeded logs an error summary and snapshot if Record() was called prior.
func (pSnap *PeriodicSnapshotter) CaptureIfNeeded() {
	if len(pSnap.errorMessages) > 0 {
		logSnapshot(pSnap.ioShim, pSnap.errorMessages)
		pSnap.errorMessages = make([]string, 0)
	}
}

func logSnapshot(ioShim *common.IOShim, errorMessages []string) {
	allErrorMessages := strings.Join(errorMessages, "; ")
	klog.Errorf(snapshotFormat, allErrorMessages, getACLRules(ioShim), getIPSets(ioShim))
}
