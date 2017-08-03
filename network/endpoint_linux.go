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
)

const (
	// Common prefix for all types of host network interface names.
	commonInterfacePrefix = "az"

	// Prefix for host virtual network interface names.
	hostVEthInterfacePrefix = commonInterfacePrefix + "veth"

	// Prefix for container network interface names.
	containerInterfacePrefix = "eth"
)

// newEndpointImpl creates a new endpoint in the network.
func (nw *network) newEndpointImpl(epInfo *EndpointInfo) (*endpoint, error) {
	var containerIf *net.Interface
	var ns *Namespace
	var ep *endpoint
	var err error

	// Create a veth pair.
	hostIfName := fmt.Sprintf("%s%s", hostVEthInterfacePrefix, epInfo.Id[:7])
	contIfName := fmt.Sprintf("%s%s-2", hostVEthInterfacePrefix, epInfo.Id[:7])

	log.Printf("[net] Creating veth pair %v %v.", hostIfName, contIfName)

	link := netlink.VEthLink{
		LinkInfo: netlink.LinkInfo{
			Type: netlink.LINK_TYPE_VETH,
			Name: contIfName,
		},
		PeerName: hostIfName,
	}

	err = netlink.AddLink(&link)
	if err != nil {
		log.Printf("[net] Failed to create veth pair, err:%v.", err)
		return nil, err
	}

	//
	// Host network interface setup.
	//

	// Host interface up.
	log.Printf("[net] Setting link %v state up.", hostIfName)
	err = netlink.SetLinkState(hostIfName, true)
	if err != nil {
		goto cleanup
	}

	// Connect host interface to the bridge.
	log.Printf("[net] Setting link %v master %v.", hostIfName, nw.extIf.BridgeName)
	err = netlink.SetLinkMaster(hostIfName, nw.extIf.BridgeName)
	if err != nil {
		goto cleanup
	}

	//
	// Container network interface setup.
	//

	// Query container network interface info.
	containerIf, err = net.InterfaceByName(contIfName)
	if err != nil {
		goto cleanup
	}

	// Setup rules for IP addresses on the container interface.
	for _, ipAddr := range epInfo.IPAddresses {
		// Add ARP reply rule.
		log.Printf("[net] Adding ARP reply rule for IP address %v on %v.", ipAddr.String(), contIfName)
		err = ebtables.SetArpReply(ipAddr.IP, nw.getArpReplyAddress(containerIf.HardwareAddr), ebtables.Append)
		if err != nil {
			goto cleanup
		}

		// Add MAC address translation rule.
		log.Printf("[net] Adding MAC DNAT rule for IP address %v on %v.", ipAddr.String(), contIfName)
		err = ebtables.SetDnatForIPAddress(nw.extIf.Name, ipAddr.IP, containerIf.HardwareAddr, ebtables.Append)
		if err != nil {
			goto cleanup
		}
	}

	// If a network namespace for the container interface is specified...
	if epInfo.NetNsPath != "" {
		// Open the network namespace.
		log.Printf("[net] Opening netns %v.", epInfo.NetNsPath)
		ns, err = OpenNamespace(epInfo.NetNsPath)
		if err != nil {
			goto cleanup
		}
		defer ns.Close()

		// Move the container interface to container's network namespace.
		log.Printf("[net] Setting link %v netns %v.", contIfName, epInfo.NetNsPath)
		err = netlink.SetLinkNetNs(contIfName, ns.GetFd())
		if err != nil {
			goto cleanup
		}

		// Enter the container network namespace.
		log.Printf("[net] Entering netns %v.", epInfo.NetNsPath)
		err = ns.Enter()
		if err != nil {
			goto cleanup
		}
	}

	// If a name for the container interface is specified...
	if epInfo.IfName != "" {
		// Interface needs to be down before renaming.
		log.Printf("[net] Setting link %v state down.", contIfName)
		err = netlink.SetLinkState(contIfName, false)
		if err != nil {
			goto cleanup
		}

		// Rename the container interface.
		log.Printf("[net] Setting link %v name %v.", contIfName, epInfo.IfName)
		err = netlink.SetLinkName(contIfName, epInfo.IfName)
		if err != nil {
			goto cleanup
		}
		contIfName = epInfo.IfName

		// Bring the interface back up.
		log.Printf("[net] Setting link %v state up.", contIfName)
		err = netlink.SetLinkState(contIfName, true)
		if err != nil {
			goto cleanup
		}
	}

	// Assign IP address to container network interface.
	for _, ipAddr := range epInfo.IPAddresses {
		log.Printf("[net] Adding IP address %v to link %v.", ipAddr.String(), contIfName)
		err = netlink.AddIpAddress(contIfName, ipAddr.IP, &ipAddr)
		if err != nil {
			goto cleanup
		}
	}

	// Add IP routes to container network interface.
	for _, route := range epInfo.Routes {
		log.Printf("[net] Adding IP route %+v to link %v.", route, contIfName)

		nlRoute := &netlink.Route{
			Family:    netlink.GetIpAddressFamily(route.Gw),
			Dst:       &route.Dst,
			Gw:        route.Gw,
			LinkIndex: containerIf.Index,
		}

		err = netlink.AddIpRoute(nlRoute)
		if err != nil {
			goto cleanup
		}
	}

	// If inside the container network namespace...
	if ns != nil {
		// Return to host network namespace.
		log.Printf("[net] Exiting netns %v.", epInfo.NetNsPath)
		err = ns.Exit()
		if err != nil {
			goto cleanup
		}
	}

	// Create the endpoint object.
	ep = &endpoint{
		Id:          epInfo.Id,
		IfName:      contIfName,
		HostIfName:  hostIfName,
		MacAddress:  containerIf.HardwareAddr,
		IPAddresses: epInfo.IPAddresses,
		Gateways:    []net.IP{nw.extIf.IPv4Gateway},
	}

	return ep, nil

cleanup:
	// Roll back the changes for the endpoint.
	netlink.DeleteLink(contIfName)

	return nil, err
}

// deleteEndpointImpl deletes an existing endpoint from the network.
func (nw *network) deleteEndpointImpl(ep *endpoint) error {
	// Delete the veth pair by deleting one of the peer interfaces.
	// Deleting the host interface is more convenient since it does not require
	// entering the container netns and hence works both for CNI and CNM.
	log.Printf("[net] Deleting veth pair %v %v.", ep.HostIfName, ep.IfName)
	err := netlink.DeleteLink(ep.HostIfName)
	if err != nil {
		log.Printf("[net] Failed to delete veth pair %v: %v.", ep.HostIfName, err)
		return err
	}

	// Delete rules for IP addresses on the container interface.
	for _, ipAddr := range ep.IPAddresses {
		// Delete ARP reply rule.
		log.Printf("[net] Deleting ARP reply rule for IP address %v on %v.", ipAddr.String(), ep.Id)
		err = ebtables.SetArpReply(ipAddr.IP, nw.getArpReplyAddress(ep.MacAddress), ebtables.Delete)
		if err != nil {
			log.Printf("[net] Failed to delete ARP reply rule for IP address %v: %v.", ipAddr.String(), err)
		}

		// Delete MAC address translation rule.
		log.Printf("[net] Deleting MAC DNAT rule for IP address %v on %v.", ipAddr.String(), ep.Id)
		err = ebtables.SetDnatForIPAddress(nw.extIf.Name, ipAddr.IP, ep.MacAddress, ebtables.Delete)
		if err != nil {
			log.Printf("[net] Failed to delete MAC DNAT rule for IP address %v: %v.", ipAddr.String(), err)
		}
	}

	return nil
}

// getArpReplyAddress returns the MAC address to use in ARP replies.
func (nw *network) getArpReplyAddress(epMacAddress net.HardwareAddr) net.HardwareAddr {
	var macAddress net.HardwareAddr

	if nw.Mode == opModeTunnel {
		// In tunnel mode, resolve all IP addresses to the virtual MAC address for hairpinning.
		macAddress, _ = net.ParseMAC(virtualMacAddress)
	} else {
		// Otherwise, resolve to actual MAC address.
		macAddress = epMacAddress
	}

	return macAddress
}

// getInfoImpl returns information about the endpoint.
func (ep *endpoint) getInfoImpl(epInfo *EndpointInfo) {
}
