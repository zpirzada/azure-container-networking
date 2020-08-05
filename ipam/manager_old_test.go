// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"github.com/google/uuid"
	"net"
	"testing"
)

//
// Address manager tests.
//

// Tests address spaces are created and queried correctly.
func TestAddressSpaceCreateAndGet(t *testing.T) {
	// Start with the test address space.
	var options map[string]interface{}

	am, err := createAddressManager(options)
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Test if the address spaces are returned correctly.
	local, global := am.GetDefaultAddressSpaces()

	if local != LocalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid local address space.")
	}

	if global != GlobalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid global address space.")
	}
}

// Tests updating an existing address space adds new resources and removes stale ones.
func TestAddressSpaceUpdate(t *testing.T) {
	// Start with the test address space.
	var options map[string]interface{}

	am, err := createAddressManager(options)
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}
	amImpl := am.(*addressManager)

	// Create a new local address space to update the existing one.
	localAs, err := amImpl.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		t.Errorf("newAddressSpace failed, err:%+v.", err)
	}

	// Remove addr12 and add addr13 in subnet1.
	ap, err := localAs.newAddressPool(anyInterface, anyPriority, &subnet1)
	ap.newAddressRecord(&addr11)
	ap.newAddressRecord(&addr13)

	// Remove subnet2.
	// Add subnet3 with addr31.
	ap, err = localAs.newAddressPool(anyInterface, anyPriority, &subnet3)
	ap.newAddressRecord(&addr31)

	err = amImpl.setAddressSpace(localAs)
	if err != nil {
		t.Errorf("setAddressSpace failed, err:%+v.", err)
	}

	// Test that the address space was updated correctly.
	localAs, err = amImpl.getAddressSpace(LocalDefaultAddressSpaceId)
	if err != nil {
		t.Errorf("getAddressSpace failed, err:%+v.", err)
	}

	// Subnet1 should have addr11 and addr13, but not addr12.
	ap, err = localAs.getAddressPool(subnet1.String())
	if err != nil {
		t.Errorf("Cannot find subnet1, err:%+v.", err)
	}

	_, err = ap.requestAddress(addr11.String(), nil)
	if err != nil {
		t.Errorf("Cannot find addr11, err:%+v.", err)
	}

	_, err = ap.requestAddress(addr12.String(), nil)
	if err == nil {
		t.Errorf("Found addr12.")
	}

	_, err = ap.requestAddress(addr13.String(), nil)
	if err != nil {
		t.Errorf("Cannot find addr13, err:%+v.", err)
	}

	// Subnet2 should not exist.
	ap, err = localAs.getAddressPool(subnet2.String())
	if err == nil {
		t.Errorf("Found subnet2.")
	}

	// Subnet3 should have addr31 only.
	ap, err = localAs.getAddressPool(subnet3.String())
	if err != nil {
		t.Errorf("Cannot find subnet3, err:%+v.", err)
	}

	_, err = ap.requestAddress(addr31.String(), nil)
	if err != nil {
		t.Errorf("Cannot find addr31, err:%+v.", err)
	}

	_, err = ap.requestAddress(addr32.String(), nil)
	if err == nil {
		t.Errorf("Found addr32.")
	}
}

// Tests multiple wildcard address pool requests return separate pools.
func TestAddressPoolRequestsForSeparatePools(t *testing.T) {
	// Start with the test address space.
	var options map[string]interface{}
	am, err := createAddressManager(options)
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request two separate address pools.
	poolId1, subnet1, err := am.RequestPool(LocalDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	poolId2, subnet2, err := am.RequestPool(LocalDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	// Test the poolIds and subnets do not match.
	if poolId1 == poolId2 || subnet1 == subnet2 {
		t.Errorf("Pool requests returned the same pool.")
	}

	// Release the address pools.
	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId1)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId2)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}
}

// Tests multiple identical address pool requests return the same pool and pools are referenced correctly.
func TestAddressPoolRequestsForSamePool(t *testing.T) {
	// Start with the test address space.
	var options map[string]interface{}

	am, err := createAddressManager(options)
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request the same address pool twice.
	poolId1, subnet1, err := am.RequestPool(LocalDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	poolId2, subnet2, err := am.RequestPool(LocalDefaultAddressSpaceId, poolId1, "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	// Test the subnets do not match.
	if poolId1 != poolId2 || subnet1 != subnet2 {
		t.Errorf("Pool requests returned different pools.")
	}

	// Release the address pools.
	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId1)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId2)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	// Third release should fail.
	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId1)
	if err == nil {
		t.Errorf("ReleasePool succeeded extra, err:%v", err)
	}
}

// Tests address requests from the same pool return separate addresses and releases work correctly.
func TestAddressRequestsFromTheSamePool(t *testing.T) {
	// Start with the test address space.
	var options map[string]interface{}

	am, err := createAddressManager(options)
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request a pool.
	poolId, _, err := am.RequestPool(LocalDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	options1 := make(map[string]string)
	options1[OptAddressID] = uuid.New().String()

	options2 := make(map[string]string)
	options2[OptAddressID] = uuid.New().String()

	options3 := make(map[string]string)
	options3[OptAddressID] = uuid.New().String()

	// Request two addresses from the pool.
	address1, err := am.RequestAddress(LocalDefaultAddressSpaceId, poolId, "", options1)
	if err != nil {
		t.Errorf("RequestAddress failed, err:%v", err)
	}

	addr, _, _ := net.ParseCIDR(address1)
	address1 = addr.String()

	address2, err := am.RequestAddress(LocalDefaultAddressSpaceId, poolId, "", options2)
	if err != nil {
		t.Errorf("RequestAddress failed, err:%v", err)
	}

	addr, _, _ = net.ParseCIDR(address2)
	address2 = addr.String()

	// Request four addresses from the pool.
	address3, err := am.RequestAddress(LocalDefaultAddressSpaceId, poolId, "", options3)
	if err != nil {
		t.Errorf("RequestAddress failed, err:%v", err)
	}

	addr, _, _ = net.ParseCIDR(address3)
	address3 = addr.String()

	var m map[string]string
	_, exists := m[address1]
	_, exists = m[address2]
	_, exists = m[address3]

	// Test the addresses do not match.
	if exists {
		t.Errorf("Address requests returned the same address %v.", address1)
	}

	// Release addresses and the pool.
	err = am.ReleaseAddress(LocalDefaultAddressSpaceId, poolId, address1, options1)
	if err != nil {
		t.Errorf("ReleaseAddress failed, err:%v", err)
	}

	err = am.ReleaseAddress(LocalDefaultAddressSpaceId, poolId, address2, options2)
	if err != nil {
		t.Errorf("ReleaseAddress failed, err:%v", err)
	}

	// Release addresses and the pool.
	err = am.ReleaseAddress(LocalDefaultAddressSpaceId, poolId, "", options3)
	if err != nil {
		t.Errorf("ReleaseAddress failed, err:%v", err)
	}

	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolId)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}
}
