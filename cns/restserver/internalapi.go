// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/logger"
	"github.com/Azure/azure-container-networking/cns/nmagentclient"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

// This file contains the internal functions called by either HTTP APIs (api.go) or
// internal APIs (definde in internalapi.go).
// This will be used internally (say by RequestController in case of AKS)

// GetPartitionKey - Get dnc/service partition key
func (service *HTTPRestService) GetPartitionKey() (dncPartitionKey string) {
	service.RLock()
	dncPartitionKey = service.dncPartitionKey
	service.RUnlock()
	return
}

// SetNodeOrchestrator :- Set node orchestrator after registering with mDNC
func (service *HTTPRestService) SetNodeOrchestrator(r *cns.SetOrchestratorTypeRequest) {
	body, _ := json.Marshal(r)
	req, _ := http.NewRequest(http.MethodPost, "", bytes.NewBuffer(body))
	req.Header.Set(common.ContentType, common.JsonContent)
	service.setOrchestratorType(httptest.NewRecorder(), req)
}

// SyncNodeStatus :- Retrieve the latest node state from DNC & returns the first occurence of returnCode and error with respect to contextFromCNI
func (service *HTTPRestService) SyncNodeStatus(dncEP, infraVnet, nodeID string, contextFromCNI json.RawMessage) (returnCode int, errStr string) {
	logger.Printf("[Azure CNS] SyncNodeStatus")
	var (
		response         *http.Response
		err              error
		nodeInfoResponse cns.NodeInfoResponse
		req              *http.Request
		body             []byte
		httpc            = common.GetHttpClient()
	)

	// try to retrieve NodeInfoResponse from mDNC
	response, err = httpc.Get(fmt.Sprintf(common.SyncNodeNetworkContainersURLFmt, dncEP, infraVnet, nodeID, dncApiVersion))
	if err == nil {
		if response.StatusCode == http.StatusOK {
			err = json.NewDecoder(response.Body).Decode(&nodeInfoResponse)
		} else {
			err = fmt.Errorf("%d", response.StatusCode)
		}

		response.Body.Close()
	}

	if err != nil {
		returnCode = UnexpectedError
		errStr = fmt.Sprintf("[Azure-CNS] Failed to sync node with error: %+v", err)
		logger.Errorf(errStr)
		return
	}

	var (
		ncsToBeAdded   = make(map[string]cns.CreateNetworkContainerRequest)
		ncsToBeDeleted = make(map[string]bool)
	)

	// determine new NCs and NCs to be deleted
	service.RLock()
	for ncid := range service.state.ContainerStatus {
		ncsToBeDeleted[ncid] = true
	}

	for _, nc := range nodeInfoResponse.NetworkContainers {
		ncid := nc.NetworkContainerid
		delete(ncsToBeDeleted, ncid)
		if savedNc, exists := service.state.ContainerStatus[ncid]; !exists || savedNc.CreateNetworkContainerRequest.Version < nc.Version {
			ncsToBeAdded[ncid] = nc
		}
	}
	service.RUnlock()

	// check if the version is valid and save it to service state
	for ncid, nc := range ncsToBeAdded {
		var (
			versionURL = fmt.Sprintf(nmagentclient.GetNetworkContainerVersionURLFmt,
				nmagentclient.WireserverIP,
				nc.PrimaryInterfaceIdentifier,
				nc.NetworkContainerid,
				nc.AuthorizationToken)
			w = httptest.NewRecorder()
		)

		ncVersionURLs.Store(nc.NetworkContainerid, versionURL)
		waitingForUpdate, _, _ := service.isNCWaitingForUpdate(nc.Version, nc.NetworkContainerid)

		body, _ = json.Marshal(nc)
		req, _ = http.NewRequest(http.MethodPost, "", bytes.NewBuffer(body))
		req.Header.Set(common.ContentType, common.JsonContent)
		service.createOrUpdateNetworkContainer(w, req)
		if w.Result().StatusCode == http.StatusOK {
			var resp cns.CreateNetworkContainerResponse
			if err = json.Unmarshal(w.Body.Bytes(), &resp); err == nil && resp.Response.ReturnCode == Success {
				service.Lock()
				ncstatus, _ := service.state.ContainerStatus[ncid]
				ncstatus.VfpUpdateComplete = !waitingForUpdate
				service.state.ContainerStatus[ncid] = ncstatus
				service.Unlock()
			}
		}
	}

	service.Lock()
	service.saveState()
	service.Unlock()

	if nodeInfoResponse.NmAgentApisMissing {
		// RegisterNode again with NmAgent Apis list
		retErr := RegisterNode(httpc, service, dncEP, infraVnet, nodeID)
		if retErr != nil {
			logger.Errorf("[Azure-CNS] Failed to register Node ID: %s with error: %s", nodeID, err.Error())
		}
	}

	// delete dangling NCs
	for nc := range ncsToBeDeleted {
		var body bytes.Buffer
		json.NewEncoder(&body).Encode(&cns.DeleteNetworkContainerRequest{NetworkContainerid: nc})

		req, err = http.NewRequest(http.MethodPost, "", &body)
		if err == nil {
			req.Header.Set(common.JsonContent, common.JsonContent)
			service.deleteNetworkContainer(httptest.NewRecorder(), req)
		} else {
			logger.Errorf("[Azure-CNS] Failed to delete NC request to sync state: %s", err.Error())
		}

		ncVersionURLs.Delete(nc)
	}

	return
}

// This API will be called by CNS RequestController on CRD update.
func (service *HTTPRestService) ReconcileNCState(ncRequest *cns.CreateNetworkContainerRequest, podInfoByIp map[string]cns.KubernetesPodInfo, scalar nnc.Scaler, spec nnc.NodeNetworkConfigSpec) int {
	// check if ncRequest is null, then return as there is no CRD state yet
	if ncRequest == nil {
		log.Logf("CNS starting with no NC state, podInfoMap count %d", len(podInfoByIp))
		return Success
	}

	returnCode := service.CreateOrUpdateNetworkContainerInternal(*ncRequest, scalar, spec)

	// If the NC was created successfully, then reconcile the allocated pod state
	if returnCode != Success {
		return returnCode
	}

	// now parse the secondaryIP list, if it exists in PodInfo list, then allocate that ip
	for _, secIpConfig := range ncRequest.SecondaryIPConfigs {
		if podInfo, exists := podInfoByIp[secIpConfig.IPAddress]; exists {
			log.Logf("SecondaryIP %+v is allocated to Pod. %+v, ncId: %s", secIpConfig, podInfo, ncRequest.NetworkContainerid)

			kubernetesPodInfo := cns.KubernetesPodInfo{
				PodName:      podInfo.PodName,
				PodNamespace: podInfo.PodNamespace,
			}
			jsonContext, _ := json.Marshal(kubernetesPodInfo)

			ipconfigRequest := cns.IPConfigRequest{
				DesiredIPAddress:    secIpConfig.IPAddress,
				OrchestratorContext: jsonContext,
			}

			if _, err := requestIPConfigHelper(service, ipconfigRequest); err != nil {
				log.Errorf("AllocateIPConfig failed for SecondaryIP %+v, podInfo %+v, ncId %s, error: %v", secIpConfig, podInfo, ncRequest.NetworkContainerid, err)
				return FailedToAllocateIpConfig
			}
		} else {
			log.Logf("SecondaryIP %+v is not allocated. ncId: %s", secIpConfig, ncRequest.NetworkContainerid)
		}
	}

	err := service.MarkExistingIPsAsPending(spec.IPsNotInUse)
	if err != nil {
		logger.Errorf("[Azure CNS] Error. Failed to mark IP's as pending %v", spec.IPsNotInUse)
		return UnexpectedError
	}

	return 0
}

// This API will be called by CNS RequestController on CRD update.
func (service *HTTPRestService) CreateOrUpdateNetworkContainerInternal(req cns.CreateNetworkContainerRequest, scalar nnc.Scaler, spec nnc.NodeNetworkConfigSpec) int {
	if req.NetworkContainerid == "" {
		logger.Errorf("[Azure CNS] Error. NetworkContainerid is empty")
		return NetworkContainerNotSpecified
	}

	// For now only RequestController uses this API which will be initialized only for AKS scenario.
	// Validate ContainerType is set as Docker
	if service.state.OrchestratorType != cns.KubernetesCRD {
		logger.Errorf("[Azure CNS] Error. Unsupported OrchestratorType: %s", service.state.OrchestratorType)
		return UnsupportedOrchestratorType
	}

	// Validate PrimaryCA must never be empty
	err := validateIPSubnet(req.IPConfiguration.IPSubnet)
	if err != nil {
		logger.Errorf("[Azure CNS] Error. PrimaryCA is invalid, NC Req: %v", req)
		return InvalidPrimaryIPConfig
	}

	// Validate SecondaryIPConfig
	for _, secIpconfig := range req.SecondaryIPConfigs {
		// Validate Ipconfig
		if secIpconfig.IPAddress == "" {
			logger.Errorf("Failed to add IPConfig to state: %+v, empty IPSubnet.IPAddress", secIpconfig)
			return InvalidSecondaryIPConfig
		}
	}

	// Validate if state exists already
	existingNCInfo, ok := service.getNetworkContainerDetails(req.NetworkContainerid)

	if ok {
		existingReq := existingNCInfo.CreateNetworkContainerRequest
		if reflect.DeepEqual(existingReq.IPConfiguration, req.IPConfiguration) != true {
			logger.Errorf("[Azure CNS] Error. PrimaryCA is not same, NCId %s, old CA %s, new CA %s", req.NetworkContainerid, existingReq.PrimaryInterfaceIdentifier, req.PrimaryInterfaceIdentifier)
			return PrimaryCANotSame
		}
	}

	// This will Create Or Update the NC state.
	returnCode, returnMessage := service.saveNetworkContainerGoalState(req)

	// If the NC was created successfully, log NC snapshot.
	if returnCode == 0 {
		logNCSnapshot(req)
	} else {
		logger.Errorf(returnMessage)
	}

	if err = service.IPAMPoolMonitor.Update(scalar, spec); err != nil {
		logger.Errorf("[cns-rc] Error creating or updating IPAM Pool Monitor: %v", err)
		// requeue
		return UnexpectedError
	}

	return returnCode
}

// RegisterNode - Tries to register node with DNC when CNS is started in managed DNC mode
func RegisterNode(httpc *http.Client, httpRestService cns.HTTPService, dncEP, infraVnet, nodeID string) error {
	logger.Printf("[Azure CNS] Registering node %s with Infrastructure Network: %s PrivateEndpoint: %s", nodeID, infraVnet, dncEP)

	var (
		numCPU              = runtime.NumCPU()
		url                 = fmt.Sprintf(common.RegisterNodeURLFmt, dncEP, infraVnet, nodeID, dncApiVersion)
		nodeRegisterRequest cns.NodeRegisterRequest
	)

	nodeRegisterRequest.NumCPU = numCPU
	supportedApis, retErr := nmagentclient.GetNmAgentSupportedApis(httpc, "")

	if retErr != nil {
		logger.Errorf("[Azure CNS] Failed to retrieve SupportedApis from NMagent of node %s with Infrastructure Network: %s PrivateEndpoint: %s",
			nodeID, infraVnet, dncEP)
		return retErr
	}

	//To avoid any null-pointer deferencing errors.
	if supportedApis == nil {
		supportedApis = []string{}
	}

	nodeRegisterRequest.NmAgentSupportedApis = supportedApis

	nodeRegisterTicker := time.NewTicker(common.FiveSeconds)
	responseChan := make(chan error)

	for {
		select {
		case responseErr := <-responseChan:
			return responseErr
		case <-nodeRegisterTicker.C:
			go sendRegisterNodeRequest(httpc, httpRestService, nodeRegisterRequest, url, responseChan)
		}
	}
}

// sendRegisterNodeRequest func helps in registering the node until there is an error.
func sendRegisterNodeRequest(
	httpc *http.Client,
	httpRestService cns.HTTPService,
	nodeRegisterRequest cns.NodeRegisterRequest,
	registerURL string,
	responseChan chan<- error) {

	var (
		body     bytes.Buffer
		response *http.Response
		err      = fmt.Errorf("")
	)

	err = json.NewEncoder(&body).Encode(nodeRegisterRequest)
	if err != nil {
		log.Errorf("encoding json failed with %v", err)
		responseChan <- err
		return
	}

	response, err = httpc.Post(registerURL, "application/json", &body)
	if err == nil {
		if response.StatusCode == http.StatusCreated {
			var req cns.SetOrchestratorTypeRequest
			decodeErr := json.NewDecoder(response.Body).Decode(&req)
			if decodeErr != nil {
				log.Errorf("decoding Node Resgister response json failed with %v", err)
				responseChan <- err
				return
			}
			httpRestService.SetNodeOrchestrator(&req)

			logger.Printf("[Azure CNS] Node Registered")
			responseChan <- nil
		} else {
			err = fmt.Errorf("[Azure CNS] Failed to register node with http status code %s", strconv.Itoa(response.StatusCode))
			logger.Errorf(err.Error())
			responseChan <- err
		}

		response.Body.Close()
	} else {
		logger.Errorf("[Azure CNS] Failed to register node with err: %+v", err)
	}
}
