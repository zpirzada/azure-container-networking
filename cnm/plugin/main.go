// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/azure-container-networking/cnm/ipam"
	"github.com/Azure/azure-container-networking/cnm/network"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/store"
)

const (
	// Plugin name.
	name = "azure"
)

// Version is populated by make during build.
var version string

// Command line arguments for CNM plugin.
var args = common.ArgumentList{
	{
		Name:         common.OptEnvironment,
		Shorthand:    common.OptEnvironmentAlias,
		Description:  "Set the operating environment",
		Type:         "string",
		DefaultValue: common.OptEnvironmentAzure,
		ValueMap: map[string]interface{}{
			common.OptEnvironmentAzure: 0,
			common.OptEnvironmentMAS:   0,
		},
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
		Name:         common.OptIpamQueryInterval,
		Shorthand:    common.OptIpamQueryIntervalAlias,
		Description:  "Set the IPAM plugin query interval",
		Type:         "int",
		DefaultValue: "",
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
	fmt.Printf("Azure CNM (libnetwork) plugin\n")
	fmt.Printf("Version %v\n", version)
}

// Main is the entry point for CNM plugin.
func main() {
	// Initialize and parse command line arguments.
	common.ParseArgs(&args, printVersion)

	environment := common.GetArg(common.OptEnvironment).(string)
	logLevel := common.GetArg(common.OptLogLevel).(int)
	logTarget := common.GetArg(common.OptLogTarget).(int)
	ipamQueryInterval, _ := common.GetArg(common.OptIpamQueryInterval).(int)
	vers := common.GetArg(common.OptVersion).(bool)

	if vers {
		printVersion()
		os.Exit(0)
	}

	// Initialize plugin common configuration.
	var config common.PluginConfig
	config.Name = name
	config.Version = version

	// Create network plugin.
	netPlugin, err := network.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create network plugin, err:%v.\n", err)
		return
	}

	// Create IPAM plugin.
	ipamPlugin, err := ipam.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin, err:%v.\n", err)
		return
	}

	// Create a channel to receive unhandled errors from the plugins.
	config.ErrChan = make(chan error, 1)

	// Create the key value store.
	config.Store, err = store.NewJsonFileStore("")
	if err != nil {
		fmt.Printf("Failed to create store: %v\n", err)
		return
	}

	// Create logging provider.
	log.SetLevel(logLevel)
	err = log.SetTarget(logTarget)
	if err != nil {
		fmt.Printf("Failed to configure logging: %v\n", err)
		return
	}

	// Log platform information.
	common.LogPlatformInfo()
	common.LogNetworkInterfaces()

	// Set plugin options.
	ipamPlugin.SetOption(common.OptEnvironment, environment)
	ipamPlugin.SetOption(common.OptIpamQueryInterval, ipamQueryInterval)

	// Start plugins.
	if netPlugin != nil {
		err = netPlugin.Start(&config)
		if err != nil {
			fmt.Printf("Failed to start network plugin, err:%v.\n", err)
			return
		}
	}

	if ipamPlugin != nil {
		err = ipamPlugin.Start(&config)
		if err != nil {
			fmt.Printf("Failed to start IPAM plugin, err:%v.\n", err)
			return
		}
	}

	// Shutdown on two conditions:
	//    a. Unhandled exceptions in plugins
	//    b. Explicit OS signal
	osSignalChannel := make(chan os.Signal, 1)

	// Relay these incoming signals to OS signal channel.
	signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Wait until receiving a signal.
	select {
	case sig := <-osSignalChannel:
		fmt.Printf("\nCaught signal <" + sig.String() + "> shutting down...\n")
	case err := <-config.ErrChan:
		if err != nil {
			fmt.Printf("\nReceived unhandled error %v, shutting down...\n", err)
		}
	}

	// Cleanup.
	if netPlugin != nil {
		netPlugin.Stop()
	}

	if ipamPlugin != nil {
		ipamPlugin.Stop()
	}
}
