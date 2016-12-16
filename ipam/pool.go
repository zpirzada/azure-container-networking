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

var (
	// Azure VNET well-known host IDs.
	defaultGatewayHostId = net.ParseIP("::1")
	dnsPrimaryHostId     = net.ParseIP("::2")
	dnsSecondaryHostId   = net.ParseIP("::3")
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
	RefCount  int
	epoch     int
}

// AddressPoolInfo contains information about an address pool.
type AddressPoolInfo struct {
	Subnet     net.IPNet
	Gateway    net.IP
	DnsServers []net.IP
	IsIPv6     bool
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
			pv.as = as
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
					av.epoch = as.epoch
				} else {
					// This address record already exists.
					ar.epoch = as.epoch
				}

				delete(pv.Addresses, ak)
			}

			pv.as = nil
		}

		delete(newas.Pools, pk)
	}

	// Cleanup stale pools and addresses from the old epoch.
	// Those currently in use will be deleted after they are released.
	for pk, pv := range as.Pools {
		if pv.epoch < as.epoch {
			// This pool may have stale addresses.
			for ak, av := range pv.Addresses {
				if av.epoch == as.epoch || av.InUse {
					// Pool has at least one valid or in-use address.
					pv.epoch = as.epoch
				} else {
					// This address is no longer available.
					delete(pv.Addresses, ak)
				}
			}

			// Delete the pool if it has no addresses left.
			if pv.epoch < as.epoch && !pv.isInUse() {
				pv.as = nil
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
		// Note sharing of pools is allowed when specifically requested.
		ap = as.Pools[poolId]
		if ap == nil {
			return nil, errAddressPoolNotFound
		}
	} else {
		// Return any available address pool.
		highestPriority := -1
		highestNumAddr := -1

		for _, pool := range as.Pools {
			// Skip if pool is already in use.
			if pool.isInUse() {
				continue
			}

			// Pick a pool from the same address family.
			if pool.IsIPv6 != v6 {
				continue
			}

			// Prefer the pool with the highest priority.
			if pool.Priority > highestPriority {
				highestPriority = pool.Priority
				ap = pool
			}

			// Prefer the pool with the highest number of addresses.
			if len(pool.Addresses) > highestNumAddr {
				highestNumAddr = len(pool.Addresses)
				ap = pool
			}
		}

		if ap == nil {
			return nil, errNoAvailableAddressPools
		}
	}

	ap.RefCount++

	return ap, nil
}

// Releases a previously requested address pool back to its address space.
func (as *addressSpace) releasePool(poolId string) error {
	ap, ok := as.Pools[poolId]
	if !ok {
		return errAddressPoolNotFound
	}

	if !ap.isInUse() {
		return errAddressPoolNotInUse
	}

	ap.RefCount--

	// Delete address pool if it is no longer available.
	if ap.epoch < as.epoch && !ap.isInUse() {
		delete(as.Pools, poolId)
	}

	return nil
}

//
// AddressPool
//

// Returns if an address pool is currently in use.
func (ap *addressPool) getInfo() *AddressPoolInfo {
	// Generate default gateway address from subnet.
	gateway := generateAddress(&ap.Subnet, defaultGatewayHostId)

	// Generate DNS server addresses from subnet.
	dnsPrimary := generateAddress(&ap.Subnet, dnsPrimaryHostId)
	dnsSecondary := generateAddress(&ap.Subnet, dnsSecondaryHostId)

	info := &AddressPoolInfo{
		Subnet:     ap.Subnet,
		Gateway:    gateway,
		DnsServers: []net.IP{dnsPrimary, dnsSecondary},
		IsIPv6:     ap.IsIPv6,
	}

	return info
}

// Returns if an address pool is currently in use.
func (ap *addressPool) isInUse() bool {
	return ap.RefCount > 0
}

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
			ar = nil
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
