// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/ipam"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/store"
)

const (
	// Plugin name.
	name = "azure"

	// Command line options.
	OptEnvironment = "environment"
	OptLogLevel    = "log-level"
	OptLogTarget   = "log-target"
	OptVersion     = "version"
)

// Version is populated by make during build.
var version string

// Command line arguments for CNM plugin.
var args = common.ArgumentList{
	{
		Name:         OptEnvironment,
		Shorthand:    "e",
		Description:  "Set the operating environment",
		Type:         "string",
		DefaultValue: "azure",
		ValueMap: map[string]interface{}{
			"azure": 0,
			"mas":   0,
		},
	},
	{
		Name:         OptLogLevel,
		Shorthand:    "l",
		Description:  "Set the logging level",
		Type:         "int",
		DefaultValue: "info",
		ValueMap: map[string]interface{}{
			"info":  log.LevelInfo,
			"debug": log.LevelDebug,
		},
	},
	{
		Name:         OptLogTarget,
		Shorthand:    "t",
		Description:  "Set the logging target",
		Type:         "int",
		DefaultValue: "logfile",
		ValueMap: map[string]interface{}{
			"syslog":  log.TargetSyslog,
			"stderr":  log.TargetStderr,
			"logfile": log.TargetLogfile,
		},
	},
	{
		Name:         OptVersion,
		Shorthand:    "v",
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
	var netPlugin network.NetPlugin
	var ipamPlugin ipam.IpamPlugin
	var config common.PluginConfig
	var err error

	// Initialize and parse command line arguments.
	common.ParseArgs(&args, printVersion)

	environment := common.GetArg(OptEnvironment).(string)
	logLevel := common.GetArg(OptLogLevel).(int)
	logTarget := common.GetArg(OptLogTarget).(int)
	vers := common.GetArg(OptVersion).(bool)

	if vers {
		printVersion()
		os.Exit(0)
	}

	// Initialize plugin common configuration.
	config.Name = name
	config.Version = version

	// Create network plugin.
	netPlugin, err = network.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create network plugin %v\n", err)
		return
	}

	// Create IPAM plugin.
	ipamPlugin, err = ipam.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin %v\n", err)
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
	ipamPlugin.SetOption(common.OptEnvironmentKey, environment)

	// Start plugins.
	if netPlugin != nil {
		err = netPlugin.Start(&config)
		if err != nil {
			fmt.Printf("Failed to start network plugin %v\n", err)
			return
		}
	}

	if ipamPlugin != nil {
		err = ipamPlugin.Start(&config)
		if err != nil {
			fmt.Printf("Failed to start IPAM plugin %v\n", err)
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
