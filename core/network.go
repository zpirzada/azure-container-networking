// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Azure/Aqua/log"
	"github.com/Azure/Aqua/netfilter"
	"github.com/Azure/Aqua/netlink"
	"golang.org/x/sys/unix"
)

const (
	// Prefix for bridge names.
	bridgePrefix = "aqua"

	// Prefix for host virtual network interface names.
	hostInterfacePrefix = "veth"

	// Prefix for container network interface names.
	containerInterfacePrefix = "eth"
)

// Network represents a container network.
type Network struct {
	Id        string
	ExtIf     *externalInterface
	Endpoints map[string]*Endpoint
}

// Endpoint represents a container network interface.
type Endpoint struct {
	IPv4Address net.IPNet
	IPv6Address net.IPNet
	MacAddress  net.HardwareAddr
	SrcName     string
	DstPrefix   string
	IPv4Gateway net.IP
	IPv6Gateway net.IP
}

// ExternalInterface represents a host network interface that bridges containers to external networks.
type externalInterface struct {
	name        string
	macAddress  net.HardwareAddr
	ipAddresses []*net.IPNet
	routes      []*netlink.Route
	ipv4Gateway net.IP
	ipv6Gateway net.IP
	subnets     []string
	bridgeName  string
	networks    map[string]*Network
}

// Core network state.
var state struct {
	externalInterfaces map[string]*externalInterface
	sync.Mutex
}

// Initializes core network module.
func init() {
	state.externalInterfaces = make(map[string]*externalInterface)
}

// Adds an interface to the list of available external interfaces.
func NewExternalInterface(ifName string, subnet string) error {
	state.Lock()
	defer state.Unlock()

	// Check whether the external interface is already configured.
	if state.externalInterfaces[ifName] != nil {
		return nil
	}

	// Find the host interface.
	hostIf, err := net.InterfaceByName(ifName)
	if err != nil {
		return err
	}

	extIf := externalInterface{
		name:        ifName,
		macAddress:  hostIf.HardwareAddr,
		ipv4Gateway: net.IPv4zero,
		ipv6Gateway: net.IPv6unspecified,
	}

	extIf.subnets = append(extIf.subnets, subnet)
	extIf.networks = make(map[string]*Network)

	state.externalInterfaces[ifName] = &extIf

	log.Printf("[core] Added ExternalInterface %v for subnet %v\n", ifName, subnet)

	return nil
}

// Removes an interface from the list of available external interfaces.
func DeleteExternalInterface(ifName string) error {
	state.Lock()
	defer state.Unlock()

	delete(state.externalInterfaces, ifName)

	log.Printf("[core] Removed ExternalInterface %v\n", ifName)

	return nil
}

// Finds an external interface connected to the given subnet.
func findExternalInterfaceBySubnet(subnet string) *externalInterface {
	for _, extIf := range state.externalInterfaces {
		for _, s := range extIf.subnets {
			if s == subnet {
				return extIf
			}
		}
	}

	return nil
}

// Connects a host interface to a bridge.
func connectExternalInterface(extIf *externalInterface) error {
	var addrs []net.Addr

	log.Printf("[core] Connecting interface %v\n", extIf.name)

	// Find the external interface.
	hostIf, err := net.InterfaceByName(extIf.name)
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
	extIf.routes, err = netlink.GetIpRoute(&netlink.Route{Dst: &net.IPNet{}, LinkIndex: hostIf.Index})
	if err != nil {
		log.Printf("[core] Failed to query routes, err=%v\n", err)
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

		extIf.ipAddresses = append(extIf.ipAddresses, ipNet)
		log.Printf("[core] Moving IP address %v to bridge %v\n", ipNet, bridgeName)

		err = netlink.DeleteIpAddress(extIf.name, ipAddr, ipNet)
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
	for _, route := range extIf.routes {
		if route.Dst == nil {
			if route.Family == unix.AF_INET {
				extIf.ipv4Gateway = route.Gw
			} else if route.Family == unix.AF_INET6 {
				extIf.ipv6Gateway = route.Gw
			}
		}

		route.LinkIndex = bridge.Index
		err = netlink.AddIpRoute(route)
		route.LinkIndex = hostIf.Index

		if err != nil {
			log.Printf("[core] Failed to add route %+v, err=%v\n", route, err)
			goto cleanup
		}

		log.Printf("[core] Added IP route %+v\n", route)
	}

	extIf.bridgeName = bridgeName

	log.Printf("[core] Connected interface %v to bridge %v\n", extIf.name, extIf.bridgeName)

	return nil

cleanup:
	// Roll back the changes for the network.
	ebtables.CleanupDnatForArpReplies(extIf.name)
	ebtables.CleanupSnatForOutgoingPackets(extIf.name, extIf.macAddress.String())

	netlink.DeleteLink(bridgeName)

	return err
}

// Disconnects a host interface from its bridge.
func disconnectExternalInterface(extIf *externalInterface) error {
	log.Printf("[core] Disconnecting interface %v\n", extIf.name)

	// Cleanup MAC address translation rules.
	ebtables.CleanupDnatForArpReplies(extIf.name)
	ebtables.CleanupSnatForOutgoingPackets(extIf.name, extIf.macAddress.String())

	// Disconnect external interface from its bridge.
	err := netlink.SetLinkMaster(extIf.name, "")
	if err != nil {
		log.Printf("[core] Failed to disconnect interface %v from bridge, err=%v\n", extIf.name, err)
	}

	// Delete the bridge.
	err = netlink.DeleteLink(extIf.bridgeName)
	if err != nil {
		log.Printf("[core] Failed to delete bridge %v, err=%v\n", extIf.bridgeName, err)
	}

	extIf.bridgeName = ""

	// Restore IP addresses.
	for _, addr := range extIf.ipAddresses {
		log.Printf("[core] Moving IP address %v to interface %v\n", addr, extIf.name)
		err = netlink.AddIpAddress(extIf.name, addr.IP, addr)
		if err != nil {
			log.Printf("[core] Failed to add IP address %v, err=%v\n", addr, err)
		}
	}

	extIf.ipAddresses = nil

	// Restore routes.
	for _, route := range extIf.routes {
		log.Printf("[core] Adding IP route %v to interface %v\n", route, extIf.name)
		err = netlink.AddIpRoute(route)
		if err != nil {
			log.Printf("[core] Failed to add IP route %v, err=%v\n", route, err)
		}
	}

	extIf.routes = nil

	log.Printf("[core] Disconnected interface %v\n", extIf.name)

	return nil
}

// Restarts an interface by setting its operational state down and back up.
func restartInterface(ifName string) error {
	err := netlink.SetLinkState(ifName, false)
	if err != nil {
		return err
	}

	// Delay for the state to settle.
	time.Sleep(2 * time.Second)

	err = netlink.SetLinkState(ifName, true)
	return err
}

// Creates a container network.
func CreateNetwork(networkId string, ipv4Pool string, ipv6Pool string) (*Network, error) {
	state.Lock()
	defer state.Unlock()

	log.Printf("[core] Creating network %v for subnet %v %v\n", networkId, ipv4Pool, ipv6Pool)

	// Find the external interface for this subnet.
	extIf := findExternalInterfaceBySubnet(ipv4Pool)
	if extIf == nil {
		return nil, fmt.Errorf("Pool not found")
	}

	if extIf.bridgeName == "" {
		err := connectExternalInterface(extIf)
		if err != nil {
			return nil, err
		}
	}

	nw := &Network{
		Id:    networkId,
		ExtIf: extIf,
	}

	extIf.networks[networkId] = nw

	log.Printf("[core] Created network %v on interface %v\n", networkId, extIf.name)

	return nw, nil
}

// Deletes a container network.
func DeleteNetwork(nw *Network) error {
	state.Lock()
	defer state.Unlock()

	log.Printf("[core] Deleting network %+v\n", nw)

	delete(nw.ExtIf.networks, nw.Id)

	// Disconnect the interface if this was the last network using it.
	if len(nw.ExtIf.networks) == 0 {
		disconnectExternalInterface(nw.ExtIf)
	}

	log.Printf("[core] Deleted network %+v\n", nw)

	return nil
}

// Creates a new endpoint.
func CreateEndpoint(nw *Network, endpointId string, ipAddress string) (*Endpoint, error) {
	var containerIf *net.Interface
	var ep *Endpoint

	// Parse IP address.
	ipAddr, ipNet, err := net.ParseCIDR(ipAddress)
	ipNet.IP = ipAddr
	if err != nil {
		return nil, err
	}

	state.Lock()
	defer state.Unlock()

	log.Printf("[core] Creating endpoint %v in network %v\n", endpointId, nw.Id)

	// Create a veth pair.
	contIfName := fmt.Sprintf("%s%s-2", hostInterfacePrefix, endpointId[:7])
	hostIfName := fmt.Sprintf("%s%s", hostInterfacePrefix, endpointId[:7])

	err = netlink.AddVethPair(contIfName, hostIfName)
	if err != nil {
		log.Printf("[core] Failed to create veth pair, err=%v\n", err)
		return nil, err
	}

	// Assign IP address to container network interface.
	err = netlink.AddIpAddress(contIfName, ipAddr, ipNet)
	if err != nil {
		goto cleanup
	}

	// Host interface up.
	err = netlink.SetLinkState(hostIfName, true)
	if err != nil {
		goto cleanup
	}

	// Connect host interface to the bridge.
	err = netlink.SetLinkMaster(hostIfName, nw.ExtIf.bridgeName)
	if err != nil {
		goto cleanup
	}

	// Query container network interface info.
	containerIf, err = net.InterfaceByName(contIfName)
	if err != nil {
		goto cleanup
	}

	// Setup MAC address translation rules for container interface.
	err = ebtables.SetupDnatBasedOnIPV4Address(ipAddr.String(), containerIf.HardwareAddr.String())
	if err != nil {
		goto cleanup
	}

	ep = &Endpoint{
		IPv4Address: *ipNet,
		IPv6Address: net.IPNet{},
		MacAddress:  containerIf.HardwareAddr,
		SrcName:     contIfName,
		DstPrefix:   containerInterfacePrefix,
		IPv4Gateway: nw.ExtIf.ipv4Gateway,
		IPv6Gateway: nw.ExtIf.ipv6Gateway,
	}

	log.Printf("[core] Created endpoint: %+v\n", ep)

	return ep, nil

cleanup:
	// Roll back the changes for the endpoint.
	netlink.DeleteLink(contIfName)

	return nil, err
}

// Deletes an existing endpoint.
func DeleteEndpoint(ep *Endpoint) error {
	state.Lock()
	defer state.Unlock()

	log.Printf("[core] Deleting endpoint: %+v\n", ep)

	// Delete veth pair.
	netlink.DeleteLink(ep.SrcName)

	// Cleanup MAC address translation rules.
	err := ebtables.RemoveDnatBasedOnIPV4Address(ep.IPv4Address.IP.String(), ep.MacAddress.String())

	log.Printf("[core] Deleted endpoint: %+v\n", ep)

	return err
}
