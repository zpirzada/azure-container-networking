// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package cns

// Container Network Service remote API Contract
const (	
	SetEnvironmentPath          = "/Network/Environment"
	CreateNetworkPath           = "/Network/Create"
	DeleteNetworkPath           = "/Network/Delete"
	ReserveIPAddressPath        = "/Network/IP/Reserve"
	ReleaseIPAddressPath        = "/Network/IP/Release"	
	GetHostLocalIPPath          = "/Network/IP/HostLocal"
	GetIPAddressUtilizationPath = "/Network/IP/Utilization"
	GetAvailableIPAddressesPath = "/Network/IPAddresses/Available"
	GetReservedIPAddressesPath  = "/Network/IPAddresses/Reserved"
	GetGhostIPAddressesPath     = "/Network/IPAddresses/Ghost"
	GetAllIPAddressesPath       = "/Network/IPAddresses/All"
	GetHealthReportPath         = "/Network/Health"
)

// SetEnvironmentRequest describes the Request to set the environment in HNS. 
type SetEnvironmentRequest struct {
	Location string
	NetworkType string
}

// OverlayConfiguration describes configuration for all the nodes that are part of overlay.
type OverlayConfiguration struct {
	NodeCount int
	LocalNodeIP string
	OverlaySubent Subnet
	NodeConfig []NodeConfiguration
}

// CreateNetworkRequest describes request to create the network.
type CreateNetworkRequest struct {
	NetworkName string
	OverlayConfiguration OverlayConfiguration
}

// DeleteNetworkRequest describes request to delete the network.
type DeleteNetworkRequest struct {
	NetworkName string
}

// ReserveIPAddressRequest describes request to reserve an IP Address
type ReserveIPAddressRequest struct {
	ReservationID string
}

// ReserveIPAddressResponse describes response to reserve an IP address.
type ReserveIPAddressResponse struct {
	Response Response
	IPAddress string
}

// ReleaseIPAddressRequest describes request to release an IP Address.
type ReleaseIPAddressRequest struct {
	ReservationID string
}

// IPAddressesUtilizationResponse describes response for ip address utilization.
type IPAddressesUtilizationResponse struct {
	Response Response
	Available int
	Reserved int
	Ghost int
}

// GetIPAddressesResponse describes response containing requested ip addresses.
type GetIPAddressesResponse struct {
	Response Response
	IPAddresses []IPAddress
}

// HostLocalIPAddressResponse describes reponse that returns the host local IP Address.
type HostLocalIPAddressResponse struct {
	Response Response
	IPAddress string
}

// IPAddress Contains information about an ip address.
type IPAddress struct {
	IPAddress string
	ReservationID string
	IsGhost bool
}

// Subnet contains the ip address and the number of bits in prefix.
type Subnet struct {
	IPAddress string
	PrefixLength int
}

// NodeConfiguration describes confguration for a node in overlay network.
type NodeConfiguration struct {
	NodeIP string
	NodeID string
	NodeSubnet Subnet
}

// Response describes generic response from CNS.
type Response struct {
	ReturnCode int
	Message string
}

// OptionMap describes generic options that can be passed to CNS.
type OptionMap map[string]interface{}

// Response to a failed request.
type errorResponse struct {
	Err string
}