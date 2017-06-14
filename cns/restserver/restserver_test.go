// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/cns"
)

var service HTTPService
var mux *http.ServeMux

// Wraps the test run with service setup and teardown.
func TestMain(m *testing.M) {
	var config common.ServiceConfig
	var err error

	// Create the service.
	service, err = NewHTTPRestService(&config)
	if err != nil {
		fmt.Printf("Failed to create CNS object %v\n", err)
		os.Exit(1)
	}

	// Configure test mode.
	service.(*httpRestService).Name = "cns-test-server"

	// Start the service.
	err = service.Start(&config)
	if err != nil {
		fmt.Printf("Failed to start CNS %v\n", err)
		os.Exit(2)
	}	

	// Get the internal http mux as test hook.
	mux = service.(*httpRestService).Listener.GetMux()

	// Run tests.
	exitCode := m.Run()

	// Cleanup.
	service.Stop()

	os.Exit(exitCode)
}

// Decodes service's responses to test requests.
func decodeResponse(w *httptest.ResponseRecorder, response interface{}) error {
	if w.Code != http.StatusOK {
		return fmt.Errorf("Request failed with HTTP error %d", w.Code)
	}

	if w.Body == nil {
		return fmt.Errorf("Response body is empty")
	}

	return json.NewDecoder(w.Body).Decode(&response)
}

func setEnv(t *testing.T) (*httptest.ResponseRecorder) {
	envRequest := cns.SetEnvironmentRequest{Location:"Azure", NetworkType: "Underlay"}
	envRequestJSON := new(bytes.Buffer)
    json.NewEncoder(envRequestJSON).Encode(envRequest)

	req, err := http.NewRequest(http.MethodPost, cns.V1Prefix + cns.SetEnvironmentPath, envRequestJSON)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}
func TestSetEnvironment(t *testing.T) {	
	fmt.Println("Test: SetEnvironment")
	var resp cns.Response
	w := setEnv(t)
	err := decodeResponse(w, &resp)
	if err != nil || resp.ReturnCode != 0 {
		t.Errorf("SetEnvironment failed with response %+v", resp)
	} else {
		fmt.Printf ("SetEnvironment Responded with %+v\n", resp);
	}
}

// Tests CreateNetwork functionality.
func TestCreateNetwork(t *testing.T) {
	fmt.Println("Test: CreateNetwork")	
	var body bytes.Buffer
	setEnv(t)
	info := &cns.CreateNetworkRequest{
		NetworkName: "azurenet",
	}
	json.NewEncoder(&body).Encode(info)
	req, err := http.NewRequest(http.MethodPost, cns.CreateNetworkPath, &body)
	if err != nil {
		t.Fatal(err)
	}	
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var resp cns.Response
	err = decodeResponse(w, &resp)
	if err != nil || resp.ReturnCode != 0 {
		t.Errorf("CreateNetwork failed with response %+v", resp)
	} else {
		fmt.Printf ("CreateNetwork Responded with %+v\n", resp);
	}
}

// Tests DeleteNetwork functionality.
func TestDeleteNetwork(t *testing.T) {
	fmt.Println("Test: DeleteNetwork")	
	var body bytes.Buffer
	setEnv(t)
	info := &cns.DeleteNetworkRequest{
		NetworkName: "azurenet",
	}
	json.NewEncoder(&body).Encode(info)
	req, err := http.NewRequest(http.MethodPost, cns.DeleteNetworkPath, &body)
	if err != nil {
		t.Fatal(err)
	}	
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var resp cns.Response
	err = decodeResponse(w, &resp)
	if err != nil || resp.ReturnCode != 0 {
		t.Errorf("DeleteNetwork failed with response %+v", resp)
	} else {
		fmt.Printf ("DeleteNetwork Responded with %+v\n", resp);
	}
}

func TestReserveIPAddress(t *testing.T){
	fmt.Println("Test: ReserveIPAddress")

	reserveIPRequest := cns.ReserveIPAddressRequest{ReservationID:"ip01"}
    reserveIPRequestJSON := new(bytes.Buffer)
    json.NewEncoder(reserveIPRequestJSON).Encode(reserveIPRequest)

	envRequest := cns.SetEnvironmentRequest{Location:"Azure", NetworkType: "Underlay"}
	envRequestJSON := new(bytes.Buffer)
    json.NewEncoder(envRequestJSON).Encode(envRequest)

	req, err := http.NewRequest(http.MethodGet, cns.ReserveIPAddressPath, envRequestJSON)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var reserveIPAddressResponse cns.ReserveIPAddressResponse
	err = decodeResponse(w, &reserveIPAddressResponse)

	if err != nil || reserveIPAddressResponse.Response.ReturnCode != 0 {
		t.Errorf("SetEnvironment failed with response %+v", reserveIPAddressResponse)
	} else {
		fmt.Printf ("SetEnvironment Responded with %+v\n", reserveIPAddressResponse);
	}
}

func TestReleaseIPAddress(t *testing.T){
	fmt.Println("Test: ReleaseIPAddress")
	releaseIPRequest := cns.ReleaseIPAddressRequest{ReservationID:"ip01"}
    releaseIPAddressRequestJSON := new(bytes.Buffer)
    json.NewEncoder(releaseIPAddressRequestJSON).Encode(releaseIPRequest)

	req, err := http.NewRequest(http.MethodGet, cns.ReleaseIPAddressPath, releaseIPAddressRequestJSON)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var releaseIPAddressResponse cns.Response	
	err = decodeResponse(w, &releaseIPAddressResponse)

	if err != nil || releaseIPAddressResponse.ReturnCode != 0 {
		t.Errorf("SetEnvironment failed with response %+v", releaseIPAddressResponse)
	} else {
		fmt.Printf ("SetEnvironment Responded with %+v\n", releaseIPAddressResponse);
	}
}
func TestGetIPAddressUtilization(t *testing.T){
	fmt.Println("Test: GetIPAddressUtilization")

	req, err := http.NewRequest(http.MethodGet, cns.GetIPAddressUtilizationPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var iPAddressesUtilizationResponse cns.IPAddressesUtilizationResponse
	err = decodeResponse(w, &iPAddressesUtilizationResponse)

	if err != nil || iPAddressesUtilizationResponse.Response.ReturnCode != 0 {
		t.Errorf("GetIPAddressUtilization failed with response %+v", iPAddressesUtilizationResponse)
	} else {
		fmt.Printf ("GetIPAddressUtilization Responded with %+v\n", iPAddressesUtilizationResponse);
	}
}

func TestGetHostLocalIP(t *testing.T){
	fmt.Println("Test: GetHostLocalIP")
	setEnv(t)
	req, err := http.NewRequest(http.MethodGet, cns.GetHostLocalIPPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	
	var hostLocalIPAddressResponse cns.HostLocalIPAddressResponse
	err = decodeResponse(w, &hostLocalIPAddressResponse)

	if err != nil || hostLocalIPAddressResponse.Response.ReturnCode != 0 {
		t.Errorf("GetHostLocalIP failed with response %+v", hostLocalIPAddressResponse)
	} else {
		fmt.Printf ("GetHostLocalIP Responded with %+v\n", hostLocalIPAddressResponse);
	}
}

func TestGetUnhealthyIPAddresses(t *testing.T){	
	fmt.Println("Test: GetGhostIPAddresses")
	req, err := http.NewRequest(http.MethodGet, cns.GetUnhealthyIPAddressesPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

    var getIPAddressesResponse cns.GetIPAddressesResponse
	err = decodeResponse(w, &getIPAddressesResponse)

	if err != nil || getIPAddressesResponse.Response.ReturnCode != 0 {
		t.Errorf("GetUnhealthyIPAddresses failed with response %+v", getIPAddressesResponse)
	} else {
		fmt.Printf ("GetUnhealthyIPAddresses Responded with %+v\n", getIPAddressesResponse);
	}
}

