// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package network

import (
	"net"

	"github.com/Azure/azure-container-networking/log"
)

// Endpoint represents a container network interface.
type endpoint struct {
	Id          string
	HnsId       string `json:",omitempty"`
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
	ContainerID string
	NetNsPath   string
	IfName      string
	IPAddresses []net.IPNet
	Routes      []RouteInfo
	DNS         DNSInfo
}

// RouteInfo contains information about an IP route.
type RouteInfo struct {
	Dst net.IPNet
	Gw  net.IP
}

// NewEndpoint creates a new endpoint in the network.
func (nw *network) newEndpoint(epInfo *EndpointInfo) (*endpoint, error) {
	var ep *endpoint
	var err error

	log.Printf("[net] Creating endpoint %+v in network %v.", epInfo, nw.Id)

	if nw.Endpoints[epInfo.Id] != nil {
		err = errEndpointExists
		goto fail
	}

	// Call the platform implementation.
	ep, err = nw.newEndpointImpl(epInfo)
	if err != nil {
		goto fail
	}

	nw.Endpoints[epInfo.Id] = ep

	log.Printf("[net] Created endpoint %+v.", ep)

	return ep, nil

fail:
	log.Printf("[net] Creating endpoint %v failed, err:%v.", epInfo.Id, err)

	return nil, err
}

// DeleteEndpoint deletes an existing endpoint from the network.
func (nw *network) deleteEndpoint(endpointId string) error {
	log.Printf("[net] Deleting endpoint %v from network %v.", endpointId, nw.Id)

	// Look up the endpoint.
	ep, err := nw.getEndpoint(endpointId)
	if err != nil {
		goto fail
	}

	// Call the platform implementation.
	err = nw.deleteEndpointImpl(ep)
	if err != nil {
		goto fail
	}

	// Remove the endpoint object.
	delete(nw.Endpoints, endpointId)

	log.Printf("[net] Deleted endpoint %+v.", ep)

	return nil

fail:
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
func (ep *endpoint) attach(sandboxKey string) error {
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
