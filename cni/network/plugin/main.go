// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/network"
	"github.com/Azure/azure-container-networking/common"
)

// Version is populated by make during build.
var version string

// Main is the entry point for CNI network plugin.
func main() {
	var config common.PluginConfig
	config.Version = version

	netPlugin, err := network.NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create network plugin, err:%v.\n", err)
		os.Exit(1)
	}

	err = netPlugin.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start network plugin, err:%v.\n", err)
		os.Exit(1)
	}

	err = netPlugin.Execute(cni.PluginApi(netPlugin))

	netPlugin.Stop()

	if err != nil {
		os.Exit(1)
	}
}
