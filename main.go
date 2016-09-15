// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/ipam"
	"github.com/Azure/Aqua/log"
	"github.com/Azure/Aqua/network"
	"github.com/Azure/Aqua/store"
)

// Binary version
const version = "v0.1"

// Libnetwork plugin names
const netPluginName = "aquanet"
const ipamPluginName = "aquaipam"

// Prints description and usage information.
func printHelp() {
	fmt.Println("Usage: aqua [net] [ipam]")
}

func main() {
	var netPlugin network.NetPlugin
	var ipamPlugin ipam.IpamPlugin
	var config common.PluginConfig
	var err error

	// Set defaults.
	logTarget := log.TargetStderr

	// Parse command line arguments.
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		return
	}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			// Process commands.
			switch arg {
			case "net":
				netPlugin, err = network.NewPlugin(netPluginName, version)
				if err != nil {
					fmt.Printf("Failed to create network plugin %v\n", err)
					return
				}

			case "ipam":
				ipamPlugin, err = ipam.NewPlugin(ipamPluginName, version)
				if err != nil {
					fmt.Printf("Failed to create IPAM plugin %v\n", err)
					return
				}

			default:
				fmt.Printf("Invalid command: %s\n", arg)
				printHelp()
				return
			}
		} else {
			// Process options of format "--obj-option=value".
			obj := strings.SplitN(arg[2:], "-", 2)
			opt := strings.SplitN(obj[1], "=", 2)

			switch obj[0] {
			case "ipam":
				ipamPlugin.SetOption(opt[0], opt[1])

			case "log":
				if opt[0] == "target" && opt[1] == "syslog" {
					logTarget = log.TargetSyslog
				}

			default:
				fmt.Printf("Invalid option: %v\n", arg)
				printHelp()
				return
			}
		}
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
	err = log.SetTarget(logTarget)
	if err != nil {
		fmt.Printf("Failed to configure logging: %v\n", err)
		return
	}

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
