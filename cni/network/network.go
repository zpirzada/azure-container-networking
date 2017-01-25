// Copyright Microsoft Corp.
// All rights reserved.

package network

import (
	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/platform"

	cniIpam "github.com/containernetworking/cni/pkg/ipam"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

const (
	// Plugin name.
	name = "azure-vnet"
)

// NetPlugin represents the CNI network plugin.
type netPlugin struct {
	*cni.Plugin
	nm network.NetworkManager
}

// NewPlugin creates a new netPlugin object.
func NewPlugin(config *common.PluginConfig) (*netPlugin, error) {
	// Setup base plugin.
	plugin, err := cni.NewPlugin(name, config.Version)
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

	// Log platform information.
	log.Printf("[cni-net] Plugin %v version %v.", plugin.Name, plugin.Version)
	log.Printf("[cni-net] Running on %v", platform.GetOSInfo())
	common.LogNetworkInterfaces()

	// Initialize network manager.
	err = plugin.nm.Initialize(config)
	if err != nil {
		log.Printf("[cni-net] Failed to initialize network manager, err:%v.", err)
		return err
	}

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
		result, err = cniIpam.ExecAdd(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			log.Printf("[cni-net] Failed to allocate pool, err:%v.", err)
			return nil
		}

		log.Printf("[cni-net] IPAM plugin returned result %v.", result)

		// Derive the subnet from allocated IP address.
		subnet := result.IP4.IP
		subnet.IP = subnet.IP.Mask(subnet.Mask)

		// Add the master as an external interface.
		err = plugin.nm.AddExternalInterface(nwCfg.Master, subnet.String())
		if err != nil {
			log.Printf("[cni-net] Failed to add external interface, err:%v.", err)
			return nil
		}

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

		log.Printf("[cni-net] Created network %v with subnet %v.", networkId, subnet.String())
	} else {
		// Network already exists.
		log.Printf("[cni-net] Found network %v with subnet %v.", networkId, nwInfo.Subnets[0])

		// Call into IPAM plugin to allocate an address for the endpoint.
		nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
		result, err = cniIpam.ExecAdd(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			log.Printf("[cni-net] Failed to allocate address, err:%v.", err)
			return nil
		}

		log.Printf("[cni-net] IPAM plugin returned result %v.", result)
	}

	// Initialize endpoint info.
	epInfo := &network.EndpointInfo{
		Id:        endpointId,
		IfName:    args.IfName,
		NetNsPath: args.Netns,
	}

	// Populate addresses and routes.
	if result.IP4 != nil {
		epInfo.IPAddresses = append(epInfo.IPAddresses, result.IP4.IP)

		for _, route := range result.IP4.Routes {
			epInfo.Routes = append(epInfo.Routes, network.RouteInfo{Dst: route.Dst, Gw: route.GW})
		}
	}

	// Create the endpoint.
	log.Printf("[cni-net] Creating endpoint %+v", epInfo)
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

	// Call into IPAM plugin to release the endpoint's addresses.
	nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
	for _, address := range epInfo.IPAddresses {
		nwCfg.Ipam.Address = address.IP.String()
		err = cniIpam.ExecDel(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			log.Printf("[cni-net] Failed to release address, err:%v.", err)
			return nil
		}
	}

	log.Printf("[cni-net] DEL succeeded.")

	return nil
}
