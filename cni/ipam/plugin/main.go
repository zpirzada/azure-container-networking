// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"

	"github.com/Azure/azure-container-networking/cni/ipam"
	"github.com/Azure/azure-container-networking/common"
)

// Version is populated by make during build.
var version string

// Main is the entry point for CNI IPAM plugin.
func main() {
	// Initialize plugin common configuration.
	var config common.PluginConfig
	config.Version = version

	// Create IPAM plugin.
	ipamPlugin, err := ipam.NewPlugin(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to create IPAM plugin, err:%v.\n", err)
		return
	}

	err = ipamPlugin.Start(&config)
	if err != nil {
		fmt.Printf("[cni] Failed to start IPAM plugin, err:%v.\n", err)
		return
	}

	ipamPlugin.Stop()
}
