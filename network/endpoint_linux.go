// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net"

	"github.com/Azure/azure-container-networking/ebtables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
)

const (
	// Prefix for host virtual network interface names.
	hostInterfacePrefix = "veth"

	// Prefix for container network interface names.
	containerInterfacePrefix = "eth"
)

// Endpoint represents a container network interface.
type endpoint struct {
	Id          string
	SandboxKey  string
	IfName      string
	HostIfName  string
	MacAddress  net.HardwareAddr
	IPAddresses []net.IPNet
	Gateways    []net.IP
}

// EndpointInfo contains read-only information about an endpoint.
type EndpointInfo struct {
	Id          string
	IfName      string
	NetNsPath   string
	IPAddresses []net.IPNet
	Routes      []RouteInfo
}

// RouteInfo contains information about an IP route.
type RouteInfo struct {
	Dst net.IPNet
	Gw  net.IP
}

// NewEndpoint creates a new endpoint in the network.
func (nw *network) newEndpoint(epInfo *EndpointInfo) (*endpoint, error) {
	var containerIf *net.Interface
	var ns *Namespace
	var ep *endpoint
	var err error

	log.Printf("[net] Creating endpoint %v in network %v.", epInfo.Id, nw.Id)

	if nw.Endpoints[epInfo.Id] != nil {
		return nil, errEndpointExists
	}

	// Create a veth pair.
	hostIfName := fmt.Sprintf("%s%s", hostInterfacePrefix, epInfo.Id[:7])
	contIfName := fmt.Sprintf("%s%s-2", hostInterfacePrefix, epInfo.Id[:7])

	log.Printf("[net] Creating veth pair %v %v.", hostIfName, contIfName)
	err = netlink.AddVethPair(contIfName, hostIfName)
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

	// Setup MAC address translation rules for container interface.
	log.Printf("[net] Setting up MAC address translation rules for endpoint %v.", contIfName)
	for _, ipAddr := range epInfo.IPAddresses {
		err = ebtables.SetDnatForIPAddress(ipAddr.IP, containerIf.HardwareAddr, ebtables.Append)
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

	nw.Endpoints[epInfo.Id] = ep

	log.Printf("[net] Created endpoint %+v.", ep)

	return ep, nil

cleanup:
	log.Printf("[net] Creating endpoint %v failed, err:%v.", contIfName, err)

	// Roll back the changes for the endpoint.
	netlink.DeleteLink(contIfName)

	return nil, err
}

// DeleteEndpoint deletes an existing endpoint from the network.
func (nw *network) deleteEndpoint(endpointId string) error {
	log.Printf("[net] Deleting endpoint %v from network %v.", endpointId, nw.Id)

	// Look up the endpoint.
	ep, err := nw.getEndpoint(endpointId)
	if err != nil {
		goto cleanup
	}

	// Delete the veth pair by deleting one of the peer interfaces.
	// Deleting the host interface is more convenient since it does not require
	// entering the container netns and hence works both for CNI and CNM.
	log.Printf("[net] Deleting veth pair %v %v.", ep.HostIfName, ep.IfName)
	err = netlink.DeleteLink(ep.HostIfName)
	if err != nil {
		goto cleanup
	}

	// Delete MAC address translation rule.
	log.Printf("[net] Deleting MAC address translation rules for endpoint %v.", endpointId)
	for _, ipAddr := range ep.IPAddresses {
		err = ebtables.SetDnatForIPAddress(ipAddr.IP, ep.MacAddress, ebtables.Delete)
		if err != nil {
			goto cleanup
		}
	}

	// Remove the endpoint object.
	delete(nw.Endpoints, endpointId)

	log.Printf("[net] Deleted endpoint %+v.", ep)

	return nil

cleanup:
	log.Printf("[net] Deleting endpoint %v failed, err:%v.", endpointId, err)

	return err
}

// GetEndpoint returns the endpoint with the given ID.
func (nw *network) getEndpoint(endpointId string) (*endpoint, error) {
	ep := nw.Endpoints[endpointId]

	if ep == nil {
		return nil, errEndpointNotFound
	}

	return ep, nil
}

//
// Endpoint
//

// GetInfo returns information about the endpoint.
func (ep *endpoint) getInfo() *EndpointInfo {
	info := &EndpointInfo{
		Id:          ep.Id,
		IPAddresses: ep.IPAddresses,
	}

	return info
}

// Attach attaches an endpoint to a sandbox.
func (ep *endpoint) attach(sandboxKey string, options map[string]interface{}) error {
	if ep.SandboxKey != "" {
		return errEndpointInUse
	}

	ep.SandboxKey = sandboxKey

	log.Printf("[net] Attached endpoint %v to sandbox %v.", ep.Id, sandboxKey)

	return nil
}

// Detach detaches an endpoint from its sandbox.
func (ep *endpoint) detach() error {
	if ep.SandboxKey == "" {
		return errEndpointNotInUse
	}

	log.Printf("[net] Detached endpoint %v from sandbox %v.", ep.Id, ep.SandboxKey)

	ep.SandboxKey = ""

	return nil
}
