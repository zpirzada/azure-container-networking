// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net/http"
	"sync"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/log"
)

// Libnetwork IPAM plugin endpoint type
const endpointType = "IpamDriver"

// IpamPlugin object and interface
type ipamPlugin struct {
	common.Plugin
	sync.Mutex
}

type IpamPlugin interface {
	Start(chan error) error
	Stop()
}

// Creates a new IpamPlugin object.
func NewPlugin(name string, version string) (IpamPlugin, error) {
	return &ipamPlugin{
		Plugin: common.Plugin{
			Name:         name,
			Version:      version,
			Scope:        "local",
			EndpointType: endpointType,
		},
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(errChan chan error) error {
	err := plugin.Initialize(errChan)
	if err != nil {
		log.Printf("%s: Failed to start: %v", err)
		return err
	}

	// Add protocol handlers.
	listener := plugin.Listener
	listener.AddHandler(endpointType, "GetCapabilities", plugin.getCapabilities)
	listener.AddHandler(endpointType, "GetDefaultAddressSpaces", plugin.getDefaultAddressSpaces)
	listener.AddHandler(endpointType, "RequestPool", plugin.requestPool)
	listener.AddHandler(endpointType, "ReleasePool", plugin.releasePool)
	listener.AddHandler(endpointType, "RequestAddress", plugin.requestAddress)
	listener.AddHandler(endpointType, "ReleaseAddress", plugin.releaseAddress)

	log.Printf("%s: Plugin started.", plugin.Name)

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.Uninitialize()
	log.Printf("%s: Plugin stopped.\n", plugin.Name)
}

//
// Libnetwork remote IPAM plugin APIs
//

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.Name, "GetCapabilities", nil, nil)

	resp := map[string]string{"Scope": plugin.Scope}
	err := plugin.Listener.Encode(w, resp)

	log.Response(plugin.Name, "GetCapabilities", resp, err)
}

type defaultAddressSpacesResponseFormat struct {
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.Name, "GetDefaultAddressSpaces", nil, nil)

	resp := &defaultAddressSpacesResponseFormat{
		LocalDefaultAddressSpace:  "",
		GlobalDefaultAddressSpace: "",
	}

	err := plugin.Listener.Encode(w, resp)

	log.Response(plugin.Name, "GetDefaultAddressSpaces", resp, err)
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

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "RequestPool", req, err)

	if err == nil {
		data := make(map[string]string)
		resp := &requestPoolResponseFormat{"", "0.0.0.0/8", data}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "RequestPool", resp, err)
	}
}

type releasePoolRequestFormat struct {
	PoolID string
}

type releasePoolResponseFormat struct {
}

func (plugin *ipamPlugin) releasePool(w http.ResponseWriter, r *http.Request) {
	var req releasePoolRequestFormat

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "ReleasePool", req, err)

	if err == nil {
		resp := &releasePoolRequestFormat{}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "ReleasePool", resp, err)
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

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "RequestAddress", req, err)

	if err == nil {
		resp := &requestAddressResponseFormat{"", "", make(map[string]string)}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "RequestAddress", resp, err)
	}
}

type releaseAddressRequestFormat struct {
	PoolID  string
	Address string
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequestFormat

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "ReleaseAddress", req, err)

	if err == nil {
		resp := map[string]string{}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "ReleaseAddress", resp, err)
	}
}
