// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package network

import (
	"net"

	"github.com/Azure/azure-container-networking/log"
)

const (
	// Operational modes.
	OpModeBridge = "bridge"
)

// ExternalInterface is a host network interface that bridges containers to external networks.
type externalInterface struct {
	Name        string
	Networks    map[string]*network
	Subnets     []string
	BridgeName  string
	MacAddress  net.HardwareAddr
	IPAddresses []*net.IPNet
	Routes      []*route
	IPv4Gateway net.IP
	IPv6Gateway net.IP
}

// A container network is a set of endpoints allowed to communicate with each other.
type network struct {
	Id        string
	HnsId     string `json:",omitempty"`
	Mode      string
	Endpoints map[string]*endpoint
	extIf     *externalInterface
}

// NetworkInfo contains read-only information about a container network.
type NetworkInfo struct {
	Id         string
	Mode       string
	Subnets    []string
	BridgeName string
	Options    map[string]interface{}
}

// NewExternalInterface adds a host interface to the list of available external interfaces.
func (nm *networkManager) newExternalInterface(ifName string, subnet string) error {
	// Check whether the external interface is already configured.
	if nm.ExternalInterfaces[ifName] != nil {
		return nil
	}

	// Find the host interface.
	hostIf, err := net.InterfaceByName(ifName)
	if err != nil {
		return err
	}

	extIf := externalInterface{
		Name:        ifName,
		Networks:    make(map[string]*network),
		MacAddress:  hostIf.HardwareAddr,
		IPv4Gateway: net.IPv4zero,
		IPv6Gateway: net.IPv6unspecified,
	}

	extIf.Subnets = append(extIf.Subnets, subnet)

	nm.ExternalInterfaces[ifName] = &extIf

	log.Printf("[net] Added ExternalInterface %v for subnet %v.", ifName, subnet)

	return nil
}

// DeleteExternalInterface removes an interface from the list of available external interfaces.
func (nm *networkManager) deleteExternalInterface(ifName string) error {
	delete(nm.ExternalInterfaces, ifName)

	log.Printf("[net] Deleted ExternalInterface %v.", ifName)

	return nil
}

// FindExternalInterfaceBySubnet finds an external interface connected to the given subnet.
func (nm *networkManager) findExternalInterfaceBySubnet(subnet string) *externalInterface {
	for _, extIf := range nm.ExternalInterfaces {
		for _, s := range extIf.Subnets {
			if s == subnet {
				return extIf
			}
		}
	}

	return nil
}

// NewNetwork creates a new container network.
func (nm *networkManager) newNetwork(nwInfo *NetworkInfo) (*network, error) {
	var nw *network
	var err error

	log.Printf("[net] Creating network %+v.", nwInfo)

	// Find the external interface for this subnet.
	extIf := nm.findExternalInterfaceBySubnet(nwInfo.Subnets[0])
	if extIf == nil {
		err = errSubnetNotFound
		goto fail
	}

	// Make sure this network does not already exist.
	if extIf.Networks[nwInfo.Id] != nil {
		err = errNetworkExists
		goto fail
	}

	// Call the OS-specific implementation.
	nw, err = nm.newNetworkImpl(nwInfo, extIf)
	if err != nil {
		goto fail
	}

	// Add the network object.
	extIf.Networks[nwInfo.Id] = nw

	log.Printf("[net] Created network %v on interface %v.", nwInfo.Id, extIf.Name)
	return nw, nil

fail:
	log.Printf("[net] Failed to create network %v, err:%v.", nwInfo.Id, err)
	return nil, err
}

// DeleteNetwork deletes an existing container network.
func (nm *networkManager) deleteNetwork(networkId string) error {
	log.Printf("[net] Deleting network %v.", networkId)

	// Find the network.
	nw, err := nm.getNetwork(networkId)
	if err != nil {
		goto fail
	}

	// Call the OS-specific implementation.
	err = nm.deleteNetworkImpl(nw)
	if err != nil {
		goto fail
	}

	// Remove the network object.
	delete(nw.extIf.Networks, networkId)

	log.Printf("[net] Deleted network %+v.", nw)
	return nil

fail:
	log.Printf("[net] Failed to delete network %v, err:%v.", networkId, err)
	return err
}

// GetNetwork returns the network with the given ID.
func (nm *networkManager) getNetwork(networkId string) (*network, error) {
	for _, extIf := range nm.ExternalInterfaces {
		nw, ok := extIf.Networks[networkId]
		if ok {
			return nw, nil
		}
	}

	return nil, errNetworkNotFound
}
