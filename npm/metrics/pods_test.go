package metrics

import (
	"testing"

	"github.com/Azure/azure-container-networking/npm/metrics/promutil"
	"github.com/stretchr/testify/require"
)

func TestRecordControllerPodExecTime(t *testing.T) {
	testStopAndRecordCRUDExecTime(t, &crudExecMetric{
		RecordControllerPodExecTime,
		GetControllerPodExecCount,
	})
}

func TestIncPodEventTotal(t *testing.T) {
	InitializeAll()
	for _, op := range []OperationKind{CreateOp, UpdateOp, DeleteOp, UpdateWithEmptyIPOp} {
		IncPodEventTotal(op)
		val, err := getPodEventTotal(op)
		promutil.NotifyIfErrors(t, err)
		require.Equal(t, 1, val, "expected metric count to be incremented for op: %s", op)
	}
}
