// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sharmasushant/penguin/ipam"
	"github.com/sharmasushant/penguin/log"
	"github.com/sharmasushant/penguin/network"
)

// Plugins versions
const networkVersion string = "V0"
const ipamVersion string = "V0"

// Prints description and usage information.
func printHelp() {
	fmt.Println("Usage: penguin [net] [ipam]")
}

func main() {
	var netPlugin network.NetPlugin
	var ipamPlugin ipam.IpamPlugin
	var err error

	// Set defaults.
	logTarget := log.TargetStderr

	// Parse command line arguments.
	args := os.Args

	if len(args) == 1 {
		printHelp()
		return
	}

	for i, arg := range args {
		if i == 0 {
			continue
		}

		switch arg {
		case "net":
			netPlugin, err = network.NewPlugin(networkVersion)
			if err != nil {
				fmt.Printf("Failed to create network plugin %v\n", err)
				return
			}

		case "ipam":
			ipamPlugin, err = ipam.NewPlugin(ipamVersion)
			if err != nil {
				fmt.Printf("Failed to create IPAM plugin %v\n", err)
				return
			}

		case "--log-target=syslog":
			logTarget = log.TargetSyslog

		default:
			fmt.Printf("Unknown argument: %s\n", arg)
			printHelp()
			return
		}
	}

	// Create a channel to receive unhandled errors from the plugins.
	errorChan := make(chan error, 1)

	// Create logging provider.
	err = log.SetTarget(logTarget)
	if err != nil {
		fmt.Printf("Failed to configure logging: %v\n", err)
		return
	}

	// Start plugins.
	if netPlugin != nil {
		err = netPlugin.Start(errorChan)
		if err != nil {
			fmt.Printf("Failed to start network plugin %v\n", err)
			return
		}
	}

	if ipamPlugin != nil {
		err = ipamPlugin.Start(errorChan)
		if err != nil {
			fmt.Printf("Failed to start IPAM plugin %v\n", err)
			return
		}
	}

	// For now, driver can shutdown on two conditions
	//    a. If some unhandled exceptions happens in the driver
	//    b. If we receive explicit signal
	// To receive explicit os signals, create a channel that can receive os signals.
	osSignalChannel := make(chan os.Signal, 1)

	// Relay incoming signals to channel
	// If no signals are provided, all incoming signals will be relayed to channel.
	// Otherwise, just the provided signals will.
	signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Wait until receiving a signal.
	select {
	case sig := <-osSignalChannel:
		fmt.Println("\nCaught signal <" + sig.String() + "> shutting down..")
	case err := <-errorChan:
		if err != nil {
			fmt.Println("\nDriver received an unhandled error.. ", err)
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
