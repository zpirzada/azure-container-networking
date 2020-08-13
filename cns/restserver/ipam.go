// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/logger"
)

// used to request an IPConfig from the CNS state
func (service *HTTPRestService) requestIPConfigHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err             error
		ipconfigRequest cns.GetIPConfigRequest
		ipconfiguration cns.IPConfiguration
		returnCode      int
		returnMessage   string
	)

	err = service.Listener.Decode(w, r, &ipconfigRequest)
	logger.Request(service.Name, &ipconfigRequest, err)
	if err != nil {
		return
	}

	// retrieve ipconfig from nc
	returnCode, returnMessage = service.validateIpConfigRequest(ipconfigRequest)
	if returnCode == Success {
		if ipconfiguration, err = requestIPConfigHelper(service, ipconfigRequest); err != nil {
			returnCode = FailedToAllocateIpConfig
			returnMessage = fmt.Sprintf("AllocateIPConfig failed: %v", err)
		}
	}

	resp := cns.Response{
		ReturnCode: returnCode,
		Message:    returnMessage,
	}

	reserveResp := &cns.GetIPConfigResponse{
		Response: resp,
	}
	reserveResp.IPConfiguration = ipconfiguration

	err = service.Listener.Encode(w, &reserveResp)
	logger.Response(service.Name, reserveResp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
}

func (service *HTTPRestService) releaseIPConfigHandler(w http.ResponseWriter, r *http.Request) {
	var (
		podInfo       cns.KubernetesPodInfo
		req           cns.GetIPConfigRequest
		statusCode    int
		returnMessage string
	)

	statusCode = UnexpectedError

	err := service.Listener.Decode(w, r, &req)
	logger.Request(service.Name, &req, err)
	if err != nil {
		returnMessage = err.Error()
		return
	}

	defer func() {
		resp := cns.Response{}

		if err != nil {
			resp.ReturnCode = statusCode
			resp.Message = returnMessage
		}

		err = service.Listener.Encode(w, &resp)
		logger.Response(service.Name, resp, resp.ReturnCode, ReturnCodeToString(resp.ReturnCode), err)
	}()

	statusCode, returnMessage = service.validateIpConfigRequest(req)

	if err = service.releaseIPConfig(podInfo); err != nil {
		statusCode = NotFound
		returnMessage = err.Error()
		return
	}
	return
}

func (service *HTTPRestService) GetAllocatedIPConfigs() []ipConfigurationStatus {
	service.RLock()
	defer service.RUnlock()
	return filterIPConfigMap(service.PodIPConfigState, func(ipconfig ipConfigurationStatus) bool {
		return ipconfig.State == cns.Allocated
	})
}

func (service *HTTPRestService) GetAvailableIPConfigs() []ipConfigurationStatus {
	service.RLock()
	defer service.RUnlock()
	return filterIPConfigMap(service.PodIPConfigState, func(ipconfig ipConfigurationStatus) bool {
		return ipconfig.State == cns.Available
	})
}

func filterIPConfigMap(toBeAdded map[string]ipConfigurationStatus, f func(ipConfigurationStatus) bool) []ipConfigurationStatus {
	vsf := make([]ipConfigurationStatus, 0)
	for _, v := range toBeAdded {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

//SetIPConfigAsAllocated takes a lock of the service, and sets the ipconfig in the CNS state as allocated, does not take a lock
func (service *HTTPRestService) setIPConfigAsAllocated(ipconfig ipConfigurationStatus, podInfo cns.KubernetesPodInfo, marshalledOrchestratorContext json.RawMessage) ipConfigurationStatus {
	ipconfig.State = cns.Allocated
	ipconfig.OrchestratorContext = marshalledOrchestratorContext
	service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()] = ipconfig.ID
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	return service.PodIPConfigState[ipconfig.ID]
}

//SetIPConfigAsAllocated and sets the ipconfig in the CNS state as allocated, does not take a lock
func (service *HTTPRestService) setIPConfigAsAvailable(ipconfig ipConfigurationStatus, podInfo cns.KubernetesPodInfo) ipConfigurationStatus {
	ipconfig.State = cns.Available
	ipconfig.OrchestratorContext = nil
	service.PodIPConfigState[ipconfig.ID] = ipconfig
	delete(service.PodIPIDByOrchestratorContext, podInfo.GetOrchestratorContextKey())
	return service.PodIPConfigState[ipconfig.ID]
}

////SetIPConfigAsAllocated takes a lock of the service, and sets the ipconfig in the CNS stateas Available
// Todo - CNI should also pass the IPAddress which needs to be released to validate if that is the right IP allcoated
// in the first place.
func (service *HTTPRestService) releaseIPConfig(podInfo cns.KubernetesPodInfo) error {
	service.Lock()
	defer service.Unlock()

	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if ipconfig, isExist := service.PodIPConfigState[ipID]; isExist {
			service.setIPConfigAsAvailable(ipconfig, podInfo)
		} else {
			logger.Errorf("Failed to get release ipconfig. Pod to IPID exists, but IPID to IPConfig doesn't exist, CNS State potentially corrupt")
			return fmt.Errorf("releaseIPConfig failed. Pod to IPID exists, but IPID to IPConfig doesn't exist, CNS State potentially corrupt")
		}
	} else {
		logger.Printf("SetIPConfigAsAvailable failed to release, no allocation found for pod")
		return nil
	}
	return nil
}

func (service *HTTPRestService) GetExistingIPConfig(podInfo cns.KubernetesPodInfo) (cns.IPConfiguration, bool, error) {
	var (
		ipConfiguration cns.IPConfiguration
		isExist         bool
	)

	service.RLock()
	defer service.RUnlock()

	ipID := service.PodIPIDByOrchestratorContext[podInfo.GetOrchestratorContextKey()]
	if ipID != "" {
		if ipState, isExist := service.PodIPConfigState[ipID]; isExist {
			err := service.populateIpConfigInfoUntransacted(ipState, &ipConfiguration)
			return ipConfiguration, isExist, err
		}

		logger.Errorf("Failed to get existing ipconfig. Pod to IPID exists, but IPID to IPConfig doesn't exist, CNS State potentially corrupt")
		return ipConfiguration, isExist, fmt.Errorf("Failed to get existing ipconfig. Pod to IPID exists, but IPID to IPConfig doesn't exist, CNS State potentially corrupt")
	}

	return ipConfiguration, isExist, nil
}

func (service *HTTPRestService) AllocateDesiredIPConfig(podInfo cns.KubernetesPodInfo, desiredIPAddress string, orchestratorContext json.RawMessage) (cns.IPConfiguration, error) {
	var ipConfiguration cns.IPConfiguration
	service.Lock()
	defer service.Unlock()

	found := false
	for _, ipState := range service.PodIPConfigState {
		if ipState.IPAddress == desiredIPAddress {
			if ipState.State == cns.Allocated {
				// This IP has already been allocated, if it is allocated to same pod, then return the same
				// IPconfiguration
				if bytes.Equal(orchestratorContext, ipState.OrchestratorContext) == true {
					found = true
				} else {
					var pInfo cns.KubernetesPodInfo
					json.Unmarshal(ipState.OrchestratorContext, &pInfo)
					return ipConfiguration, fmt.Errorf("Desired IP is already allocated %+v to Pod: %+v, requested for pod %+v", ipState, pInfo, podInfo)
				}
			} else if ipState.State == cns.Available {
				service.setIPConfigAsAllocated(ipState, podInfo, orchestratorContext)
				found = true
			} else {
				return ipConfiguration, fmt.Errorf("Desired IP is not available %+v", ipState)
			}

			if found {
				err := service.populateIpConfigInfoUntransacted(ipState, &ipConfiguration)
				return ipConfiguration, err
			}
		}
	}
	return ipConfiguration, fmt.Errorf("Requested IP not found in pool")
}

func (service *HTTPRestService) AllocateAnyAvailableIPConfig(podInfo cns.KubernetesPodInfo, orchestratorContext json.RawMessage) (cns.IPConfiguration, error) {
	var ipConfiguration cns.IPConfiguration

	service.Lock()
	defer service.Unlock()

	for _, ipState := range service.PodIPConfigState {
		if ipState.State == cns.Available {
			err := service.populateIpConfigInfoUntransacted(ipState, &ipConfiguration)
			if err == nil {
				service.setIPConfigAsAllocated(ipState, podInfo, orchestratorContext)
			}
			return ipConfiguration, err
		}
	}

	return ipConfiguration, fmt.Errorf("No more free IP's available, trigger batch")
}

// If IPConfig is already allocated for pod, it returns that else it returns one of the available ipconfigs.
func requestIPConfigHelper(service *HTTPRestService, req cns.GetIPConfigRequest) (cns.IPConfiguration, error) {
	var (
		podInfo         cns.KubernetesPodInfo
		ipConfiguration cns.IPConfiguration
		isExist         bool
		err             error
	)

	// check if ipconfig already allocated for this pod and return if exists or error
	// if error, ipstate is nil, if exists, ipstate is not nil and error is nil
	json.Unmarshal(req.OrchestratorContext, &podInfo)
	if ipConfiguration, isExist, err = service.GetExistingIPConfig(podInfo); err != nil || isExist {
		return ipConfiguration, err
	}

	// return desired IPConfig
	if req.DesiredIPConfig.IPAddress != "" {
		return service.AllocateDesiredIPConfig(podInfo, req.DesiredIPConfig.IPAddress, req.OrchestratorContext)
	}

	// return any free IPConfig
	return service.AllocateAnyAvailableIPConfig(podInfo, req.OrchestratorContext)
}
