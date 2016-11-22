// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"fmt"
	"net"
	"strings"
)

const (
	// Default address space IDs.
	localDefaultAddressSpaceId  = "LocalDefaultAddressSpace"
	globalDefaultAddressSpaceId = "GlobalDefaultAddressSpace"

	// Address space scopes.
	localScope  = "local"
	globalScope = "global"
)

// Represents the key to an address pool.
type addressPoolId struct {
	AsId        string
	Subnet      string
	ChildSubnet string
}

// Represents a set of non-overlapping address pools.
type addressSpace struct {
	Id    string
	Scope string
	Pools map[string]*addressPool
	epoch int
}

// Represents a subnet and the set of addresses in it.
type addressPool struct {
	as        *addressSpace
	Id        string
	IfName    string
	Subnet    net.IPNet
	Addresses map[string]*addressRecord
	IsIPv6    bool
	Priority  int
	InUse     bool
	epoch     int
}

// Represents an IP address in a pool.
type addressRecord struct {
	Addr  net.IP
	InUse bool
	epoch int
}

//
// AddressPoolId
//

// Creates a new address pool ID object.
func NewAddressPoolId(asId string, subnet string, childSubnet string) *addressPoolId {
	return &addressPoolId{
		AsId:        asId,
		Subnet:      subnet,
		ChildSubnet: childSubnet,
	}
}

// Creates a new pool ID from a string representation.
func NewAddressPoolIdFromString(s string) (*addressPoolId, error) {
	var pid addressPoolId

	p := strings.Split(s, "|")
	if len(p) > 3 {
		return nil, errInvalidPoolId
	}

	pid.AsId = p[0]
	if len(p) >= 2 {
		pid.Subnet = p[1]
	}
	if len(p) == 3 {
		pid.ChildSubnet = p[2]
	}

	return &pid, nil
}

// Returns the string representation of a pool ID.
func (pid *addressPoolId) String() string {
	s := fmt.Sprintf("%s|%s", pid.AsId, pid.Subnet)
	if pid.ChildSubnet != "" {
		s = fmt.Sprintf("%s|%s", s, pid.ChildSubnet)
	}
	return s
}

//
// AddressSpace
//

// Creates a new addressSpace object.
func (am *addressManager) newAddressSpace(id string, scope string) (*addressSpace, error) {
	if scope != localScope && scope != globalScope {
		return nil, errInvalidScope
	}

	return &addressSpace{
		Id:    id,
		Scope: scope,
		Pools: make(map[string]*addressPool),
	}, nil
}

// Returns the address space with the given ID.
func (am *addressManager) getAddressSpace(id string) (*addressSpace, error) {
	as := am.AddrSpaces[id]
	if as == nil {
		return nil, errInvalidAddressSpace
	}

	return as, nil
}

// Sets a new or updates an existing address space.
func (am *addressManager) setAddressSpace(as *addressSpace) error {
	as1, ok := am.AddrSpaces[as.Id]
	if !ok {
		am.AddrSpaces[as.Id] = as
	} else {
		as1.merge(as)
	}

	// Notify NetPlugin of external interfaces.
	if am.netApi != nil {
		for _, ap := range as.Pools {
			am.netApi.AddExternalInterface(ap.IfName, ap.Subnet.String())
		}
	}

	am.save()

	return nil
}

// Merges a new address space to an existing one.
func (as *addressSpace) merge(newas *addressSpace) {
	// The new epoch after the merge.
	as.epoch++

	// Add new pools and addresses.
	for pk, pv := range newas.Pools {
		ap := as.Pools[pk]

		if ap == nil {
			// This is a new address pool.
			// Merge it to the existing address space.
			as.Pools[pk] = pv
			delete(newas.Pools, pk)
			pv.epoch = as.epoch
		} else {
			// This pool already exists.
			// Compare address records one by one.
			for ak, av := range pv.Addresses {
				ar := ap.Addresses[ak]

				if ar == nil {
					// This is a new address record.
					// Merge it to the existing address pool.
					ap.Addresses[ak] = av
					delete(ap.Addresses, ak)
					av.epoch = as.epoch
				} else {
					// This address record already exists.
					ar.epoch = as.epoch
				}
			}

			ap.epoch = as.epoch
		}
	}

	// Cleanup stale pools and addresses from the old epoch.
	// Those currently in use will be deleted after they are released.
	for pk, pv := range as.Pools {
		if pv.epoch < as.epoch {
			for ak, av := range pv.Addresses {
				if !av.InUse {
					delete(pv.Addresses, ak)
				}
			}

			if !pv.InUse {
				delete(as.Pools, pk)
			}
		}
	}

	return
}

// Creates a new addressPool object.
func (as *addressSpace) newAddressPool(ifName string, priority int, subnet *net.IPNet) (*addressPool, error) {
	id := subnet.String()

	pool, ok := as.Pools[id]
	if ok {
		return pool, errAddressPoolExists
	}

	v6 := (subnet.IP.To4() == nil)

	pool = &addressPool{
		as:        as,
		Id:        id,
		IfName:    ifName,
		Subnet:    *subnet,
		Addresses: make(map[string]*addressRecord),
		IsIPv6:    v6,
		Priority:  priority,
		epoch:     as.epoch,
	}

	as.Pools[id] = pool

	return pool, nil
}

// Returns the address pool with the given pool ID.
func (as *addressSpace) getAddressPool(poolId string) (*addressPool, error) {
	ap := as.Pools[poolId]
	if ap == nil {
		return nil, errInvalidPoolId
	}

	return ap, nil
}

// Requests a new address pool from the address space.
func (as *addressSpace) requestPool(poolId string, subPoolId string, options map[string]string, v6 bool) (*addressPool, error) {
	var ap *addressPool

	if poolId != "" {
		// Return the specific address pool requested.
		ap = as.Pools[poolId]
		if ap == nil {
			return nil, errAddressPoolNotFound
		}

		// Fail if requested pool is already in use.
		if ap.InUse {
			return nil, errAddressPoolInUse
		}
	} else {
		// Return any available address pool.
		highestPriority := -1

		for _, pool := range as.Pools {
			// Skip if pool is already in use.
			if pool.InUse {
				continue
			}

			// Pick a pool from the same address family.
			if pool.IsIPv6 != v6 {
				continue
			}

			// Pick the pool with the highest priority.
			if pool.Priority > highestPriority {
				highestPriority = pool.Priority
				ap = pool
			}
		}

		if ap == nil {
			return nil, errNoAvailableAddressPools
		}
	}

	ap.InUse = true

	return ap, nil
}

// Releases a previously requested address pool back to its address space.
func (as *addressSpace) releasePool(poolId string) error {
	ap, ok := as.Pools[poolId]
	if !ok {
		return errAddressPoolNotFound
	}

	if !ap.InUse {
		return errAddressPoolNotInUse
	}

	ap.InUse = false

	// Delete address pool if it is no longer available.
	if ap.epoch < as.epoch {
		delete(as.Pools, poolId)
	}

	return nil
}

//
// AddressPool
//

// Creates a new addressRecord object.
func (ap *addressPool) newAddressRecord(addr *net.IP) (*addressRecord, error) {
	id := addr.String()

	if !ap.Subnet.Contains(*addr) {
		return nil, errInvalidAddress
	}

	ar, ok := ap.Addresses[id]
	if ok {
		return ar, errAddressExists
	}

	ar = &addressRecord{
		Addr:  *addr,
		epoch: ap.epoch,
	}

	ap.Addresses[id] = ar

	return ar, nil
}

// Requests a new address from the address pool.
func (ap *addressPool) requestAddress(address string, options map[string]string) (string, error) {
	var ar *addressRecord

	if address != "" {
		// Return the specific address requested.
		ar = ap.Addresses[address]
		if ar == nil {
			return "", errAddressNotFound
		}
		if ar.InUse {
			return "", errAddressInUse
		}
	} else {
		// Return any available address.
		for _, ar = range ap.Addresses {
			if !ar.InUse {
				break
			}
		}

		if ar == nil {
			return "", errNoAvailableAddresses
		}
	}

	ar.InUse = true

	// Return address in CIDR notation.
	addr := net.IPNet{
		IP:   ar.Addr,
		Mask: ap.Subnet.Mask,
	}

	return addr.String(), nil
}

// Releases a previously requested address back to its address pool.
func (ap *addressPool) releaseAddress(address string) error {
	ar := ap.Addresses[address]
	if ar == nil {
		return errAddressNotFound
	}
	if !ar.InUse {
		return errAddressNotInUse
	}

	ar.InUse = false

	// Delete address record if it is no longer available.
	if ar.epoch < ap.as.epoch {
		delete(ap.Addresses, address)
	}

	return nil
}
