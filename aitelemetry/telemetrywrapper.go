package aitelemetry

import (
	"runtime"
	"time"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/store"
	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
)

const (
	resourceGroupStr  = "ResourceGroup"
	vmSizeStr         = "VMSize"
	osVersionStr      = "OSVersion"
	locationStr       = "Region"
	appNameStr        = "AppName"
	subscriptionIDStr = "SubscriptionID"
	vmNameStr         = "VMName"
	defaultTimeout    = 10
)

var debugMode bool

func messageListener() appinsights.DiagnosticsMessageListener {
	if debugMode {
		return appinsights.NewDiagnosticsMessageListener(func(msg string) error {
			debuglog("[AppInsights] [%s] %s\n", time.Now().Format(time.UnixDate), msg)
			return nil
		})
	}

	return nil
}

func debuglog(format string, args ...interface{}) {
	if debugMode {
		log.Printf(format, args...)
	}
}

func getMetadata(th *telemetryHandle) {
	var metadata common.Metadata
	var err error

	if th.refreshTimeout < 4 {
		th.refreshTimeout = defaultTimeout
	}

	// check if metadata in memory otherwise initiate wireserver request
	for {
		metadata, err = common.GetHostMetadata(metadataFile)
		if err == nil || th.disableMetadataRefreshThread {
			break
		}

		debuglog("[AppInsights] Error getting metadata %v. Sleep for %d", err, th.refreshTimeout)
		time.Sleep(time.Duration(th.refreshTimeout) * time.Second)
	}

	if err != nil {
		debuglog("[AppInsights] Error getting metadata %v", err)
		return
	}

	//acquire write lock before writing metadata to telemetry handle
	th.rwmutex.Lock()
	th.metadata = metadata
	th.rwmutex.Unlock()

	// Save metadata retrieved from wireserver to a file
	kvs, err := store.NewJsonFileStore(metadataFile)
	if err != nil {
		debuglog("[AppInsights] Error initializing kvs store: %v", err)
		return
	}

	kvs.Lock(true)
	err = common.SaveHostMetadata(th.metadata, metadataFile)
	kvs.Unlock(true)
	if err != nil {
		debuglog("[AppInsights] saving host metadata failed with :%v", err)
	}
}

// NewAITelemetry creates telemetry handle with user specified appinsights id.
func NewAITelemetry(
	id string,
	aiConfig AIConfig,
) TelemetryHandle {

	telemetryConfig := appinsights.NewTelemetryConfiguration(id)
	telemetryConfig.MaxBatchSize = aiConfig.BatchSize
	telemetryConfig.MaxBatchInterval = time.Duration(aiConfig.BatchInterval) * time.Second
	debugMode = aiConfig.DebugMode

	th := &telemetryHandle{
		client:                       appinsights.NewTelemetryClientFromConfig(telemetryConfig),
		appName:                      aiConfig.AppName,
		appVersion:                   aiConfig.AppVersion,
		diagListener:                 messageListener(),
		disableMetadataRefreshThread: aiConfig.DisableMetadataRefreshThread,
		refreshTimeout:               aiConfig.RefreshTimeout,
	}

	if th.disableMetadataRefreshThread {
		getMetadata(th)
	} else {
		go getMetadata(th)
	}

	return th
}

// TrackLog function sends report (trace) to appinsights resource. It overrides few of the existing columns with app information
// and for rest it uses custom dimesion
func (th *telemetryHandle) TrackLog(report Report) {
	// Initialize new trace message
	trace := appinsights.NewTraceTelemetry(report.Message, appinsights.Warning)

	//Override few of existing columns with metadata
	trace.Tags.User().SetAuthUserId(runtime.GOOS)
	trace.Tags.Operation().SetId(report.Context)
	trace.Tags.Operation().SetParentId(th.appVersion)

	// copy app specified custom dimension
	for key, value := range report.CustomDimensions {
		trace.Properties[key] = value
	}

	trace.Properties[appNameStr] = th.appName

	// Acquire read lock to read metadata
	th.rwmutex.RLock()
	metadata := th.metadata
	th.rwmutex.RUnlock()

	// Check if metadata is populated
	if metadata.SubscriptionID != "" {
		// copy metadata from wireserver to trace
		trace.Tags.User().SetAccountId(th.metadata.SubscriptionID)
		trace.Tags.User().SetId(th.metadata.VMName)
		trace.Properties[locationStr] = th.metadata.Location
		trace.Properties[resourceGroupStr] = th.metadata.ResourceGroupName
		trace.Properties[vmSizeStr] = th.metadata.VMSize
		trace.Properties[osVersionStr] = th.metadata.OSVersion
	}

	// send to appinsights resource
	th.client.Track(trace)
}

// TrackMetric function sends metric to appinsights resource. It overrides few of the existing columns with app information
// and for rest it uses custom dimesion
func (th *telemetryHandle) TrackMetric(metric Metric) {
	// Initialize new metric
	aimetric := appinsights.NewMetricTelemetry(metric.Name, metric.Value)

	// Acquire read lock to read metadata
	th.rwmutex.RLock()
	metadata := th.metadata
	th.rwmutex.RUnlock()

	// Check if metadata is populated
	if metadata.SubscriptionID != "" {
		aimetric.Properties[locationStr] = th.metadata.Location
		aimetric.Properties[subscriptionIDStr] = th.metadata.SubscriptionID
		aimetric.Properties[vmNameStr] = th.metadata.VMName
	}

	// copy custom dimensions
	for key, value := range metric.CustomDimensions {
		aimetric.Properties[key] = value
	}

	// send metric to appinsights
	th.client.Track(aimetric)
}

// Close - should be called for each NewAITelemetry call. Will release resources acquired
func (th *telemetryHandle) Close(timeout int) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// wait for items to be sent otherwise timeout
	<-th.client.Channel().Close(time.Duration(timeout) * time.Second)

	// Remove diganostic message listener
	if th.diagListener != nil {
		th.diagListener.Remove()
		th.diagListener = nil
	}
}
