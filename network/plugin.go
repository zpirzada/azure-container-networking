// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/sharmasushant/penguin/core"
)

// Libnetwork network plugin name
const pluginName = "penguin"

// Libnetwork network plugin endpoint name
const endpointName = "NetworkDriver"

// NetPlugin object and its interface
type netPlugin struct {
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
		version: version,
		scope:   "local",
	}, nil
}

// Starts the plugin.
func (plugin *netPlugin) Start(errChan chan error) error {

	// Create the listener.
	listener, err := core.NewListener(pluginName)
	if err != nil {
		fmt.Printf("Failed to create listener %v", err)
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
		fmt.Printf("Failed to start listener %v", err)
		return err
	}

	fmt.Println("Network plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *netPlugin) Stop() {
	plugin.listener.Stop()
	fmt.Println("Network plugin stopped.")
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

func (plugin *netPlugin) status(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintln("azure network plugin", plugin.version))
}

type activateResponse struct {
	Implements []string
}

func (plugin *netPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	core.LogRequest(pluginName, "Activate", nil)

	resp := &activateResponse{[]string{endpointName}}
	err := plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "Activate", resp, err)
}

func (plugin *netPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	core.LogRequest(pluginName, "GetCapabilities", nil)

	resp := map[string]string{"Scope": plugin.scope}
	err := plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "GetCapabilities", resp, err)
}

// All request and response formats are well known and are published by libnetwork
type createNetworkRequestFormat struct {
	NetworkID string
	Options   map[string]interface{}
}

func (plugin *netPlugin) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "CreateNetwork", err)

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

	core.LogResponse(pluginName, "CreateNetwork", resp, err)

	fmt.Println("Persisted network creation request for network:", netID)
}

type networkDeleteRequestFormat struct {
	NetworkID string
}

func (plugin *netPlugin) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req networkDeleteRequestFormat

	err := plugin.listener.Decode(w, r, &req)

	core.LogRequest(pluginName, "DeleteNetwork", err)

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

	core.LogResponse(pluginName, "DeleteNetwork", resp, err)

	fmt.Printf("Deleted network %s.\n", req.NetworkID)
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

	core.LogRequest(pluginName, "CreateEndpoint", err)

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
			fmt.Println("Received request to attach following interface: ", value)
		}

		if key == "com.docker.network.endpoint.ipaddresstoattach" {
			ipaddressToAttach = value.(string)
			fmt.Println("Received request to attach following ipaddress: ", value)
		}
	}

	// Now with ipam driver, docker will provide an interface
	// The values in that interface can be empty (in case of null ipam driver)
	// or they can contain some pre filled values (if ipam allocates ip addresses)
	if req.Interface != nil {
		ipaddressToAttach := req.Interface.Address
		message :=
			fmt.Sprintf(`Interface found in endpoint creation request:
			Addr:%s, ID:%v, Ipv6:%s, DstPrefix:%s, GatewayIpv4:%s, MacAddress:%s, SrcName:%s`,
				ipaddressToAttach, req.Interface.ID,
				req.Interface.AddressIPV6,
				req.Interface.DstPrefix, req.Interface.GatewayIPv4,
				req.Interface.MacAddress, req.Interface.SrcName)
		fmt.Println(message)
		//plugin.listener.SendError(w, errMessage)
		//return
	}

	fmt.Printf("Trying to create an endpoint\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

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

	fmt.Printf("Endpoint created successfully "+
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

	core.LogResponse(pluginName, "CreateEndpoint", resp, err)

	fmt.Printf("Created endpoint %s: %+v", endID, resp)
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

	core.LogRequest(pluginName, "Join", err)

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

	core.LogResponse(pluginName, "Join", resp, err)

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

	core.LogRequest(pluginName, "DeleteEndpoint", err)

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

	core.LogResponse(pluginName, "DeleteEndpoint", resp, err)

	fmt.Printf("Deleted endpoint %s", req.EndpointID)
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

	core.LogRequest(pluginName, "Leave", err)

	if err != nil {
		return
	}

	// Empty response indicates success.
	resp := &leaveResponse{}
	err = plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "Leave", resp, err)

	fmt.Printf("Successfully executed leave\n Network: %s\n Endpoint: %s \n",
		req.NetworkID, req.EndpointID)
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

	core.LogRequest(pluginName, "EndpointOperInfo", err)

	if err != nil {
		return
	}

	value := make(map[string]interface{})
	//value["com.docker.network.endpoint.macaddress"] = macAddress
	//value["MacAddress"] = macAddress

	resp := &endpointOperInfoResponseFormat{Value: value}
	err = plugin.listener.Encode(w, resp)

	core.LogResponse(pluginName, "EndpointOperInfo", resp, err)

	fmt.Println("Successfully responded to endpoint Oper Info request: ", req.EndpointID)
}
