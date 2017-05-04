// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package platform

import (
	"net"
)

// AddressFamily specifies a protocol address family number.
type AddressFamily int

const (
	AfUnspec AddressFamily = 0
	AfINET   AddressFamily = 0x2
	AfINET6  AddressFamily = 0xa
)

// GetAddressFamily returns the address family of an IP address.
func GetAddressFamily(address *net.IP) AddressFamily {
	var family AddressFamily

	if address.To4() == nil {
		family = AfINET
	} else {
		family = AfINET6
	}

	return family
}

// GenerateAddress generates an IP address from the given network and host ID.
func GenerateAddress(subnet *net.IPNet, hostID net.IP) net.IP {
	// Use IPv6 addresses so it works both for IPv4 and IPv6.
	address := net.ParseIP("::")
	networkID := subnet.IP.To16()

	for i := 0; i < len(address); i++ {
		address[i] = networkID[i] | hostID[i]
	}

	return address
}

// ConvertStringToIPNet converts the given IP address string to a net.IPNet object.
func ConvertStringToIPNet(address string) (*net.IPNet, error) {
	addr, ipnet, err := net.ParseCIDR(address)
	if err != nil {
		return nil, err
	}

	ipnet.IP = addr
	return ipnet, nil
}

// ConvertStringToIPAddress converts the given IP address string to a net.IP object.
// The input string can be in regular dotted notation or CIDR notation.
func ConvertStringToIPAddress(address string) net.IP {
	addr, _, err := net.ParseCIDR(address)
	if err != nil {
		addr = net.ParseIP(address)
	}
	return addr
}
