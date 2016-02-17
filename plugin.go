// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

    "github.com/sharmasushant/penguin/core"
    "github.com/sharmasushant/penguin/network"
	ipamNull "github.com/sharmasushant/penguin/ipam/null"
)

// Plugin name
const pluginName = "penguin"

// Plugins versions
const networkVersion string = "V0"
const ipamVersion string = "V0"

// Plugin object
type Plugin struct {
    version string
    listener *core.Listener
    netPlugin network.NetPlugin
    ipamPlugin ipamNull.IpamPlugin
}

func main() {
    var plugin Plugin
    var err error

    plugin.version = "1"

    fmt.Printf("Plugin %s starting...\n", pluginName)

    // Create the listener.
    plugin.listener, err = core.NewListener(pluginName)
    if err != nil {
        fmt.Printf("Failed to create listener %v", err)
		return
    }

    // Parse command line arguments.
    args := os.Args

    for i, arg := range args {
        if i == 0 {
            continue
        }

        switch arg {
        case "net":
            plugin.netPlugin, err = network.NewPlugin(networkVersion)
            if err != nil {
                fmt.Printf("Failed to create network plugin %v", err)
            }

            err = plugin.netPlugin.Start(plugin.listener)
            if err != nil {
                fmt.Printf("Failed to start network plugin %v", err)
            }

        case "ipam":
            plugin.ipamPlugin, err = ipamNull.NewPlugin(ipamVersion)
            if err != nil {
                fmt.Printf("Failed to create IPAM plugin %v", err)
            }

            err = plugin.ipamPlugin.Start(plugin.listener)
            if err != nil {
                fmt.Printf("Failed to start IPAM plugin %v", err)
                return
            }

        default:
            fmt.Printf("Unknown argument %s", arg)
        }
    }

    // Create a channel to receive unhandled errors from the listener.
    errorChan := make(chan error, 1)

    // Start listener.
    plugin.listener.Start()

    // For now, driver can shutdown on two conditions
    //    a. If some unhandled exceptions happens in the driver
    //    b. If we receive explicit signal
    // To receive explicit os signals, create a channel that can receive os signals.
    osSignalChannel := make(chan os.Signal, 1)

    // Relay incoming signals to channel
    // If no signals are provided, all incoming signals will be relayed to channel.
    // Otherwise, just the provided signals will.
    signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

    // select can be used to wait on multiple channels in parallel
    select {
    case sig := <- osSignalChannel:
        fmt.Println("\nCaught signal <" + sig.String() + "> shutting down..")
    case err := <- errorChan:
        if err != nil {
            fmt.Println("\nDriver received an unhandled error.. ", err)
            os.Exit(1)
        }
    }

    // Stop to cleanup listener state.
    plugin.listener.Stop()

    fmt.Println("Plugin exited.")
}
