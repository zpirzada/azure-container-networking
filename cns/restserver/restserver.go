// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"time"
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/dockerclient"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/store"
	"fmt"
)

const (
	// Key against which CNS state is persisted.
	storeKey = "ContainerNetworkService"
)

// httpRestService represents http listener for CNS - Container Networking Service.
type httpRestService struct {
	*cns.Service
	dockerClient *dockerclient.DockerClient
	store store.KeyValueStore
	state httpRestServiceState	
}

// httpRestServiceState contrains the state we would like to persist
type httpRestServiceState struct {
	Location 	string
	NetworkType	string	
	TimeStamp	time.Time	
}

// HTTPService describes the min API interface that every service should have.
type HTTPService interface {
	common.ServiceAPI
}

// NewHTTPRestService creates a new HTTP Service object.
func NewHTTPRestService(config *common.ServiceConfig) (HTTPService, error) {
	service, err := cns.NewService(config.Name, config.Version, config.Store)
	if err != nil {
		return nil, err
	}

	dc, err := dockerclient.NewDefaultDockerClient()
	if(err != nil){
		return nil, err
	}

	return &httpRestService{
		Service: service,
		store: service.Service.Store,
		dockerClient: dc,
	}, nil
}

// Start starts the CNS listener.
func (service *httpRestService) Start(config *common.ServiceConfig) error {

	err := service.Initialize(config)
	if err != nil {
		log.Printf("[Azure CNS]  Failed to initialize base service, err:%v.", err)
		return err
	}

	// Add handlers.
	listener := service.Listener
	listener.AddHandler(cns.SetEnvironmentPath, service.setEnvironment)
	listener.AddHandler(cns.CreateNetworkPath, service.createNetwork)
	listener.AddHandler(cns.DeleteNetworkPath, service.deleteNetwork)
	listener.AddHandler(cns.ReserveIPAddressPath, service.reserveIPAddress)
	listener.AddHandler(cns.ReleaseIPAddressPath, service.releaseIPAddress)
	listener.AddHandler(cns.GetHostLocalIPPath, service.getHostLocalIP)
	listener.AddHandler(cns.GetIPAddressUtilizationPath, service.getIPAddressUtilization)
	listener.AddHandler(cns.GetAvailableIPAddressesPath, service.getAvailableIPAddresses)
	listener.AddHandler(cns.GetReservedIPAddressesPath, service.getReservedIPAddresses)
	listener.AddHandler(cns.GetGhostIPAddressesPath, service.getGhostIPAddresses)
	listener.AddHandler(cns.GetAllIPAddressesPath, service.getAllIPAddresses)
	listener.AddHandler(cns.GetHealthReportPath, service.getHealthReport)

	log.Printf("[Azure CNS]  Listening.")
	return nil
}

// Stop stops the CNS.
func (service *httpRestService) Stop() {
	service.Uninitialize()
	log.Printf("[Azure CNS]  Service stopped.")
}

// Handles requests to set the environment type.
func (service *httpRestService) setEnvironment(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] setEnvironment")
	var req cns.SetEnvironmentRequest
	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}

	switch r.Method {
	case "POST":
		log.Printf("[Azure CNS]  POST received for SetEnvironment.")
		service.state.Location = req.Location
		service.state.NetworkType = req.NetworkType		
		service.saveState()
	default:
	}

	resp := &cns.Response{ReturnCode: 0}
	err = service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// Handles CreateNetwork requests.
func (service *httpRestService) createNetwork(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] createNetwork")
	var req cns.CreateNetworkRequest
	returnCode := 0
	returnMessage := ""
	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}
	switch r.Method {
		case "POST":
			dc := service.dockerClient
			err := dc.NetworkExists(req.NetworkName)
			
			// Network does not exist
			if(err != nil) {
				log.Printf("[Azure CNS] Goign to create network with name %v", req.NetworkName)
				err := dc.CreateNetwork(req.NetworkName)
				if(err != nil) {
					returnMessage = fmt.Sprintf("[Azure CNS] Error. CreateNetwork failed %v.", err.Error())
					returnCode = UnexpectedError
				}
			} else {
				log.Printf("[Azure CNS] Received a request to create an already existing network %v", req.NetworkName)
			}
			
		default:
			returnMessage = "[Azure CNS] Error. CreateNetwork did not receive a POST."			
			returnCode = InvalidParameter
	}

	resp := &cns.Response{
		ReturnCode: returnCode, 
		Message: returnMessage,
	}
	err = service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// Handles DeleteNetwork requests.
func (service *httpRestService) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] deleteNetwork")
	var req cns.DeleteNetworkRequest
	returnCode := 0
	returnMessage := ""
	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}
	switch r.Method {
		case "POST":
			dc := service.dockerClient
			err := dc.NetworkExists(req.NetworkName)
			
			// Network does exist
			if(err == nil) {
				log.Printf("[Azure CNS] Goign to delete network with name %v", req.NetworkName)
				err := dc.DeleteNetwork(req.NetworkName)
				if(err != nil) {
					returnMessage = fmt.Sprintf("[Azure CNS] Error. DeleteNetwork failed %v.", err.Error())
					returnCode = UnexpectedError
				}
			} else {
				log.Printf("[Azure CNS] Received a request to delete network that does not exist: %v", req.NetworkName)
			}
			
		default:
			returnMessage = "[Azure CNS] Error. DeleteNetwork did not receive a POST."			
			returnCode = InvalidParameter
	}

	resp := &cns.Response{
		ReturnCode: returnCode, 
		Message: returnMessage,
	}
	err = service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// Handles ip reservation requests.
func (service *httpRestService) reserveIPAddress(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] reserveIPAddress")
	var req cns.ReserveIPAddressRequest

	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}
	switch r.Method {
		case "POST":
		default:
	}

	resp := cns.Response{ReturnCode: 0}
	reserveResp := &cns.ReserveIPAddressResponse{Response: resp, IPAddress: "0.0.0.0"}
	err = service.Listener.Encode(w, &reserveResp)
	log.Response(service.Name, reserveResp, err)
}

// Handles release ip reservation requests.
func (service *httpRestService) releaseIPAddress(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] releaseIPAddress")
	var req cns.ReleaseIPAddressRequest

	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}
	switch r.Method {
		case "POST":
		default:
	}

	resp := &cns.Response{ReturnCode: 0}
	err = service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// Retrieves the host local ip address.
func (service *httpRestService) getHostLocalIP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getHostLocalIP")
	log.Request(service.Name, "getHostLocalIP", nil)	
	switch r.Method {
		case "GET":
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	hostLocalIPResponse := &cns.HostLocalIPAddressResponse{
		Response:  resp,
		IPAddress: "0.0.0.0",
	}
	err := service.Listener.Encode(w, &hostLocalIPResponse)
	log.Response(service.Name, hostLocalIPResponse, err)
}

// Handles ip address utiliztion requests.
func (service *httpRestService) getIPAddressUtilization(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getIPAddressUtilization")
	log.Request(service.Name, "getIPAddressUtilization", nil)	
	switch r.Method {
		case "GET":
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	utilResponse := &cns.IPAddressesUtilizationResponse{
		Response:  resp,
		Available: 0,
	}
	err := service.Listener.Encode(w, &utilResponse)
	log.Response(service.Name, utilResponse, err)
}

// Handles retrieval of ip addresses that are available to be reserved from ipam driver.
func (service *httpRestService) getAvailableIPAddresses(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getAvailableIPAddresses")
	log.Request(service.Name, "getAvailableIPAddresses", nil)
	switch r.Method {
		case "GET":
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	ipResp := &cns.GetIPAddressesResponse{Response: resp}
	err := service.Listener.Encode(w, &ipResp)
	log.Response(service.Name, ipResp, err)
}

// Handles retrieval of reserved ip addresses from ipam driver.
func (service *httpRestService) getReservedIPAddresses(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getReservedIPAddresses")
	log.Request(service.Name, "getReservedIPAddresses", nil)
	switch r.Method {
		case "GET":	
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	ipResp := &cns.GetIPAddressesResponse{Response: resp}
	err := service.Listener.Encode(w, &ipResp)
	log.Response(service.Name, ipResp, err)
}

// Handles retrieval of ghost ip addresses from ipam driver.
func (service *httpRestService) getGhostIPAddresses(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getGhostIPAddresses")
	log.Request(service.Name, "getGhostIPAddresses", nil)
	switch r.Method {
		case "GET":
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	ipResp := &cns.GetIPAddressesResponse{Response: resp}
	err := service.Listener.Encode(w, &ipResp)
	log.Response(service.Name, ipResp, err)
}

// getAllIPAddresses retrieves all ip addresses from ipam driver.
func (service *httpRestService) getAllIPAddresses(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getAllIPAddresses")
	log.Request(service.Name, "getAllIPAddresses", nil)
	switch r.Method {
		case "GET":
		default:
	}
	resp := cns.Response{ReturnCode: 0}
	ipResp := &cns.GetIPAddressesResponse{Response: resp}
	err := service.Listener.Encode(w, &ipResp)
	log.Response(service.Name, ipResp, err)
}

// Handles health report requests.
func (service *httpRestService) getHealthReport(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Azure CNS] getHealthReport")
	log.Request(service.Name, "getHealthReport", nil)	
	switch r.Method {
		case "GET":
		default:
	}

	resp := &cns.Response{ReturnCode: 0}
	err := service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// saveState writes CNS state to persistent store.
func (service *httpRestService) saveState() error {
	log.Printf("[Azure CNS] saveState")
	// Skip if a store is not provided.
	if service.store == nil {
		log.Printf("[Azure CNS]  store not initialized.")
		return nil
	}

	// Update time stamp.
	service.state.TimeStamp = time.Now()	
	err := service.store.Write(storeKey, &service.state)
	if err == nil {
		log.Printf("[Azure CNS]  State saved successfully.\n")
	} else {
		log.Printf("[Azure CNS]  Failed to save state., err:%v\n", err)
	}
	return err
}

// restoreState restores CNS state from persistent store.
func (service *httpRestService) restoreState() error {
	log.Printf("[Azure CNS] restoreState")
	// Skip if a store is not provided.
	if service.store == nil {
		log.Printf("[Azure CNS]  store not initialized.")
		return nil
	}

	// Read any persisted state.
	err := service.store.Read(storeKey, &service.state)
	if err != nil {
		if err == store.ErrKeyNotFound {
			// Nothing to restore.
			log.Printf("[Azure CNS]  No state to restore.\n")
			return nil
		}
		
		log.Printf("[Azure CNS]  Failed to restore state, err:%v\n", err)
		return err
	}

	log.Printf("[Azure CNS]  Restored state, %+v\n", service.state)
	return nil
}
