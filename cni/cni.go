// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	"encoding/json"
)

// NetworkConfig represents the Azure CNI plugin's network configuration.
type NetworkConfig struct {
	CniVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Bridge     string `json:"bridge,omitempty"`
	IfName     string `json:"ifName,omitempty"`
	Ipam       struct {
		Type      string `json:"type"`
		AddrSpace string `json:"addressSpace,omitempty"`
		Subnet    string `json:"subnet,omitempty"`
		Address   string `json:"ipAddress,omitempty"`
	}
}

// ParseNetworkConfig unmarshals network configuration from bytes.
func ParseNetworkConfig(b []byte) (*NetworkConfig, error) {
	nwCfg := NetworkConfig{}

	err := json.Unmarshal(b, &nwCfg)
	if err != nil {
		return nil, err
	}

	return &nwCfg, nil
}

// Serialize marshals a network configuration to bytes.
func (nwcfg *NetworkConfig) Serialize() []byte {
	bytes, _ := json.Marshal(nwcfg)
	return bytes
}
