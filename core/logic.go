// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"net"
	"strings"
)

type interfaceDetails struct {
	Address     net.IPNet
	AddressIPV6 net.IPNet
	MacAddress  net.HardwareAddr
	ID          int
	SrcName     string
	DstPrefix   string
	GatewayIPv4 net.IP
}

// GetInterfaceToAttach is a function that contains the logic to create/select
// the interface that will be attached to the container
func GetInterfaceToAttach(interfaceNameToAttach string, ipAddressToAttach string) (net.IPNet, net.IPNet, net.HardwareAddr, int, string, string, net.IP, string) {

	printHostInterfaces()

	fmt.Println("Request came for", ipAddressToAttach)
	var selectedInterface net.Interface
	selected := false

	hostInterfaces, err := net.Interfaces()
	if err != nil {
		ermsg := "Azure: Got error while retrieving interfaces"
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, ermsg
	}

	fmt.Println("Azure: Going to select an interface for container")
	for _, hostInterface := range hostInterfaces {

		addresses, ok := hostInterface.Addrs()
		flag := hostInterface.Flags & net.FlagBroadcast
		loopbackFlag := hostInterface.Flags & net.FlagLoopback
		canBeSelected := ok == nil &&
			// interface is configured with some ip address
			len(addresses) > 0 &&
			// interface supports broadcast access capability
			flag == net.FlagBroadcast &&
			// interface is not a loopback interface
			loopbackFlag != net.FlagLoopback // &&
			//strings.Contains(hostInterface.Name, "veth")

		if ipAddressToAttach == "" {
			if canBeSelected && interfaceNameToAttach != "" {
				isThisSameAsRequested := hostInterface.Name == interfaceNameToAttach
				canBeSelected = canBeSelected && isThisSameAsRequested
			}
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		} else {
			if canBeSelected {
				doesThisInterfaceHaveSameIPAsRequested := false
				addrs, _ := hostInterface.Addrs()
				for _, addr := range addrs {
					address := addr.String()
					if strings.Split(address, "/")[0] == ipAddressToAttach {
						doesThisInterfaceHaveSameIPAsRequested = true
						break
					}
				}
				canBeSelected = canBeSelected && doesThisInterfaceHaveSameIPAsRequested
			}
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		}
	}

	if !selected {
		ermsg := "Azure: Interface Not Found Error. " +
			"It is possible that none of the interfaces is configured properly, " +
			"or none of configured interfaces match the selection criteria."
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, ermsg
	}

	fmt.Println("Selected interface: ", selectedInterface.Name)

	addresses, _ := selectedInterface.Addrs()
	address := addresses[0].String()
	ipv4, ipv4Net, _ := net.ParseCIDR(address)
	ipv4Net.IP = ipv4
	bytes := strings.Split(address, ".")
	gateway := bytes[0] + "." + bytes[1] + "." + bytes[2] + ".1"
	gatewayIpv4 := net.ParseIP(gateway)
	srcName := selectedInterface.Name
	macAddress, _ := net.ParseMAC(selectedInterface.HardwareAddr.String())

	fmt.Println("Azure: Interface ip/netmask: ",
		ipv4Net.IP.String(), "/", ipv4Net.Mask.String())
	fmt.Println("Azure: Gateway IP: ", gatewayIpv4.String())

	retval := &interfaceDetails{
		Address:     *ipv4Net,
		MacAddress:  macAddress,
		SrcName:     srcName,
		DstPrefix:   srcName + "eth",
		GatewayIPv4: gatewayIpv4,
	}
	fmt.Println("Azure: Successfully selected interface ", retval)
	return *ipv4Net, net.IPNet{}, macAddress, -1, srcName, srcName + "eth", gatewayIpv4, ""
}
