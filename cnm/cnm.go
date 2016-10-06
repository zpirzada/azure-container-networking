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

const (
	// Plugin name.
	name = "azure"

	// Plugin version.
	version = "0.4"
)

// Prints description and usage information.
func printHelp() {
	fmt.Printf("Azure container networking plugin\n")
	fmt.Printf("Version %v\n\n", version)
	fmt.Printf("Usage: aqua [OPTIONS]\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  -e, --environment={azure|mas}         Set the operating environment.\n")
	fmt.Printf("  -l, --log-level={info|debug}          Set the logging level.\n")
	fmt.Printf("  -t, --log-target={syslog|stderr}      Set the logging target.\n")
	fmt.Printf("  -?, --help                            Print usage and version information.\n\n")
}

func main() {
	var netPlugin network.NetPlugin
	var ipamPlugin ipam.IpamPlugin
	var config common.PluginConfig
	var err error

	// Set defaults.
	environment := common.OptEnvironmentAzure
	logLevel := log.LevelInfo
	logTarget := log.TargetStderr

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

	// Parse command line arguments.
	args := os.Args[1:]

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// Process commands.
			switch arg {

			default:
				fmt.Printf("Invalid command: %s\n", arg)
				printHelp()
				return
			}
		} else {
			// Process options of format "--key=value".
			arg = strings.TrimLeft(arg, "-")
			opt := strings.SplitN(arg, "=", 2)
			if len(opt) == 1 {
				opt = append(opt, "")
			}

			switch opt[0] {
			case common.OptEnvironmentKey, common.OptEnvironmentKeyShort:
				environment = opt[1]

			case common.OptLogLevelKey, common.OptLogLevelKeyShort:
				switch opt[1] {
				case common.OptLogLevelInfo:
					logLevel = log.LevelInfo
				case common.OptLogLevelDebug:
					logLevel = log.LevelDebug
				default:
					fmt.Printf("Invalid option: %v\nSee --help.\n", arg)
					return
				}

			case common.OptLogTargetKey, common.OptLogTargetKeyShort:
				switch opt[1] {
				case common.OptLogTargetStderr:
					logTarget = log.TargetStderr
				case common.OptLogTargetSyslog:
					logTarget = log.TargetSyslog
				default:
					fmt.Printf("Invalid option: %v\nSee --help.\n", arg)
					return
				}

			case common.OptHelpKey, common.OptHelpKeyShort:
				printHelp()
				return

			default:
				fmt.Printf("Invalid option: %v\nSee --help.\n", arg)
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
