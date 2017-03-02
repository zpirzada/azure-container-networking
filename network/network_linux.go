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
	if nwInfo.Type == "" {
		nwInfo.Type = NetworkTypeBridge
	}

	// Connect the external interface.
	switch nwInfo.Type {
	case NetworkTypeBridge:
		err := nm.connectExternalInterface(extIf, nwInfo.BridgeName)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errNetworkTypeInvalid
	}

	// Create the network object.
	nw := &network{
		Id:        nwInfo.Id,
		Type:      nwInfo.Type,
		Endpoints: make(map[string]*endpoint),
		extIf:     extIf,
	}

	return nw, nil
}

// DeleteNetworkImpl deletes an existing container network.
func (nm *networkManager) deleteNetworkImpl(nw *network) error {
	// Disconnect the interface if this was the last network using it.
	if len(nw.extIf.Networks) == 1 {
		nm.disconnectExternalInterface(nw.extIf)
	}

	return nil
}

//  SaveIPConfig saves the IP configuration of an interface.
func (nm *networkManager) saveIPConfig(hostIf *net.Interface, extIf *externalInterface) error {
	// Save global unicast IP addresses on the interface.
	addrs, err := hostIf.Addrs()
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

		log.Printf("[net] Deleting IP address %v from interface %v.", ipNet, hostIf.Name)

		err = netlink.DeleteIpAddress(hostIf.Name, ipAddr, ipNet)
		if err != nil {
			break
		}
	}

	// Save the default routes on the interface.
	routes, err := netlink.GetIpRoute(&netlink.Route{Dst: &net.IPNet{}, LinkIndex: hostIf.Index})
	if err != nil {
		log.Printf("[net] Failed to query routes: %v.", err)
		return err
	}

	for _, r := range routes {
		if r.Dst == nil {
			if r.Family == unix.AF_INET {
				extIf.IPv4Gateway = r.Gw
			} else if r.Family == unix.AF_INET6 {
				extIf.IPv6Gateway = r.Gw
			}
		}

		extIf.Routes = append(extIf.Routes, (*route)(r))
	}

	log.Printf("[net] Saved interface IP configuration %+v.", extIf)

	return err
}

// ApplyIPConfig applies a previously saved IP configuration to an interface.
func (nm *networkManager) applyIPConfig(extIf *externalInterface, targetIf *net.Interface) error {
	// Add IP addresses.
	for _, addr := range extIf.IPAddresses {
		ipAddr, ipNet, err := net.ParseCIDR(addr.String())
		ipNet.IP = ipAddr
		if err != nil {
			return err
		}

		log.Printf("[net] Adding IP address %v to interface %v.", ipNet, targetIf.Name)

		err = netlink.AddIpAddress(targetIf.Name, ipAddr, ipNet)
		if err != nil {
			log.Printf("[net] Failed to add IP address %v: %v.", addr, err)
			return err
		}
	}

	// Add IP routes.
	for _, route := range extIf.Routes {
		route.LinkIndex = targetIf.Index

		log.Printf("[net] Adding IP route %+v.", route)

		err := netlink.AddIpRoute((*netlink.Route)(route))
		if err != nil {
			log.Printf("[net] Failed to add IP route %v: %v.", route, err)
			return err
		}
	}

	return nil
}

// ConnectExternalInterface connects the given host interface to a bridge.
func (nm *networkManager) connectExternalInterface(extIf *externalInterface, bridgeName string) error {
	log.Printf("[net] Connecting interface %v.", extIf.Name)

	// Check whether this interface is already connected.
	if extIf.BridgeName != "" {
		log.Printf("[net] Interface is already connected to bridge %v.", extIf.BridgeName)
		return nil
	}

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

		link := netlink.BridgeLink{
			LinkInfo: netlink.LinkInfo{
				Type: netlink.LINK_TYPE_BRIDGE,
				Name: bridgeName,
			},
		}

		err = netlink.AddLink(&link)
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

	// Save host IP configuration.
	err = nm.saveIPConfig(hostIf, extIf)
	if err != nil {
		log.Printf("[net] Failed to save IP configuration for interface %v: %v.", hostIf.Name, err)
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

	// Apply IP configuration to the bridge for host traffic.
	err = nm.applyIPConfig(extIf, bridge)
	if err != nil {
		log.Printf("[net] Failed to apply interface IP configuration: %v.", err)
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

	// Restore IP configuration.
	hostIf, _ := net.InterfaceByName(extIf.Name)
	err = nm.applyIPConfig(extIf, hostIf)
	if err != nil {
		log.Printf("[net] Failed to apply IP configuration: %v.", err)
	}

	extIf.IPAddresses = nil
	extIf.Routes = nil

	log.Printf("[net] Disconnected interface %v.", extIf.Name)

	return nil
}
