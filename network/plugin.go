// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/sharmasushant/penguin/core"
	"github.com/sharmasushant/penguin/log"
)

// Libnetwork network plugin name
const pluginName = "penguin"

// Libnetwork network plugin endpoint name
const endpointName = "NetworkDriver"

// NetPlugin object and its interface
type netPlugin struct {
	name     string
	version  string
	scope    string
	listener *core.Listener
	networks map[string]*azureNetwork
	sync.Mutex
}

type NetPlugin interface {
	Start(chan error) error
	Stop()
}

// Creates a new NetPlugin object.
func NewPlugin(version string) (NetPlugin, error) {
	return &netPlugin{
		name:    pluginName,
		version: version,
		scope:   "local",
	}, nil
}

// Starts the plugin.
func (plugin *netPlugin) Start(errChan chan error) error {

	// Create the listener.
	listener, err := core.NewListener(plugin.name)
	if err != nil {
		log.Printf("Failed to create listener %v", err)
		return err
	}

	// Add protocol handlers.
	listener.AddHandler("Plugin", "Activate", plugin.activatePlugin)
	listener.AddHandler(endpointName, "GetCapabilities", plugin.getCapabilities)
	listener.AddHandler(endpointName, "CreateNetwork", plugin.createNetwork)
	listener.AddHandler(endpointName, "DeleteNetwork", plugin.deleteNetwork)
	listener.AddHandler(endpointName, "CreateEndpoint", plugin.createEndpoint)
	listener.AddHandler(endpointName, "DeleteEndpoint", plugin.deleteEndpoint)
	listener.AddHandler(endpointName, "Join", plugin.join)
	listener.AddHandler(endpointName, "Leave", plugin.leave)
	listener.AddHandler(endpointName, "EndpointOperInfo", plugin.endpointOperInfo)

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
func (plugin *netPlugin) Stop() {
	plugin.listener.Stop()
	log.Printf("%s: Plugin stopped.\n", plugin.name)
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

type activateResponse struct {
	Implements []string
}

func (plugin *netPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.name, "Activate", nil, nil)

	resp := &activateResponse{[]string{endpointName}}
	err := plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "Activate", resp, err)
}

func (plugin *netPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.name, "GetCapabilities", nil, nil)

	resp := map[string]string{"Scope": plugin.scope}
	err := plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "GetCapabilities", resp, err)
}

// All request and response formats are well known and are published by libnetwork
type createNetworkRequestFormat struct {
	NetworkID string
	Options   map[string]interface{}
}

func (plugin *netPlugin) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "CreateNetwork", req, err)

	if err != nil {
		return
	}

	netID := req.NetworkID
	if plugin.networkExists(netID) {
		plugin.listener.SendError(w, "Network with same Id already exists")
		return
	}

	plugin.Lock()
	if plugin.networkExists(netID) {
		plugin.listener.SendError(w, "Network with same Id already exists")
		plugin.Unlock()
		return
	}

	if plugin.networks == nil {
		plugin.networks = make(map[string]*azureNetwork)
	}

	plugin.networks[netID] = &azureNetwork{networkId: netID}
	plugin.Unlock()

	// Empty response indicates success.
	resp := map[string]string{}
	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "CreateNetwork", resp, err)
}

type networkDeleteRequestFormat struct {
	NetworkID string
}

func (plugin *netPlugin) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req networkDeleteRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "DeleteNetwork", req, err)

	if err != nil {
		return
	}

	if plugin.networkExists(req.NetworkID) {
		plugin.Lock()
		if plugin.networkExists(req.NetworkID) {
			delete(plugin.networks, req.NetworkID)
		}
		plugin.Unlock()
	}

	// Empty response indicates success.
	resp := map[string]string{}
	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "DeleteNetwork", resp, err)
}

type azInterface struct {
	Address     string
	AddressIPV6 string
	MacAddress  string
	ID          int
	SrcName     string
	DstPrefix   string
	GatewayIPv4 string
}

type createEndpointRequestFormat struct {
	NetworkID  string
	EndpointID string
	Options    map[string]interface{}
	Interface  *azInterface
}

type endpointResponse struct {
	Interface azInterface
}

func (plugin *netPlugin) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var req createEndpointRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "CreateEndpoint", req, err)

	if err != nil {
		return
	}

	netID := req.NetworkID
	endID := req.EndpointID

	if !plugin.networkExists(netID) {
		plugin.listener.SendError(w, fmt.Sprintf("Could not find the network on which endpoint is requested: %s", netID))
		return
	}

	var interfaceToAttach string
	interfaceToAttach = ""
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

	// Now with ipam driver, docker will provide an interface
	// The values in that interface can be empty (in case of null ipam driver)
	// or they can contain some pre filled values (if ipam allocates ip addresses)
	if req.Interface != nil {
		ipaddressToAttach := req.Interface.Address
		log.Printf(
			"Interface found in endpoint creation request: " +
			"Addr:%s, ID:%v, Ipv6:%s, DstPrefix:%s, GatewayIpv4:%s, MacAddress:%s, SrcName:%s",
			ipaddressToAttach, req.Interface.ID,
			req.Interface.AddressIPV6,
			req.Interface.DstPrefix, req.Interface.GatewayIPv4,
			req.Interface.MacAddress, req.Interface.SrcName)
		//plugin.listener.SendError(w, errMessage)
		//return
	}

	log.Printf("Trying to create an endpoint\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

	// lets lock driver for now.. will optimize later
	plugin.Lock()
	if !plugin.networkExists(netID) {
		plugin.listener.SendError(w, fmt.Sprintf("Could not find [networkID:%s]\n", netID))
		return
	}
	if plugin.endpointExists(netID, endID) {
		plugin.listener.SendError(w, fmt.Sprintf("Endpoint already exists [networkID:%s endpointID:%s]\n", netID, endID))
		return
	}

	log.Printf("Endpoint created successfully "+
		"\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

	rAddress,
		rAddressIPV6,
		rMacAddress,
		rID,
		rSrcName,
		rDstPrefix,
		rGatewayIPv4, ermsg := core.GetInterfaceToAttach(interfaceToAttach, ipaddressToAttach)

	if ermsg != "" {
		plugin.listener.SendError(w, ermsg)
		plugin.Unlock()
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

	/*defer func() {
	if err != nil {
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()
	}
	}()*/

	/********END**********/

	respIface := &azInterface{
		Address:     targetInterface.Address.String(),
		MacAddress:  targetInterface.MacAddress.String(),
		GatewayIPv4: targetInterface.GatewayIPv4.String(),
	}
	resp := &endpointResponse{
		Interface: *respIface,
	}

	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "CreateEndpoint", resp, err)
}

type joinRequestFormat struct {
	NetworkID  string
	EndpointID string
	SandboxKey string
	Options    map[string]interface{}
}

type interfaceToJoin struct {
	SrcName   string
	DstPrefix string
}

type joinResponseFormat struct {
	InterfaceName interfaceToJoin
	Gateway       string
	GatewayIPv6   string
	StaticRoutes  []*staticRoute
}

type staticRoute struct {
	Destination string
	RouteType   int
	NextHop     string
}

func (plugin *netPlugin) join(w http.ResponseWriter, r *http.Request) {
	var req joinRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "Join", req, err)

	if err != nil {
		return
	}

	endID := req.EndpointID
	netID := req.NetworkID
	sandboxKey := req.SandboxKey
	fmt.Println("Received a request to join endpoint: ", endID, " network: ", netID)

	if !plugin.endpointExists(netID, endID) {
		plugin.listener.SendError(w, "cannot find endpoint for which join is requested")
		return
	}

	endpoint := plugin.networks[netID].endpoints[endID]

	ifname := &interfaceToJoin{
		SrcName:   endpoint.azureInterface.SrcName,
		DstPrefix: endpoint.azureInterface.DstPrefix,
	}

	resp := &joinResponseFormat{
		InterfaceName: *ifname,
		Gateway:       endpoint.azureInterface.GatewayIPv4.String(),
	}

	plugin.Lock()
	endpoint.sandboxKey = sandboxKey
	plugin.Unlock()

	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "Join", resp, err)

	fmt.Printf("srcname: %s dstPRefix:%s \n", ifname.SrcName, ifname.DstPrefix)

	fmt.Printf("Joined endpoint\n Network: %s\n Endpoint: %s\n Sandbox: %s\n",
		req.NetworkID, req.EndpointID, req.SandboxKey)
}

type endpointDeleteRequestFormat struct {
	NetworkID  string
	EndpointID string
}

func (plugin *netPlugin) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var req endpointDeleteRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "DeleteEndpoint", req, err)

	if err != nil {
		return
	}

	netID := req.NetworkID
	endID := req.EndpointID

	plugin.Lock()
	if !plugin.endpointExists(netID, endID) {
		// idempotent or throw error?
		fmt.Println("Endpoint not found network: ", netID, " endpointID: ", endID)
	} else {
		network := plugin.networks[netID]
		delete(network.endpoints, endID)
	}
	plugin.Unlock()

	// Empty response indicates success.
	resp := &map[string]string{}
	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "DeleteEndpoint", resp, err)
}

type leaveRequestFormat struct {
	NetworkID  string
	EndpointID string
}

type leaveResponse struct {
}

func (plugin *netPlugin) leave(w http.ResponseWriter, r *http.Request) {
	var req leaveRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "Leave", req, err)

	if err != nil {
		return
	}

	// Empty response indicates success.
	resp := &leaveResponse{}
	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "Leave", resp, err)
}

type endpointOperInfoRequestFormat struct {
	NetworkID  string
	EndpointID string
}

type endpointOperInfoResponseFormat struct {
	Value map[string]interface{}
}

func (plugin *netPlugin) endpointOperInfo(w http.ResponseWriter, r *http.Request) {
	var req endpointOperInfoRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	log.Request(plugin.name, "EndpointOperInfo", req, err)

	if err != nil {
		return
	}

	value := make(map[string]interface{})
	//value["com.docker.network.endpoint.macaddress"] = macAddress
	//value["MacAddress"] = macAddress

	resp := &endpointOperInfoResponseFormat{Value: value}
	err = plugin.listener.Encode(w, resp)

	log.Response(plugin.name, "EndpointOperInfo", resp, err)
}
