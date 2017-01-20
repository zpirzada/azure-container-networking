// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"

	"github.com/Azure/azure-container-networking/cni/network"
	"github.com/Azure/azure-container-networking/common"
)

// Version is populated by make during build.
var version string

// Main is the entry point for CNI network plugin.
func main() {
	// Initialize plugin common configuration.
	var config common.PluginConfig
	config.Version = version

	// Create network plugin.
	netPlugin, err := network.NewPlugin(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to create network plugin, err:%v.\n", err)
		return
	}

	err = netPlugin.Start(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to start network plugin, err:%v.\n", err)
		return
	}

	netPlugin.Stop()
}
