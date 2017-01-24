// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/ipam"
	"github.com/Azure/azure-container-networking/cni/network"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
	"github.com/Azure/azure-container-networking/store"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniVers "github.com/containernetworking/cni/pkg/version"
)

const (
	// Plugin name.
	name = "azure-vnet"
)

// Version is populated by make during build.
var version string

// Main is the entry point for CNI plugin.
func main() {
	// Initialize plugin common configuration.
	var config common.PluginConfig
	config.Name = name
	config.Version = version

	// Create network plugin.
	netPlugin, err := network.NewPlugin(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to create network plugin, err:%v.\n", err)
		return
	}

	// Create IPAM plugin.
	ipamPlugin, err := ipam.NewPlugin(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to create IPAM plugin, err:%v.\n", err)
		return
	}

	// Create a channel to receive unhandled errors from the plugins.
	config.ErrChan = make(chan error, 1)

	// Create the key value store.
	config.Store, err = store.NewJsonFileStore(platform.RuntimePath + name + ".json")
	if err != nil {
		log.Printf("[cni] Failed to create store, err:%v.", err)
		return
	}

	// Acquire store lock.
	err = config.Store.Lock(true)
	if err != nil {
		log.Printf("[cni] Timed out on locking store, err:%v.", err)
		return
	}

	// Create logging provider.
	log.SetLevel(log.LevelInfo)
	err = log.SetTarget(log.TargetLogfile)
	if err != nil {
		fmt.Printf("[cni] Failed to configure logging, err:%v.\n", err)
		return
	}

	// Log platform information.
	log.Printf("[cni] Plugin enter.")
	log.Printf("Running on %v", platform.GetOSInfo())
	common.LogNetworkInterfaces()

	// Set plugin options.
	ipamPlugin.SetOption(common.OptEnvironment, common.OptEnvironmentAzure)

	// Start plugins.
	if netPlugin != nil {
		err = netPlugin.Start(&config)
		if err != nil {
			fmt.Printf("[cni] Failed to start network plugin, err:%v.\n", err)
			return
		}
	}

	if ipamPlugin != nil {
		err = ipamPlugin.Start(&config)
		if err != nil {
			fmt.Printf("[cni] Failed to start IPAM plugin, err:%v.\n", err)
			return
		}
	}

	// Set supported CNI versions.
	pluginInfo := cniVers.PluginSupports(cni.Version)

	// Parse args and call the appropriate cmd handler.
	cniSkel.PluginMain(netPlugin.Add, netPlugin.Delete, pluginInfo)

	// Cleanup.
	if netPlugin != nil {
		netPlugin.Stop()
	}

	if ipamPlugin != nil {
		ipamPlugin.Stop()
	}

	err = config.Store.Unlock()
	if err != nil {
		log.Printf("[cni] Failed to unlock store, err:%v.", err)
	}

	log.Printf("[cni] Plugin exit.")
}
