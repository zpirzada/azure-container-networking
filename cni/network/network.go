// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

const (
	// Plugin name.
	name = "net"
)

// NetPlugin object and its interface
type netPlugin struct {
	*common.Plugin
	nm         network.NetworkManager
	ipamPlugin cni.CniPlugin
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
		log.Printf("[cni-net] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Initialize network manager.
	err = plugin.nm.Initialize(config)
	if err != nil {
		log.Printf("[cni-net] Failed to initialize network manager, err:%v.", err)
		return err
	}

	plugin.ipamPlugin = config.IpamApi.(cni.CniPlugin)

	log.Printf("[cni-net] Plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *netPlugin) Stop() {
	plugin.nm.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[cni-net] Plugin stopped.")
}

//
// CNI implementation
// https://github.com/containernetworking/cni/blob/master/SPEC.md
//

// Add handles CNI add commands.
func (plugin *netPlugin) Add(args *cniSkel.CmdArgs) error {
	log.Printf("[cni-net] Processing ADD command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[cni-net] Failed to parse network configuration, err:%v.", err)
		return nil
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	// Initialize values from network config.
	var result *cniTypes.Result
	networkId := nwCfg.Name
	endpointId := args.ContainerID

	// Check whether the network already exists.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		// Network does not exist.
		log.Printf("[cni-net] Creating network.")

		// Call into IPAM plugin to allocate an address pool for the network.
		result, err = cni.CallPlugin(plugin.ipamPlugin, cni.CmdAdd, args, nwCfg)
		if err != nil {
			log.Printf("[cni-net] Failed to allocate pool, err:%v.", err)
			return nil
		}

		// Derive the subnet from allocated IP address.
		subnet := result.IP4.IP
		subnet.IP = subnet.IP.Mask(subnet.Mask)

		log.Printf("[cni-net] IPAM plugin returned subnet %v and address %v.", subnet, result.IP4.IP.String())

		// Create the network.
		nwInfo := network.NetworkInfo{
			Id:         networkId,
			Subnets:    []string{subnet.String()},
			BridgeName: nwCfg.Bridge,
		}

		err = plugin.nm.CreateNetwork(&nwInfo)
		if err != nil {
			log.Printf("[cni-net] Failed to create network, err:%v.", err)
			return nil
		}

		log.Printf("[cni-net] Created network %v with subnet %v.", networkId, subnet)
	} else {
		// Network already exists.
		log.Printf("[cni-net] Found network %v with subnet %v.", networkId, nwInfo.Subnets[0])

		// Call into IPAM plugin to allocate an address for the endpoint.
		nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
		result, err = cni.CallPlugin(plugin.ipamPlugin, cni.CmdAdd, args, nwCfg)
		if err != nil {
			log.Printf("[cni-net] Failed to allocate address, err:%v.", err)
			return nil
		}

		log.Printf("[cni-net] IPAM plugin returned address %v.", result.IP4.IP.String())
	}

	// Create the endpoint.
	epInfo := &network.EndpointInfo{
		Id:          endpointId,
		IfName:      args.IfName,
		IPv4Address: result.IP4.IP,
		NetNsPath:   args.Netns,
	}

	err = plugin.nm.CreateEndpoint(networkId, epInfo)
	if err != nil {
		log.Printf("[cni-net] Failed to create endpoint, err:%v.", err)
		return nil
	}

	// Output the result to stdout.
	result.Print()

	log.Printf("[cni-net] ADD succeeded with output %+v.", result)

	return nil
}

// Delete handles CNI delete commands.
func (plugin *netPlugin) Delete(args *cniSkel.CmdArgs) error {
	log.Printf("[cni-net] Processing DEL command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[cni-net] Failed to parse network configuration, err:%v.", err)
		return nil
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	// Initialize values from network config.
	networkId := nwCfg.Name
	endpointId := args.ContainerID

	// Query the network.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		log.Printf("[cni-net] Failed to query network, err:%v.", err)
		return nil
	}

	// Query the endpoint.
	epInfo, err := plugin.nm.GetEndpointInfo(networkId, endpointId)
	if err != nil {
		log.Printf("[cni-net] Failed to query endpoint, err:%v.", err)
		return nil
	}

	// Delete the endpoint.
	err = plugin.nm.DeleteEndpoint(networkId, endpointId)
	if err != nil {
		log.Printf("[cni-net] Failed to delete endpoint, err:%v.", err)
		return nil
	}

	// Call into IPAM plugin to release the endpoint's address.
	nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
	nwCfg.Ipam.Address = epInfo.IPv4Address.IP.String()
	_, err = cni.CallPlugin(plugin.ipamPlugin, cni.CmdDel, args, nwCfg)
	if err != nil {
		log.Printf("[cni-net] Failed to release address, err:%v.", err)
		return nil
	}

	log.Printf("[cni-net] DEL succeeded.")

	return nil
}
