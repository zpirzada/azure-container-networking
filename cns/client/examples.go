// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
)

const (
	defaultCNSServerURL = "http://localhost:10090"
)

func setEnvironment() error{	
	envRequest := cns.SetEnvironmentRequest{Location:"Azure", NetworkType: "Underlay"}
    envRequestJSON := new(bytes.Buffer)
    json.NewEncoder(envRequestJSON).Encode(envRequest)
    res, err := 
	http.Post(
		defaultCNSServerURL+cns.SetEnvironmentPath, 
		"application/json; charset=utf-8", 
		envRequestJSON)
	if err != nil {
		fmt.Printf("Error received in Set Env: %v ", err.Error())
		return err
	}    
	var setEnvironmentResponse cns.Response
	err = json.NewDecoder(res.Body).Decode(&setEnvironmentResponse)
	if err != nil{
		fmt.Printf("Error received in decoding response from SetEnvironment: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for SetEnvironment: %+v\n", setEnvironmentResponse)	
	return nil
}

func createNetwork() error{	
	netRequest := cns.CreateNetworkRequest{NetworkName:"azurenet"}
    netRequestJSON := new(bytes.Buffer)
    json.NewEncoder(netRequestJSON).Encode(netRequest)
    res, err := 
	http.Post(
		defaultCNSServerURL+cns.CreateNetworkPath, 
		"application/json; charset=utf-8", 
		netRequestJSON)
	if err != nil {
		fmt.Printf("Error received in CreateNetwork post: %v ", err.Error())
		return err
	}    
	
	var createNetworkResponse cns.Response
	err = json.NewDecoder(res.Body).Decode(&createNetworkResponse)
	if err != nil{
		fmt.Printf("Error received in decoding response from CreateNEtwork: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for CreateNetwork: %+v\n", createNetworkResponse)
	return nil
}

func deleteNetwork() error{	
	netRequest := cns.DeleteNetworkRequest{NetworkName:"azurenet"}
    netRequestJSON := new(bytes.Buffer)
    json.NewEncoder(netRequestJSON).Encode(netRequest)
    res, err := 
	http.Post(
		defaultCNSServerURL+cns.DeleteNetworkPath, 
		"application/json; charset=utf-8", 
		netRequestJSON)
	if err != nil {
		fmt.Printf("Error received in DeleteNetwork post: %v ", err.Error())
		return err
	}    
	
	var deleteNetworkResponse cns.Response
	err = json.NewDecoder(res.Body).Decode(&deleteNetworkResponse)
	if err != nil{
		fmt.Printf("Error received in decoding response from DeleteNetwork: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for DeleteNetwork: %+v\n", deleteNetworkResponse)
	return nil
}

func reserveIPAddress() error{	
	reserveIPRequest := cns.ReserveIPAddressRequest{ReservationID:"ip01"}
    reserveIPRequestJSON := new(bytes.Buffer)
    json.NewEncoder(reserveIPRequestJSON).Encode(reserveIPRequest)
    res, err := 
	http.Post(
		defaultCNSServerURL+cns.ReserveIPAddressPath, 
		"application/json; charset=utf-8", 
		reserveIPRequestJSON)
	if err != nil {
		fmt.Printf("Error received in reserveIPAddress post: %v ", err.Error())
		return err
	}    
	var reserveIPAddressResponse cns.ReserveIPAddressResponse
	err = json.NewDecoder(res.Body).Decode(&reserveIPAddressResponse)
	if err != nil{
		fmt.Printf("Error received in decoding response from reserveIPAddress: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for reserveIPAddress: %+v\n", reserveIPAddressResponse)
	return nil
}

func releaseIPAddress() error{	
	releaseIPRequest := cns.ReleaseIPAddressRequest{ReservationID:"ip01"}
    releaseIPAddressRequestJSON := new(bytes.Buffer)
    json.NewEncoder(releaseIPAddressRequestJSON).Encode(releaseIPRequest)
    res, err := 
	http.Post(
		defaultCNSServerURL+cns.ReleaseIPAddressPath, 
		"application/json; charset=utf-8", 
		releaseIPAddressRequestJSON)
	if err != nil {
		fmt.Printf("Error received in releaseIPAddress post: %v ", err.Error())
		return err
	}    
	var releaseIPAddressResponse cns.Response	
	err = json.NewDecoder(res.Body).Decode(&releaseIPAddressResponse)
	if err != nil{
		fmt.Printf("Error received in decoding response from releaseIPAddress: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for releaseIPAddress: %+v\n", releaseIPAddressResponse)
	return nil
}

func getIPAddressUtilization() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetIPAddressUtilizationPath)
	if err != nil {
		fmt.Printf("Error received in getIPAddressUtilization GET: %v ", err.Error())
		return err
	}
	var iPAddressesUtilizationResponse cns.IPAddressesUtilizationResponse
	err = json.NewDecoder(res.Body).Decode(&iPAddressesUtilizationResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from getIPAddressUtilization: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for getIPAddressUtilization: %+v\n", iPAddressesUtilizationResponse)
	return nil
}

func getHostLocalIP() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetHostLocalIPPath)
	if err != nil {
		fmt.Printf("Error received in getHostLocalIP GET: %v ", err.Error())
		return err
	}
	var hostLocalIPAddressResponse cns.HostLocalIPAddressResponse
	err = json.NewDecoder(res.Body).Decode(&hostLocalIPAddressResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from getHostLocalIP: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for getHostLocalIP: %+v\n", hostLocalIPAddressResponse)
	return nil
}

func getAvailableIPAddresses() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetAvailableIPAddressesPath)
	if err != nil {
		fmt.Printf("Error received in GetAvailable IP Addresses: %v ", err.Error())
		return err
	}
	var getIPAddressesResponse cns.GetIPAddressesResponse
	err = json.NewDecoder(res.Body).Decode(&getIPAddressesResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from GetAvailableIPAddresses: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for GetAvailableIPAddresses: %+v\n", getIPAddressesResponse)
	return nil
}

func getReservedIPAddresses() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetReservedIPAddressesPath)
	if err != nil {
		fmt.Printf("Error received in GetReserved IP Addresses: %v ", err.Error())
		return err
	}
    var getIPAddressesResponse cns.GetIPAddressesResponse
	err = json.NewDecoder(res.Body).Decode(&getIPAddressesResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from getReservedIPAddresses: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for getReservedIPAddresses: %+v\n", getIPAddressesResponse)
	return nil
}

func getGhostIPAddresses() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetGhostIPAddressesPath)
	if err != nil {
		fmt.Printf("Error received in GetGhost IP Addresses: %v ", err.Error())
		return err
	}
    var getIPAddressesResponse cns.GetIPAddressesResponse
	err = json.NewDecoder(res.Body).Decode(&getIPAddressesResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from getGhostIPAddresses: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for getGhostIPAddresses: %+v\n", getIPAddressesResponse)
	return nil
}

func getAllIPAddresses() error{	
	res, err := 
	http.Get(defaultCNSServerURL+cns.GetAllIPAddressesPath)
	if err != nil {
		fmt.Printf("Error received in GetAll IP Addresses: %v ", err.Error())
		return err
	}
    var getIPAddressesResponse cns.GetIPAddressesResponse
	err = json.NewDecoder(res.Body).Decode(&getIPAddressesResponse)
	if err != nil {
		fmt.Printf("Error received in decoding response from getAllIPAddresses: %v ", err.Error())
		return err
	}
	fmt.Printf("Response for getAllIPAddresses: %+v\n", getIPAddressesResponse)
	return nil
}

func main() {
	setEnvironment()
	createNetwork()
	deleteNetwork()
	reserveIPAddress()
	releaseIPAddress()
	getAvailableIPAddresses()
	getReservedIPAddresses()
	getGhostIPAddresses()
	getAllIPAddresses()
	getIPAddressUtilization()
	getHostLocalIP()
}
