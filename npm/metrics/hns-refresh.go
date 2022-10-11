package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// bin endpoints by 25 (max cardinaltiy of 10)
	endpointsBinSeparator  = 25
	endpointsMaxUpperLimit = 250
	failSafeBin            = "unknown"
)

type binThreshold struct {
	name       string
	lowerLimit int
	upperLimit int
}

// thresholds are like "0-25" or "125-150"
var thresholds []*binThreshold

// RecordHNSRefreshExecTime adds an observation of HNS refresh exec time for the specified number of endpoints.
// The execution time is from the timer's start until now.
func RecordHNSRefreshExecTime(timer *Timer, numEndpoints int) {
	timer.stop()
	labels := getHNSRefreshLabels(numEndpoints)
	hnsRefreshExecTime.With(labels).Observe(timer.timeElapsed())
}

// RecordHNSRefreshExecCount returns the number of observations for HNS refresh exec time for the specified number of endpoints.
// This function is slow.
func RecordHNSRefreshExecCount(numEndpoints int) (int, error) {
	return getCountVecValue(controllerPodExecTime, getHNSRefreshLabels(numEndpoints))
}

func getHNSRefreshLabels(numEndpoints int) prometheus.Labels {
	label := failSafeBin
	for _, threshold := range getThresholds() {
		if threshold.lowerLimit <= numEndpoints && numEndpoints <= threshold.upperLimit {
			label = threshold.name
			break
		}
	}

	return prometheus.Labels{
		hnsRefreshNumEndpointsLabel: label,
	}
}

func getThresholds() []*binThreshold {
	if thresholds != nil {
		return thresholds
	}

	k := 0
	for k <= endpointsMaxUpperLimit {
		upperLimit := endpointsMaxUpperLimit
		if k+endpointsBinSeparator <= upperLimit {
			upperLimit = k + endpointsBinSeparator
		}

		threshold := &binThreshold{
			name:       fmt.Sprintf("%d-%d", k, k+25),
			lowerLimit: k,
			upperLimit: upperLimit,
		}
		thresholds = append(thresholds, threshold)
		k += endpointsBinSeparator
	}
	return thresholds
}
