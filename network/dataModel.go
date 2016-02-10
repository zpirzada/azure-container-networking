// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"sync"
	"net"
)


type azureNetwork struct {
	networkId        string
	endpoints map[string]*azureEndpoint // key: endpoint id
	sync.Mutex
}

type azureEndpoint struct {
	endpointID string
	networkID string
	azureInterface azureInterface // lets support only one now
	sandboxKey string
}

type azureInterface struct {
	Address    net.IPNet
	AddressIPV6 net.IPNet
	MacAddress net.HardwareAddr
	ID         int
	SrcName    string
	DstPrefix  string
	GatewayIPv4 net.IP
}

var eid string
var srcName string
var dstName string
var azAddress net.IPNet
var macAddress net.HardwareAddr
var gatewayIPv4 net.IP
