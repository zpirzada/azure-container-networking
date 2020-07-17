// Copyright 2020 Microsoft. All rights reserved.
// MIT License

package restserver

import (
)

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