// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	remoteApi "github.com/docker/libnetwork/drivers/remote/api"
)

var plugin NetPlugin
var mux *http.ServeMux

// Wraps the test run with plugin setup and teardown.
func TestMain(m *testing.M) {
	var err error

	// Create the plugin.
	plugin, err = NewPlugin("test", "")
	if err != nil {
		fmt.Printf("Failed to create network plugin %v\n", err)
		return
	}

	err = plugin.Start(nil)
	if err != nil {
		fmt.Printf("Failed to start network plugin %v\n", err)
		return
	}

	// Get the internal http mux as test hook.
	mux = plugin.(*netPlugin).listener.GetMux()

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

	req, err := http.NewRequest(http.MethodGet, "/NetworkDriver.GetCapabilities", nil)
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

	info := &remoteApi.CreateNetworkRequest{
		NetworkID:  "N1",
	}

	json.NewEncoder(&body).Encode(info)

	req, err := http.NewRequest(http.MethodGet, "/NetworkDriver.CreateNetwork", &body)
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
		NetworkID:  "N1",
	}

	json.NewEncoder(&body).Encode(info)

	req, err := http.NewRequest(http.MethodGet, "/NetworkDriver.DeleteNetwork", &body)
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

	req, err := http.NewRequest(http.MethodGet, "/NetworkDriver.EndpointOperInfo", &body)
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
