// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/common"
)

var plugin IpamPlugin
var mux *http.ServeMux
var sink addressConfigSink

var local *addressSpace
var global *addressSpace

var poolId1 string
var address1 string

// Wraps the test run with plugin setup and teardown.
func TestMain(m *testing.M) {
	var config common.PluginConfig
	var err error

	// Create the plugin.
	plugin, err = NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin %v\n", err)
		return
	}

	// Configure test mode.
	plugin.(*ipamPlugin).Name = "test"

	// Start the plugin.
	err = plugin.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start IPAM plugin %v\n", err)
		return
	}

	// Get the internal http mux as test hook.
	mux = plugin.(*ipamPlugin).Listener.GetMux()

	// Get the internal config sink interface.
	sink = plugin.(*ipamPlugin).am.(*addressManager)

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	plugin.Stop()

	os.Exit(exitCode)
}

// Decodes plugin's responses to test requests.
func decodeResponse(w *httptest.ResponseRecorder, response interface{}) error {
	if w.Code != http.StatusOK {
		return fmt.Errorf("Request failed with HTTP error %s", w.Code)
	}

	if w.Body == nil {
		return fmt.Errorf("Response body is empty")
	}

	return json.NewDecoder(w.Body).Decode(&response)
}

//
// Libnetwork remote IPAM API compliance tests
// https://github.com/docker/libnetwork/blob/master/docs/ipam.md
//

// Tests Plugin.Activate functionality.
func TestActivate(t *testing.T) {
	fmt.Println("Test: Activate")

	var resp struct {
		Implements []string
	}

	req, err := http.NewRequest(http.MethodGet, "/Plugin.Activate", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil || resp.Implements[0] != "IpamDriver" {
		t.Errorf("Activate response is invalid %+v", resp)
	}
}

// Tests IpamDriver.GetCapabilities functionality.
func TestGetCapabilities(t *testing.T) {
	fmt.Println("Test: GetCapabilities")

	var resp struct {
		RequiresMACAddress bool
	}

	req, err := http.NewRequest(http.MethodGet, getCapabilitiesPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("GetCapabilities response is invalid %+v", resp)
	}
}

// Tests address space management functionality.
func TestAddAddressSpace(t *testing.T) {
	fmt.Println("Test: AddAddressSpace")

	var anyInterface = "any"
	var anyPriority = 42
	var err error

	// Configure the local default address space.
	local, err = sink.newAddressSpace(localDefaultAddressSpaceId, localScope)
	if err != nil {
		t.Errorf("newAddressSpace failed %+v", err)
		return
	}

	addr1 := net.IPv4(192, 168, 1, 1)
	addr2 := net.IPv4(192, 168, 1, 2)
	subnet := net.IPNet{
		IP:   net.IPv4(192, 168, 1, 0),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}
	ap, err := local.newAddressPool(anyInterface, anyPriority, &subnet)
	ap.newAddressRecord(&addr1)
	ap.newAddressRecord(&addr2)

	addr1 = net.IPv4(192, 168, 2, 1)
	subnet = net.IPNet{
		IP:   net.IPv4(192, 168, 2, 0),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}
	ap, err = local.newAddressPool(anyInterface, anyPriority, &subnet)
	ap.newAddressRecord(&addr1)

	sink.setAddressSpace(local)

	// Configure the global default address space.
	global, err = sink.newAddressSpace(globalDefaultAddressSpaceId, globalScope)
	if err != nil {
		t.Errorf("newAddressSpace failed %+v", err)
		return
	}

	sink.setAddressSpace(global)
}

// Tests IpamDriver.GetDefaultAddressSpaces functionality.
func TestGetDefaultAddressSpaces(t *testing.T) {
	fmt.Println("Test: GetDefaultAddressSpaces")

	var resp getDefaultAddressSpacesResponse

	req, err := http.NewRequest(http.MethodGet, getAddressSpacesPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil ||
		resp.LocalDefaultAddressSpace != localDefaultAddressSpaceId ||
		resp.GlobalDefaultAddressSpace != globalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces response is invalid %+v", resp)
	}
}

// Tests IpamDriver.RequestPool functionality.
func TestRequestPool(t *testing.T) {
	fmt.Println("Test: RequestPool")

	var body bytes.Buffer
	var resp requestPoolResponse

	payload := &requestPoolRequest{
		AddressSpace: localDefaultAddressSpaceId,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, requestPoolPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("RequestPool response is invalid %+v", resp)
	}

	poolId1 = resp.PoolID
}

// Tests IpamDriver.RequestAddress functionality.
func TestRequestAddress(t *testing.T) {
	fmt.Println("Test: RequestAddress")

	var body bytes.Buffer
	var resp requestAddressResponse

	payload := &requestAddressRequest{
		PoolID:  poolId1,
		Address: "",
		Options: nil,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, requestAddressPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("RequestAddress response is invalid %+v", resp)
	}

	address, _, _ := net.ParseCIDR(resp.Address)
	address1 = address.String()
}

// Tests IpamDriver.ReleaseAddress functionality.
func TestReleaseAddress(t *testing.T) {
	fmt.Println("Test: ReleaseAddress")

	var body bytes.Buffer
	var resp releaseAddressResponse

	payload := &releaseAddressRequest{
		PoolID:  poolId1,
		Address: address1,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, releaseAddressPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("ReleaseAddress response is invalid %+v", resp)
	}
}

// Tests IpamDriver.ReleasePool functionality.
func TestReleasePool(t *testing.T) {
	fmt.Println("Test: ReleasePool")

	var body bytes.Buffer
	var resp releasePoolResponse

	payload := &releasePoolRequest{
		PoolID: poolId1,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, releasePoolPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("ReleasePool response is invalid %+v", resp)
	}
}
