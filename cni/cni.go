// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	"encoding/json"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

const (
	// Supported CNI versions.
	Version = "0.2.0"

	// CNI commands.
	CmdAdd = "ADD"
	CmdDel = "DEL"

	Internal = "internal"
)

// CNI contract.
type PluginApi interface {
	Add(args *cniSkel.CmdArgs) error
	Delete(args *cniSkel.CmdArgs) error
}

// CallPlugin calls the given CNI plugin through the internal interface.
func CallPlugin(plugin PluginApi, cmd string, args *cniSkel.CmdArgs, nwCfg *NetworkConfig) (*cniTypes.Result, error) {
	var err error

	savedType := nwCfg.Ipam.Type
	nwCfg.Ipam.Type = Internal
	args.StdinData = nwCfg.Serialize()

	// Call the plugin's internal interface.
	if cmd == CmdAdd {
		err = plugin.Add(args)
	} else {
		err = plugin.Delete(args)
	}

	nwCfg.Ipam.Type = savedType

	if err != nil {
		return nil, err
	}

	// Read back the result.
	var result cniTypes.Result
	err = json.Unmarshal(args.StdinData, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// NetworkConfig represents the Azure CNI plugin's network configuration.
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
