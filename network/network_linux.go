// Copyright 2017 Microsoft. All rights reserved.
// MIT License

// +build linux

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
	bridgePrefix = "azure"
)

// Linux implementation of route.
type route netlink.Route

// NewNetworkImpl creates a new container network.
func (nm *networkManager) newNetworkImpl(nwInfo *NetworkInfo, extIf *externalInterface) (*network, error) {
	// Connect the external interface if not already connected.
	if extIf.BridgeName == "" {
		err := nm.connectExternalInterface(extIf, nwInfo.BridgeName)
		if err != nil {
			return nil, err
		}
	}

	// Create the network object.
	nw := &network{
		Id:        nwInfo.Id,
		Endpoints: make(map[string]*endpoint),
		extIf:     extIf,
	}

	return nw, nil
}

// DeleteNetworkImpl deletes an existing container network.
func (nm *networkManager) deleteNetworkImpl(nw *network) error {
	// Disconnect the interface if this was the last network using it.
	if len(nw.extIf.Networks) == 0 {
		nm.disconnectExternalInterface(nw.extIf)
	}

	return nil
}

// ConnectExternalInterface connects the given host interface to a bridge.
func (nm *networkManager) connectExternalInterface(extIf *externalInterface, bridgeName string) error {
	var addrs []net.Addr
	var routes []*netlink.Route

	log.Printf("[net] Connecting interface %v.", extIf.Name)

	// Find the external interface.
	hostIf, err := net.InterfaceByName(extIf.Name)
	if err != nil {
		return err
	}

	// If a bridge name is not specified, generate one based on the external interface index.
	if bridgeName == "" {
		bridgeName = fmt.Sprintf("%s%d", bridgePrefix, hostIf.Index)
	}

	// Check if the bridge already exists.
	bridge, err := net.InterfaceByName(bridgeName)
	if err != nil {
		// Create the bridge.
		log.Printf("[net] Creating bridge %v.", bridgeName)
		err = netlink.AddLink(bridgeName, "bridge")
		if err != nil {
			return err
		}

		bridge, err = net.InterfaceByName(bridgeName)
		if err != nil {
			goto cleanup
		}
	} else {
		// Use the existing bridge.
		log.Printf("[net] Found existing bridge %v.", bridgeName)
	}

	// Query the default routes on the external interface.
	routes, err = netlink.GetIpRoute(&netlink.Route{Dst: &net.IPNet{}, LinkIndex: hostIf.Index})
	if err != nil {
		log.Printf("[net] Failed to query routes, err:%v.", err)
		goto cleanup
	}
	for _, r := range routes {
		extIf.Routes = append(extIf.Routes, (*route)(r))
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
	log.Printf("[net] Setting up MAC address translation rules for %v.", hostIf.Name)
	err = ebtables.SetSnatForInterface(hostIf.Name, hostIf.HardwareAddr, ebtables.Append)
	if err != nil {
		goto cleanup
	}

	err = ebtables.SetDnatForArpReplies(hostIf.Name, ebtables.Append)
	if err != nil {
		goto cleanup
	}

	// External interface down.
	log.Printf("[net] Setting link %v state down.", hostIf.Name)
	err = netlink.SetLinkState(hostIf.Name, false)
	if err != nil {
		goto cleanup
	}

	// Connect the external interface to the bridge.
	log.Printf("[net] Setting link %v master %v.", hostIf.Name, bridgeName)
	err = netlink.SetLinkMaster(hostIf.Name, bridgeName)
	if err != nil {
		goto cleanup
	}

	// External interface up.
	log.Printf("[net] Setting link %v state up.", hostIf.Name)
	err = netlink.SetLinkState(hostIf.Name, true)
	if err != nil {
		goto cleanup
	}

	// Bridge up.
	log.Printf("[net] Setting link %v state up.", bridgeName)
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
		log.Printf("[net] Adding IP route %+v.", route)
		err = netlink.AddIpRoute((*netlink.Route)(route))
		route.LinkIndex = hostIf.Index

		if err != nil {
			goto cleanup
		}
	}

	extIf.BridgeName = bridgeName

	log.Printf("[net] Connected interface %v to bridge %v.", extIf.Name, extIf.BridgeName)

	return nil

cleanup:
	log.Printf("[net] Connecting interface %v failed, err:%v.", extIf.Name, err)

	// Roll back the changes for the network.
	ebtables.SetDnatForArpReplies(extIf.Name, ebtables.Delete)
	ebtables.SetSnatForInterface(extIf.Name, extIf.MacAddress, ebtables.Delete)

	netlink.DeleteLink(bridgeName)

	return err
}

// DisconnectExternalInterface disconnects a host interface from its bridge.
func (nm *networkManager) disconnectExternalInterface(extIf *externalInterface) error {
	log.Printf("[net] Disconnecting interface %v.", extIf.Name)

	// Cleanup MAC address translation rules.
	ebtables.SetDnatForArpReplies(extIf.Name, ebtables.Delete)
	ebtables.SetSnatForInterface(extIf.Name, extIf.MacAddress, ebtables.Delete)

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
		err = netlink.AddIpRoute((*netlink.Route)(route))
		if err != nil {
			log.Printf("[net] Failed to add IP route %v, err:%v.", route, err)
		}
	}

	extIf.Routes = nil

	log.Printf("[net] Disconnected interface %v.", extIf.Name)

	return nil
}
