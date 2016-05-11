// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net/http"
	"sync"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/log"
)

// Plugin capabilities.
const (
	requiresMACAddress = false
)

// IpamPlugin object and interface
type ipamPlugin struct {
	*common.Plugin
	addrSpaces map[string]*addressSpace
	source     configSource
	sync.Mutex
}

type IpamPlugin interface {
	Start(chan error) error
	Stop()

	SetOption(string, string)

	setAddressSpace(*addressSpace) error
}

// Creates a new IpamPlugin object.
func NewPlugin(name string, version string) (IpamPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, version, endpointType)
	if err != nil {
		return nil, err
	}

	return &ipamPlugin{
		Plugin:     plugin,
		addrSpaces: make(map[string]*addressSpace),
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(errChan chan error) error {
	err := plugin.Initialize(errChan)
	if err != nil {
		log.Printf("%s: Failed to start: %v", plugin.Name, err)
		return err
	}

	// Add protocol handlers.
	listener := plugin.Listener
	listener.AddHandler(getCapabilitiesPath, plugin.getCapabilities)
	listener.AddHandler(getAddressSpacesPath, plugin.getDefaultAddressSpaces)
	listener.AddHandler(requestPoolPath, plugin.requestPool)
	listener.AddHandler(releasePoolPath, plugin.releasePool)
	listener.AddHandler(requestAddressPath, plugin.requestAddress)
	listener.AddHandler(releaseAddressPath, plugin.releaseAddress)

	// Start configuration source.
	err = plugin.startSource()
	if err != nil {
		log.Printf("%s: Failed to start: %v", plugin.Name, err)
		return err
	}

	log.Printf("%s: Plugin started.", plugin.Name)

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.stopSource()
	plugin.Uninitialize()
	log.Printf("%s: Plugin stopped.\n", plugin.Name)
}

// Sets a new address space for the plugin to serve to clients.
func (plugin *ipamPlugin) setAddressSpace(as *addressSpace) error {
	plugin.Lock()

	as1, ok := plugin.addrSpaces[as.id]
	if !ok {
		plugin.addrSpaces[as.id] = as
		plugin.Unlock()
	} else {
		plugin.Unlock()
		as1.merge(as)
	}

	return nil
}

// Parses the given pool ID string and returns the address space and pool objects.
func (plugin *ipamPlugin) parsePoolId(poolId string) (*addressSpace, *addressPool, error) {
	apId, err := newAddressPoolIdFromString(poolId)
	if err != nil {
		return nil, nil, err
	}

	plugin.Lock()
	as := plugin.addrSpaces[apId.asId]
	plugin.Unlock()

	if as == nil {
		return nil, nil, errInvalidAddressSpace
	}

	var ap *addressPool
	if apId.subnet != "" {
		ap, err = as.getAddressPool(poolId)
		if err != nil {
			return nil, nil, err
		}
	}

	return as, ap, nil
}

//
// Libnetwork remote IPAM API implementation
// https://github.com/docker/libnetwork/blob/master/docs/ipam.md
//

// Handles GetCapabilities requests.
func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	var req getCapabilitiesRequest

	log.Request(plugin.Name, &req, nil)

	resp := getCapabilitiesResponse{
		RequiresMACAddress: requiresMACAddress,
	}

	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles GetDefaultAddressSpaces requests.
func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	var req getDefaultAddressSpacesRequest
	var resp getDefaultAddressSpacesResponse

	log.Request(plugin.Name, &req, nil)

	plugin.refreshSource()

	plugin.Lock()

	local := plugin.addrSpaces[localDefaultAddressSpaceId]
	if local != nil {
		resp.LocalDefaultAddressSpace = local.id
	}

	global := plugin.addrSpaces[globalDefaultAddressSpaceId]
	if global != nil {
		resp.GlobalDefaultAddressSpace = global.id
	}

	plugin.Unlock()

	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles RequestPool requests.
func (plugin *ipamPlugin) requestPool(w http.ResponseWriter, r *http.Request) {
	var req requestPoolRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	plugin.refreshSource()

	// Process request.
	as, _, err := plugin.parsePoolId(req.AddressSpace)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	poolId, err := as.requestPool(req.Pool, req.SubPool, req.Options, req.V6)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	data := make(map[string]string)
	resp := requestPoolResponse{PoolID: poolId.String(), Pool: poolId.subnet, Data: data}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles ReleasePool requests.
func (plugin *ipamPlugin) releasePool(w http.ResponseWriter, r *http.Request) {
	var req releasePoolRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	as, _, err := plugin.parsePoolId(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	err = as.releasePool(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := releasePoolResponse{}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles RequestAddress requests.
func (plugin *ipamPlugin) requestAddress(w http.ResponseWriter, r *http.Request) {
	var req requestAddressRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	plugin.refreshSource()

	// Process request.
	_, ap, err := plugin.parsePoolId(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	addr, err := ap.requestAddress(req.Address, req.Options)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	data := make(map[string]string)
	resp := requestAddressResponse{Address: addr, Data: data}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles ReleaseAddress requests.
func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var req releaseAddressRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	_, ap, err := plugin.parsePoolId(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	err = ap.releaseAddress(req.Address)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := releaseAddressResponse{}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
