// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"sync"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/store"
)

const (
	// IPAM store key.
	storeKey = "IPAM"
)

// AddressManager manages the set of address spaces and pools allocated to containers.
type addressManager struct {
	AddrSpaces map[string]*addressSpace `json:"AddressSpaces"`
	store      store.KeyValueStore
	source     addressConfigSource
	netApi     network.NetApi
	sync.Mutex
}

// AddressConfigSource configures the address pools managed by AddressManager.
type addressConfigSource interface {
	start(sink addressConfigSink) error
	stop()
	refresh() error
}

// AddressConfigSink interface is used by AddressConfigSources to configure address pools.
type addressConfigSink interface {
	newAddressSpace(id string, scope string) (*addressSpace, error)
	setAddressSpace(*addressSpace) error
}

// Creates a new address manager.
func newAddressManager() (*addressManager, error) {
	am := &addressManager{
		AddrSpaces: make(map[string]*addressSpace),
	}

	return am, nil
}

// Initialize configures address manager.
func (am *addressManager) Initialize(config *common.PluginConfig, environment string) error {
	am.store = config.Store
	am.netApi, _ = config.NetApi.(network.NetApi)

	// Restore persisted state.
	err := am.restore()
	if err != nil {
		return err
	}

	// Start source.
	err = am.startSource(environment)

	return err
}

// Uninitialize cleans up address manager.
func (am *addressManager) Uninitialize() {
	am.stopSource()
}

// Restore reads address manager state from persistent store.
func (am *addressManager) restore() error {
	// Skip if a store is not provided.
	if am.store == nil {
		return nil
	}

	// Read any persisted state.
	err := am.store.Read(storeKey, am)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return nil
		} else {
			log.Printf("[ipam] Failed to restore state, err:%v\n", err)
			return err
		}
	}

	// Populate pointers.
	for _, as := range am.AddrSpaces {
		for _, ap := range as.Pools {
			ap.as = as
		}
	}

	log.Printf("[ipam] Restored state, %+v\n", am)

	return nil
}

// Save writes address manager state to persistent store.
func (am *addressManager) save() error {
	// Skip if a store is not provided.
	if am.store == nil {
		return nil
	}

	err := am.store.Write(storeKey, am)
	if err == nil {
		log.Printf("[ipam] Save succeeded.\n")
	} else {
		log.Printf("[ipam] Save failed, err:%v\n", err)
	}
	return err
}

// Starts configuration source.
func (am *addressManager) startSource(environment string) error {
	var err error

	switch environment {
	case common.OptEnvironmentAzure:
		am.source, err = newAzureSource()

	case common.OptEnvironmentMAS:
		am.source, err = newMasSource()

	case "null":
		am.source, err = newNullSource()

	case "":
		am.source = nil

	default:
		return errInvalidConfiguration
	}

	if am.source != nil {
		err = am.source.start(am)
	}

	return err
}

// Stops the configuration source.
func (am *addressManager) stopSource() {
	if am.source != nil {
		am.source.stop()
		am.source = nil
	}
}

// Signals configuration source to refresh.
func (am *addressManager) refreshSource() {
	if am.source != nil {
		err := am.source.refresh()
		if err != nil {
			log.Printf("[ipam] Source refresh failed, err:%v.\n", err)
		}
	}
}

//
// AddressManager API
//
// Provides atomic stateful wrappers around core IPAM functionality.
//

// GetDefaultAddressSpaces returns the default local and global address space IDs.
func (am *addressManager) GetDefaultAddressSpaces() (string, string) {
	var localId, globalId string

	am.Lock()
	defer am.Unlock()

	am.refreshSource()

	local := am.AddrSpaces[localDefaultAddressSpaceId]
	if local != nil {
		localId = local.Id
	}

	global := am.AddrSpaces[globalDefaultAddressSpaceId]
	if global != nil {
		globalId = global.Id
	}

	return localId, globalId
}

// RequestPool reserves an address pool.
func (am *addressManager) RequestPool(asId, poolId, subPoolId string, options map[string]string, v6 bool) (string, string, error) {
	am.Lock()
	defer am.Unlock()

	am.refreshSource()

	as, err := am.getAddressSpace(asId)
	if err != nil {
		return "", "", err
	}

	pool, err := as.requestPool(poolId, subPoolId, options, v6)
	if err != nil {
		return "", "", err
	}

	err = am.save()
	if err != nil {
		return "", "", err
	}

	return pool.Id, pool.Subnet.String(), nil
}

// ReleasePool releases a previously reserved address pool.
func (am *addressManager) ReleasePool(asId string, poolId string) error {
	am.Lock()
	defer am.Unlock()

	am.refreshSource()

	as, err := am.getAddressSpace(asId)
	if err != nil {
		return err
	}

	err = as.releasePool(poolId)
	if err != nil {
		return err
	}

	err = am.save()
	if err != nil {
		return err
	}

	return nil
}

// RequestAddress reserves a new address from the address pool.
func (am *addressManager) RequestAddress(asId, poolId, address string, options map[string]string) (string, error) {
	am.Lock()
	defer am.Unlock()

	am.refreshSource()

	as, err := am.getAddressSpace(asId)
	if err != nil {
		return "", err
	}

	ap, err := as.getAddressPool(poolId)
	if err != nil {
		return "", err
	}

	addr, err := ap.requestAddress(address, options)
	if err != nil {
		return "", err
	}

	err = am.save()
	if err != nil {
		return "", err
	}

	return addr, nil
}

// ReleaseAddress releases a previously reserved address.
func (am *addressManager) ReleaseAddress(asId string, poolId string, address string) error {
	am.Lock()
	defer am.Unlock()

	am.refreshSource()

	as, err := am.getAddressSpace(asId)
	if err != nil {
		return err
	}

	ap, err := as.getAddressPool(poolId)
	if err != nil {
		return err
	}

	err = ap.releaseAddress(address)
	if err != nil {
		return err
	}

	err = am.save()
	if err != nil {
		return err
	}

	return nil
}
