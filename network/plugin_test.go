// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var plugin NetPlugin
var mux *http.ServeMux

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

	mux = plugin.GetListener().GetMux()

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	plugin.Stop()

	os.Exit(exitCode)
}

//
// Libnetwork remote API compliance tests
// github.com/docker/libnetwork/drivers/remote/api
//

func TestActivate(t *testing.T) {
	fmt.Println("Test: Activate")

	req, err := http.NewRequest(http.MethodGet, "/Plugin.Activate", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("Activate request failed")
	}

	//fmt.Printf("%d - %s", w.Code, w.Body.String())
}

func TestGetCapabilities(t *testing.T) {
	fmt.Println("Test: GetCapabilities")

	req, err := http.NewRequest(http.MethodGet, "/NetworkDriver.GetCapabilities", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("GetCapabilities request failed")
	}

	//resp api.GetCapabilityResponse
}
