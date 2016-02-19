// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"encoding/json"
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
    version string
    scope string
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
    return &netPlugin {
        version:    version,
        scope:      "local",
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

func router(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Handler invoked")

	switch req.Method {
	case "GET":
		fmt.Println("receiver GET request", req.URL.Path)
	case "POST":
		fmt.Println("receiver POST request", req.URL.Path)
		switch req.URL.Path {
		case "/Plugin.Activate":
			fmt.Println("/Plugin.Activate received")
		}
	default:
		fmt.Println("receiver unexpected request", req.Method, "->", req.URL.Path)
	}
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

type activationResponse struct {
	Implements []string
}

func (plugin *netPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	response := &activationResponse{[]string{"NetworkDriver"}}
	sendResponse(w, response,
		"error activating plugin",
		"Plugin activation finished")
}

func (plugin *netPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	capabilities := map[string]string{"Scope": "local"}
	sendResponse(w, capabilities,
		"error getting capabilities:",
		fmt.Sprintf("returned following capabilites %+v", capabilities))
}

// All request and response formats are well known and are published by libnetwork
type createNetworkRequestFormat struct {
	NetworkID string
	Options   map[string]interface{}
}

func (plugin *netPlugin) createNetwork(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Received a network creation request. Going to check for validity.")

	var createNetworkRequest createNetworkRequestFormat

	decodeReceivedRequest(w, r, &createNetworkRequest,
		"Error decoding create network request",
		"Successfully decoded a network creation request")

	netID := createNetworkRequest.NetworkID
	if plugin.networkExists(netID) {
		setErrorInResponseWriter(w, "Network with same Id already exists")
		return
	}

	plugin.Lock()
	if plugin.networkExists(netID) {
		setErrorInResponseWriter(w, "Network with same Id already exists")
		return
	}
	if plugin.networks == nil {
		plugin.networks = make (map[string]*azureNetwork)
	}
	plugin.networks[netID] = &azureNetwork{networkId: netID}
	plugin.Unlock()

	// docker do not expect anything in response to a create network call
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Println("Persisted network creation request for network:", netID)
}

type networkDeleteRequestFormat struct {
	NetworkID string
}

func (plugin *netPlugin) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var deleteNetworkRequest networkDeleteRequestFormat

	decodeReceivedRequest(w, r, &deleteNetworkRequest,
		"Error decoding delete network request",
		"Successfully decoded a network deletion request")

	deleted := false
	if !plugin.networkExists(deleteNetworkRequest.NetworkID) {
		deleted = true
	}

	if(!deleted){
		plugin.Lock()
		if plugin.networkExists(deleteNetworkRequest.NetworkID) {
			delete(plugin.networks, deleteNetworkRequest.NetworkID)
		}
		plugin.Unlock()
	}

	// docker do not expect anything in response to a delete network call
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Printf("Deleted network %s.\n", deleteNetworkRequest.NetworkID)
}

type azInterface struct{
	Address string
	AddressIPV6 string
	MacAddress string
	ID         int
	SrcName    string
	DstPrefix  string
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
	var createEndpointRequest createEndpointRequestFormat

	decodeReceivedRequest(w, r, &createEndpointRequest,
		"Error decoding create endpoint request",
		"Successfully decoded the endpoint creation request")

	netID := createEndpointRequest.NetworkID
	endID := createEndpointRequest.EndpointID

	if !plugin.networkExists(netID) {
		setErrorInResponseWriter(w, fmt.Sprintf("Could not find the network on which endpoint is requested: %s", netID))
		return
	}

	var interfaceToAttach string
	interfaceToAttach = ""
	var ipaddressToAttach string

	for key, value := range createEndpointRequest.Options {

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
	if createEndpointRequest.Interface != nil {
		ipaddressToAttach := createEndpointRequest.Interface.Address
		message :=
		fmt.Sprintf(`Interface found in endpoint creation request:
			Addr:%s, ID:%v, Ipv6:%s, DstPrefix:%s, GatewayIpv4:%s, MacAddress:%s, SrcName:%s`,
			ipaddressToAttach, createEndpointRequest.Interface.ID,
			createEndpointRequest.Interface.AddressIPV6,
			createEndpointRequest.Interface.DstPrefix, createEndpointRequest.Interface.GatewayIPv4,
			createEndpointRequest.Interface.MacAddress, createEndpointRequest.Interface.SrcName)
		fmt.Println (message)
		//setErrorInResponseWriter(w, errMessage)
		//return
	}

	fmt.Printf("Trying to create an endpoint\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

	// lets lock driver for now.. will optimize later
	plugin.Lock()
	if !plugin.networkExists(netID) {
		setErrorInResponseWriter(w, fmt.Sprintf("Could not find [networkID:%s]\n", netID))
		return
	}
	if plugin.endpointExists(netID, endID) {
		setErrorInResponseWriter(w, fmt.Sprintf("Endpoint already exists [networkID:%s endpointID:%s]\n", netID, endID))
		return
	}

	fmt.Printf("Endpoint created successfully " +
		"\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

	rAddress,
	rAddressIPV6,
	rMacAddress,
	rID,
	rSrcName,
	rDstPrefix,
	rGatewayIPv4, ermsg := core.GetInterfaceToAttach(interfaceToAttach, ipaddressToAttach)

	if(ermsg != "" ){
		setErrorInResponseWriter(w, ermsg)
		plugin.Unlock()
		return
	}

	targetInterface := azureInterface{
		Address: rAddress,
		AddressIPV6: rAddressIPV6,
		MacAddress: rMacAddress,
		ID: rID,
		SrcName: rSrcName,
		DstPrefix: rDstPrefix,
		GatewayIPv4: rGatewayIPv4,
	}
	network := plugin.networks[netID]
	if(network.endpoints == nil){
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
		Address:    targetInterface.Address.String(),
		MacAddress: targetInterface.MacAddress.String(),
		GatewayIPv4: targetInterface.GatewayIPv4.String(),
	}
	resp := &endpointResponse{
		Interface: *respIface,
	}

	sendResponse(w, resp,
		"Failed to send endpoint creation response",
		"Successfully responded with craeted endpoint info")
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
	var joinRequest joinRequestFormat
	decodeReceivedRequest(w, r, &joinRequest,
		"Unable to decode join request",
		"Successfully decoded join request")

	endID := joinRequest.EndpointID
	netID := joinRequest.NetworkID
	sandboxKey := joinRequest.SandboxKey
	fmt.Println("Received a request to join endpoint: ", endID, " network: ", netID)

	if !plugin.endpointExists(netID, endID) {
		setErrorInResponseWriter(w, "cannot find endpoint for which join is requested")
		return
	}
	endpoint := plugin.networks[netID].endpoints[endID]
	ifname := &interfaceToJoin{
		SrcName:   endpoint.azureInterface.SrcName,
		DstPrefix: endpoint.azureInterface.DstPrefix,
	}

	res := &joinResponseFormat{
		InterfaceName: *ifname,
		Gateway: endpoint.azureInterface.GatewayIPv4.String(),
	}

	plugin.Lock()
	endpoint.sandboxKey = sandboxKey
	plugin.Unlock()

	sendResponse(w, res,
		"Failed to send response for endpoint join",
		"Successfully responded with the interface to Join")

	fmt.Printf("srcname: %s dstPRefix:%s \n", ifname.SrcName, ifname.DstPrefix)

	fmt.Printf("Joined endpoint\n Network: %s\n Endpoint: %s\n Sandbox: %s\n",
		joinRequest.NetworkID, joinRequest.EndpointID, joinRequest.SandboxKey)
}

type endpointDeleteRequestFormat struct {
	NetworkID  string
	EndpointID string
}

func (plugin *netPlugin) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var endpointDeleteRequest endpointDeleteRequestFormat

	decodeReceivedRequest(w, r, &endpointDeleteRequest,
		"Unable to decode endpointDeleteRequest",
		"Successfully decoded endpointDeleteRequest")

	netID := endpointDeleteRequest.NetworkID
	endID := endpointDeleteRequest.EndpointID
	plugin.Lock()
	if !plugin.endpointExists(netID, endID) {
		// idempotent or throw error?
		fmt.Println("Endpoint not found network: ", netID, " endpointID: ", endID)
	}else{
		network := plugin.networks[netID]
		delete(network.endpoints, endID)
	}
	plugin.Unlock()
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Printf("Deleted endpoint %s", endpointDeleteRequest.EndpointID)
}

type leaveRequestFormat struct {
	NetworkID  string
	EndpointID string
}

type leaveResponse struct {
}

func (plugin *netPlugin) leave(w http.ResponseWriter, r *http.Request) {
	var leaveRequest leaveRequestFormat

	decodeReceivedRequest(w, r, &leaveRequest,
		"Unable to decode leaveRequest",
		"Successfully decoded leaveRequest")

	fmt.Printf("Successfully executed leave\n Network: %s\n Endpoint: %s \n",
		leaveRequest.NetworkID, leaveRequest.EndpointID)

	res := &leaveResponse{}
	sendResponse(w, res,
		"Failed to send response for leave",
		"Successfully responded to leave")
}

type endpointOperInfoRequestFormat struct {
	NetworkID  string
	EndpointID string
}

type endpointOperInfoResponseFormat struct {
	Value map[string]interface{}
}

func (plugin *netPlugin) endpointOperInfo(w http.ResponseWriter, r *http.Request) {
	var endpointOperInfoRequest endpointOperInfoRequestFormat

	decodeReceivedRequest(w, r, &endpointOperInfoRequest,
		"Unable to decode endpointOperationInfoRequest",
		"Successfully decoded endpointOperationInfoRequest")

	resp := make(map[string]interface{})
	//resp["com.docker.network.endpoint.macaddress"] = macAddress
	// resp["MacAddress"] = macAddress

	res := &endpointOperInfoResponseFormat{Value: resp}

	sendResponse(w, res,
		"Failed to send response for endpointOperationInfoRequest",
		"Successfully responded to endpointOperationInfoRequest")

	fmt.Println("Successfully responded to endpoint Oper Info request: ",
		endpointOperInfoRequest.EndpointID)
}
