// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package cni

import (
	"encoding/json"
)

// NetworkConfig represents Azure CNI plugin network configuration.
type NetworkConfig struct {
	CniVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Master     string `json:"master"`
	Bridge     string `json:"bridge,omitempty"`
	LogLevel   string `json:"logLevel,omitempty"`
	LogTarget  string `json:"logTarget,omitempty"`
	Ipam       struct {
		Type          string `json:"type"`
		Environment   string `json:"environment,omitempty"`
		AddrSpace     string `json:"addressSpace,omitempty"`
		Subnet        string `json:"subnet,omitempty"`
		Address       string `json:"ipAddress,omitempty"`
		QueryInterval string `json:"queryInterval,omitempty"`
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
