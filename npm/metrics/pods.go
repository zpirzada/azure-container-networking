package metrics

import "github.com/prometheus/client_golang/prometheus"

// RecordControllerPodExecTime adds an observation of pod exec time for the specified operation (unless the operation is NoOp).
// The execution time is from the timer's start until now.
func RecordControllerPodExecTime(timer *Timer, op OperationKind, hadError bool) {
	timer.stopAndRecordCRUDExecTime(controllerPodExecTime, op, hadError)
}

// GetControllerPodExecCount returns the number of observations for pod exec time for the specified operation.
// This function is slow.
func GetControllerPodExecCount(op OperationKind, hadError bool) (int, error) {
	return getCountVecValue(controllerPodExecTime, getCRUDExecTimeLabels(op, hadError))
}

func IncPodEventCount(op OperationKind) {
	podEventCount.With(getPodEventCountLabels(op)).Inc()
}

func getPodEventCount(op OperationKind) (int, error) {
	return getCounterVecValue(podEventCount, getPodEventCountLabels(op))
}

func getPodEventCountLabels(op OperationKind) prometheus.Labels {
	return prometheus.Labels{operationLabel: string(op)}
}
