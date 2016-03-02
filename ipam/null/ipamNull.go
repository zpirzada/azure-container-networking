// Copyright Microsoft Corp.
// All rights reserved.

package ipamNull

import (
	"net/http"
	"sync"

	"github.com/sharmasushant/penguin/core"
	"github.com/sharmasushant/penguin/log"
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
		log.Printf("Failed to create listener %v", err)
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
		log.Printf("Failed to start listener %v", err)
		return err
	}

	log.Printf("%s: Plugin started.", pluginName)

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.listener.Stop()
	log.Printf("%s: Plugin stopped.\n", pluginName)
}

type activateResponse struct {
	Implements []string
}

func (plugin *ipamPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	log.Request(pluginName, "Activate", nil, nil)

	resp := &activateResponse{[]string{endpointName}}
	err := plugin.listener.Encode(w, resp)

	log.Response(pluginName, "Activate", resp, err)
}

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Request(pluginName, "GetCapabilities", nil, nil)

	resp := map[string]string{"Scope": "local"}
	err := plugin.listener.Encode(w, resp)

	log.Response(pluginName, "GetCapabilities", resp, err)
}

type defaultAddressSpacesResponseFormat struct {
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	log.Request(pluginName, "GetDefaultAddressSpaces", nil, nil)

	resp := &defaultAddressSpacesResponseFormat{
		LocalDefaultAddressSpace:  "",
		GlobalDefaultAddressSpace: "",
	}

	err := plugin.listener.Encode(w, resp)

	log.Response(pluginName, "GetDefaultAddressSpaces", resp, err)
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

	log.Request(pluginName, "RequestPool", req, err)

	if err == nil {
		data := make(map[string]string)
		resp := &requestPoolResponseFormat{"", "0.0.0.0/8", data}

		err = plugin.listener.Encode(w, resp)

		log.Response(pluginName, "RequestPool", resp, err)
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

	log.Request(pluginName, "ReleasePool", req, err)

	if err == nil {
		resp := &releasePoolRequestFormat{}

		err = plugin.listener.Encode(w, resp)

		log.Response(pluginName, "ReleasePool", resp, err)
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

	log.Request(pluginName, "RequestAddress", req, err)

	if err == nil {
		resp := &requestAddressResponseFormat{"", "", make(map[string]string)}

		err = plugin.listener.Encode(w, resp)

		log.Response(pluginName, "RequestAddress", resp, err)
	}
}

type releaseAddressRequestFormat struct {
	PoolID  string
	Address string
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(pluginName, "ReleaseAddress", req, err)

	if err == nil {
		resp := map[string]string{}

		err = plugin.listener.Encode(w, resp)

		log.Response(pluginName, "ReleaseAddress", resp, err)
	}
}
