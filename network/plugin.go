// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Azure/Aqua/common"
	"github.com/Azure/Aqua/core"
	"github.com/Azure/Aqua/log"
)

// Plugin capabilities.
const (
	scope = "local"
)

// NetPlugin object and its interface
type netPlugin struct {
	*common.Plugin
	scope    string
	listener *common.Listener
	networks map[string]*azureNetwork
	sync.Mutex
}

type NetPlugin interface {
	Start(chan error) error
	Stop()
}

// Creates a new NetPlugin object.
func NewPlugin(name string, version string) (NetPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, version, endpointType)
	if err != nil {
		return nil, err
	}

	return &netPlugin{
		Plugin:   plugin,
		scope:    scope,
		networks: make(map[string]*azureNetwork),
	}, nil
}

// Starts the plugin.
func (plugin *netPlugin) Start(errChan chan error) error {
	err := plugin.Initialize(errChan)
	if err != nil {
		log.Printf("%s: Failed to start: %v", plugin.Name, err)
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

	log.Printf("%s: Plugin started.", plugin.Name)

	return nil
}

// Stops the plugin.
func (plugin *netPlugin) Stop() {
	plugin.Uninitialize()
	core.FreeSlaves()
	log.Printf("%s: Plugin stopped.\n", plugin.Name)
}

func (plugin *netPlugin) networkExists(networkID string) bool {
	if plugin.networks[networkID] != nil {
		return true
	}
	return false
}

func (plugin *netPlugin) endpointExists(networkID string, endpointID string) bool {
	network := plugin.networks[networkID]
	if network == nil {
		return false
	}

	if network.endpoints[endpointID] == nil {
		return false
	}

	return true
}

// Handles GetCapabilities requests.
func (plugin *netPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	var req getCapabilitiesRequest

	log.Request(plugin.Name, &req, nil)

	resp := getCapabilitiesResponse{Scope: plugin.scope}
	err := plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles CreateNetwork requests.
func (plugin *netPlugin) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	plugin.Lock()
	if plugin.networkExists(req.NetworkID) {
		plugin.listener.SendError(w, "Network with same Id already exists")
		plugin.Unlock()
		return
	}

	plugin.networks[req.NetworkID] = &azureNetwork{networkId: req.NetworkID}
	plugin.Unlock()

	// Encode response.
	resp := createNetworkResponse{}
	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles DeleteNetwork requests.
func (plugin *netPlugin) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req deleteNetworkRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	plugin.Lock()
	if plugin.networkExists(req.NetworkID) {
		delete(plugin.networks, req.NetworkID)
	}
	plugin.Unlock()

	// Encode response.
	resp := deleteNetworkResponse{}
	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles CreateEndpoint requests.
func (plugin *netPlugin) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var req createEndpointRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	netID := req.NetworkID
	endID := req.EndpointID

	var interfaceToAttach string
	var ipaddressToAttach string

	for key, value := range req.Options {
		if key == "eth" {
			interfaceToAttach = value.(string)
			log.Printf("Received request to attach following interface: %s", value)
		}

		if key == "com.docker.network.endpoint.ipaddresstoattach" {
			ipaddressToAttach = value.(string)
			log.Printf("Received request to attach following ipaddress: %s", value)
		}
	}

	// The values in that interface can be empty (in case of null ipam driver)
	// or they can contain some pre filled values (if ipam allocates ip addresses)
	if req.Interface != nil {
		ipaddressToAttach = req.Interface.Address
	}

	plugin.Lock()

	if !plugin.networkExists(netID) {
		plugin.Unlock()
		plugin.listener.SendError(w, fmt.Sprintf("Could not find [networkID:%s]\n", netID))
		return
	}
	if plugin.endpointExists(netID, endID) {
		plugin.Unlock()
		plugin.listener.SendError(w, fmt.Sprintf("Endpoint already exists [networkID:%s endpointID:%s]\n", netID, endID))
		return
	}

	rAddress,
		rAddressIPV6,
		rMacAddress,
		rID,
		rSrcName,
		rDstPrefix,
		rGatewayIPv4, ermsg := core.GetTargetInterface(interfaceToAttach, ipaddressToAttach)

	if ermsg != "" {
		plugin.Unlock()
		plugin.listener.SendError(w, ermsg)
		return
	}

	targetInterface := azureInterface{
		Address:     rAddress,
		AddressIPV6: rAddressIPV6,
		MacAddress:  rMacAddress,
		ID:          rID,
		SrcName:     rSrcName,
		DstPrefix:   rDstPrefix,
		GatewayIPv4: rGatewayIPv4,
	}
	network := plugin.networks[netID]
	if network.endpoints == nil {
		network.endpoints = make(map[string]*azureEndpoint)
	}
	network.endpoints[endID] = &azureEndpoint{endpointID: endID, networkID: netID}
	network.endpoints[endID].azureInterface = targetInterface

	plugin.Unlock()

	// Encode response.
	epInterface := endpointInterface{
		Address:     targetInterface.Address.String(),
		MacAddress:  targetInterface.MacAddress.String(),
		GatewayIPv4: targetInterface.GatewayIPv4.String(),
	}
	resp := createEndpointResponse{
		Interface: &epInterface,
	}

	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles Join requests.
func (plugin *netPlugin) join(w http.ResponseWriter, r *http.Request) {
	var req joinRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	endID := req.EndpointID
	netID := req.NetworkID
	sandboxKey := req.SandboxKey

	if !plugin.endpointExists(netID, endID) {
		plugin.listener.SendError(w, "cannot find endpoint for which join is requested")
		return
	}

	endpoint := plugin.networks[netID].endpoints[endID]

	plugin.Lock()
	endpoint.sandboxKey = sandboxKey
	plugin.Unlock()

	// Encode response.
	ifname := interfaceName{
		SrcName:   endpoint.azureInterface.SrcName,
		DstPrefix: endpoint.azureInterface.DstPrefix,
	}

	resp := joinResponse{
		InterfaceName: ifname,
		Gateway:       endpoint.azureInterface.GatewayIPv4.String(),
	}

	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles DeleteEndpoint requests.
func (plugin *netPlugin) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var req deleteEndpointRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	netID := req.NetworkID
	endID := req.EndpointID

	plugin.Lock()
	defer plugin.Unlock()
	if !plugin.endpointExists(netID, endID) {
		// idempotent or throw error?
		fmt.Println("Endpoint not found network: ", netID, " endpointID: ", endID)
	} else {
		network := plugin.networks[netID]
		ep := network.endpoints[endID]
		iface := ep.azureInterface
		err = core.CleanupAfterContainerDeletion(iface.SrcName, iface.MacAddress)
		if err != nil {
			log.Printf("%s: DeleteEndpoint cleanup failure %s", plugin.Name, err.Error())
		}
		delete(network.endpoints, endID)
	}

	// Encode response.
	resp := deleteEndpointResponse{}
	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles Leave requests.
func (plugin *netPlugin) leave(w http.ResponseWriter, r *http.Request) {
	var req leaveRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	// Encode response.
	resp := leaveResponse{}
	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Handles EndpointOperInfo requests.
func (plugin *netPlugin) endpointOperInfo(w http.ResponseWriter, r *http.Request) {
	var req endpointOperInfoRequest

	// Decode request.
	err := plugin.listener.Decode(w, r, &req)
	log.Request(plugin.Name, &req, err)
	if err != nil {
		return
	}

	value := make(map[string]interface{})
	//value["com.docker.network.endpoint.macaddress"] = macAddress
	//value["MacAddress"] = macAddress

	// Encode response.
	resp := endpointOperInfoResponse{Value: value}
	err = plugin.listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
