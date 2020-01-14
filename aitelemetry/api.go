package aitelemetry

import (
	"sync"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
)

// Application trace/log structure
type Report struct {
	Message          string
	Context          string
	CustomDimensions map[string]string
}

// Application metrics structure
type Metric struct {
	Name             string
	Value            float64
	CustomDimensions map[string]string
}

type AIConfig struct {
	AppName                      string
	AppVersion                   string
	BatchSize                    int
	BatchInterval                int
	DisableMetadataRefreshThread bool
	RefreshTimeout               int
	GetEnvRetryCount             int
	GetEnvRetryWaitTimeInSecs    int
	DebugMode                    bool
}

// TelmetryHandle holds appinsight handles and metadata
type telemetryHandle struct {
	telemetryConfig              *appinsights.TelemetryConfiguration
	appName                      string
	appVersion                   string
	metadata                     common.Metadata
	diagListener                 appinsights.DiagnosticsMessageListener
	client                       appinsights.TelemetryClient
	disableMetadataRefreshThread bool
	refreshTimeout               int
	rwmutex                      sync.RWMutex
}

// Telemetry Interface to send metrics/Logs to appinsights
type TelemetryHandle interface {
	// TrackLog function sends report (trace) to appinsights resource. It overrides few of the existing columns with app information
	// and for rest it uses custom dimesion
	TrackLog(report Report)
	// TrackMetric function sends metric to appinsights resource. It overrides few of the existing columns with app information
	// and for rest it uses custom dimesion
	TrackMetric(metric Metric)
	// Close - should be called for each NewAITelemetry call. Will release resources acquired
	Close(timeout int)
}
