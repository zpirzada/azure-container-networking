// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"encoding/json"
	"fmt"
	"net/http"
)

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

func (dockerdriver *netPlugin) networkExists(networkID string) bool{
	if dockerdriver.networks[networkID] != nil {
		return true
	}
	return false
}

func (dockerdriver *netPlugin) endpointExists(networkID string, endpointID string) bool{
	network := dockerdriver.networks[networkID]
	if network == nil {
		return false
	}

	if(network.endpoints[endpointID] == nil){
		return false
	}

	return true
}
