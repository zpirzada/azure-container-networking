package metrics

import "github.com/prometheus/client_golang/prometheus"

// RecordHNSRefreshExecTime adds an observation of HNS refresh exec time for the specified refresh method and number of endpoints.
// The execution time is from the timer's start until now.
func RecordHNSRefreshExecTime(timer *Timer, method string, numEndpoints int) {
	timer.stop()
	labels := getHNSRefreshLabels(method, numEndpoints)
	hnsRefreshExecTime.With(labels).Observe(timer.timeElapsed())
}

// RecordHNSRefreshExecCount returns the number of observations for HNS refresh exec time for the specified refresh method and number of endpoints.
// This function is slow.
func RecordHNSRefreshExecCount(method string, numEndpoints int) (int, error) {
	return getCountVecValue(controllerPodExecTime, getHNSRefreshLabels(method, numEndpoints))
}

func getHNSRefreshLabels(method string, numEndpoints int) prometheus.Labels {
	return prometheus.Labels{
		hnsRefreshMethodLabel:       method,
		hnsRefreshNumEndpointsLabel: string(numEndpoints),
	}
}
