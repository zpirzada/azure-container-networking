package metrics

import (
	"net/http"

	"github.com/Azure/azure-container-networking/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"
)

const (
	namespace                = "npm"
	dataplaneHealthSubsystem = "dataplane_health"
)

// Prometheus Metrics
// Gauge metrics have the methods Inc(), Dec(), and Set(float64)
// Summary metrics have the method Observe(float64)
// For any Vector metric, you can call With(prometheus.Labels) before the above methods
//   e.g. SomeGaugeVec.With(prometheus.Labels{label1: val1, label2: val2, ...).Dec()

// Customer cluster and node metrics
var (
	numPolicies        prometheus.Gauge
	addPolicyExecTime  prometheus.Summary
	numACLRules        prometheus.Gauge
	addACLRuleExecTime prometheus.Summary
	numIPSets          prometheus.Gauge
	addIPSetExecTime   prometheus.Summary
	numIPSetEntries    prometheus.Gauge
	ipsetInventory     *prometheus.GaugeVec
)

// Constants for customer cluster and node metric names and descriptions as well as exported labels for Vector metrics
const (
	numPoliciesName = "num_policies"
	numPoliciesHelp = "The number of current network policies for this node"

	addPolicyExecTimeName = "add_policy_exec_time"
	addPolicyExecTimeHelp = "Execution time in milliseconds for adding a network policy"

	numACLRulesName = "num_iptables_rules"
	numACLRulesHelp = "The number of current IPTable rules for this node"

	addACLRuleExecTimeName = "add_iptables_rule_exec_time"
	addACLRuleExecTimeHelp = "Execution time in milliseconds for adding an IPTable rule to a chain"

	numIPSetsName = "num_ipsets"
	numIPSetsHelp = "The number of current IP sets for this node"

	addIPSetExecTimeName = "add_ipset_exec_time"
	addIPSetExecTimeHelp = "Execution time in milliseconds for creating an IP set"

	numIPSetEntriesName = "num_ipset_entries"
	numIPSetEntriesHelp = "The total number of entries in every IPSet"

	ipsetInventoryName = "ipset_counts"
	ipsetInventoryHelp = "The number of entries in each individual IPSet"
	setNameLabel       = "set_name"
	setHashLabel       = "set_hash"
)

// Dataplane health metrics
var (
	numApplyIPSetFailures         *prometheus.CounterVec
	numAddPolicyFailures          *prometheus.CounterVec
	numDeletePolicyFailures       *prometheus.CounterVec
	numPeriodicPolicyTaskFailures *prometheus.CounterVec
	numValidatePolicyFailures     *prometheus.CounterVec
	numValidateIPSetFailures      *prometheus.CounterVec
)

// Constants for DataplaneHealthMetrics names and descriptions as well as exported labels for Vector metrics
const (
	numApplyIPSetFailuresName = "num_apply_ipset_failures"
	numApplyIPSetFailuresHelp = "The number of times the dataplane failed to apply ipsets"

	numAddPolicyFailuresName = "num_add_policy_failures"
	numAddPolicyFailuresHelp = "The number of times the dataplane failed to add a network policy"

	numDeletePolicyFailuresName = "num_delete_policy_failures"
	numDeletePolicyFailuresHelp = "The number of times the dataplane failed to delete a network policy"

	numPeriodicPolicyTaskFailuresName = "num_periodic_policy_task_failures"
	numPeriodicPolicyTaskFailuresHelp = "The number of times the dataplane failed to run a periodic policy task"

	numValidatePolicyFailuresName = "num_validate_policy_failures"
	numValidatePolicyFailuresHelp = "The number of times the dataplane failed to validate a network policy"

	numValidateIPSetFailuresName = "num_validate_ipset_failures"
	numValidateIPSetFailuresHelp = "The number of times the dataplane failed to validate an ipset"

	errorTypeLabel = "error_type"
)

type RegistryType string

const (
	CustomerNodeMetrics    RegistryType = "customer-node"
	CustomerClusterMetrics RegistryType = "customer-cluster"
	DataplaneHealthMetrics RegistryType = "dataplane-health"
)

var (
	allRegistries   = []RegistryType{CustomerNodeMetrics, CustomerClusterMetrics, DataplaneHealthMetrics}
	registries      = make(map[RegistryType]*prometheus.Registry)
	haveInitialized = false
)

// InitializeAll creates all the Prometheus Metrics. The metrics will be nil before this method is called.
func InitializeAll() {
	if !haveInitialized {
		// TODO optimize to only create the registries/metrics that are needed?
		createRegistries()

		// customer node and cluster metrics
		numPolicies = createGauge(numPoliciesName, numPoliciesHelp, CustomerClusterMetrics)
		addPolicyExecTime = createSummary(addPolicyExecTimeName, addPolicyExecTimeHelp, CustomerNodeMetrics)
		numACLRules = createGauge(numACLRulesName, numACLRulesHelp, CustomerClusterMetrics)
		addACLRuleExecTime = createSummary(addACLRuleExecTimeName, addACLRuleExecTimeHelp, CustomerNodeMetrics)
		numIPSets = createGauge(numIPSetsName, numIPSetsHelp, CustomerClusterMetrics)
		addIPSetExecTime = createSummary(addIPSetExecTimeName, addIPSetExecTimeHelp, CustomerNodeMetrics)
		numIPSetEntries = createGauge(numIPSetEntriesName, numIPSetEntriesHelp, CustomerClusterMetrics)
		ipsetInventory = createGaugeVec(ipsetInventoryName, ipsetInventoryHelp, CustomerClusterMetrics, setNameLabel, setHashLabel)

		// vanilla dataplane health metrics
		numApplyIPSetFailures = createDataplaneHealthCounterVec(numApplyIPSetFailuresName, numApplyIPSetFailuresHelp)
		numAddPolicyFailures = createDataplaneHealthCounterVec(numAddPolicyFailuresName, numAddPolicyFailuresHelp)
		numDeletePolicyFailures = createDataplaneHealthCounterVec(numDeletePolicyFailuresName, numDeletePolicyFailuresHelp)
		numPeriodicPolicyTaskFailures = createDataplaneHealthCounterVec(numPeriodicPolicyTaskFailuresName, numPeriodicPolicyTaskFailuresHelp)
		numValidatePolicyFailures = createDataplaneHealthCounterVec(numValidatePolicyFailuresName, numValidatePolicyFailuresHelp)
		numValidateIPSetFailures = createDataplaneHealthCounterVec(numValidateIPSetFailuresName, numValidateIPSetFailuresHelp)

		log.Logf("Finished initializing all Prometheus metrics")
		haveInitialized = true
	}
}

// GetHandler returns the HTTP handler for the metrics endpoint
func GetHandler(registryType RegistryType) http.Handler {
	if !haveInitialized {
		// not sure if this will ever happen, but just in case
		klog.Infof("in GetHandler, metrics weren't initialized. Initializing now")
		InitializeAll()
	}
	registry := registries[registryType]
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func createRegistries() {
	for _, registryType := range allRegistries {
		registries[registryType] = prometheus.NewRegistry()
	}
}

func register(collector prometheus.Collector, name string, registryType RegistryType) {
	registry := registries[registryType]
	err := registry.Register(collector)
	if err != nil {
		log.Errorf("Error creating metric %s", name)
	}
}

func createGauge(name string, helpMessage string, registryType RegistryType) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      name,
			Help:      helpMessage,
		},
	)
	register(gauge, name, registryType)
	return gauge
}

func createGaugeVec(name string, helpMessage string, registryType RegistryType, labels ...string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      name,
			Help:      helpMessage,
		},
		labels,
	)
	register(gaugeVec, name, registryType)
	return gaugeVec
}

func createSummary(name string, helpMessage string, registryType RegistryType) prometheus.Summary {
	summary := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       name,
			Help:       helpMessage,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			// quantiles e.g. the "0.5 quantile" will actually be the phi quantile for some phi in [0.5 - 0.05, 0.5 + 0.05]
		},
	)
	register(summary, name, registryType)
	return summary
}

func createDataplaneHealthCounterVec(name string, helpMessage string) *prometheus.CounterVec {
	counter := prometheus.NewCounterVec(
		// fully-qualified name will be namespace_subsystem_name
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: dataplaneHealthSubsystem,
			Name:      name,
			Help:      helpMessage,
		},
		[]string{errorTypeLabel},
	)
	register(counter, name, DataplaneHealthMetrics)
	return counter
}
