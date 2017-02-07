// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	cniSkel "github.com/containernetworking/cni/pkg/skel"
)

const (
	// CNI commands.
	CmdAdd = "ADD"
	CmdDel = "DEL"
)

// Supported CNI versions.
var supportedVersions = []string{"0.1.0", "0.2.0", "0.3.0"}

// CNI contract.
type PluginApi interface {
	Add(args *cniSkel.CmdArgs) error
	Delete(args *cniSkel.CmdArgs) error
}
