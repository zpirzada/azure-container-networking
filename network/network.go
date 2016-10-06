// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net"

	"github.com/Azure/azure-container-networking/ebtables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
	"golang.org/x/sys/unix"
)

const (
	// Prefix for bridge names.
	bridgePrefix = "aqua"
)

// ExternalInterface represents a host network interface that bridges containers to external networks.
type externalInterface struct {
	Name        string
	Networks    map[string]*network
	Subnets     []string
	BridgeName  string
	MacAddress  net.HardwareAddr
	IPAddresses []*net.IPNet
	Routes      []*netlink.Route
	IPv4Gateway net.IP
	IPv6Gateway net.IP
}

// A container network is a set of endpoints allowed to communicate with each other.
type network struct {
	Id        string
	Endpoints map[string]*endpoint
	extIf     *externalInterface
}

type options map[string]interface{}

// NewExternalInterface adds a host interface to the list of available external interfaces.
func (nm *networkManager) newExternalInterface(ifName string, subnet string) error {
	// Check whether the external interface is already configured.
	if nm.ExternalInterfaces[ifName] != nil {
		return nil
	}

	// Find the host interface.
	hostIf, err := net.InterfaceByName(ifName)
	if err != nil {
		return err
	}

	extIf := externalInterface{
		Name:        ifName,
		Networks:    make(map[string]*network),
		MacAddress:  hostIf.HardwareAddr,
		IPv4Gateway: net.IPv4zero,
		IPv6Gateway: net.IPv6unspecified,
	}

	extIf.Subnets = append(extIf.Subnets, subnet)

	nm.ExternalInterfaces[ifName] = &extIf

	log.Printf("[net] Added ExternalInterface %v for subnet %v.", ifName, subnet)

	return nil
}

// DeleteExternalInterface removes an interface from the list of available external interfaces.
func (nm *networkManager) deleteExternalInterface(ifName string) error {
	delete(nm.ExternalInterfaces, ifName)

	log.Printf("[net] Deleted ExternalInterface %v.", ifName)

	return nil
}

// FindExternalInterfaceBySubnet finds an external interface connected to the given subnet.
func (nm *networkManager) findExternalInterfaceBySubnet(subnet string) *externalInterface {
	for _, extIf := range nm.ExternalInterfaces {
		for _, s := range extIf.Subnets {
			if s == subnet {
				return extIf
			}
		}
	}

	return nil
}

// ConnectExternalInterface connects the given host interface to a bridge.
func (nm *networkManager) connectExternalInterface(extIf *externalInterface) error {
	var addrs []net.Addr

	log.Printf("[net] Connecting interface %v.", extIf.Name)

	// Find the external interface.
	hostIf, err := net.InterfaceByName(extIf.Name)
	if err != nil {
		return err
	}

	// Create the bridge.
	bridgeName := fmt.Sprintf("%s%d", bridgePrefix, hostIf.Index)
	bridge, err := net.InterfaceByName(bridgeName)
	if err != nil {
		err = netlink.AddLink(bridgeName, "bridge")
		if err != nil {
			return err
		}

		bridge, err = net.InterfaceByName(bridgeName)
		if err != nil {
			goto cleanup
		}
	}

	// Query the default routes on the external interface.
	extIf.Routes, err = netlink.GetIpRoute(&netlink.Route{Dst: &net.IPNet{}, LinkIndex: hostIf.Index})
	if err != nil {
		log.Printf("[net] Failed to query routes, err:%v.", err)
		goto cleanup
	}

	// Assign external interface's IP addresses to the bridge for host traffic.
	addrs, _ = hostIf.Addrs()
	for _, addr := range addrs {
		ipAddr, ipNet, err := net.ParseCIDR(addr.String())
		ipNet.IP = ipAddr
		if err != nil {
			continue
		}

		if !ipAddr.IsGlobalUnicast() {
			continue
		}

		extIf.IPAddresses = append(extIf.IPAddresses, ipNet)
		log.Printf("[net] Moving IP address %v to bridge %v.", ipNet, bridgeName)

		err = netlink.DeleteIpAddress(extIf.Name, ipAddr, ipNet)
		if err != nil {
			goto cleanup
		}

		err = netlink.AddIpAddress(bridgeName, ipAddr, ipNet)
		if err != nil {
			goto cleanup
		}
	}

	// Setup MAC address translation rules for external interface.
	err = ebtables.SetupSnatForOutgoingPackets(hostIf.Name, hostIf.HardwareAddr.String())
	if err != nil {
		goto cleanup
	}

	err = ebtables.SetupDnatForArpReplies(hostIf.Name)
	if err != nil {
		goto cleanup
	}

	// External interface down.
	err = netlink.SetLinkState(hostIf.Name, false)
	if err != nil {
		goto cleanup
	}

	// Connect the external interface to the bridge.
	err = netlink.SetLinkMaster(hostIf.Name, bridgeName)
	if err != nil {
		goto cleanup
	}

	// External interface up.
	err = netlink.SetLinkState(hostIf.Name, true)
	if err != nil {
		goto cleanup
	}

	// Bridge up.
	err = netlink.SetLinkState(bridgeName, true)
	if err != nil {
		goto cleanup
	}

	// Setup routes on bridge.
	for _, route := range extIf.Routes {
		if route.Dst == nil {
			if route.Family == unix.AF_INET {
				extIf.IPv4Gateway = route.Gw
			} else if route.Family == unix.AF_INET6 {
				extIf.IPv6Gateway = route.Gw
			}
		}

		route.LinkIndex = bridge.Index
		err = netlink.AddIpRoute(route)
		route.LinkIndex = hostIf.Index

		if err != nil {
			log.Printf("[net] Failed to add route %+v, err:%v.", route, err)
			goto cleanup
		}

		log.Printf("[net] Added IP route %+v.", route)
	}

	extIf.BridgeName = bridgeName

	log.Printf("[net] Connected interface %v to bridge %v.", extIf.Name, extIf.BridgeName)

	return nil

cleanup:
	// Roll back the changes for the network.
	ebtables.CleanupDnatForArpReplies(extIf.Name)
	ebtables.CleanupSnatForOutgoingPackets(extIf.Name, extIf.MacAddress.String())

	netlink.DeleteLink(bridgeName)

	return err
}

// DisconnectExternalInterface disconnects a host interface from its bridge.
func (nm *networkManager) disconnectExternalInterface(extIf *externalInterface) error {
	log.Printf("[net] Disconnecting interface %v.", extIf.Name)

	// Cleanup MAC address translation rules.
	ebtables.CleanupDnatForArpReplies(extIf.Name)
	ebtables.CleanupSnatForOutgoingPackets(extIf.Name, extIf.MacAddress.String())

	// Disconnect external interface from its bridge.
	err := netlink.SetLinkMaster(extIf.Name, "")
	if err != nil {
		log.Printf("[net] Failed to disconnect interface %v from bridge, err:%v.", extIf.Name, err)
	}

	// Delete the bridge.
	err = netlink.DeleteLink(extIf.BridgeName)
	if err != nil {
		log.Printf("[net] Failed to delete bridge %v, err:%v.", extIf.BridgeName, err)
	}

	extIf.BridgeName = ""

	// Restore IP addresses.
	for _, addr := range extIf.IPAddresses {
		log.Printf("[net] Moving IP address %v to interface %v.", addr, extIf.Name)
		err = netlink.AddIpAddress(extIf.Name, addr.IP, addr)
		if err != nil {
			log.Printf("[net] Failed to add IP address %v, err:%v.", addr, err)
		}
	}

	extIf.IPAddresses = nil

	// Restore routes.
	for _, route := range extIf.Routes {
		log.Printf("[net] Adding IP route %v to interface %v.", route, extIf.Name)
		err = netlink.AddIpRoute(route)
		if err != nil {
			log.Printf("[net] Failed to add IP route %v, err:%v.", route, err)
		}
	}

	extIf.Routes = nil

	log.Printf("[net] Disconnected interface %v.", extIf.Name)

	return nil
}

// NewNetwork creates a new container network.
func (nm *networkManager) newNetwork(networkId string, options map[string]interface{}, ipv4Data, ipv6Data []ipamData) (*network, error) {
	// Assume single pool per address family.
	var ipv4Pool, ipv6Pool string
	if len(ipv4Data) > 0 {
		ipv4Pool = ipv4Data[0].Pool
	}

	if len(ipv6Data) > 0 {
		ipv6Pool = ipv6Data[0].Pool
	}

	log.Printf("[net] Creating network %v for subnet %v %v.", networkId, ipv4Pool, ipv6Pool)

	// Find the external interface for this subnet.
	extIf := nm.findExternalInterfaceBySubnet(ipv4Pool)
	if extIf == nil {
		return nil, fmt.Errorf("Pool not found")
	}

	if extIf.Networks[networkId] != nil {
		return nil, errNetworkExists
	}

	// Connect the external interface if not already connected.
	if extIf.BridgeName == "" {
		err := nm.connectExternalInterface(extIf)
		if err != nil {
			return nil, err
		}
	}

	// Create the network object.
	nw := &network{
		Id:        networkId,
		Endpoints: make(map[string]*endpoint),
		extIf:     extIf,
	}

	extIf.Networks[networkId] = nw

	log.Printf("[net] Created network %v on interface %v.", networkId, extIf.Name)

	return nw, nil
}

// DeleteNetwork deletes an existing container network.
func (nm *networkManager) deleteNetwork(networkId string) error {
	nw, err := nm.getNetwork(networkId)
	if err != nil {
		return err
	}

	log.Printf("[net] Deleting network %+v.", nw)

	// Remove the network object.
	delete(nw.extIf.Networks, networkId)

	// Disconnect the interface if this was the last network using it.
	if len(nw.extIf.Networks) == 0 {
		nm.disconnectExternalInterface(nw.extIf)
	}

	log.Printf("[net] Deleted network %+v.", nw)

	return nil
}

// GetNetwork returns the network with the given ID.
func (nm *networkManager) getNetwork(networkId string) (*network, error) {
	for _, extIf := range nm.ExternalInterfaces {
		nw, ok := extIf.Networks[networkId]
		if ok {
			return nw, nil
		}
	}

	return nil, errNetworkNotFound
}
