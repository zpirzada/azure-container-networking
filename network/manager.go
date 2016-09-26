// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"sync"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/log"
	"github.com/Azure/Aqua/store"
)

const (
	// Network store key.
	storeKey = "Network"
)

// NetworkManager manages the set of container networking resources.
type networkManager struct {
	ExternalInterfaces map[string]*externalInterface
	store              store.KeyValueStore
	sync.Mutex
}

// NetworkManager API.
type NetApi interface {
	AddExternalInterface(ifName string, subnet string) error
}

// Creates a new network manager.
func newNetworkManager() (*networkManager, error) {
	nm := &networkManager{
		ExternalInterfaces: make(map[string]*externalInterface),
	}

	return nm, nil
}

// Initialize configures network manager.
func (nm *networkManager) Initialize(config *common.PluginConfig) error {
	nm.store = config.Store

	// Restore persisted state.
	err := nm.restore()
	return err
}

// Uninitialize cleans up network manager.
func (nm *networkManager) Uninitialize() {
}

// Restore reads network manager state from persistent store.
func (nm *networkManager) restore() error {
	// Read any persisted state.
	err := nm.store.Read(storeKey, nm)
	if err != nil {
		if err == store.ErrKeyNotFound {
			// Considered successful.
			return nil
		} else {
			log.Printf("[net] Failed to restore state, err:%v\n", err)
			return err
		}
	}

	// Populate pointers.
	for _, extIf := range nm.ExternalInterfaces {
		for _, nw := range extIf.Networks {
			nw.extIf = extIf
		}
	}

	log.Printf("[net] Restored state, %+v\n", nm)

	return nil
}

// Save writes network manager state to persistent store.
func (nm *networkManager) save() error {
	err := nm.store.Write(storeKey, nm)
	if err == nil {
		log.Printf("[net] Save succeeded.\n")
	} else {
		log.Printf("[net] Save failed, err:%v\n", err)
	}
	return err
}

//
// NetworkManager API
//
// Provides atomic stateful wrappers around core networking functionality.
//

// AddExternalInterface adds a host interface to the list of available external interfaces.
func (nm *networkManager) AddExternalInterface(ifName string, subnet string) error {
	nm.Lock()
	defer nm.Unlock()

	err := nm.newExternalInterface(ifName, subnet)
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}

// CreateNetwork creates a new container network.
func (nm *networkManager) CreateNetwork(networkId string, options map[string]interface{}, ipv4Data, ipv6Data []ipamData) error {
	nm.Lock()
	defer nm.Unlock()

	_, err := nm.newNetwork(networkId, options, ipv4Data, ipv6Data)
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}

// DeleteNetwork deletes an existing container network.
func (nm *networkManager) DeleteNetwork(networkId string) error {
	nm.Lock()
	defer nm.Unlock()

	err := nm.deleteNetwork(networkId)
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}

// CreateEndpoint creates a new container endpoint.
func (nm *networkManager) CreateEndpoint(networkId string, endpointId string, ipAddress string) error {
	nm.Lock()
	defer nm.Unlock()

	nw, err := nm.getNetwork(networkId)
	if err != nil {
		return err
	}

	_, err = nw.newEndpoint(endpointId, ipAddress)
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}

// DeleteEndpoint deletes an existing container endpoint.
func (nm *networkManager) DeleteEndpoint(networkId string, endpointId string) error {
	nm.Lock()
	defer nm.Unlock()

	nw, err := nm.getNetwork(networkId)
	if err != nil {
		return err
	}

	err = nw.deleteEndpoint(endpointId)
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}

// AttachEndpoint attaches an endpoint to a sandbox.
func (nm *networkManager) AttachEndpoint(networkId string, endpointId string, sandboxKey string) (*endpoint, error) {
	nm.Lock()
	defer nm.Unlock()

	nw, err := nm.getNetwork(networkId)
	if err != nil {
		return nil, err
	}

	ep, err := nw.getEndpoint(endpointId)
	if err != nil {
		return nil, err
	}

	err = ep.attach(sandboxKey, nil)
	if err != nil {
		return nil, err
	}

	err = nm.save()
	if err != nil {
		return nil, err
	}

	return ep, nil
}

// DetachEndpoint detaches an endpoint from its sandbox.
func (nm *networkManager) DetachEndpoint(networkId string, endpointId string) error {
	nm.Lock()
	defer nm.Unlock()

	nw, err := nm.getNetwork(networkId)
	if err != nil {
		return err
	}

	ep, err := nw.getEndpoint(endpointId)
	if err != nil {
		return err
	}

	err = ep.detach()
	if err != nil {
		return err
	}

	err = nm.save()
	if err != nil {
		return err
	}

	return nil
}
