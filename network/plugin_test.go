// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/netlink"
	driverApi "github.com/docker/libnetwork/driverapi"
	remoteApi "github.com/docker/libnetwork/drivers/remote/api"
)

var plugin NetPlugin
var mux *http.ServeMux

var anyInterface = "test0"
var anySubnet = "192.168.1.0/24"

// Wraps the test run with plugin setup and teardown.
func TestMain(m *testing.M) {
	var config common.PluginConfig
	var err error

	// Create the plugin.
	plugin, err = NewPlugin(&config)
	if err != nil {
		fmt.Printf("Failed to create network plugin %v\n", err)
		os.Exit(1)
	}

	// Configure test mode.
	plugin.(*netPlugin).Name = "test"

	// Start the plugin.
	err = plugin.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start network plugin %v\n", err)
		os.Exit(2)
	}

	// Create a dummy test network interface.
	err = netlink.AddLink(anyInterface, "dummy")
	if err != nil {
		fmt.Printf("Failed to create test network interface, err:%v.\n", err)
		os.Exit(3)
	}

	err = plugin.(*netPlugin).nm.AddExternalInterface(anyInterface, anySubnet)
	if err != nil {
		fmt.Printf("Failed to add test network interface, err:%v.\n", err)
		os.Exit(4)
	}

	// Get the internal http mux as test hook.
	mux = plugin.(*netPlugin).Listener.GetMux()

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	netlink.DeleteLink(anyInterface)
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
// Libnetwork remote API compliance tests
// https://github.com/docker/libnetwork/blob/master/docs/remote.md
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

	if err != nil || resp.Implements[0] != "NetworkDriver" {
		t.Errorf("Activate response is invalid %+v", resp)
	}
}

// Tests NetworkDriver.GetCapabilities functionality.
func TestGetCapabilities(t *testing.T) {
	fmt.Println("Test: GetCapabilities")

	var resp remoteApi.GetCapabilityResponse

	req, err := http.NewRequest(http.MethodGet, getCapabilitiesPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil || resp.Err != "" || resp.Scope != "local" {
		t.Errorf("GetCapabilities response is invalid %+v", resp)
	}
}

// Tests NetworkDriver.CreateNetwork functionality.
func TestCreateNetwork(t *testing.T) {
	fmt.Println("Test: CreateNetwork")

	var body bytes.Buffer
	var resp remoteApi.CreateNetworkResponse

	_, pool, _ := net.ParseCIDR(anySubnet)

	info := &remoteApi.CreateNetworkRequest{
		NetworkID: "N1",
		IPv4Data: []driverApi.IPAMData{
			{
				Pool: pool,
			},
		},
	}

	json.NewEncoder(&body).Encode(info)

	req, err := http.NewRequest(http.MethodGet, createNetworkPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil || resp.Err != "" {
		t.Errorf("CreateNetwork response is invalid %+v", resp)
	}
}

// Tests NetworkDriver.DeleteNetwork functionality.
func TestDeleteNetwork(t *testing.T) {
	fmt.Println("Test: DeleteNetwork")

	var body bytes.Buffer
	var resp remoteApi.DeleteNetworkResponse

	info := &remoteApi.DeleteNetworkRequest{
		NetworkID: "N1",
	}

	json.NewEncoder(&body).Encode(info)

	req, err := http.NewRequest(http.MethodGet, deleteNetworkPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil || resp.Err != "" {
		t.Errorf("DeleteNetwork response is invalid %+v", resp)
	}
}

// Tests NetworkDriver.EndpointOperInfo functionality.
func TestEndpointOperInfo(t *testing.T) {
	fmt.Println("Test: EndpointOperInfo")

	var body bytes.Buffer
	var resp remoteApi.EndpointInfoResponse

	info := &remoteApi.EndpointInfoRequest{
		NetworkID:  "N1",
		EndpointID: "E1",
	}

	json.NewEncoder(&body).Encode(info)

	req, err := http.NewRequest(http.MethodGet, endpointOperInfoPath, &body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	err = decodeResponse(w, &resp)

	if err != nil {
		t.Errorf("EndpointOperInfo response is invalid %+v", resp)
	}
}
