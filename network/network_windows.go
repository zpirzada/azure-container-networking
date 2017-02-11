// Copyright 2017 Microsoft. All rights reserved.
// MIT License

// +build windows

package network

import (
	"encoding/json"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Microsoft/hcsshim"
)

const (
	// HNS network types.
	HnsL2bridge = "l2bridge"
	HnsL2tunnel = "l2tunnel"
)

// Windows implementation of route.
type route interface{}

// NewNetworkImpl creates a new container network.
func (nm *networkManager) newNetworkImpl(nwInfo *NetworkInfo, extIf *externalInterface) (*network, error) {
	// Initialize HNS network.
	hnsNetwork := &hcsshim.HNSNetwork{
		Name:               nwInfo.Id,
		Type:               HnsL2bridge,
		NetworkAdapterName: extIf.Name,
		SourceMac:          extIf.MacAddress.String(),
		DNSSuffix:          "",
		DNSServerList:      "10.1.1.2",
	}

	// Populate subnets.
	for _, subnet := range nwInfo.Subnets {
		hnsSubnet := hcsshim.Subnet{
			AddressPrefix:  subnet,
			GatewayAddress: "10.1.1.1",
		}

		hnsNetwork.Subnets = append(hnsNetwork.Subnets, hnsSubnet)
	}

	// Marshal the request.
	buffer, err := json.Marshal(hnsNetwork)
	if err != nil {
		return nil, err
	}
	hnsRequest := string(buffer)

	// Create the HNS network.
	log.Printf("[net] HNSNetworkRequest POST request:%+v", hnsRequest)
	hnsResponse, err := hcsshim.HNSNetworkRequest("POST", "", hnsRequest)
	log.Printf("[net] HNSNetworkRequest POST response:%+v err:%v.", hnsResponse, err)
	if err != nil {
		return nil, err
	}

	// Create the network object.
	nw := &network{
		Id:        nwInfo.Id,
		HnsId:     hnsResponse.Id,
		Endpoints: make(map[string]*endpoint),
		extIf:     extIf,
	}

	return nw, nil
}

// DeleteNetworkImpl deletes an existing container network.
func (nm *networkManager) deleteNetworkImpl(nw *network) error {
	// Delete the HNS network.
	log.Printf("[net] HNSNetworkRequest DELETE id:%v", nw.HnsId)
	hnsResponse, err := hcsshim.HNSNetworkRequest("DELETE", nw.HnsId, "")
	log.Printf("[net] HNSNetworkRequest DELETE response:%+v err:%v.", hnsResponse, err)

	return err
}
