package network

import (
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

// handleConsecutiveAdd is a dummy function for Linux platform.
func handleConsecutiveAdd(containerId, endpointId string, nwInfo *NetworkInfo, nwCfg *NetworkConfig) (*cniTypesCurr.Result, error) {
	return nil, nil
}
