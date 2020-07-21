// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package restserver

import (
	"fmt"
	"testing"
	"strconv"

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
	secondaryIPConfigs := make(map[string]cns.SecondaryIPConfig)

	var startingIndex = 6
	for i := 0; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, 32)
		ipId, err := uuid.NewUUID()

		if (err != nil) {
			t.Errorf("Failed to generate UUID for secondaryipconfig, err:%s", err)
			t.Fatal(err)
		}
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}
	
	createAndValidateNCRequest(t, secondaryIPConfigs)

	// now Validate Update, add more secondaryIpConfig and it should handle the update
	fmt.Println("Validate Scaleup")
	for i := 0; i < secondaryIpCount; i++ {
		ipaddress := "10.0.0." + strconv.Itoa(startingIndex)
		secIpConfig := newSecondaryIPConfig(ipaddress, 32)
		ipId, err := uuid.NewUUID()

		if (err != nil) {
			t.Errorf("Failed to generate UUID for secondaryipconfig, err:%s", err)
			t.Fatal(err)
		}
		secondaryIPConfigs[ipId.String()] = secIpConfig
		startingIndex++
	}

	createAndValidateNCRequest(t, secondaryIPConfigs)

	// now Scale down, delete 3 ipaddresses from secondaryIpConfig req
	fmt.Println("Validate Scalesown")
	var count = 0
	for ipid, _ := range secondaryIPConfigs {
		delete(secondaryIPConfigs, ipid)
		count++

		if count > secondaryIpCount {
			break
		}
	}

	createAndValidateNCRequest(t, secondaryIPConfigs)

	// Cleanup all SecondaryIps
	fmt.Println("Validate no SecondaryIpconfigs")
	for ipid, _ := range secondaryIPConfigs {
		delete(secondaryIPConfigs, ipid)
	}

	createAndValidateNCRequest(t, secondaryIPConfigs)

	return nil
}

func createAndValidateNCRequest(t *testing.T, secondaryIPConfigs map[string]cns.SecondaryIPConfig) {
	req := generateNetworkContainerRequest(secondaryIPConfigs)
	returnCode := svc.CreateOrUpdateNetworkContainerInternal(req)
	if returnCode != 0 {
		t.Errorf("Failed to createNetworkContainerRequest, req: %+v, err: %d", req, returnCode)
		t.Fatal()
	}
	validateNetworkRequest(t, req)
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
				if ipStatus.IPConfig.IPSubnet != secondaryIpConfig.IPSubnet {
					t.Errorf("IPId: %s IPSubnet doesnt match: expected %+v, actual: %+v", ipid, secondaryIpConfig.IPSubnet, ipStatus.IPConfig.IPSubnet)
					t.Fatal()
				}

				// Validate IP state
				if ipStatus.State != cns.Available {
					t.Errorf("IPId: %s State is not Available, ipStatus: %+v", ipid, ipStatus)
					t.Fatal()
				}

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

func generateNetworkContainerRequest(secondaryIps map[string]cns.SecondaryIPConfig) cns.CreateNetworkContainerRequest{
    var ipConfig cns.IPConfiguration
	ipConfig.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
	ipConfig.GatewayIPAddress = gatewayIp
	var ipSubnet cns.IPSubnet
	ipSubnet.IPAddress = primaryIp
	ipSubnet.PrefixLength = 32
	ipConfig.IPSubnet = ipSubnet

	req := cns.CreateNetworkContainerRequest{
		NetworkContainerType:       dockerContainerType,
		NetworkContainerid:         "testNcId1",
		IPConfiguration:            ipConfig,
	}

	req.SecondaryIPConfigs = make(map[string]cns.SecondaryIPConfig)
	for k, v := range secondaryIps {
		req.SecondaryIPConfigs[k] = v
	}

	return req
}