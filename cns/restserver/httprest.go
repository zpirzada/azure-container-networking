// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
)

// httpRestService represents http listener for CNS - Container Networking Service.
type httpRestService struct {
	*cns.Service
}

// HTTPService describes the min API interface that every service should have.
type HTTPService interface {
	common.ServiceAPI
}

// NewHTTPRestService creates a new HTTP Service object.
func NewHTTPRestService(config *common.ServiceConfig) (HTTPService, error) {
	service, err := cns.NewService(config.Name, config.Version)
	if err != nil {
		return nil, err
	}

	return &httpRestService{
		Service: service,
	}, nil
}

// Start starts the CNS listener.
func (service *httpRestService) Start(config *common.ServiceConfig) error {

	err := service.Initialize(config)
	if err != nil {
		log.Printf("[Azure CNS] Failed to initialize base service, err:%v.", err)
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

	log.Printf("[Azure CNS] Listening.")

	return nil
}

// Stop stops the CNS.
func (service *httpRestService) Stop() {
	service.Uninitialize()
	log.Printf("[Azure CNS] Service stopped.")
}

// Handles requests to set the environment type.
func (service *httpRestService) setEnvironment(w http.ResponseWriter, r *http.Request) {
	var req cns.SetEnvironmentRequest
	err := service.Listener.Decode(w, r, &req)
	log.Request(service.Name, &req, err)
	if err != nil {
		return
	}

	switch r.Method {
	case "GET":
		log.Printf("[Azure CNS] GET received for SetEnvironment.")
	case "POST":
		log.Printf("[Azure CNS] POST received for SetEnvironment.")
	default:
	}

	resp := &cns.Response{ReturnCode: 0}
	err = service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}

// Handles CreateNetwork requests.
func (service *httpRestService) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req cns.CreateNetworkRequest

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

// Handles DeleteNetwork requests.
func (service *httpRestService) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var req cns.DeleteNetworkRequest

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

// Handles ip reservation requests.
func (service *httpRestService) reserveIPAddress(w http.ResponseWriter, r *http.Request) {
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

// Handles retrieval of all ip addresses from ipam driver.
func (service *httpRestService) getAllIPAddresses(w http.ResponseWriter, r *http.Request) {
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
	log.Request(service.Name, "getHealthReport", nil)
	
	switch r.Method {
		case "GET":
		default:
	}

	resp := &cns.Response{ReturnCode: 0}
	err := service.Listener.Encode(w, &resp)
	log.Response(service.Name, resp, err)
}
