// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var plugin IpamPlugin
var mux *http.ServeMux

// Wraps the test run with plugin setup and teardown.
func TestMain(m *testing.M) {
	var err error

	// Create the plugin.
	plugin, err = NewPlugin("test", "")
	if err != nil {
		fmt.Printf("Failed to create IPAM plugin %v\n", err)
		return
	}

	err = plugin.Start(nil)
	if err != nil {
		fmt.Printf("Failed to start IPAM plugin %v\n", err)
		return
	}

	// Get the internal http mux as test hook.
	mux = plugin.(*ipamPlugin).listener.GetMux()

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

	req, err := http.NewRequest(http.MethodGet, "/IpamDriver.GetCapabilities", nil)
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
