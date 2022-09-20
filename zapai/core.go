package zapai

import (
	"github.com/Azure/azure-container-networking/aitelemetry"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/processlock"
	"github.com/Azure/azure-container-networking/store"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	resourceGroupStr            = "ResourceGroup"
	vmSizeStr                   = "VMSize"
	osVersionStr                = "OSVersion"
	osStr                       = "OS"
	locationStr                 = "Region"
	appNameStr                  = "AppName"
	subscriptionIDStr           = "SubscriptionID"
	vmNameStr                   = "VMName"
	vmIDStr                     = "VMID"
	versionStr                  = "AppVersion"
	hostNameKey                 = "hostname"
	defaultRefreshTimeoutInSecs = 10
)

var levelToSev = map[zapcore.Level]contracts.SeverityLevel{
	zapcore.DebugLevel:  contracts.Verbose,
	zapcore.InfoLevel:   contracts.Information,
	zapcore.WarnLevel:   contracts.Warning,
	zapcore.ErrorLevel:  contracts.Error,
	zapcore.DPanicLevel: contracts.Critical,
	zapcore.PanicLevel:  contracts.Critical,
	zapcore.FatalLevel:  contracts.Critical,
}

var _ zapcore.Core = (*Core)(nil)
var debugMode bool

// Core implements zapcore.Core for appinsights.
// Core only knows how to write to, and should only be used with, a corresponding appinsights Sink.
//
// Internally, the Core implements Write(zapcore.Entry, []zapcore.Field) by building an appinsights.TraceTelemetry
// out of the zap inputs, then serializing it to a []byte using a shared common gob.Encoder, and passes that to
// the Sink.Write([]byte) (which knows how to decode it back in to a TraceTelemetry).
type Core struct {
	zapcore.LevelEnabler
	enc            traceEncoder
	fieldMappers   map[string]fieldTagMapper
	fields         []zapcore.Field
	out            zapcore.WriteSyncer
	appName        string
	appVersion     string
	metadata       *common.Metadata
	refreshTimeout int
	writeLock      *sync.Mutex
	metaRWMutex    sync.RWMutex
}

// NewCore creates a new appinsights zap core. Should only be initialized using an appinsights Sink as the
// zapcore.WriteSyncer argument - other sinks will not error but will also not produce meaningful output.
func NewCore(le zapcore.LevelEnabler, out zapcore.WriteSyncer) *Core {
	return &Core{
		LevelEnabler: le,
		enc:          newTraceEncoder(),
		fieldMappers: make(map[string]fieldTagMapper),
		out:          out,
		writeLock:    &sync.Mutex{},
		metadata:     &common.Metadata{},
	}
}

func NewCoreWithVMMeta(le zapcore.LevelEnabler, out zapcore.WriteSyncer, config aitelemetry.AIConfig) *Core {
	setAIConfigDefaults(&config)
	core := &Core{
		LevelEnabler:   le,
		enc:            newTraceEncoder(),
		fieldMappers:   make(map[string]fieldTagMapper),
		out:            out,
		appName:        config.AppName,
		appVersion:     config.AppVersion,
		refreshTimeout: config.RefreshTimeout,
		metadata:       &common.Metadata{},
		writeLock:      &sync.Mutex{},
	}

	if config.DisableMetadataRefreshThread {
		getMetadata(core)
	} else {
		go getMetadata(core)
	}
	return core
}

func (c *Core) WithFieldMappers(fieldMappers ...map[string]string) *Core {
	clone := c.clone()
	for _, fieldMapper := range fieldMappers {
		for field, tag := range fieldMapper {
			tag := tag // anchor tag var in the closure
			clone.fieldMappers[field] = func(t *appinsights.TraceTelemetry, val string) {
				// set the tag to the value of the field
				t.Tags[tag] = val
			}
		}
	}
	return clone
}

func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	clone.fields = append(clone.fields, fields...)
	return clone
}

// Check implements zapcore.Core
//
//nolint:gocritic // ignore hugeparam in interface impl
func (c *Core) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

// Write implements zapcore.Core
//
//nolint:gocritic // ignore hugeparam in interface impl
func (c *Core) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	t := appinsights.NewTraceTelemetry(entry.Message, levelToSev[entry.Level])

	// add fields from core
	fields = append(c.fields, fields...)

	// reset the traceTelemetry in encoder
	c.enc.setTraceTelemetry(t)

	t.Properties[appNameStr] = c.appName
	t.Properties[versionStr] = c.appVersion
	t.Properties[osStr] = runtime.GOOS
	t.Properties[hostNameKey], _ = os.Hostname()
	// set caller
	if entry.Caller.Defined {
		t.Properties["caller"] = entry.Caller.String()
	}

	// Acquire read lock to read metadata
	c.metaRWMutex.RLock()
	metadata := c.metadata
	c.metaRWMutex.RUnlock()

	// Check if metadata is populated
	if metadata.SubscriptionID != "" {
		t.Tags.User().SetAccountId(metadata.SubscriptionID)
		t.Tags.User().SetId(metadata.VMName)
		t.Properties[locationStr] = metadata.Location
		t.Properties[resourceGroupStr] = metadata.ResourceGroupName
		t.Properties[vmIDStr] = metadata.VMID
		t.Properties[vmNameStr] = metadata.VMName
		t.Properties[vmSizeStr] = metadata.VMSize
		t.Properties[osVersionStr] = metadata.OSVersion
		t.Properties[subscriptionIDStr] = subscriptionIDStr
		t.Tags.Session().SetId(metadata.VMID)
	}

	// set fields
	for i := range fields {
		// handle zap object first
		if fields[i].Type == zapcore.ObjectMarshalerType {
			fields[i].AddTo(c.enc)
		} else if mapper, ok := c.fieldMappers[fields[i].Key]; ok {
			// check mapped fields
			mapper(t, fieldStringer(&fields[i]))
		} else {
			t.Properties[fields[i].Key] = fieldStringer(&fields[i])
		}
	}
	b, err := c.enc.encode(t)
	if err != nil {
		return errors.Wrap(err, "core failed to encode trace")
	}
	if _, err = c.out.Write(b); err != nil {
		return errors.Wrap(err, "core failed to write to sink")
	}
	return nil
}

func (c *Core) Sync() error {
	return errors.Wrap(c.out.Sync(), "core failed to sync")
}

// clone derives a new Core from this Core, copying the references to the encoder and sink
// and duplicating the contents of the fields and fieldMappers so that children and parent
// cores may mutate their fields and mappers independently after cloning.
func (c *Core) clone() *Core {
	fieldMappers := make(map[string]fieldTagMapper, len(c.fieldMappers))
	for k, v := range c.fieldMappers {
		fieldMappers[k] = v
	}
	fields := make([]zapcore.Field, len(c.fields))
	copy(fields, c.fields)
	return &Core{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc,
		fieldMappers: fieldMappers,
		fields:       fields,
		out:          c.out,
		writeLock:    c.writeLock,
		appName:      c.appName,
		appVersion:   c.appVersion,
		metadata:     c.metadata,
	}
}

func getMetadata(c *Core) {
	var metadata common.Metadata
	var err error

	for {
		metadata, err = common.GetHostMetadata(metadataFile)
		if err == nil {
			break
		}

		debugLog("[AppInsights] Error getting metadata %v. Sleep for %d", err, 0)
		time.Sleep(time.Duration(c.refreshTimeout) * time.Second)
	}

	if err != nil {
		debugLog("[AppInsights] Error getting metadata %v", err)
		return
	}

	// acquire write lock before writing metadata to core
	c.metaRWMutex.Lock()
	copyMetadata(c.metadata, metadata)
	c.metaRWMutex.Unlock()

	lockClient, err := processlock.NewFileLock(metadataFile + store.LockExtension)

	if err != nil {
		log.Printf("Error initializing file lock:%v", err)
		return
	}

	// Save metadata retrieved from wireserver to a file
	kvs, err := store.NewJsonFileStore(metadataFile, lockClient)
	if err != nil {
		debugLog("[AppInsights] Error initializing kvs store: %v", err)
		return
	}

	kvs.Lock(store.DefaultLockTimeout)
	if err != nil {
		log.Errorf("getMetadata: Not able to acquire lock:%v", err)
		return
	}

	metadataErr := common.SaveHostMetadata(*c.metadata, metadataFile)
	err = kvs.Unlock()

	if err != nil {
		log.Errorf("getMetadata: Not able to release lock:%v", err)
	}

	if metadataErr != nil {
		debugLog("[AppInsights] saving host metadata failed with :%v", err)
	}

}

func setAIConfigDefaults(config *aitelemetry.AIConfig) {
	if config.RefreshTimeout == 0 {
		config.RefreshTimeout = defaultRefreshTimeoutInSecs
	}

}

func copyMetadata(dest *common.Metadata, src common.Metadata) {
	dest.Location = src.Location
	dest.VMName = src.VMName
	dest.VMID = src.VMID
	dest.OSVersion = src.OSVersion
	dest.ResourceGroupName = src.ResourceGroupName
	dest.SubscriptionID = src.SubscriptionID
	dest.VMSize = src.VMSize
}

func debugLog(format string, args ...interface{}) {
	if debugMode {
		log.Printf(format, args...)
	}
}
