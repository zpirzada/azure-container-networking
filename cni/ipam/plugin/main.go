// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/ipam"
	"github.com/Azure/azure-container-networking/common"
)

// Version is populated by make during build.
var version string

// Main is the entry point for CNI IPAM plugin.
func main() {
	var config common.PluginConfig
	config.Version = version

	ipamPlugin, err := ipam.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin, err:%v.\n", err)
		os.Exit(1)
	}

	err = ipamPlugin.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start IPAM plugin, err:%v.\n", err)
		os.Exit(1)
	}

	err = ipamPlugin.Execute(cni.PluginApi(ipamPlugin))

	ipamPlugin.Stop()

	if err != nil {
		os.Exit(1)
	}
}
