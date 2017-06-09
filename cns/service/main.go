// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	
	"github.com/Azure/azure-container-networking/cns/restserver"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
	"github.com/Azure/azure-container-networking/store"
)

const (
	// Service name.
	name = "azure-cns"
)

// Version is populated by make during build.
var version string

// Command line arguments for CNM plugin.
var args = common.ArgumentList{
	{
		Name:         common.OptAPIServerURL,
		Shorthand:    common.OptAPIServerURLAlias,
		Description:  "Set the API server URL",
		Type:         "string",
		DefaultValue: "",
	},
	{
		Name:         common.OptLogLevel,
		Shorthand:    common.OptLogLevelAlias,
		Description:  "Set the logging level",
		Type:         "int",
		DefaultValue: common.OptLogLevelInfo,
		ValueMap: map[string]interface{}{
			common.OptLogLevelInfo:  log.LevelInfo,
			common.OptLogLevelDebug: log.LevelDebug,
		},
	},
	{
		Name:         common.OptLogTarget,
		Shorthand:    common.OptLogTargetAlias,
		Description:  "Set the logging target",
		Type:         "int",
		DefaultValue: common.OptLogTargetFile,
		ValueMap: map[string]interface{}{
			common.OptLogTargetSyslog: log.TargetSyslog,
			common.OptLogTargetStderr: log.TargetStderr,
			common.OptLogTargetFile:   log.TargetLogfile,
		},
	},
	{
		Name:         common.OptVersion,
		Shorthand:    common.OptVersionAlias,
		Description:  "Print version information",
		Type:         "bool",
		DefaultValue: false,
	},
}

// Prints description and version information.
func printVersion() {
	fmt.Printf("Azure Container Network Service\n")
	fmt.Printf("Version %v\n", version)
}

// Main is the entry point for CNS.
func main() {
	// Initialize and parse command line arguments.
	common.ParseArgs(&args, printVersion)
	
	url := common.GetArg(common.OptAPIServerURL).(string)
	logLevel := common.GetArg(common.OptLogLevel).(int)
	logTarget := common.GetArg(common.OptLogTarget).(int)
	vers := common.GetArg(common.OptVersion).(bool)

	if vers {
		printVersion()
		os.Exit(0)
	}

	// Initialize CNS.
	var config common.ServiceConfig
	config.Version = version
	config.Name = name

	// Create a channel to receive unhandled errors from CNS.
	config.ErrChan = make(chan error, 1)	

	// Create the key value store.
	var err error
	config.Store, err = store.NewJsonFileStore(platform.RuntimePath + name + ".json")
	if err != nil {
		fmt.Printf("Failed to create store: %v\n", err)
		return
	}

	// Create CNS object.
	httpRestService, err := restserver.NewHTTPRestService(&config)
	if err != nil {
		fmt.Printf("Failed to create CNS object, err:%v.\n", err)
		return
	}

	// Create logging provider.
	log.SetName(name)
	log.SetLevel(logLevel)
	err = log.SetTarget(logTarget)
	if err != nil {
		fmt.Printf("Failed to configure logging: %v\n", err)
		return
	}

	// Log platform information.
	log.Printf("Running on %v", platform.GetOSInfo())

	// Set CNS options.
	httpRestService.SetOption(common.OptAPIServerURL, url)

	// Start CNS.
	if httpRestService != nil {
		err = httpRestService.Start(&config)
		if err != nil {
			fmt.Printf("Failed to start CNS, err:%v.\n", err)
			return
		}
	}

	// Relay these incoming signals to OS signal channel.
	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Wait until receiving a signal.
	select {
	case sig := <-osSignalChannel:
		log.Printf("CNS Received OS signal <" + sig.String() + ">, shutting down.")
	case err := <-config.ErrChan:
		log.Printf("CNS Received unhandled error %v, shutting down.", err)
	}

	// Cleanup.
	if httpRestService != nil {
		httpRestService.Stop()
	}
}
