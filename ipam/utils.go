// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net"
)

// generateAddress generates an IP address from the given network and host ID.
func generateAddress(subnet *net.IPNet, hostId net.IP) net.IP {
	// Use IPv6 addresses so it works both for IPv4 and IPv6.
	address := net.ParseIP("::")
	networkId := subnet.IP.To16()

	for i := 0; i < len(address); i++ {
		address[i] = networkId[i] | hostId[i]
	}

	return address
}
