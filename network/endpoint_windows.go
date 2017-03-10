// Copyright 2017 Microsoft. All rights reserved.
// MIT License

// +build windows

package network

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Microsoft/hcsshim"
)

// newEndpointImpl creates a new endpoint in the network.
func (nw *network) newEndpointImpl(epInfo *EndpointInfo) (*endpoint, error) {
	// Initialize HNS endpoint.
	hnsEndpoint := &hcsshim.HNSEndpoint{
		Name:           epInfo.Id,
		VirtualNetwork: nw.HnsId,
		DNSSuffix:      epInfo.DNS.Suffix,
		DNSServerList:  strings.Join(epInfo.DNS.Servers, ","),
	}

	// HNS currently supports only one IP address per endpoint.
	if epInfo.IPAddresses != nil {
		hnsEndpoint.IPAddress = epInfo.IPAddresses[0].IP
		pl, _ := epInfo.IPAddresses[0].Mask.Size()
		hnsEndpoint.PrefixLength = uint8(pl)
	}

	// Marshal the request.
	buffer, err := json.Marshal(hnsEndpoint)
	if err != nil {
		return nil, err
	}
	hnsRequest := string(buffer)

	// Create the HNS endpoint.
	log.Printf("[net] HNSEndpointRequest POST request:%+v", hnsRequest)
	hnsResponse, err := hcsshim.HNSEndpointRequest("POST", "", hnsRequest)
	log.Printf("[net] HNSEndpointRequest POST response:%+v err:%v.", hnsResponse, err)
	if err != nil {
		return nil, err
	}

	// Attach the endpoint.
	log.Printf("[net] Attaching endpoint %v to container %v.", hnsResponse.Id, epInfo.ContainerID)
	err = hcsshim.HotAttachEndpoint(epInfo.ContainerID, hnsResponse.Id)
	if err != nil {
		log.Printf("[net] Failed to attach endpoint: %v.", err)
		return nil, err
	}

	// Create the endpoint object.
	ep := &endpoint{
		Id:          epInfo.Id,
		HnsId:       hnsResponse.Id,
		SandboxKey:  epInfo.ContainerID,
		IfName:      epInfo.IfName,
		IPAddresses: epInfo.IPAddresses,
		Gateways:    []net.IP{net.ParseIP(hnsEndpoint.GatewayAddress)},
	}

	ep.MacAddress, _ = net.ParseMAC(hnsResponse.MacAddress)

	return ep, nil
}

// deleteEndpointImpl deletes an existing endpoint from the network.
func (nw *network) deleteEndpointImpl(ep *endpoint) error {
	// Delete the HNS endpoint.
	log.Printf("[net] HNSEndpointRequest DELETE id:%v", ep.HnsId)
	hnsResponse, err := hcsshim.HNSEndpointRequest("DELETE", ep.HnsId, "")
	log.Printf("[net] HNSEndpointRequest DELETE response:%+v err:%v.", hnsResponse, err)

	return err
}
