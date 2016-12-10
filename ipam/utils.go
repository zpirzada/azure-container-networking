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

// ConvertAddressToIPNet returns the given IP address as an IPNet object.
func ConvertAddressToIPNet(address string) (*net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(address)
	if err != nil {
		return nil, err
	}

	ipnet.IP = ip
	return ipnet, nil
}
