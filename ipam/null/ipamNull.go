// Copyright Microsoft Corp.
// All rights reserved.

package ipamNull

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

    "github.com/sharmasushant/penguin/core"
)

// Libnetwork IPAM plugin name
const pluginName = "nullipam"

// Libnetwork IPAM plugin endpoint name
const endpointName = "IpamDriver"

// IpamPlugin object and interface
type ipamPlugin struct {
    version string
    listener *core.Listener
	sync.Mutex
}

type IpamPlugin interface {
    Start(chan error) error
    Stop()
}

// Creates a new IpamPlugin object.
func NewPlugin(version string) (IpamPlugin, error) {
    return &ipamPlugin {
        version: version,
        }, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(errChan chan error) error {

    // Create the listener.
    listener, err := core.NewListener(pluginName)
    if err != nil {
        fmt.Printf("Failed to create listener %v", err)
		return err
    }

    // Add protocol handlers.
    listener.AddHandler("Plugin", "Activate", plugin.activatePlugin)
    listener.AddHandler(endpointName, "GetCapabilities", plugin.getCapabilities)
	listener.AddHandler(endpointName, "GetDefaultAddressSpaces", plugin.getDefaultAddressSpaces)
	listener.AddHandler(endpointName, "RequestPool", plugin.requestPool)
	listener.AddHandler(endpointName, "ReleasePool", plugin.releasePool)
	listener.AddHandler(endpointName, "RequestAddress", plugin.requestAddress)
	listener.AddHandler(endpointName, "ReleaseAddress", plugin.releaseAddress)

    plugin.listener = listener

	err = listener.Start(errChan)
	if err != nil {
		fmt.Printf("Failed to start listener %v", err)
		return err
	}

    fmt.Println("IPAM plugin started.")

    return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.listener.Stop()
    fmt.Println("IPAM plugin stopped.")
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

func sendResponse(w http.ResponseWriter, response interface{}, errMessage string, successMessage string){
	encoder := json.NewEncoder(w)
	err := encoder.Encode(response)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		fmt.Println("errMessage:", err)
		return
	}
	fmt.Println(successMessage)
}

func decodeReceivedRequest(w http.ResponseWriter, r *http.Request, request interface{}, errMessage string, successMessage string){

	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		errorMessage := errMessage + err.Error()
		fmt.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}
	fmt.Println(fmt.Sprintf("%s: %+v", successMessage, request))
}

func setErrorInResponseWriter(w http.ResponseWriter, errMessage string){
	fmt.Println(errMessage)
	json.NewEncoder(w).Encode(map[string]string{"Err": errMessage,})
}

type activationResponse struct {
	Implements []string
}

func (plugin *ipamPlugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	response := &activationResponse{[]string{"IpamDriver"}}
	sendResponse(w, response,
		"error activating ipam plugin",
		"Ipam plugin activation finished")
}

func (plugin *ipamPlugin) getCapabilities(w http.ResponseWriter, r *http.Request) {
	capabilities := map[string]string{"Scope": "local"}
	sendResponse(w, capabilities,
		"error getting capabilities:",
		fmt.Sprintf("returned following capabilites %+v", capabilities))
}

type defaultAddressSpacesResponseFormat struct {
	LocalDefaultAddressSpace string
	GlobalDefaultAddressSpace string
}

func (plugin *ipamPlugin) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Get default address space request received")

	response := &defaultAddressSpacesResponseFormat{
		LocalDefaultAddressSpace: "",
		GlobalDefaultAddressSpace: "",
	}
	sendResponse(w, response,
		"error getDefaultAddressSpaces",
		"successfully returned empty default address spaces")
}

type requestPoolRequestFormat struct {
	AddressSpace 	string
	Pool			string
	SubPool			string
	Options			map[string]string
	V6				bool
}

type requestPoolResponseFormat struct {
	PoolID	string
	Pool	string
	Data	map[string]string
}

func (plugin *ipamPlugin) requestPool(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Request pool request received")
	var requestPoolRequest requestPoolRequestFormat

	decodeReceivedRequest(w, r, &requestPoolRequest,
		"Error decoding request pool request",
		"Successfully decoded a request pool request")

	data := make(map[string]string)

	response := &requestPoolResponseFormat{"", "0.0.0.0/8", data}
	sendResponse(w, response,
		"error responding to request pool",
		"Responded to request pool with empty response")
}

type releasePoolRequestFormat struct{
	PoolID	string
}

type releasePoolResponseFormat struct{
}

func (plugin *ipamPlugin) releasePool(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Release pool request received")

	var releasePoolRequest releasePoolRequestFormat

	decodeReceivedRequest(w, r, &releasePoolRequest,
		"Error decoding release pool request",
		"Successfully decoded a release pool request")

	response := &releasePoolRequestFormat{}

	sendResponse(w, response,
		"error responding to relase pool request capabilities:",
		fmt.Sprintf("successfully responded to release pool request for poolId: %+v", releasePoolRequest.PoolID))
}

type requestAddressRequestFormat struct {
	PoolID	string
	Address	string
	Options	map[string]string
}

type requestAddressResponseFormat struct {
	PoolID	string
	Address	string
	Options	map[string]string
}

func (plugin *ipamPlugin) requestAddress(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Received request to reserve an ip address.")

	var requestAddressRequest requestAddressRequestFormat

	decodeReceivedRequest(w, r, &requestAddressRequest,
		"Error decoding request for reserving ip address",
		"Successfully decoded request for reserving ip address")

	response := &requestPoolResponseFormat{"", "", make(map[string]string)}
	sendResponse(w, response,
		"error responding to ip addess reservation request",
		"successfully responded to ip address reservation request")
}

type releaseAddressRequestFormat struct {
	PoolID	string
	Address	string
}

func (plugin *ipamPlugin) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var releaseAddressRequest releaseAddressRequestFormat

	decodeReceivedRequest(w, r, &releaseAddressRequest,
		"Error decoding release Address request",
		"Successfully decoded release address request")

	// docker do not expect anything in response to release Address call
	json.NewEncoder(w).Encode(map[string]string{})
	fmt.Printf("Release address %s.\n", releaseAddressRequest.Address)
}
