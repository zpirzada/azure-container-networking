// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/fakes"
	"github.com/Azure/azure-container-networking/cns/types"
	"github.com/Azure/azure-container-networking/crd/nodenetworkconfig/api/v1alpha"
	nma "github.com/Azure/azure-container-networking/nmagent"
	"github.com/google/uuid"
)

const (
	primaryIp           = "10.0.0.5"
	gatewayIp           = "10.0.0.1"
	subnetPrfixLength   = 24
	dockerContainerType = cns.Docker
	releasePercent      = 50
	requestPercent      = 100
	batchSize           = 10
	initPoolSize        = 10
)

var dnsservers = []string{"8.8.8.8", "8.8.4.4"}

func TestCreateOrUpdateNetworkContainerInternal(t *testing.T) {
	restartService()

	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)
	// NC version set as -1 which is the same as default host version value.
	validateCreateOrUpdateNCInternal(t, 2, "-1")
}

func TestCreateOrUpdateNCWithLargerVersionComparedToNMAgent(t *testing.T) {
	restartService()

	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)
	// NC version set as 1 which is larger than NC version get from mock nmagent.
	validateCreateNCInternal(t, 2, "1")
}

func TestCreateAndUpdateNCWithSecondaryIPNCVersion(t *testing.T) {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)
	// NC version set as 0 which is the default initial value.
	ncVersion := 0
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)
	ncID := "testNc1"

	// Build secondaryIPConfig, it will have one item as {IPAddress:"10.0.0.16", NCVersion: 0}
	ipAddress := "10.0.0.16"
	secIPConfig := newSecondaryIPConfig(ipAddress, ncVersion)
	ipId := uuid.New()
	secondaryIPConfigs[ipId.String()] = secIPConfig
	req := createNCReqInternal(t, secondaryIPConfigs, ncID, strconv.Itoa(ncVersion))
	containerStatus := svc.state.ContainerStatus[req.NetworkContainerid]
	// Validate secondary IPs' NC version has been updated by NC request
	receivedSecondaryIPConfigs := containerStatus.CreateNetworkContainerRequest.SecondaryIPConfigs
	if len(receivedSecondaryIPConfigs) != 1 {
		t.Fatalf("receivedSecondaryIPConfigs lenth must be 1, but recieved %d", len(receivedSecondaryIPConfigs))
	}
	for _, secIPConfig := range receivedSecondaryIPConfigs {
		if secIPConfig.IPAddress != "10.0.0.16" || secIPConfig.NCVersion != 0 {
			t.Fatalf("nc request version is %d, secondary ip %s nc version is %d, expected nc version is 0",
				ncVersion, secIPConfig.IPAddress, secIPConfig.NCVersion)
		}
	}

	// now Validate Update, simulate the CRD status where have 2 IP addresses as "10.0.0.16" and "10.0.0.17" with NC version 1
	// The secondaryIPConfigs build from CRD will be {[IPAddress:"10.0.0.16", NCVersion: 1,]; [IPAddress:"10.0.0.17", NCVersion: 1,]}
	// However, when CNS saveNetworkContainerGoalState, since "10.0.0.16" already with NC version 0, it will set it back to 0.
	ncVersion++
	secIPConfig = newSecondaryIPConfig(ipAddress, ncVersion)
	// "10.0.0.16" will be update to NC version 1 in CRD, reuse the same uuid with it.
	secondaryIPConfigs[ipId.String()] = secIPConfig

	// Add {IPAddress:"10.0.0.17", NCVersion: 1} in secondaryIPConfig
	ipAddress = "10.0.0.17"
	secIPConfig = newSecondaryIPConfig(ipAddress, ncVersion)
	ipId = uuid.New()
	secondaryIPConfigs[ipId.String()] = secIPConfig
	req = createNCReqInternal(t, secondaryIPConfigs, ncID, strconv.Itoa(ncVersion))
	// Validate secondary IPs' NC version has been updated by NC request
	containerStatus = svc.state.ContainerStatus[req.NetworkContainerid]
	receivedSecondaryIPConfigs = containerStatus.CreateNetworkContainerRequest.SecondaryIPConfigs
	if len(receivedSecondaryIPConfigs) != 2 {
		t.Fatalf("receivedSecondaryIPConfigs must be 2, but received %d", len(receivedSecondaryIPConfigs))
	}
	for _, secIPConfig := range receivedSecondaryIPConfigs {
		switch secIPConfig.IPAddress {
		case "10.0.0.16":
			// Though "10.0.0.16" IP exists in NC version 1, secodanry IP still keep its original NC version 0
			if secIPConfig.NCVersion != 0 {
				t.Fatalf("nc request version is %d, secondary ip %s nc version is %d, expected nc version is 0",
					ncVersion, secIPConfig.IPAddress, secIPConfig.NCVersion)
			}
		case "10.0.0.17":
			if secIPConfig.NCVersion != 1 {
				t.Fatalf("nc request version is %d, secondary ip %s nc version is %d, expected nc version is 1",
					ncVersion, secIPConfig.IPAddress, secIPConfig.NCVersion)
			}
		default:
			t.Fatalf("nc request version is %d, secondary ip %s nc version is %d should not exist in receivedSecondaryIPConfigs map",
				ncVersion, secIPConfig.IPAddress, secIPConfig.NCVersion)
		}
	}
}

func TestSyncHostNCVersion(t *testing.T) {
	// cns.KubernetesCRD has one more logic compared to other orchestrator type, so test both of them
	orchestratorTypes := []string{cns.Kubernetes, cns.KubernetesCRD}
	for _, orchestratorType := range orchestratorTypes {
		t.Run(orchestratorType, func(t *testing.T) {
			req := createNCReqeustForSyncHostNCVersion(t)
			containerStatus := svc.state.ContainerStatus[req.NetworkContainerid]
			if containerStatus.HostVersion != "-1" {
				t.Errorf("Unexpected containerStatus.HostVersion %s, expected host version should be -1 in string", containerStatus.HostVersion)
			}
			if containerStatus.CreateNetworkContainerRequest.Version != "0" {
				t.Errorf("Unexpected nc version in containerStatus as %s, expected VM version should be 0 in string", containerStatus.CreateNetworkContainerRequest.Version)
			}

			mnma := &fakes.NMAgentClientFake{
				GetNCVersionListF: func(_ context.Context) (nma.NCVersionList, error) {
					return nma.NCVersionList{
						Containers: []nma.NCVersion{
							{
								NetworkContainerID: req.NetworkContainerid,
								Version:            "0",
							},
						},
					}, nil
				},
			}
			cleanup := setMockNMAgent(svc, mnma)
			defer cleanup()

			// When syncing the host NC version, it will use the orchestratorType passed
			// in.
			svc.SyncHostNCVersion(context.Background(), orchestratorType)
			containerStatus = svc.state.ContainerStatus[req.NetworkContainerid]
			if containerStatus.HostVersion != "0" {
				t.Errorf("Unexpected containerStatus.HostVersion %s, expected host version should be 0 in string", containerStatus.HostVersion)
			}
			if containerStatus.CreateNetworkContainerRequest.Version != "0" {
				t.Errorf("Unexpected nc version in containerStatus as %s, expected VM version should be 0 in string", containerStatus.CreateNetworkContainerRequest.Version)
			}
		})
	}
}

func TestPendingIPsGotUpdatedWhenSyncHostNCVersion(t *testing.T) {
	req := createNCReqeustForSyncHostNCVersion(t)
	containerStatus := svc.state.ContainerStatus[req.NetworkContainerid]

	receivedSecondaryIPConfigs := containerStatus.CreateNetworkContainerRequest.SecondaryIPConfigs
	if len(receivedSecondaryIPConfigs) != 1 {
		t.Errorf("Unexpected receivedSecondaryIPConfigs length %d, expeted length is 1", len(receivedSecondaryIPConfigs))
	}
	for i := range receivedSecondaryIPConfigs {
		podIPConfigState := svc.PodIPConfigState[i]
		if podIPConfigState.GetState() != types.PendingProgramming {
			t.Errorf("Unexpected State %s, expected State is %s, IP address is %s", podIPConfigState.GetState(), types.PendingProgramming, podIPConfigState.IPAddress)
		}
	}

	mnma := &fakes.NMAgentClientFake{
		GetNCVersionListF: func(_ context.Context) (nma.NCVersionList, error) {
			return nma.NCVersionList{
				Containers: []nma.NCVersion{
					{
						NetworkContainerID: req.NetworkContainerid,
						Version:            "0",
					},
				},
			}, nil
		},
	}
	cleanup := setMockNMAgent(svc, mnma)
	defer cleanup()

	svc.SyncHostNCVersion(context.Background(), cns.CRD)
	containerStatus = svc.state.ContainerStatus[req.NetworkContainerid]

	receivedSecondaryIPConfigs = containerStatus.CreateNetworkContainerRequest.SecondaryIPConfigs
	if len(receivedSecondaryIPConfigs) != 1 {
		t.Errorf("Unexpected receivedSecondaryIPConfigs length %d, expeted length is 1", len(receivedSecondaryIPConfigs))
	}
	for i := range receivedSecondaryIPConfigs {
		podIPConfigState := svc.PodIPConfigState[i]
		if podIPConfigState.GetState() != types.Available {
			t.Errorf("Unexpected State %s, expeted State is %s, IP address is %s", podIPConfigState.GetState(), types.Available, podIPConfigState.IPAddress)
		}
	}
}

func createNCReqeustForSyncHostNCVersion(t *testing.T) cns.CreateNetworkContainerRequest {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)

	// NC version set as 0 which is the default initial value.
	ncVersion := 0
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)
	ncID := "testNc1"

	// Build secondaryIPConfig, it will have one item as {IPAddress:"10.0.0.16", NCVersion: 0}
	ipAddress := "10.0.0.16"
	secIPConfig := newSecondaryIPConfig(ipAddress, ncVersion)
	ipID := uuid.New()
	secondaryIPConfigs[ipID.String()] = secIPConfig
	req := createNCReqInternal(t, secondaryIPConfigs, ncID, strconv.Itoa(ncVersion))
	return req
}

func TestReconcileNCWithEmptyState(t *testing.T) {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)

	expectedNcCount := len(svc.state.ContainerStatus)
	expectedAssignedPods := make(map[string]cns.PodInfo)
	returnCode := svc.ReconcileNCState(nil, expectedAssignedPods, &v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	if returnCode != types.Success {
		t.Errorf("Unexpected failure on reconcile with no state %d", returnCode)
	}

	validateNCStateAfterReconcile(t, nil, expectedNcCount, expectedAssignedPods)
}

func TestReconcileNCWithExistingState(t *testing.T) {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)

	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)

	startingIndex := 6
	for i := 0; i < 4; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, -1)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}
	req := generateNetworkContainerRequest(secondaryIPConfigs, "reconcileNc1", "-1")

	expectedAssignedPods := map[string]cns.PodInfo{
		"10.0.0.6": cns.NewPodInfo("", "", "reconcilePod1", "PodNS1"),
		"10.0.0.7": cns.NewPodInfo("", "", "reconcilePod2", "PodNS1"),
	}

	expectedNcCount := len(svc.state.ContainerStatus)
	returnCode := svc.ReconcileNCState(req, expectedAssignedPods, &v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	if returnCode != types.Success {
		t.Errorf("Unexpected failure on reconcile with no state %d", returnCode)
	}

	validateNCStateAfterReconcile(t, req, expectedNcCount+1, expectedAssignedPods)
}

func TestReconcileNCWithExistingStateFromInterfaceID(t *testing.T) {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)
	cns.GlobalPodInfoScheme = cns.InterfaceIDPodInfoScheme
	defer func() { cns.GlobalPodInfoScheme = cns.KubernetesPodInfoScheme }()

	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)

	startingIndex := 6
	for i := 0; i < 4; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, -1)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}
	req := generateNetworkContainerRequest(secondaryIPConfigs, "reconcileNc1", "-1")

	expectedAssignedPods := map[string]cns.PodInfo{
		"10.0.0.6": cns.NewPodInfo("abcdef", "recon1-eth0", "reconcilePod1", "PodNS1"),
		"10.0.0.7": cns.NewPodInfo("abcxyz", "recon2-eth0", "reconcilePod2", "PodNS1"),
	}

	expectedNcCount := len(svc.state.ContainerStatus)
	returnCode := svc.ReconcileNCState(req, expectedAssignedPods, &v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	if returnCode != types.Success {
		t.Errorf("Unexpected failure on reconcile with no state %d", returnCode)
	}

	validateNCStateAfterReconcile(t, req, expectedNcCount+1, expectedAssignedPods)
}

func TestReconcileNCWithSystemPods(t *testing.T) {
	restartService()
	setEnv(t)
	setOrchestratorTypeInternal(cns.KubernetesCRD)

	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)

	startingIndex := 6
	for i := 0; i < 4; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, -1)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}
	req := generateNetworkContainerRequest(secondaryIPConfigs, uuid.New().String(), "-1")

	expectedAssignedPods := make(map[string]cns.PodInfo)
	expectedAssignedPods["10.0.0.6"] = cns.NewPodInfo("", "", "customerpod1", "PodNS1")

	// Allocate non-vnet IP for system  pod
	expectedAssignedPods["192.168.0.1"] = cns.NewPodInfo("", "", "systempod", "kube-system")

	expectedNcCount := len(svc.state.ContainerStatus)
	returnCode := svc.ReconcileNCState(req, expectedAssignedPods, &v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	if returnCode != types.Success {
		t.Errorf("Unexpected failure on reconcile with no state %d", returnCode)
	}

	delete(expectedAssignedPods, "192.168.0.1")
	validateNCStateAfterReconcile(t, req, expectedNcCount, expectedAssignedPods)
}

func setOrchestratorTypeInternal(orchestratorType string) {
	fmt.Println("setOrchestratorTypeInternal")
	svc.state.OrchestratorType = orchestratorType
}

func validateCreateNCInternal(t *testing.T, secondaryIpCount int, ncVersion string) {
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)
	ncId := "testNc1"
	ncVersionInInt, _ := strconv.Atoi(ncVersion)
	startingIndex := 6
	for i := 0; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, ncVersionInInt)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}

	createAndValidateNCRequest(t, secondaryIPConfigs, ncId, ncVersion)
}

func validateCreateOrUpdateNCInternal(t *testing.T, secondaryIpCount int, ncVersion string) {
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)
	ncId := "testNc1"
	ncVersionInInt, _ := strconv.Atoi(ncVersion)
	startingIndex := 6
	for i := 0; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, ncVersionInInt)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}

	createAndValidateNCRequest(t, secondaryIPConfigs, ncId, ncVersion)

	// now Validate Update, add more secondaryIpConfig and it should handle the update
	fmt.Println("Validate Scaleup")
	for i := 0; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, ncVersionInInt)
		ipId := uuid.New()
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}

	createAndValidateNCRequest(t, secondaryIPConfigs, ncId, ncVersion)

	// now Scale down, delete 3 ipaddresses from secondaryIpConfig req
	fmt.Println("Validate Scale down")
	count := 0
	for ipid := range secondaryIPConfigs {
		delete(secondaryIPConfigs, ipid)
		count++

		if count > secondaryIpCount {
			break
		}
	}

	createAndValidateNCRequest(t, secondaryIPConfigs, ncId, ncVersion)

	// Cleanup all SecondaryIps
	fmt.Println("Validate no SecondaryIpconfigs")
	for ipid := range secondaryIPConfigs {
		delete(secondaryIPConfigs, ipid)
	}

	createAndValidateNCRequest(t, secondaryIPConfigs, ncId, ncVersion)
}

func createAndValidateNCRequest(t *testing.T, secondaryIPConfigs map[string]cns.SecondaryIPConfig, ncId, ncVersion string) {
	req := generateNetworkContainerRequest(secondaryIPConfigs, ncId, ncVersion)
	returnCode := svc.CreateOrUpdateNetworkContainerInternal(req)
	if returnCode != 0 {
		t.Fatalf("Failed to createNetworkContainerRequest, req: %+v, err: %d", req, returnCode)
	}
	_ = svc.IPAMPoolMonitor.Update(&v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	validateNetworkRequest(t, *req)
}

// Validate the networkRequest is persisted.
func validateNetworkRequest(t *testing.T, req cns.CreateNetworkContainerRequest) {
	containerStatus := svc.state.ContainerStatus[req.NetworkContainerid]

	if containerStatus.ID != req.NetworkContainerid {
		t.Fatalf("Failed as NCId is not persisted, expected:%s, actual %s", req.NetworkContainerid, containerStatus.ID)
	}

	actualReq := containerStatus.CreateNetworkContainerRequest
	if actualReq.NetworkContainerType != req.NetworkContainerType {
		t.Fatalf("Failed as ContainerTyper doesnt match, expected:%s, actual %s", req.NetworkContainerType, actualReq.NetworkContainerType)
	}

	if actualReq.IPConfiguration.IPSubnet.IPAddress != req.IPConfiguration.IPSubnet.IPAddress {
		t.Fatalf("Failed as Primary IPAddress doesnt match, expected:%s, actual %s", req.IPConfiguration.IPSubnet.IPAddress, actualReq.IPConfiguration.IPSubnet.IPAddress)
	}

	// Validate Secondary ips are added in the PodMap
	if len(svc.PodIPConfigState) != len(req.SecondaryIPConfigs) {
		t.Fatalf("Failed as Secondary IP count doesnt match in PodIpConfig state, expected:%d, actual %d", len(req.SecondaryIPConfigs), len(svc.PodIPConfigState))
	}

	var expectedIPStatus types.IPState
	// 0 is the default NMAgent version return from fake GetNetworkContainerInfoFromHost
	if containerStatus.CreateNetworkContainerRequest.Version > "0" {
		expectedIPStatus = types.PendingProgramming
	} else {
		expectedIPStatus = types.Available
	}
	t.Logf("NC version in container status is %s, HostVersion is %s", containerStatus.CreateNetworkContainerRequest.Version, containerStatus.HostVersion)
	alreadyValidated := make(map[string]string)
	for ipid, ipStatus := range svc.PodIPConfigState {
		if ipaddress, found := alreadyValidated[ipid]; !found {
			if secondaryIpConfig, ok := req.SecondaryIPConfigs[ipid]; !ok {
				t.Fatalf("PodIpConfigState has stale ipId: %s, config: %+v", ipid, ipStatus)
			} else {
				if ipStatus.IPAddress != secondaryIpConfig.IPAddress {
					t.Fatalf("IPId: %s IPSubnet doesnt match: expected %+v, actual: %+v", ipid, secondaryIpConfig.IPAddress, ipStatus.IPAddress)
				}

				// Validate IP state
				if ipStatus.PodInfo != nil {
					if _, exists := svc.PodIPIDByPodInterfaceKey[ipStatus.PodInfo.Key()]; exists {
						if ipStatus.GetState() != types.Assigned {
							t.Fatalf("IPId: %s State is not Assigned, ipStatus: %+v", ipid, ipStatus)
						}
					} else {
						t.Fatalf("Failed to find podContext for assigned ip: %+v, podinfo :%+v", ipStatus, ipStatus.PodInfo)
					}
				} else if ipStatus.GetState() != expectedIPStatus {
					// Todo: Validate for pendingRelease as well
					t.Fatalf("IPId: %s State is not as expected, ipStatus is : %+v, expected status is %+v", ipid, ipStatus.GetState(), expectedIPStatus)
				}

				alreadyValidated[ipid] = ipStatus.IPAddress
			}
		} else {
			// if ipaddress is not same, then fail
			if ipaddress != ipStatus.IPAddress {
				t.Fatalf("Added the same IP guid :%s with different ipaddress, expected:%s, actual %s", ipid, ipStatus.IPAddress, ipaddress)
			}
		}
	}
}

func generateNetworkContainerRequest(secondaryIps map[string]cns.SecondaryIPConfig, ncID, ncVersion string) *cns.CreateNetworkContainerRequest {
	var ipConfig cns.IPConfiguration
	ipConfig.DNSServers = dnsservers
	ipConfig.GatewayIPAddress = gatewayIp
	var ipSubnet cns.IPSubnet
	ipSubnet.IPAddress = primaryIp
	ipSubnet.PrefixLength = subnetPrfixLength
	ipConfig.IPSubnet = ipSubnet

	req := cns.CreateNetworkContainerRequest{
		NetworkContainerType: dockerContainerType,
		NetworkContainerid:   ncID,
		IPConfiguration:      ipConfig,
		Version:              ncVersion,
	}

	ncVersionInInt, _ := strconv.Atoi(ncVersion)
	req.SecondaryIPConfigs = make(map[string]cns.SecondaryIPConfig)
	for k, v := range secondaryIps {
		req.SecondaryIPConfigs[k] = v
		ipconfig := req.SecondaryIPConfigs[k]
		ipconfig.NCVersion = ncVersionInInt
	}

	fmt.Printf("NC Request %+v", req)

	return &req
}

func validateNCStateAfterReconcile(t *testing.T, ncRequest *cns.CreateNetworkContainerRequest, expectedNcCount int, expectedAssignedPods map[string]cns.PodInfo) {
	if ncRequest == nil {
		// check svc ContainerStatus will be empty
		if len(svc.state.ContainerStatus) != expectedNcCount {
			t.Fatalf("CNS has some stale ContainerStatus, count: %d, state: %+v", len(svc.state.ContainerStatus), svc.state.ContainerStatus)
		}
	} else {
		validateNetworkRequest(t, *ncRequest)
	}

	if len(expectedAssignedPods) != len(svc.PodIPIDByPodInterfaceKey) {
		t.Fatalf("Unexpected assigned pods, actual: %d, expected: %d", len(svc.PodIPIDByPodInterfaceKey), len(expectedAssignedPods))
	}

	for ipaddress, podInfo := range expectedAssignedPods {	
		for _, IPId := range svc.PodIPIDByPodInterfaceKey[podInfo.Key()] {
			ipConfigstate := svc.PodIPConfigState[ipId]
			if ipConfigstate.GetState() != types.Assigned {
				t.Fatalf("IpAddress %s is not marked as assigned to Pod: %+v, ipState: %+v", ipaddress, podInfo, ipConfigstate)
			}

			// Validate if IPAddress matches
			if ipConfigstate.IPAddress != ipaddress {
				t.Fatalf("IpAddress %s is not same, for Pod: %+v, actual ipState: %+v", ipaddress, podInfo, ipConfigstate)
			}

			// Valdate pod context
			if reflect.DeepEqual(ipConfigstate.PodInfo, podInfo) != true {
				t.Fatalf("OrchestrationContext: is not same, expected: %+v, actual %+v", ipConfigstate.PodInfo, podInfo)
			}

			// Validate this IP belongs to a valid NCRequest
			nc := svc.state.ContainerStatus[ipConfigstate.NCID]
			if _, exists := nc.CreateNetworkContainerRequest.SecondaryIPConfigs[ipConfigstate.ID]; !exists {
				t.Fatalf("Secondary IP config doest exist in NC, ncid: %s, ipId %s", ipConfigstate.NCID, ipConfigstate.ID)
			}
		}
	}

	// validate rest of Secondary IPs in Available state
	if ncRequest != nil {
		for secIpId, secIpConfig := range ncRequest.SecondaryIPConfigs {
			if _, exists := expectedAssignedPods[secIpConfig.IPAddress]; exists {
				continue
			}

			// Validate IP state
			if secIpConfigState, found := svc.PodIPConfigState[secIpId]; found {
				if secIpConfigState.GetState() != types.Available {
					t.Fatalf("IPId: %s State is not Available, ipStatus: %+v", secIpId, secIpConfigState)
				}
			} else {
				t.Fatalf("IPId: %s, IpAddress: %+v State doesnt exists in PodIp Map", secIpId, secIpConfig)
			}
		}
	}
}

func createNCReqInternal(t *testing.T, secondaryIPConfigs map[string]cns.SecondaryIPConfig, ncID, ncVersion string) cns.CreateNetworkContainerRequest {
	req := generateNetworkContainerRequest(secondaryIPConfigs, ncID, ncVersion)
	returnCode := svc.CreateOrUpdateNetworkContainerInternal(req)
	if returnCode != 0 {
		t.Fatalf("Failed to createNetworkContainerRequest, req: %+v, err: %d", req, returnCode)
	}
	_ = svc.IPAMPoolMonitor.Update(&v1alpha.NodeNetworkConfig{
		Status: v1alpha.NodeNetworkConfigStatus{
			Scaler: v1alpha.Scaler{
				BatchSize:               batchSize,
				ReleaseThresholdPercent: releasePercent,
				RequestThresholdPercent: requestPercent,
			},
		},
		Spec: v1alpha.NodeNetworkConfigSpec{
			RequestedIPCount: initPoolSize,
		},
	})
	return *req
}

func restartService() {
	fmt.Println("Restart Service")

	service.Stop()
	if err := startService(); err != nil {
		fmt.Printf("Failed to restart CNS Service. Error: %v", err)
		os.Exit(1)
	}
}
