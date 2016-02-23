// Copyright Microsoft Corp.
// All rights reserved.

package ipamNull

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/sharmasushant/penguin/core"
)

// Libnetwork IPAM plugin name
const pluginName = "nullipam"

// Libnetwork IPAM plugin endpoint name
const endpointName = "IpamDriver"

// IpamPlugin object and interface
type ipamPlugin struct {
	version  string
	listener *core.Listener
	sync.Mutex
}

type IpamPlugin interface {
	Start(chan error) error
	Stop()
}

// Creates a new IpamPlugin object.
func NewPlugin(version string) (IpamPlugin, error) {
	return &ipamPlugin{
		version: version,
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(errChan chan error) error {

	// Create the listener.
	listener, err := core.NewListener(pluginName)
	if err != nil {
		fmt.Printf("Failed to create listener %v", err)
		return err
	}

	// Add protocol handlers.
	listener.AddHandler("Plugin", "Activate", plugin.activatePlugin)
	listener.AddHandler(endpointName, "GetCapabilities", plugin.getCapabilities)
	listener.AddHandler(endpointName, "GetDefaultAddressSpaces", plugin.getDefaultAddressSpaces)
	listener.AddHandler(endpointName, "RequestPool", plugin.requestPool)
	listener.AddHandler(endpointName, "ReleasePool", plugin.releasePool)
	listener.AddHandler(endpointName, "RequestAddress", plugin.requestAddress)
	listener.AddHandler(endpointName, "ReleaseAddress", plugin.releaseAddress)

	plugin.listener = listener

	err = listener.Start(errChan)
	if err != nil {
		fmt.Printf("Failed to start listener %v", err)
		return err
	}

	fmt.Println("IPAM plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.listener.Stop()
	fmt.Println("IPAM plugin stopped.")
}

type activateResponse struct {
	Implements []string
}

func (plugin *ipamPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	core.LogRequest(pluginName, "Activate", nil)

	resp := &activateResponse{[]string{endpointName}}
	err := plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "Activate", resp, err)
}

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	core.LogRequest(pluginName, "GetCapabilities", nil)

	resp := map[string]string{"Scope": "local"}
	err := plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "GetCapabilities", resp, err)
}

type defaultAddressSpacesResponseFormat struct {
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	core.LogRequest(pluginName, "GetDefaultAddressSpaces", nil)

	resp := &defaultAddressSpacesResponseFormat{
		LocalDefaultAddressSpace:  "",
		GlobalDefaultAddressSpace: "",
	}

	err := plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "GetDefaultAddressSpaces", resp, err)
}

type requestPoolRequestFormat struct {
	AddressSpace string
	Pool         string
	SubPool      string
	Options      map[string]string
	V6           bool
}

type requestPoolResponseFormat struct {
	PoolID string
	Pool   string
	Data   map[string]string
}

func (plugin *ipamPlugin) requestPool(w http.ResponseWriter, r *http.Request) {
	var req requestPoolRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "RequestPool", err)

	if err == nil {
		data := make(map[string]string)
		resp := &requestPoolResponseFormat{"", "0.0.0.0/8", data}

		err = plugin.listener.Encode(w, resp)

		core.LogResponse(pluginName, "RequestPool", resp, err)
	}
}

type releasePoolRequestFormat struct {
	PoolID string
}

type releasePoolResponseFormat struct {
}

func (plugin *ipamPlugin) releasePool(w http.ResponseWriter, r *http.Request) {
	var req releasePoolRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "ReleasePool", err)

	if err == nil {
		resp := &releasePoolRequestFormat{}

		err = plugin.listener.Encode(w, resp)

		core.LogResponse(pluginName, "ReleasePool", resp, err)
	}
}

type requestAddressRequestFormat struct {
	PoolID  string
	Address string
	Options map[string]string
}

type requestAddressResponseFormat struct {
	PoolID  string
	Address string
	Options map[string]string
}

func (plugin *ipamPlugin) requestAddress(w http.ResponseWriter, r *http.Request) {
	var req requestAddressRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "RequestAddress", err)

	if err == nil {
		resp := &requestAddressResponseFormat{"", "", make(map[string]string)}

		err = plugin.listener.Encode(w, resp)

		core.LogResponse(pluginName, "RequestAddress", resp, err)
	}
}

type releaseAddressRequestFormat struct {
	PoolID  string
	Address string
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "ReleaseAddress", err)

	if err == nil {
		resp := map[string]string{}

		err = plugin.listener.Encode(w, resp)

		core.LogResponse(pluginName, "ReleaseAddress", resp, err)
	}
}
