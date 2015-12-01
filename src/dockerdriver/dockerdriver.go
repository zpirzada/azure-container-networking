package dockerdriver

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"core"
)

type dockerdriver struct {
	version string
	networks map[string]*azureNetwork
	sync.Mutex
}

type DockerDriver interface {
	StartListening(net.Listener) error
}

func NewInstance(version string) (DockerDriver, error) {

	return &dockerdriver{
		version: version,
	}, nil
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

func (dockerdriver *dockerdriver) StartListening(listener net.Listener) error {

	fmt.Println("Going to listen ...")
	mux := http.NewServeMux()
	mux.HandleFunc("/status", dockerdriver.status)
	mux.HandleFunc("/Plugin.Activate", dockerdriver.activatePlugin)
	mux.HandleFunc("/NetworkDriver.GetCapabilities", dockerdriver.getCapabilities)
	mux.HandleFunc("/NetworkDriver.CreateNetwork", dockerdriver.createNetwork)
	mux.HandleFunc("/NetworkDriver.DeleteNetwork", dockerdriver.deleteNetwork)
	mux.HandleFunc("/NetworkDriver.CreateEndpoint", dockerdriver.createEndpoint)
	mux.HandleFunc("/NetworkDriver.Join", dockerdriver.join)
	mux.HandleFunc("/NetworkDriver.DeleteEndpoint", dockerdriver.deleteEndpoint)
	mux.HandleFunc("/NetworkDriver.Leave", dockerdriver.leave)
	mux.HandleFunc("/NetworkDriver.EndpointOperInfo", dockerdriver.endpointOperInfo)
	fmt.Println("listening ...")
	return http.Serve(listener, mux)
}

func (dockerdriver *dockerdriver) status(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintln("azure network plugin", dockerdriver.version))
}

type activationResponse struct {
	Implements []string
}

func (dockerdriver *dockerdriver) activatePlugin(w http.ResponseWriter, r *http.Request) {
	response := &activationResponse{[]string{"NetworkDriver"}}
	sendResponse(w, response,
		"error activating plugin",
		"Plugin activation finished")
}

func (dockerdriver *dockerdriver) getCapabilities(w http.ResponseWriter, r *http.Request) {
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

func (dockerdriver *dockerdriver) createNetwork(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Received a network creation request. Going to check for validity.")

	var createNetworkRequest createNetworkRequestFormat

	decodeReceivedRequest(w, r, &createNetworkRequest,
		"Error decoding create network request",
		"Successfully decoded a network creation request")

	netID := createNetworkRequest.NetworkID
	if dockerdriver.networkExists(netID) {
		setErrorInResponseWriter(w, "Network with same Id already exists")
		return
	}

	dockerdriver.Lock()
		if dockerdriver.networkExists(netID) {
			setErrorInResponseWriter(w, "Network with same Id already exists")
			return
		}
		if(dockerdriver.networks == nil){
			dockerdriver.networks = make (map[string]*azureNetwork)
		}
		dockerdriver.networks[netID] =
					&azureNetwork{networkId: netID}
	dockerdriver.Unlock()

	// docker do not expect anything in response to a create network call
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Println("Persisted network creation request for network:", netID)
}

type networkDeleteRequestFormat struct {
	NetworkID string
}

func (dockerdriver *dockerdriver) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var deleteNetworkRequest networkDeleteRequestFormat

	decodeReceivedRequest(w, r, &deleteNetworkRequest,
		"Error decoding delete network request",
		"Successfully decoded a network deletion request")

	deleted := false
	if(!dockerdriver.networkExists(deleteNetworkRequest.NetworkID)){
		deleted = true
	}

	if(!deleted){
		dockerdriver.Lock()
			if(dockerdriver.networkExists(deleteNetworkRequest.NetworkID)){
				delete(dockerdriver.networks, deleteNetworkRequest.NetworkID)
			}
		dockerdriver.Unlock()
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

func (dockerdriver *dockerdriver) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var createEndpointRequest createEndpointRequestFormat

	decodeReceivedRequest(w, r, &createEndpointRequest,
		"Error decoding create endpoint request",
		"Successfully decoded the endpoint creation request")

	netID := createEndpointRequest.NetworkID
	endID := createEndpointRequest.EndpointID

	if(!dockerdriver.networkExists(netID)){
		setErrorInResponseWriter(w, fmt.Sprintf("Could not find the network on which endpoint is requested: %s", netID))
		return
	}

	var interfaceToAttach string
	interfaceToAttach = ""
	var ipaddressToAttach string
	ipaddressToAttach = "127.0.0.2"

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

	if createEndpointRequest.Interface != nil {
		errMessage := "Interface in endpoint creation request is not supported." + createEndpointRequest.Interface.Address
		fmt.Println (errMessage)
		setErrorInResponseWriter(w, errMessage)
		return
	}

	fmt.Printf("Trying to create an endpoint\n\tn/w-id:%s \n\tep-id:%s\n", string(netID), string(endID))

	// lets lock driver for now.. will optimize later
	dockerdriver.Lock()
		if(!dockerdriver.networkExists(netID)){
			setErrorInResponseWriter(w, fmt.Sprintf("Could not find [networkID:%s]\n", netID))
			return
		}
		if(dockerdriver.endpointExists(netID, endID)){
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
 		rGatewayIPv4, ermsg := azure.GetInterfaceToAttach(interfaceToAttach, ipaddressToAttach)

		if(ermsg != "" ){
			setErrorInResponseWriter(w, ermsg)
			dockerdriver.Unlock()
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
		network := dockerdriver.networks[netID]
		if(network.endpoints == nil){
			network.endpoints = make(map[string]*azureEndpoint)
		}
		network.endpoints[endID] = &azureEndpoint{endpointID: endID, networkID: netID}
		network.endpoints[endID].azureInterface = targetInterface

	dockerdriver.Unlock()

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

func (dockerdriver *dockerdriver) join(w http.ResponseWriter, r *http.Request) {
	var joinRequest joinRequestFormat
	decodeReceivedRequest(w, r, &joinRequest,
		"Unable to decode join request",
		"Successfully decoded join request")

	endID := joinRequest.EndpointID
	netID := joinRequest.NetworkID
	sandboxKey := joinRequest.SandboxKey
	fmt.Println("Received a request to join endpoint: ", endID, " network: ", netID)

	if(!dockerdriver.endpointExists(netID, endID)){
		setErrorInResponseWriter(w, "cannot find endpoint for which join is requested")
		return
	}
	endpoint := dockerdriver.networks[netID].endpoints[endID]
	ifname := &interfaceToJoin{
		SrcName:   endpoint.azureInterface.SrcName,
		DstPrefix: endpoint.azureInterface.DstPrefix,
	}

	res := &joinResponseFormat{
		InterfaceName: *ifname,
		Gateway: endpoint.azureInterface.GatewayIPv4.String(),
	}
	dockerdriver.Lock()
		endpoint.sandboxKey = sandboxKey
	dockerdriver.Unlock()
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

func (dockerdriver *dockerdriver) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var endpointDeleteRequest endpointDeleteRequestFormat

	decodeReceivedRequest(w, r, &endpointDeleteRequest,
		"Unable to decode endpointDeleteRequest",
		"Successfully decoded endpointDeleteRequest")

	netID := endpointDeleteRequest.NetworkID
	endID := endpointDeleteRequest.EndpointID
	dockerdriver.Lock()
		if(!dockerdriver.endpointExists(netID, endID)){
			// idempotent or throw error?
			fmt.Println("Endpoint not found network: ", netID, " endpointID: ", endID)
		}else{
			network := dockerdriver.networks[netID]
			delete(network.endpoints, endID)
		}
  dockerdriver.Unlock()
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Printf("Deleted endpoint %s", endpointDeleteRequest.EndpointID)
}

type leaveRequestFormat struct {
	NetworkID  string
	EndpointID string
}

type leaveResponse struct {
}

func (dockerdriver *dockerdriver) leave(w http.ResponseWriter, r *http.Request) {
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

func (dockerdriver *dockerdriver) endpointOperInfo(w http.ResponseWriter, r *http.Request) {
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
