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

// GetAddressFamily returns the address family of an address.
func GetAddressFamily(address *net.IP) AddressFamily {
	var family AddressFamily

	if address.To4() == nil {
		family = AfINET
	} else {
		family = AfINET6
	}

	return family
}
