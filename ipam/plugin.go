// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net/http"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/log"
)

const (
	// Plugin name.
	name = "ipam"

	// Plugin capabilities.
	requiresMACAddress = false
)

// IpamPlugin object and interface
type ipamPlugin struct {
	*common.Plugin
	am *addressManager
}

type IpamPlugin interface {
	common.PluginApi
}

// Creates a new IpamPlugin object.
func NewPlugin(config *common.PluginConfig) (IpamPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, config.Version, endpointType)
	if err != nil {
		return nil, err
	}

	// Setup address manager.
	am, err := newAddressManager()
	if err != nil {
		return nil, err
	}

	return &ipamPlugin{
		Plugin: plugin,
		am:     am,
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(config *common.PluginConfig) error {
	// Initialize base plugin.
	err := plugin.Initialize(config)
	if err != nil {
		log.Printf("[ipam] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Initialize address manager.
	err = plugin.am.Initialize(config, plugin.GetOption("source"))
	if err != nil {
		log.Printf("[ipam] Failed to initialize address manager, err:%v.", err)
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

	log.Printf("[ipam] Plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.am.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[ipam] Plugin stopped.")
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

	localId, globalId := plugin.am.GetDefaultAddressSpaces()

	resp.LocalDefaultAddressSpace = localId
	resp.GlobalDefaultAddressSpace = globalId

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

	// Process request.
	poolId, subnet, err := plugin.am.RequestPool(req.AddressSpace, req.Pool, req.SubPool, req.Options, req.V6)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	data := make(map[string]string)
	poolId = newAddressPoolId(req.AddressSpace, poolId, "").String()
	resp := requestPoolResponse{PoolID: poolId, Pool: subnet, Data: data}

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
	poolId, err := newAddressPoolIdFromString(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	err = plugin.am.ReleasePool(poolId.asId, poolId.subnet)
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

	// Process request.
	poolId, err := newAddressPoolIdFromString(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	addr, err := plugin.am.RequestAddress(poolId.asId, poolId.subnet, req.Address, req.Options)
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
	poolId, err := newAddressPoolIdFromString(req.PoolID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	err = plugin.am.ReleaseAddress(poolId.asId, poolId.subnet, req.Address)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := releaseAddressResponse{}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
