// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/google/uuid"
)

const (
	primaryIp = "10.0.0.5"
	gatewayIp = "10.0.0.1"
	dockerContainerType = cns.Docker
)

func TestCreateOrUpdateNetworkContainerInternal(t *testing.T) {
	// requires more than 30 seconds to run
	fmt.Println("Test: TestCreateOrUpdateNetworkContainerInternal")

	setEnv(t)
	setOrchastratorTypeInternal(cns.KubernetesCRD)

	err := createOrUpdateNetworkContainerInternal(t, 2)
	if err != nil {
		t.Errorf("Failed to save the goal state for network container of type JobObject "+
			" due to error: %+v", err)
		t.Fatal(err)
	}
}

func setOrchastratorTypeInternal(orchestratorType string) {
	fmt.Println("setOrchastratorTypeInternal")
	svc.state.OrchestratorType = orchestratorType
}

func createOrUpdateNetworkContainerInternal(t *testing.T, secondaryIpCount int) error {
	var ipConfig cns.IPConfiguration
	ipConfig.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
	ipConfig.GatewayIPAddress = gatewayIp
	var ipSubnet cns.IPSubnet
	ipSubnet.IPAddress = primaryIp
	ipSubnet.PrefixLength = 32
	ipConfig.IPSubnet = ipSubnet
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)

	var startingIndex = 6
	for i := 1; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + string(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, 32)
		ipId, err := uuid.NewUUID()

		if (err != nil) {
			t.Errorf("Failed to generate UUID for secondaryipconfig, err:%s", err)
			t.Fatal(err)
		}
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}
	
	req := cns.CreateNetworkContainerRequest{
		NetworkContainerType:       dockerContainerType,
		NetworkContainerid:         "testNcId1",
		IPConfiguration:            ipConfig,
		SecondaryIPConfigs: secondaryIPConfigs,
	}

	returnCode := svc.CreateOrUpdateNetworkContainerInternal(req)
	if returnCode != 0 {
		t.Errorf("Failed to createNetworkContainerRequest, req: %+v, err: %d", req, returnCode)
		t.Fatal()
	}

	validateNetworkRequest(t, req)

	return nil
}

// Validate the networkRequest is persisted.
func validateNetworkRequest(t *testing.T, req cns.CreateNetworkContainerRequest) {
	containerStatus := svc.state.ContainerStatus[req.NetworkContainerid]

	if containerStatus.ID != req.NetworkContainerid {
		t.Errorf("Failed as NCId is not persisted, expected:%s, actual %s", req.NetworkContainerid, containerStatus.ID)
		t.Fatal()
	}

	actualReq := containerStatus.CreateNetworkContainerRequest
	if actualReq.NetworkContainerType != req.NetworkContainerType {
		t.Errorf("Failed as ContainerTyper doesnt match, expected:%s, actual %s", req.NetworkContainerType, actualReq.NetworkContainerType)
		t.Fatal()
	}

    if actualReq.IPConfiguration.IPSubnet.IPAddress != req.IPConfiguration.IPSubnet.IPAddress {
		t.Errorf("Failed as Primary IPAddress doesnt match, expected:%s, actual %s", req.IPConfiguration.IPSubnet.IPAddress, actualReq.IPConfiguration.IPSubnet.IPAddress)
		t.Fatal()
	}

	// Validate Secondary ips are added in the PodMap
	if len(svc.PodIPConfigState) != len(req.SecondaryIPConfigs) {
		t.Errorf("Failed as Secondary IP count doesnt match in PodIpConfig state, expected:%d, actual %d", len(req.SecondaryIPConfigs), len(svc.PodIPConfigState))
		t.Fatal()
	}

	var alreadyValidated = make(map[string]string)
	for ipid, ipStatus := range svc.PodIPConfigState {
		if ipaddress, found := alreadyValidated[ipid]; !found {
			if secondaryIpConfig, ok := req.SecondaryIPConfigs[ipid]; !ok {
				t.Errorf("PodIpConfigState has stale ipId: %s, config: %+v", ipid, ipStatus)
				t.Fatal()
			} else {
				if ipStatus.IPConfig.IPSubnet == secondaryIpConfig.IPSubnet {
					t.Errorf("IPId: %s IPSubnet doesnt match: expected %+v, actual: %+v", ipid, secondaryIpConfig.IPSubnet, ipStatus.IPConfig.IPSubnet)
					t.Fatal()
				}

				// Validate IP state
				
				alreadyValidated[ipid] = ipStatus.IPConfig.IPSubnet.IPAddress
			}
		} else {
			// if ipaddress is not same, then fail
			if ipaddress != ipStatus.IPConfig.IPSubnet.IPAddress {
				t.Errorf("Added the same IP guid :%s with different ipaddress, expected:%s, actual %s", ipid, ipStatus.IPConfig.IPSubnet.IPAddress, ipaddress)
				t.Fatal()
			}
		}

	}
}


// func TestIPAMExpectFailWhenAddingBadIPConfig(t *testing.T) {
// 	svc := getTestService()

// 	var err error

// 	// set state as already allocated
// 	state1, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Available, testPod1Info)

// 	ipconfigs := map[string]IpConfigurationStatus{
// 		state1.ID: state1,
// 	}

// 	err = UpdatePodIpConfigState(svc, ipconfigs)
// 	if err != nil {
// 		t.Fatalf("Expected to not fail when good ipconfig is added")
// 	}

// 	// create bad ipconfig
// 	state2, _ := NewPodStateWithOrchestratorContext("", 24, "", testNCID, cns.Available, testPod1Info)

// 	ipconfigs2 := map[string]IpConfigurationStatus{
// 		state2.ID: state2,
// 	}

// 	// add a bad ipconfig
// 	err = UpdatePodIpConfigState(svc, ipconfigs2)
// 	if err == nil {
// 		t.Fatalf("Expected add to fail when bad ipconfig is added.")
// 	}

// 	// ensure state remains untouched
// 	if len(svc.PodIPConfigState) != 1 {
// 		t.Fatalf("Expected bad ipconfig to not be added added.")
// 	}
// }

// func TestIPAMExpectStateToNotChangeWhenChangingAllocatedToAvailable(t *testing.T) {
// 	svc := getTestService()
// 	// add two ipconfigs, one as available, the other as allocated
// 	state1, _ := NewPodStateWithOrchestratorContext(testIP1, 24, testPod1GUID, testNCID, cns.Available, testPod1Info)
// 	state2, _ := NewPodStateWithOrchestratorContext(testIP2, 24, testPod2GUID, testNCID, cns.Allocated, testPod2Info)

// 	ipconfigs := map[string]IpConfigurationStatus{
// 		state1.ID: state1,
// 		state2.ID: state2,
// 	}

// 	err := UpdatePodIpConfigState(svc, ipconfigs)
// 	if err != nil {
// 		t.Fatalf("Expected to not fail adding IP's to state: %+v", err)
// 	}

// 	// create state2 again, but as available
// 	state2Available, _ := NewPodStateWithOrchestratorContext(testIP2, 24, testPod2GUID, testNCID, cns.Available, testPod2Info)

// 	// add an available and allocated ipconfig
// 	ipconfigsTest := map[string]IpConfigurationStatus{
// 		state1.ID: state1,
// 		state2.ID: state2Available,
// 	}

// 	// expect to fail overwriting an allocated state with available
// 	err = UpdatePodIpConfigState(svc, ipconfigsTest)
// 	if err == nil {
// 		t.Fatalf("Expected to fail when overwriting an allocated state as available: %+v", err)
// 	}

// 	// get allocated ipconfigs, should only be one from the inital call, and not 2 from the failed call
// 	availableIPconfigs := svc.GetAvailableIPConfigs()
// 	if len(availableIPconfigs) != 1 {
// 		t.Fatalf("More than expected available IP configs in state")
// 	}

// 	// get allocated ipconfigs, should only be one from the inital call, and not 0 from the failed call
// 	allocatedIPconfigs := svc.GetAllocatedIPConfigs()
// 	if len(allocatedIPconfigs) != 1 {
// 		t.Fatalf("More than expected allocated IP configs in state")
// 	}
// }