package network

import (
	"net"
	"strings"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
)

func createEndpoint(hostVethName string, containerVethName string) error {
	log.Printf("[net] Creating veth pair %v %v.", hostVethName, containerVethName)

	link := netlink.VEthLink{
		LinkInfo: netlink.LinkInfo{
			Type: netlink.LINK_TYPE_VETH,
			Name: hostVethName,
		},
		PeerName: containerVethName,
	}

	err := netlink.AddLink(&link)
	if err != nil {
		log.Printf("[net] Failed to create veth pair, err:%v.", err)
		return err
	}

	log.Printf("[net] Setting link %v state up.", hostVethName)
	err = netlink.SetLinkState(hostVethName, true)
	if err != nil {
		return err
	}

	return nil
}

func setupContainerInterface(containerVethName string, targetIfName string) error {
	// Interface needs to be down before renaming.
	log.Printf("[net] Setting link %v state down.", containerVethName)
	if err := netlink.SetLinkState(containerVethName, false); err != nil {
		return err
	}

	// Rename the container interface.
	log.Printf("[net] Setting link %v name %v.", containerVethName, targetIfName)
	if err := netlink.SetLinkName(containerVethName, targetIfName); err != nil {
		return err
	}

	// Bring the interface back up.
	log.Printf("[net] Setting link %v state up.", targetIfName)
	return netlink.SetLinkState(targetIfName, true)
}

func assignIPToInterface(interfaceName string, ipAddresses []net.IPNet) error {
	// Assign IP address to container network interface.
	for _, ipAddr := range ipAddresses {
		log.Printf("[net] Adding IP address %v to link %v.", ipAddr.String(), interfaceName)
		err := netlink.AddIpAddress(interfaceName, ipAddr.IP, &ipAddr)
		if err != nil {
			return err
		}
	}

	return nil
}

func addRoutes(interfaceName string, routes []RouteInfo) error {
	ifIndex := 0
	interfaceIf, _ := net.InterfaceByName(interfaceName)

	for _, route := range routes {
		log.Printf("[ovs] Adding IP route %+v to link %v.", route, interfaceName)

		if route.DevName != "" {
			devIf, _ := net.InterfaceByName(route.DevName)
			ifIndex = devIf.Index
		} else {
			ifIndex = interfaceIf.Index
		}

		nlRoute := &netlink.Route{
			Family:    netlink.GetIpAddressFamily(route.Gw),
			Dst:       &route.Dst,
			Gw:        route.Gw,
			LinkIndex: ifIndex,
		}

		if err := netlink.AddIpRoute(nlRoute); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "file exists") {
				return err
			} else {
				log.Printf("route already exists")
			}
		}
	}

	return nil
}

func deleteRoutes(interfaceName string, routes []RouteInfo) error {
	ifIndex := 0
	interfaceIf, _ := net.InterfaceByName(interfaceName)

	for _, route := range routes {
		log.Printf("[ovs] Adding IP route %+v to link %v.", route, interfaceName)

		if route.DevName != "" {
			devIf, _ := net.InterfaceByName(route.DevName)
			ifIndex = devIf.Index
		} else {
			ifIndex = interfaceIf.Index
		}

		nlRoute := &netlink.Route{
			Family:    netlink.GetIpAddressFamily(route.Gw),
			Dst:       &route.Dst,
			Gw:        route.Gw,
			LinkIndex: ifIndex,
		}

		if err := netlink.DeleteIpRoute(nlRoute); err != nil {
			return err
		}
	}

	return nil
}
