// Copyright Microsoft Corp.
// All rights reserved.

package common

import (
	"net"

	"github.com/Azure/azure-container-networking/log"
)

// LogNetworkInterfaces logs the host's network interfaces in the default namespace.
func LogNetworkInterfaces() {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to query network interfaces, err:%v", err)
		return
	}

	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		log.Printf("[net] Network interface: %+v with IP addresses: %+v", iface, addrs)
	}
}
