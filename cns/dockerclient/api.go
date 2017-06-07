// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package dockerclient

import (
)

const (
	createNetworkPath	= "/Networks/Create"
	inspectNetworkPath	= "/Networks/"
)

// Config describes subnet/gateway for ipam
type Config struct {
	Subnet string
	Gateway string
}

// IPAM describes ipam details
type IPAM struct {
	Driver string
  Config *Config 
	Options map[string] string
}

// NetworkConfiguration describes configuration for docker entwork create
type NetworkConfiguration struct
{
  Name string
  Driver string
  IPAM *IPAM
  Internal bool
  Options map[string]string
  Labels map[string]string
}