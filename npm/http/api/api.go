package api

const (
	DefaultListeningIP         = "0.0.0.0"
	DefaultHttpPort            = "10091"
	NodeMetricsPath            = "/node-metrics"
	ClusterMetricsPath         = "/cluster-metrics"
	DataplaneHealthMetricsPath = "/dataplane-health-metrics"
	NPMMgrPath                 = "/npm/v1/debug/manager"
)

type DescribeIPSetRequest struct{}

type DescribeIPSetResponse struct{}
