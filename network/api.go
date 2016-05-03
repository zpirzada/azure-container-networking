// Copyright Microsoft Corp.
// All rights reserved.

package network

const (
	// Libnetwork network plugin endpoint type
	endpointType = "NetworkDriver"

	// Libnetwork network plugin remote API paths
	getCapabilitiesPath  = "/NetworkDriver.GetCapabilities"
	createNetworkPath    = "/NetworkDriver.CreateNetwork"
	deleteNetworkPath    = "/NetworkDriver.DeleteNetwork"
	createEndpointPath   = "/NetworkDriver.CreateEndpoint"
	deleteEndpointPath   = "/NetworkDriver.DeleteEndpoint"
	joinPath             = "/NetworkDriver.Join"
	leavePath            = "/NetworkDriver.Leave"
	endpointOperInfoPath = "/NetworkDriver.EndpointOperInfo"
)
