// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	cniSkel "github.com/containernetworking/cni/pkg/skel"
)

const (
	// Supported CNI versions.
	Version = "0.2.0"

	// CNI commands.
	CmdAdd = "ADD"
	CmdDel = "DEL"
)

// CNI contract.
type PluginApi interface {
	Add(args *cniSkel.CmdArgs) error
	Delete(args *cniSkel.CmdArgs) error
}
