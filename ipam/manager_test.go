// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"fmt"
	"net"
	"testing"

	"github.com/Azure/azure-container-networking/common"
)

var (
	anyInterface = "any"
	anyPriority  = 42

	// Pools and addresses used by tests.
	subnet1 = net.IPNet{IP: net.IPv4(10, 0, 1, 0), Mask: net.IPv4Mask(255, 255, 255, 0)}
	addr11  = net.IPv4(10, 0, 1, 1)
	addr12  = net.IPv4(10, 0, 1, 2)
	addr13  = net.IPv4(10, 0, 1, 3)

	subnet2 = net.IPNet{IP: net.IPv4(10, 0, 2, 0), Mask: net.IPv4Mask(255, 255, 255, 0)}
	addr21  = net.IPv4(10, 0, 2, 1)
	addr22  = net.IPv4(10, 0, 2, 2)
	addr23  = net.IPv4(10, 0, 2, 3)

	subnet3 = net.IPNet{IP: net.IPv4(10, 0, 3, 0), Mask: net.IPv4Mask(255, 255, 255, 0)}
	addr31  = net.IPv4(10, 0, 3, 1)
	addr32  = net.IPv4(10, 0, 3, 2)
	addr33  = net.IPv4(10, 0, 3, 3)
)

// createAddressManager creates an address manager with a simple test configuration.
func createAddressManager() (AddressManager, error) {
	var config common.PluginConfig
	var options map[string]interface{}

	am, err := NewAddressManager()
	if err != nil {
		return nil, err
	}

	err = am.Initialize(&config, options)
	if err != nil {
		return nil, err
	}

	err = setupTestAddressSpace(am)
	if err != nil {
		return nil, err
	}

	return am, nil
}

// dumpAddressManager dumps the contents of an address manager.
func dumpAddressManager(am AddressManager) {
	amImpl := am.(*addressManager)
	fmt.Printf("AddressManager:%+v\n", amImpl)
	for sk, sv := range amImpl.AddrSpaces {
		fmt.Printf("AddressSpace %v:%+v\n", sk, sv)
		for pk, pv := range sv.Pools {
			fmt.Printf("\tPool %v:%+v\n", pk, pv)
			for ak, av := range pv.Addresses {
				fmt.Printf("\t\tAddress %v:%+v\n", ak, av)
			}
		}
	}
}

// setupTestAddressSpace creates a simple address space used by various tests.
func setupTestAddressSpace(am AddressManager) error {
	var anyInterface = "any"
	var anyPriority = 42

	amImpl := am.(*addressManager)

	// Configure an empty global address space.
	globalAs, err := amImpl.newAddressSpace(globalDefaultAddressSpaceId, globalScope)
	if err != nil {
		return err
	}

	err = amImpl.setAddressSpace(globalAs)
	if err != nil {
		return err
	}

	// Configure a local address space.
	localAs, err := amImpl.newAddressSpace(localDefaultAddressSpaceId, localScope)
	if err != nil {
		return err
	}

	// Add subnet1 with addresses addr11 and addr12.
	ap, err := localAs.newAddressPool(anyInterface, anyPriority, &subnet1)
	ap.newAddressRecord(&addr11)
	ap.newAddressRecord(&addr12)

	// Add subnet2 with addr21.
	ap, err = localAs.newAddressPool(anyInterface, anyPriority, &subnet2)
	ap.newAddressRecord(&addr21)

	amImpl.setAddressSpace(localAs)

	return nil
}

// cleanupTestAddressSpace deletes any existing address spaces.
func cleanupTestAddressSpace(am AddressManager) error {
	amImpl := am.(*addressManager)

	// Configure an empty local address space.
	localAs, err := amImpl.newAddressSpace(localDefaultAddressSpaceId, localScope)
	if err != nil {
		return err
	}

	err = amImpl.setAddressSpace(localAs)
	if err != nil {
		return err
	}

	// Configure an empty global address space.
	globalAs, err := amImpl.newAddressSpace(globalDefaultAddressSpaceId, globalScope)
	if err != nil {
		return err
	}

	err = amImpl.setAddressSpace(globalAs)
	if err != nil {
		return err
	}

	return nil
}

//
// Address manager tests.
//

// Tests address spaces are created and queried correctly.
func TestAddressSpaceCreateAndGet(t *testing.T) {
	// Start with the test address space.
	am, err := createAddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Test if the address spaces are returned correctly.
	local, global := am.GetDefaultAddressSpaces()

	if local != localDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid local address space.")
	}

	if global != globalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid global address space.")
	}
}

// Tests updating an existing address space adds new resources and removes stale ones.
func TestAddressSpaceUpdate(t *testing.T) {
	// Start with the test address space.
	am, err := createAddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}
	amImpl := am.(*addressManager)

	// Create a new local address space to update the existing one.
	localAs, err := amImpl.newAddressSpace(localDefaultAddressSpaceId, localScope)
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
	localAs, err = amImpl.getAddressSpace(localDefaultAddressSpaceId)
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
	am, err := createAddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request two separate address pools.
	poolId1, subnet1, err := am.RequestPool(localDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	poolId2, subnet2, err := am.RequestPool(localDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	// Test the poolIds and subnets do not match.
	if poolId1 == poolId2 || subnet1 == subnet2 {
		t.Errorf("Pool requests returned the same pool.")
	}

	// Release the address pools.
	err = am.ReleasePool(localDefaultAddressSpaceId, poolId1)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	err = am.ReleasePool(localDefaultAddressSpaceId, poolId2)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}
}

// Tests multiple identical address pool requests return the same pool and pools are referenced correctly.
func TestAddressPoolRequestsForSamePool(t *testing.T) {
	// Start with the test address space.
	am, err := createAddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request the same address pool twice.
	poolId1, subnet1, err := am.RequestPool(localDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	poolId2, subnet2, err := am.RequestPool(localDefaultAddressSpaceId, poolId1, "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	// Test the subnets do not match.
	if poolId1 != poolId2 || subnet1 != subnet2 {
		t.Errorf("Pool requests returned different pools.")
	}

	// Release the address pools.
	err = am.ReleasePool(localDefaultAddressSpaceId, poolId1)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	err = am.ReleasePool(localDefaultAddressSpaceId, poolId2)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}

	// Third release should fail.
	err = am.ReleasePool(localDefaultAddressSpaceId, poolId1)
	if err == nil {
		t.Errorf("ReleasePool succeeded extra, err:%v", err)
	}
}

// Tests address requests from the same pool return separate addresses and releases work correctly.
func TestAddressRequestsFromTheSamePool(t *testing.T) {
	// Start with the test address space.
	am, err := createAddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	// Request a pool.
	poolId, _, err := am.RequestPool(localDefaultAddressSpaceId, "", "", nil, false)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	// Request two addresses from the pool.
	address1, err := am.RequestAddress(localDefaultAddressSpaceId, poolId, "", nil)
	if err != nil {
		t.Errorf("RequestAddress failed, err:%v", err)
	}

	addr, _, _ := net.ParseCIDR(address1)
	address1 = addr.String()

	address2, err := am.RequestAddress(localDefaultAddressSpaceId, poolId, "", nil)
	if err != nil {
		t.Errorf("RequestAddress failed, err:%v", err)
	}

	addr, _, _ = net.ParseCIDR(address2)
	address2 = addr.String()

	// Test the addresses do not match.
	if address1 == address2 {
		t.Errorf("Address requests returned the same address %v.", address1)
	}

	// Release addresses and the pool.
	err = am.ReleaseAddress(localDefaultAddressSpaceId, poolId, address1)
	if err != nil {
		t.Errorf("ReleaseAddress failed, err:%v", err)
	}

	err = am.ReleaseAddress(localDefaultAddressSpaceId, poolId, address2)
	if err != nil {
		t.Errorf("ReleaseAddress failed, err:%v", err)
	}

	err = am.ReleasePool(localDefaultAddressSpaceId, poolId)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}
}
