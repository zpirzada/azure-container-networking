// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"net"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/ipam"
	"github.com/Azure/azure-container-networking/log"

	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
)

const (
	// Plugin name.
	name = "ipam"

	// The default address space ID used when an explicit ID is not specified.
	defaultAddressSpaceId = "LocalDefaultAddressSpace"
)

// IpamPlugin represents a CNI IPAM plugin.
type ipamPlugin struct {
	*common.Plugin
	am ipam.AddressManager
}

// NewPlugin creates a new ipamPlugin object.
func NewPlugin(config *common.PluginConfig) (*ipamPlugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, config.Version)
	if err != nil {
		return nil, err
	}

	// Setup address manager.
	am, err := ipam.NewAddressManager()
	if err != nil {
		return nil, err
	}

	config.IpamApi = am

	return &ipamPlugin{
		Plugin: plugin,
		am:     am,
	}, nil
}

// Starts the plugin.
func (plugin *ipamPlugin) Start(config *common.PluginConfig) error {
	// Initialize base plugin.
	err := plugin.Initialize(config)
	if err != nil {
		log.Printf("[ipam] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Initialize address manager.
	environment := plugin.GetOption(common.OptEnvironmentKey)
	err = plugin.am.Initialize(config, environment)
	if err != nil {
		log.Printf("[ipam] Failed to initialize address manager, err:%v.", err)
		return err
	}

	log.Printf("[ipam] Plugin started.")

	return nil
}

// Stops the plugin.
func (plugin *ipamPlugin) Stop() {
	plugin.am.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[ipam] Plugin stopped.")
}

//
// CNI implementation
// https://github.com/containernetworking/cni/blob/master/SPEC.md
//

// Add handles CNI add commands.
func (plugin *ipamPlugin) Add(args *cniSkel.CmdArgs) error {
	log.Printf("[ipam] Processing ADD command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[ipam] Failed to parse network configuration: %v.", err)
		return nil
	}

	log.Printf("[ipam] Read network configuration %+v.", nwCfg)

	// Assume default address space if not specified.
	if nwCfg.Ipam.AddrSpace == "" {
		nwCfg.Ipam.AddrSpace = defaultAddressSpaceId
	}

	// Check if an address pool is specified.
	if nwCfg.Ipam.Subnet == "" {
		// Allocate an address pool.
		poolId, subnet, err := plugin.am.RequestPool(nwCfg.Ipam.AddrSpace, "", "", nil, false)
		if err != nil {
			log.Printf("[ipam] Failed to allocate pool, err:%v.", err)
			return nil
		}

		nwCfg.Ipam.Subnet = subnet
		log.Printf("[ipam] Allocated address poolId %v with subnet %v.", poolId, subnet)
	}

	// Allocate an address for the endpoint.
	address, err := plugin.am.RequestAddress(nwCfg.Ipam.AddrSpace, nwCfg.Ipam.Subnet, nwCfg.Ipam.Address, nil)
	if err != nil {
		log.Printf("[ipam] Failed to allocate address, err:%v.", err)
		return nil
	}

	log.Printf("[ipam] Allocated address %v.", address)

	// Output the result.
	ip, cidr, err := net.ParseCIDR(address)
	cidr.IP = ip
	if err != nil {
		log.Printf("[ipam] Failed to parse address, err:%v.", err)
		return nil
	}

	result := &cniTypes.Result{
		IP4: &cniTypes.IPConfig{IP: *cidr},
	}

	// Output response.
	if nwCfg.Ipam.Result == "" {
		result.Print()
	} else {
		args.Args = result.String()
	}

	log.Printf("[ipam] ADD succeeded with output %+v.", result)

	return err
}

// Delete handles CNI delete commands.
func (plugin *ipamPlugin) Delete(args *cniSkel.CmdArgs) error {
	log.Printf("[ipam] Processing DEL command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		log.Printf("[ipam] Failed to parse network configuration: %v.", err)
		return nil
	}

	log.Printf("[ipam] Read network configuration %+v.", nwCfg)

	// Process command.
	result, err := plugin.DeleteImpl(args, nwCfg)
	if err != nil {
		log.Printf("[ipam] Failed to process command: %v.", err)
		return nil
	}

	// Output response.
	if result != nil {
		result.Print()
	}

	log.Printf("[ipam] DEL succeeded with output %+v.", result)

	return err
}

// AddImpl handles CNI add commands.
func (plugin *ipamPlugin) AddImpl(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig) (*cniTypes.Result, error) {
	// Assume default address space if not specified.
	if nwCfg.Ipam.AddrSpace == "" {
		nwCfg.Ipam.AddrSpace = defaultAddressSpaceId
	}

	// Check if an address pool is specified.
	if nwCfg.Ipam.Subnet == "" {
		// Allocate an address pool.
		poolId, subnet, err := plugin.am.RequestPool(nwCfg.Ipam.AddrSpace, "", "", nil, false)
		if err != nil {
			log.Printf("[ipam] Failed to allocate pool, err:%v.", err)
			return nil, err
		}

		nwCfg.Ipam.Subnet = subnet
		log.Printf("[ipam] Allocated address poolId %v with subnet %v.", poolId, subnet)
	}

	// Allocate an address for the endpoint.
	address, err := plugin.am.RequestAddress(nwCfg.Ipam.AddrSpace, nwCfg.Ipam.Subnet, nwCfg.Ipam.Address, nil)
	if err != nil {
		log.Printf("[ipam] Failed to allocate address, err:%v.", err)
		return nil, err
	}

	log.Printf("[ipam] Allocated address %v.", address)

	// Output the result.
	ip, cidr, err := net.ParseCIDR(address)
	cidr.IP = ip
	if err != nil {
		log.Printf("[ipam] Failed to parse address, err:%v.", err)
		return nil, err
	}

	result := &cniTypes.Result{
		IP4: &cniTypes.IPConfig{IP: *cidr},
	}

	return result, nil
}

// DeleteImpl handles CNI delete commands.
func (plugin *ipamPlugin) DeleteImpl(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig) (*cniTypes.Result, error) {
	// Assume default address space if not specified.
	if nwCfg.Ipam.AddrSpace == "" {
		nwCfg.Ipam.AddrSpace = defaultAddressSpaceId
	}

	// If an address is specified, release that address. Otherwise, release the pool.
	if nwCfg.Ipam.Address != "" {
		// Release the address.
		err := plugin.am.ReleaseAddress(nwCfg.Ipam.AddrSpace, nwCfg.Ipam.Subnet, nwCfg.Ipam.Address)
		if err != nil {
			log.Printf("[cni] Failed to release address, err:%v.", err)
			return nil, err
		}
	} else {
		// Release the pool.
		err := plugin.am.ReleasePool(nwCfg.Ipam.AddrSpace, nwCfg.Ipam.Subnet)
		if err != nil {
			log.Printf("[cni] Failed to release pool, err:%v.", err)
			return nil, err
		}
	}

	return nil, nil
}
