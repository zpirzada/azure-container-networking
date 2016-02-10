// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"net"
)

func printHostInterfaces() {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Azure Driver: Got error while retrieving interfaces")
	} else {
		fmt.Println("Azure Driver: Found following Interfaces in default name space")
		for _, hostInterface := range hostInterfaces {

			addresses, ok := hostInterface.Addrs()
			if ok == nil && len(addresses) > 0 {
				fmt.Println("\t", hostInterface.Name, hostInterface.Index, hostInterface.Flags, hostInterface.HardwareAddr, hostInterface.Flags, hostInterface.MTU, addresses[0].String())
			} else {
				fmt.Println("\t", hostInterface.Name, hostInterface.Index, hostInterface.Flags, hostInterface.HardwareAddr, hostInterface.Flags, hostInterface.MTU)
			}
		}
	}
}
