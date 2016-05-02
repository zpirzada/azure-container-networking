// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net/http"
	"sync"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/log"
)

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
// Libnetwork remote IPAM API implementation
// https://github.com/docker/libnetwork/blob/master/docs/ipam.md
//

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.Name, "GetCapabilities", nil, nil)

	resp := &getCapabilitiesResponse{}
	err := plugin.Listener.Encode(w, resp)

	log.Response(plugin.Name, "GetCapabilities", resp, err)
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.Name, "GetDefaultAddressSpaces", nil, nil)

	resp := &getDefaultAddressSpacesResponse{
		LocalDefaultAddressSpace:  "",
		GlobalDefaultAddressSpace: "",
	}

	err := plugin.Listener.Encode(w, resp)

	log.Response(plugin.Name, "GetDefaultAddressSpaces", resp, err)
}

func (plugin *ipamPlugin) requestPool(w http.ResponseWriter, r *http.Request) {
	var req requestPoolRequest

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "RequestPool", req, err)

	if err == nil {
		data := make(map[string]string)
		resp := &requestPoolResponse{"", "0.0.0.0/8", data}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "RequestPool", resp, err)
	}
}

func (plugin *ipamPlugin) releasePool(w http.ResponseWriter, r *http.Request) {
	var req releasePoolRequest

	err := plugin.Listener.Decode(w, r, &req)
	
	log.Request(plugin.Name, "ReleasePool", req, err)
	
	if err == nil {
		resp := &releasePoolResponse{}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "ReleasePool", resp, err)
	}
}

func (plugin *ipamPlugin) requestAddress(w http.ResponseWriter, r *http.Request) {
	var req requestAddressRequest

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "RequestAddress", req, err)

	if err == nil {
		resp := &requestAddressResponse{"", make(map[string]string)}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "RequestAddress", resp, err)
	}
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequest

	err := plugin.Listener.Decode(w, r, &req)

	log.Request(plugin.Name, "ReleaseAddress", req, err)

	if err == nil {
		resp := &releaseAddressResponse{}

		err = plugin.Listener.Encode(w, resp)

		log.Response(plugin.Name, "ReleaseAddress", resp, err)
	}
}
