// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package network

import (
	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/platform"

	cniInvoke "github.com/containernetworking/cni/pkg/invoke"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesImpl "github.com/containernetworking/cni/pkg/types/020"
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

// GetEndpointID returns a unique endpoint ID based on the CNI args.
func (plugin *netPlugin) getEndpointID(args *cniSkel.CmdArgs) string {
	return args.ContainerID + "-" + args.IfName
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
		return plugin.Errorf("Failed to parse network configuration: %v.", err)
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	// Initialize values from network config.
	var result cniTypes.Result
	var resultImpl *cniTypesImpl.Result
	networkId := nwCfg.Name
	endpointId := plugin.getEndpointID(args)

	// Check whether the network already exists.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		// Network does not exist.
		log.Printf("[cni-net] Creating network.")

		// Call into IPAM plugin to allocate an address pool for the network.
		result, err = cniInvoke.DelegateAdd(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			return plugin.Errorf("Failed to allocate pool: %v", err)
		}

		resultImpl, err = cniTypesImpl.GetResult(result)

		log.Printf("[cni-net] IPAM plugin returned result %v.", resultImpl)

		// Derive the subnet from allocated IP address.
		subnet := resultImpl.IP4.IP
		subnet.IP = subnet.IP.Mask(subnet.Mask)

		// Add the master as an external interface.
		err = plugin.nm.AddExternalInterface(nwCfg.Master, subnet.String())
		if err != nil {
			return plugin.Errorf("Failed to add external interface: %v", err)
		}

		// Create the network.
		nwInfo := network.NetworkInfo{
			Id:         networkId,
			Mode:       nwCfg.Mode,
			Subnets:    []string{subnet.String()},
			BridgeName: nwCfg.Bridge,
		}

		err = plugin.nm.CreateNetwork(&nwInfo)
		if err != nil {
			return plugin.Errorf("Failed to create network: %v", err)
		}

		log.Printf("[cni-net] Created network %v with subnet %v.", networkId, subnet.String())
	} else {
		// Network already exists.
		log.Printf("[cni-net] Found network %v with subnet %v.", networkId, nwInfo.Subnets[0])

		// Call into IPAM plugin to allocate an address for the endpoint.
		nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
		result, err = cniInvoke.DelegateAdd(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			return plugin.Errorf("Failed to allocate address: %v", err)
		}

		resultImpl, err = cniTypesImpl.GetResult(result)

		log.Printf("[cni-net] IPAM plugin returned result %v.", resultImpl)
	}

	// Initialize endpoint info.
	epInfo := &network.EndpointInfo{
		Id:          endpointId,
		ContainerID: args.ContainerID,
		NetNsPath:   args.Netns,
		IfName:      args.IfName,
	}

	// Populate addresses and routes.
	if resultImpl.IP4 != nil {
		epInfo.IPAddresses = append(epInfo.IPAddresses, resultImpl.IP4.IP)

		for _, route := range resultImpl.IP4.Routes {
			epInfo.Routes = append(epInfo.Routes, network.RouteInfo{Dst: route.Dst, Gw: route.GW})
		}
	}

	// Populate DNS info.
	epInfo.DNSSuffix = resultImpl.DNS.Domain
	epInfo.DNSServers = resultImpl.DNS.Nameservers

	// Create the endpoint.
	log.Printf("[cni-net] Creating endpoint %+v", epInfo)
	err = plugin.nm.CreateEndpoint(networkId, epInfo)
	if err != nil {
		return plugin.Errorf("Failed to create endpoint: %v", err)
	}

	// Convert result to the requested CNI version.
	result, err = resultImpl.GetAsVersion(nwCfg.CniVersion)
	if err != nil {
		return plugin.Error(err)
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
		return plugin.Errorf("Failed to parse network configuration: %v", err)
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	// Initialize values from network config.
	networkId := nwCfg.Name
	endpointId := plugin.getEndpointID(args)

	// Query the network.
	nwInfo, err := plugin.nm.GetNetworkInfo(networkId)
	if err != nil {
		return plugin.Errorf("Failed to query network: %v", err)
	}

	// Query the endpoint.
	epInfo, err := plugin.nm.GetEndpointInfo(networkId, endpointId)
	if err != nil {
		return plugin.Errorf("Failed to query endpoint: %v", err)
	}

	// Delete the endpoint.
	err = plugin.nm.DeleteEndpoint(networkId, endpointId)
	if err != nil {
		return plugin.Errorf("Failed to delete endpoint: %v", err)
	}

	// Call into IPAM plugin to release the endpoint's addresses.
	nwCfg.Ipam.Subnet = nwInfo.Subnets[0]
	for _, address := range epInfo.IPAddresses {
		nwCfg.Ipam.Address = address.IP.String()
		err = cniInvoke.DelegateDel(nwCfg.Ipam.Type, nwCfg.Serialize())
		if err != nil {
			return plugin.Errorf("Failed to release address: %v", err)
		}
	}

	log.Printf("[cni-net] DEL succeeded.")

	return nil
}
