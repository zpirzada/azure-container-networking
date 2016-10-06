// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"net/http"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
)

const (
	// Plugin name.
	name = "net"

	// Plugin capabilities.
	scope = "local"
)

// NetPlugin object and its interface
type netPlugin struct {
	*common.Plugin
	scope string
	nm    *networkManager
}

type NetPlugin interface {
	common.PluginApi
}

// Creates a new NetPlugin object.
func NewPlugin(config *common.PluginConfig) (NetPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, config.Version, endpointType)
	if err != nil {
		return nil, err
	}

	// Setup network manager.
	nm, err := newNetworkManager()
	if err != nil {
		return nil, err
	}

	config.NetApi = nm

	return &netPlugin{
		Plugin: plugin,
		scope:  scope,
		nm:     nm,
	}, nil
}

// Starts the plugin.
func (plugin *netPlugin) Start(config *common.PluginConfig) error {
	// Initialize base plugin.
	err := plugin.Initialize(config)
	if err != nil {
		log.Printf("[net] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Initialize network manager.
	err = plugin.nm.Initialize(config)
	if err != nil {
		log.Printf("[net] Failed to initialize network manager, err:%v.", err)
		return err
	}

	// Add protocol handlers.
	listener := plugin.Listener
	listener.AddHandler(getCapabilitiesPath, plugin.getCapabilities)
	listener.AddHandler(createNetworkPath, plugin.createNetwork)
	listener.AddHandler(deleteNetworkPath, plugin.deleteNetwork)
	listener.AddHandler(createEndpointPath, plugin.createEndpoint)
	listener.AddHandler(deleteEndpointPath, plugin.deleteEndpoint)
	listener.AddHandler(joinPath, plugin.join)
	listener.AddHandler(leavePath, plugin.leave)
	listener.AddHandler(endpointOperInfoPath, plugin.endpointOperInfo)

	log.Printf("[net] Plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *netPlugin) Stop() {
	plugin.nm.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[net] Plugin stopped.")
}

//
// Libnetwork remote network API implementation
// https://github.com/docker/libnetwork/blob/master/docs/remote.md
//

// Handles GetCapabilities requests.
func (plugin *netPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	var req getCapabilitiesRequest

	log.Request(plugin.Name, &req, nil)

	resp := getCapabilitiesResponse{Scope: plugin.scope}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles CreateNetwork requests.
func (plugin *netPlugin) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	err = plugin.nm.CreateNetwork(req.NetworkID, req.Options, req.IPv4Data, req.IPv6Data)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := createNetworkResponse{}
	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles DeleteNetwork requests.
func (plugin *netPlugin) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req deleteNetworkRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	err = plugin.nm.DeleteNetwork(req.NetworkID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := deleteNetworkResponse{}
	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles CreateEndpoint requests.
func (plugin *netPlugin) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var req createEndpointRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	var ipv4Address string
	if req.Interface != nil {
		ipv4Address = req.Interface.Address
	}

	err = plugin.nm.CreateEndpoint(req.NetworkID, req.EndpointID, ipv4Address)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := createEndpointResponse{
		Interface: nil,
	}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles DeleteEndpoint requests.
func (plugin *netPlugin) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var req deleteEndpointRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	err = plugin.nm.DeleteEndpoint(req.NetworkID, req.EndpointID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := deleteEndpointResponse{}
	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles Join requests.
func (plugin *netPlugin) join(w http.ResponseWriter, r *http.Request) {
	var req joinRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	ep, err := plugin.nm.AttachEndpoint(req.NetworkID, req.EndpointID, req.SandboxKey)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	ifname := interfaceName{
		SrcName:   ep.SrcName,
		DstPrefix: ep.DstPrefix,
	}

	resp := joinResponse{
		InterfaceName: ifname,
		Gateway:       ep.IPv4Gateway.String(),
		GatewayIPv6:   ep.IPv6Gateway.String(),
	}

	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles Leave requests.
func (plugin *netPlugin) leave(w http.ResponseWriter, r *http.Request) {
	var req leaveRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Process request.
	err = plugin.nm.DetachEndpoint(req.NetworkID, req.EndpointID)
	if err != nil {
		plugin.SendErrorResponse(w, err)
		return
	}

	// Encode response.
	resp := leaveResponse{}
	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles EndpointOperInfo requests.
func (plugin *netPlugin) endpointOperInfo(w http.ResponseWriter, r *http.Request) {
	var req endpointOperInfoRequest

	// Decode request.
	err := plugin.Listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	value := make(map[string]interface{})
	//value["com.docker.network.endpoint.macaddress"] = macAddress
	//value["MacAddress"] = macAddress

	// Encode response.
	resp := endpointOperInfoResponse{Value: value}
	err = plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
