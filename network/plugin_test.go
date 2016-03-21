// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/azure/aqua/core"
	//"github.com/docker/libnetwork/drivers/remote/api"
)

var plugin NetPlugin
var mux *http.ServeMux

func TestMain(m *testing.M) {
	// Create the listener.
	listener, err := core.NewListener("test")
	if err != nil {
		fmt.Printf("Failed to create listener %v", err)
		return
	}

	mux = listener.GetMux()

	// Create the plugin.
	plugin, err = NewPlugin("test", "")
	if err != nil {
		fmt.Printf("Failed to create network plugin %v\n", err)
		return
	}

	err = plugin.Start(listener)
	if err != nil {
		fmt.Printf("Failed to start network plugin %v\n", err)
		return
	}

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	plugin.Stop()

	os.Exit(exitCode)
}

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
