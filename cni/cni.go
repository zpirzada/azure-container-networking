// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	"encoding/json"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

// Plugin is the interface implemented by CNI plugins.
type Plugin interface {
	Add(args *cniSkel.CmdArgs) error
	Delete(args *cniSkel.CmdArgs) error

	AddImpl(args *cniSkel.CmdArgs, nwCfg *NetworkConfig) (*cniTypes.Result, error)
	DeleteImpl(args *cniSkel.CmdArgs, nwCfg *NetworkConfig) (*cniTypes.Result, error)
}

// NetworkConfig represents the Azure CNI plugin's network configuration.
type NetworkConfig struct {
	CniVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Bridge     string `json:"bridge"`
	IfName     string `json:"ifName"`
	Ipam       struct {
		Type      string `json:"type"`
		AddrSpace string `json:"addressSpace"`
		Subnet    string `json:"subnet"`
		Address   string `json:"ipAddress"`
		Result    string `json:"result"`
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
