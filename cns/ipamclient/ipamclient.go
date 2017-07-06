// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipamclient

import (
	"bytes"
	"encoding/json"
	"net/http"

	"fmt"

	cnmIpam "github.com/Azure/azure-container-networking/cnm/ipam"
	ipam "github.com/Azure/azure-container-networking/ipam"
	"github.com/Azure/azure-container-networking/log"
)

// IpamClient specifies a client to connect to Ipam Plugin.
type IpamClient struct {
	connectionURL string
}

// NewIpamClient create a new ipam client.
func NewIpamClient(url string) (*IpamClient, error) {
	if url == "" {
		url = defaultIpamPluginURL
	}
	return &IpamClient{
		connectionURL: url,
	}, nil
}

// GetAddressSpace request to get address space ID.
func (ic *IpamClient) GetAddressSpace() (string, error) {
	log.Printf("[Azure CNS] GetAddressSpace Request")

	url := ic.connectionURL + cnmIpam.GetAddressSpacesPath

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[Azure CNS] Error received while creating http GET request for AddressSpace %v", err.Error())
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}

	res, err := client.Do(req)
	if res == nil {
		return "", err
	}

	if res.StatusCode == 200 {
		var resp cnmIpam.GetDefaultAddressSpacesResponse
		err := json.NewDecoder(res.Body).Decode(&resp)
		if err != nil {
			log.Printf("[Azure CNS] Error received while parsing GetAddressSpace response resp:%v err:%v", res.Body, err.Error())
			return "", err
		}

		if resp.Err != "" {
			log.Printf("[Azure CNS] GetAddressSpace received error response :%v", resp.Err)
			return "", fmt.Errorf(resp.Err)
		}

		return resp.LocalDefaultAddressSpace, nil
	}
	log.Printf("[Azure CNS] GetAddressSpace invalid http status code: %v err:%v", res.StatusCode, err.Error())
	return "", err
}

// GetPoolID Request to get poolID.
func (ic *IpamClient) GetPoolID(asID, subnet string) (string, error) {
	var body bytes.Buffer
	log.Printf("[Azure CNS] GetPoolID Request")

	url := ic.connectionURL + cnmIpam.RequestPoolPath

	payload := &cnmIpam.RequestPoolRequest{
		AddressSpace: asID,
		Pool:         subnet,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, url, &body)
	if err != nil {
		log.Printf("[Azure CNS] Error received while creating http GET request for GetPoolID asID: %v poolid: %v err:%v", asID, subnet, err.Error())
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	res, err := client.Do(req)

	if res == nil {
		return "", err
	}

	if res.StatusCode == 200 {
		var resp cnmIpam.RequestPoolResponse
		err := json.NewDecoder(res.Body).Decode(&resp)
		if err != nil {
			log.Printf("[Azure CNS] Error received while parsing GetPoolID response resp:%v err:%v", res.Body, err.Error())
			return "", err
		}

		if resp.Err != "" {
			log.Printf("[Azure CNS] GetPoolID received error response :%v", resp.Err)
			return "", fmt.Errorf(resp.Err)
		}

		return resp.PoolID, nil
	}
	log.Printf("[Azure CNS] GetPoolID invalid http status code: %v err:%v", res.StatusCode, err.Error())
	return "", err

}

// ReserveIPAddress request an Ip address for the reservation id.
func (ic *IpamClient) ReserveIPAddress(poolID string, reservationID string) (string, error) {
	var body bytes.Buffer
	log.Printf("[Azure CNS] ReserveIpAddress")

	url := ic.connectionURL + cnmIpam.RequestAddressPath

	payload := &cnmIpam.RequestAddressRequest{
		PoolID:  poolID,
		Address: "",
		Options: make(map[string]string),
	}
	payload.Options[ipam.OptAddressID] = reservationID
	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, url, &body)
	if err != nil {
		log.Printf("[Azure CNS] Error received while creating http GET request for reserve IP resid: %v poolid: %v err:%v", reservationID, poolID, err.Error())
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	res, err := client.Do(req)

	if res == nil {
		return "", err
	}

	if res.StatusCode == 200 {
		var reserveResp cnmIpam.RequestAddressResponse

		err = json.NewDecoder(res.Body).Decode(&reserveResp)
		if err != nil {
			log.Printf("[Azure CNS] Error received while parsing reserve response resp:%v err:%v", res.Body, err.Error())
			return "", err
		}

		if reserveResp.Err != "" {
			log.Printf("[Azure CNS] ReserveIP received error response :%v", reserveResp.Err)
			return "", fmt.Errorf(reserveResp.Err)
		}

		return reserveResp.Address, nil
	}

	log.Printf("[Azure CNS] ReserveIp invalid http status code: %v err:%v", res.StatusCode, err.Error())
	return "", err
}

// ReleaseIPAddress release an Ip address for the reservation id.
func (ic *IpamClient) ReleaseIPAddress(poolID string, reservationID string) error {
	var body bytes.Buffer
	log.Printf("[Azure CNS] ReleaseIpAddress")

	url := ic.connectionURL + cnmIpam.ReleaseAddressPath

	payload := &cnmIpam.ReleaseAddressRequest{
		PoolID:  poolID,
		Address: "",
		Options: make(map[string]string),
	}

	payload.Options[ipam.OptAddressID] = reservationID

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, url, &body)
	if err != nil {
		log.Printf("[Azure CNS] Error received while creating http GET request for ReleaseIP resid: %v poolid: %v err:%v", reservationID, poolID, err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	res, err := client.Do(req)

	if res == nil {
		return err
	}

	if res.StatusCode == 200 {
		var releaseResp cnmIpam.ReleaseAddressResponse
		err := json.NewDecoder(res.Body).Decode(&releaseResp)
		if err != nil {
			log.Printf("[Azure CNS] Error received while parsing release response :%v err:%v", res.Body, err.Error())
			return err
		}

		if releaseResp.Err != "" {
			log.Printf("[Azure CNS] ReleaseIP received error response :%v", releaseResp.Err)
			return fmt.Errorf(releaseResp.Err)
		}

		return nil
	}
	log.Printf("[Azure CNS] ReleaseIP invalid http status code: %v err:%v", res.StatusCode, err.Error())
	return err

}

// GetIPAddressUtilization - returns number of available, reserved and unhealthy addresses list.
func (ic *IpamClient) GetIPAddressUtilization(poolID string) (int, int, []string, error) {
	var body bytes.Buffer
	log.Printf("[Azure CNS] GetIPAddressUtilization")

	url := ic.connectionURL + cnmIpam.GetPoolInfoPath

	payload := &cnmIpam.GetPoolInfoRequest{
		PoolID: poolID,
	}

	json.NewEncoder(&body).Encode(payload)

	req, err := http.NewRequest(http.MethodGet, url, &body)
	if err != nil {
		log.Printf("[Azure CNS] Error received while creating http GET request for GetIPUtilization poolid: %v err:%v", poolID, err.Error())
		return 0, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	res, err := client.Do(req)

	if res == nil {
		return 0, 0, nil, err
	}

	if res.StatusCode == 200 {
		var poolInfoResp cnmIpam.GetPoolInfoResponse
		err := json.NewDecoder(res.Body).Decode(&poolInfoResp)
		if err != nil {
			log.Printf("[Azure CNS] Error received while parsing GetIPUtilization response :%v err:%v", res.Body, err.Error())
			return 0, 0, nil, err
		}

		if poolInfoResp.Err != "" {
			log.Printf("[Azure CNS] GetIPUtilization received error response :%v", poolInfoResp.Err)
			return 0, 0, nil, fmt.Errorf(poolInfoResp.Err)
		}

		return poolInfoResp.Capacity, poolInfoResp.Available, poolInfoResp.UnhealthyAddresses, nil
	}
	log.Printf("[Azure CNS] GetIPUtilization invalid http status code: %v err:%v", res.StatusCode, err.Error())
	return 0, 0, nil, err

}
