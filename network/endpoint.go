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
	SrcName     string
	DstPrefix   string
	MacAddress  net.HardwareAddr
	IPv4Address net.IPNet
	IPv6Address net.IPNet
	IPv4Gateway net.IP
	IPv6Gateway net.IP
}

// NewEndpoint creates a new endpoint in the network.
func (nw *network) newEndpoint(endpointId string, ipAddress string) (*endpoint, error) {
	var containerIf *net.Interface
	var ep *endpoint
	var err error

	if nw.Endpoints[endpointId] != nil {
		return nil, errEndpointExists
	}

	// Parse IP address.
	ipAddr, ipNet, err := net.ParseCIDR(ipAddress)
	ipNet.IP = ipAddr
	if err != nil {
		return nil, err
	}

	log.Printf("[net] Creating endpoint %v in network %v.", endpointId, nw.Id)

	// Create a veth pair.
	hostIfName := fmt.Sprintf("%s%s", hostInterfacePrefix, endpointId[:7])
	contIfName := fmt.Sprintf("%s%s-2", hostInterfacePrefix, endpointId[:7])

	log.Printf("[net] Creating veth pair %v %v.", hostIfName, contIfName)
	err = netlink.AddVethPair(contIfName, hostIfName)
	if err != nil {
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
	err = ebtables.SetupDnatBasedOnIPV4Address(ipAddr.String(), containerIf.HardwareAddr.String())
	if err != nil {
		goto cleanup
	}

	// Assign IP address to container network interface.
	log.Printf("[net] Adding IP address %v to link %v.", ipAddr, contIfName)
	err = netlink.AddIpAddress(contIfName, ipAddr, ipNet)
	if err != nil {
		goto cleanup
	}

	// Create the endpoint object.
	ep = &endpoint{
		Id:          endpointId,
		SrcName:     contIfName,
		DstPrefix:   containerInterfacePrefix,
		MacAddress:  containerIf.HardwareAddr,
		IPv4Address: *ipNet,
		IPv6Address: net.IPNet{},
		IPv4Gateway: nw.extIf.IPv4Gateway,
		IPv6Gateway: nw.extIf.IPv6Gateway,
	}

	nw.Endpoints[endpointId] = ep

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
	ep, err := nw.getEndpoint(endpointId)
	if err != nil {
		return err
	}

	log.Printf("[net] Deleting endpoint %+v.", ep)

	// Delete veth pair.
	netlink.DeleteLink(ep.SrcName)

	// Cleanup MAC address translation rules.
	err = ebtables.RemoveDnatBasedOnIPV4Address(ep.IPv4Address.IP.String(), ep.MacAddress.String())

	// Remove the endpoint object.
	delete(nw.Endpoints, endpointId)

	log.Printf("[net] Deleted endpoint %+v.", ep)

	return nil
}

// GetEndpoint returns the endpoint with the given ID.
func (nw *network) getEndpoint(endpointId string) (*endpoint, error) {
	ep := nw.Endpoints[endpointId]

	if ep == nil {
		return nil, errEndpointNotFound
	}

	return ep, nil
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
