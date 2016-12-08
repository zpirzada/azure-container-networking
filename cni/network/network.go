// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"net"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/ipam"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

const (
	// Plugin name.
	name = "net"

	// The default address space ID used when an explicit ID is not specified.
	defaultAddressSpaceId = "LocalDefaultAddressSpace"
)

// NetPlugin object and its interface
type netPlugin struct {
	*common.Plugin
	nm network.NetworkManager
	am ipam.AddressManager
}

// NewPlugin creates a new netPlugin object.
func NewPlugin(config *common.PluginConfig) (*netPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, config.Version)
	if err != nil {
		return nil, err
	}

	// Setup network manager.
	nm, err := network.NewNetworkManager()
	if err != nil {
		return nil, err
	}

	config.NetApi = nm

	return &netPlugin{
		Plugin: plugin,
		nm:     nm,
	}, nil
}

// Starts the plugin.
func (plugin *netPlugin) Start(config *common.PluginConfig) error {
	// Initialize base plugin.
	err := plugin.Initialize(config)
	if err != nil {
		log.Printf("[net] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Initialize network manager.
	err = plugin.nm.Initialize(config)
	if err != nil {
		log.Printf("[net] Failed to initialize network manager, err:%v.", err)
		return err
	}

	plugin.am, _ = config.IpamApi.(ipam.AddressManager)

	log.Printf("[net] Plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *netPlugin) Stop() {
	plugin.nm.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[net] Plugin stopped.")
}

//
// CNI implementation
// https://github.com/containernetworking/cni/blob/master/SPEC.md
//

// Add handles CNI add commands.
func (plugin *netPlugin) Add(args *cniSkel.CmdArgs) error {
	log.Printf("[cni] Processing ADD command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[cni] Failed to parse network configuration, err:%v.", err)
		return nil
	}

	log.Printf("[cni] Read network configuration %+v.", nwCfg)

	// Assume default address space if not specified.
	if nwCfg.Ipam.AddrSpace == "" {
		nwCfg.Ipam.AddrSpace = defaultAddressSpaceId
	}

	// Initialize values from network config.
	var poolId string
	var subnet string
	networkId := nwCfg.Name
	endpointId := args.ContainerID

	// Check whether the network already exists.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		// Network does not exist.
		log.Printf("[cni] Creating network.")

		// Allocate an address pool for the network.
		poolId, subnet, err = plugin.am.RequestPool(nwCfg.Ipam.AddrSpace, "", "", nil, false)
		if err != nil {
			log.Printf("[cni] Failed to allocate pool, err:%v.", err)
			return nil
		}

		log.Printf("[cni] Allocated address pool %v with subnet %v.", poolId, subnet)

		// Create the network.
		nwInfo := network.NetworkInfo{
			Id:         networkId,
			Subnets:    []string{subnet},
			BridgeName: nwCfg.Bridge,
		}

		err = plugin.nm.CreateNetwork(&nwInfo)
		if err != nil {
			log.Printf("[cni] Failed to create network, err:%v.", err)
			return nil
		}

		log.Printf("[cni] Created network %v with subnet %v.", networkId, subnet)
	} else {
		// Network already exists.
		log.Printf("[cni] Reusing network and pool.")

		// Query address pool.
		poolId = nwInfo.Subnets[0]
		subnet = nwInfo.Subnets[0]
	}

	// Allocate an address for the endpoint.
	address, err := plugin.am.RequestAddress(nwCfg.Ipam.AddrSpace, poolId, "", nil)
	if err != nil {
		log.Printf("[cni] Failed to request address, err:%v.", err)
		return nil
	}

	ip, ipv4Address, err := net.ParseCIDR(address)
	ipv4Address.IP = ip
	if err != nil {
		log.Printf("[cni] Failed to parse address %v, err:%v.", address, err)
		return nil
	}

	log.Printf("[cni] Allocated address: %v", address)

	// Create the endpoint.
	epInfo := network.EndpointInfo{
		Id:          endpointId,
		IfName:      args.IfName,
		IPv4Address: *ipv4Address,
		NetNsPath:   args.Netns,
	}

	err = plugin.nm.CreateEndpoint(networkId, &epInfo)
	if err != nil {
		log.Printf("[cni] Failed to create endpoint, err:%v.", err)
		return nil
	}

	// Output the result.
	result := &cniTypes.Result{
		IP4: &cniTypes.IPConfig{IP: *ipv4Address},
	}

	result.Print()

	log.Printf("[cni] ADD succeeded with output %+v.", result)

	return nil
}

// Delete handles CNI delete commands.
func (plugin *netPlugin) Delete(args *cniSkel.CmdArgs) error {
	log.Printf("[cni] Processing DEL command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[cni] Failed to parse network configuration, err:%v.", err)
		return nil
	}

	log.Printf("[cni] Read network configuration %+v.", nwCfg)

	// Assume default address space if not specified.
	if nwCfg.Ipam.AddrSpace == "" {
		nwCfg.Ipam.AddrSpace = defaultAddressSpaceId
	}

	// Initialize values from network config.
	networkId := nwCfg.Name
	endpointId := args.ContainerID

	// Query the network.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		log.Printf("[cni] Failed to query network, err:%v.", err)
		return nil
	}

	// Query the endpoint.
	epInfo, err := plugin.nm.GetEndpointInfo(networkId, endpointId)
	if err != nil {
		log.Printf("[cni] Failed to query endpoint, err:%v.", err)
		return nil
	}

	// Delete the endpoint.
	err = plugin.nm.DeleteEndpoint(networkId, endpointId)
	if err != nil {
		log.Printf("[cni] Failed to delete endpoint, err:%v.", err)
		return nil
	}

	// Release the address.
	err = plugin.am.ReleaseAddress(nwCfg.Ipam.AddrSpace, nwInfo.Subnets[0], epInfo.IPv4Address.IP.String())
	if err != nil {
		log.Printf("[cni] Failed to release address, err:%v.", err)
		return nil
	}

	log.Printf("[cni] DEL succeeded.")

	return nil
}
