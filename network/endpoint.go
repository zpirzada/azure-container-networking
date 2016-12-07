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
	IPv4Address net.IPNet
	IPv6Address net.IPNet
	IPv4Gateway net.IP
	IPv6Gateway net.IP
}

// EndpointInfo contains read-only information about an endpoint.
type EndpointInfo struct {
	Id          string
	IfName      string
	IPv4Address string
	NetNsPath   string
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

	// Parse IP address.
	ipAddr, ipNet, err := net.ParseCIDR(epInfo.IPv4Address)
	ipNet.IP = ipAddr
	if err != nil {
		return nil, err
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
	err = ebtables.SetupDnatBasedOnIPV4Address(ipAddr.String(), containerIf.HardwareAddr.String())
	if err != nil {
		goto cleanup
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
	log.Printf("[net] Adding IP address %v to link %v.", ipAddr, contIfName)
	err = netlink.AddIpAddress(contIfName, ipAddr, ipNet)
	if err != nil {
		goto cleanup
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
		IPv4Address: *ipNet,
		IPv6Address: net.IPNet{},
		IPv4Gateway: nw.extIf.IPv4Gateway,
		IPv6Gateway: nw.extIf.IPv6Gateway,
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
	log.Printf("[net] Deleting MAC address translation rule for endpoint %v.", endpointId)
	err = ebtables.RemoveDnatBasedOnIPV4Address(ep.IPv4Address.IP.String(), ep.MacAddress.String())
	if err != nil {
		goto cleanup
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
