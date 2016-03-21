// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net/http"
	"sync"

	"github.com/azure/aqua/core"
	"github.com/azure/aqua/log"
)

// Libnetwork IPAM plugin endpoint name
const endpointName = "IpamDriver"

// IpamPlugin object and interface
type ipamPlugin struct {
	name     string
	version  string
	scope    string
	listener *core.Listener
	sync.Mutex
}

type IpamPlugin interface {
	Start(chan error) error
	Stop()
}

// Creates a new IpamPlugin object.
func NewPlugin(name string, version string) (IpamPlugin, error) {
	return &ipamPlugin{
		name:    name,
		version: version,
		scope:   "local",
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(errChan chan error) error {

	// Create the listener.
	listener, err := core.NewListener(plugin.name)
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

	log.Printf("%s: Plugin started.", plugin.name)

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.listener.Stop()
	log.Printf("%s: Plugin stopped.\n", plugin.name)
}

type activateResponse struct {
	Implements []string
}

func (plugin *ipamPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.name, "Activate", nil, nil)

	resp := &activateResponse{[]string{endpointName}}
	err := plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "Activate", resp, err)
}

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.name, "GetCapabilities", nil, nil)

	resp := map[string]string{"Scope": plugin.scope}
	err := plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "GetCapabilities", resp, err)
}

type defaultAddressSpacesResponseFormat struct {
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.name, "GetDefaultAddressSpaces", nil, nil)

	resp := &defaultAddressSpacesResponseFormat{
		LocalDefaultAddressSpace:  "",
		GlobalDefaultAddressSpace: "",
	}

	err := plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "GetDefaultAddressSpaces", resp, err)
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

	log.Request(plugin.name, "RequestPool", req, err)

	if err == nil {
		data := make(map[string]string)
		resp := &requestPoolResponseFormat{"", "0.0.0.0/8", data}

		err = plugin.listener.Encode(w, resp)

		log.Response(plugin.name, "RequestPool", resp, err)
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

	log.Request(plugin.name, "ReleasePool", req, err)

	if err == nil {
		resp := &releasePoolRequestFormat{}

		err = plugin.listener.Encode(w, resp)

		log.Response(plugin.name, "ReleasePool", resp, err)
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

	log.Request(plugin.name, "RequestAddress", req, err)

	if err == nil {
		resp := &requestAddressResponseFormat{"", "", make(map[string]string)}

		err = plugin.listener.Encode(w, resp)

		log.Response(plugin.name, "RequestAddress", resp, err)
	}
}

type releaseAddressRequestFormat struct {
	PoolID  string
	Address string
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "ReleaseAddress", req, err)

	if err == nil {
		resp := map[string]string{}

		err = plugin.listener.Encode(w, resp)

		log.Response(plugin.name, "ReleaseAddress", resp, err)
	}
}
