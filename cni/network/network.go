// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Azure/azure-container-networking/aitelemetry"
	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/api"
	"github.com/Azure/azure-container-networking/cni/util"
	"github.com/Azure/azure-container-networking/cns"
	cnscli "github.com/Azure/azure-container-networking/cns/client"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/iptables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netio"
	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/network/policy"
	"github.com/Azure/azure-container-networking/platform"
	nnscontracts "github.com/Azure/azure-container-networking/proto/nodenetworkservice/3.302.0.744"
	"github.com/Azure/azure-container-networking/store"
	"github.com/Azure/azure-container-networking/telemetry"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/100"
	"github.com/pkg/errors"
)

const (
	dockerNetworkOption = "com.docker.network.generic"
	OpModeTransparent   = "transparent"
	// Supported IP version. Currently support only IPv4
	ipamV6                = "azure-vnet-ipamv6"
	defaultRequestTimeout = 15 * time.Second
)

// CNI Operation Types
const (
	CNI_ADD    = "ADD"
	CNI_DEL    = "DEL"
	CNI_UPDATE = "UPDATE"
)

const (
	// URL to query NMAgent version and determine whether we snat on host
	nmAgentSupportedApisURL = "http://168.63.129.16/machine/plugins/?comp=nmagent&type=GetSupportedApis"
	// Only SNAT support (no DNS support)
	nmAgentSnatSupportAPI = "NetworkManagementSnatSupport"
	// SNAT and DNS are both supported
	nmAgentSnatAndDnsSupportAPI = "NetworkManagementDNSSupport"
)

// temporary consts related func determineSnat() which is to be deleted after
// a baking period with newest NMAgent changes
const (
	jsonFileExtension = ".json"
)

// NetPlugin represents the CNI network plugin.
type NetPlugin struct {
	*cni.Plugin
	nm                 network.NetworkManager
	ipamInvoker        IPAMInvoker
	report             *telemetry.CNIReport
	tb                 *telemetry.TelemetryBuffer
	nnsClient          NnsClient
	multitenancyClient MultitenancyClient
}

type PolicyArgs struct {
	nwInfo    *network.NetworkInfo
	nwCfg     *cni.NetworkConfig
	ipconfigs []*cniTypesCurr.IPConfig
}

// client for node network service
type NnsClient interface {
	// Do network port programming for the pod via node network service.
	// podName - name of the pod as received from containerD
	// nwNamesapce - network namespace name as received from containerD
	AddContainerNetworking(ctx context.Context, podName, nwNamespace string) (*nnscontracts.ConfigureContainerNetworkingResponse, error)

	// Undo or delete network port programming for the pod via node network service.
	// podName - name of the pod as received from containerD
	// nwNamesapce - network namespace name as received from containerD
	DeleteContainerNetworking(ctx context.Context, podName, nwNamespace string) (*nnscontracts.ConfigureContainerNetworkingResponse, error)
}

// snatConfiguration contains a bool that determines whether CNI enables snat on host and snat for dns
type snatConfiguration struct {
	EnableSnatOnHost bool
	EnableSnatForDns bool
}

// NewPlugin creates a new NetPlugin object.
func NewPlugin(name string,
	config *common.PluginConfig,
	client NnsClient,
	multitenancyClient MultitenancyClient,
) (*NetPlugin, error) {
	// Setup base plugin.
	plugin, err := cni.NewPlugin(name, config.Version)
	if err != nil {
		return nil, err
	}

	nl := netlink.NewNetlink()
	// Setup network manager.
	nm, err := network.NewNetworkManager(nl, platform.NewExecClient(), &netio.NetIO{})
	if err != nil {
		return nil, err
	}

	config.NetApi = nm

	return &NetPlugin{
		Plugin:             plugin,
		nm:                 nm,
		nnsClient:          client,
		multitenancyClient: multitenancyClient,
	}, nil
}

func (plugin *NetPlugin) SetCNIReport(report *telemetry.CNIReport, tb *telemetry.TelemetryBuffer) {
	plugin.report = report
	plugin.tb = tb
}

// Starts the plugin.
func (plugin *NetPlugin) Start(config *common.PluginConfig) error {
	// Initialize base plugin.
	err := plugin.Initialize(config)
	if err != nil {
		log.Printf("[cni-net] Failed to initialize base plugin, err:%v.", err)
		return err
	}

	// Log platform information.
	log.Printf("[cni-net] Plugin %v version %v.", plugin.Name, plugin.Version)
	log.Printf("[cni-net] Running on %v", platform.GetOSInfo())
	platform.PrintDependencyPackageDetails()
	common.LogNetworkInterfaces()

	// Initialize network manager. rehyrdration not required on reboot for cni plugin
	err = plugin.nm.Initialize(config, false)
	if err != nil {
		log.Printf("[cni-net] Failed to initialize network manager, err:%v.", err)
		return err
	}

	log.Printf("[cni-net] Plugin started.")

	return nil
}

// This function for sending CNI metrics to telemetry service
func logAndSendEvent(plugin *NetPlugin, msg string) {
	log.Printf(msg)
	sendEvent(plugin, msg)
}

func sendEvent(plugin *NetPlugin, msg string) {
	eventMsg := fmt.Sprintf("[%d] %s", os.Getpid(), msg)
	plugin.report.Version = plugin.Version
	plugin.report.EventMessage = eventMsg
	telemetry.SendCNIEvent(plugin.tb, plugin.report)
}

func (plugin *NetPlugin) GetAllEndpointState(networkid string) (*api.AzureCNIState, error) {
	st := api.AzureCNIState{
		ContainerInterfaces: make(map[string]api.PodNetworkInterfaceInfo),
	}

	eps, err := plugin.nm.GetAllEndpoints(networkid)
	if err == store.ErrStoreEmpty {
		log.Printf("failed to retrieve endpoint state with err %v", err)
	} else if err != nil {
		return nil, err
	}

	for _, ep := range eps {
		id := ep.Id
		info := api.PodNetworkInterfaceInfo{
			PodName:       ep.PODName,
			PodNamespace:  ep.PODNameSpace,
			PodEndpointId: ep.Id,
			ContainerID:   ep.ContainerID,
			IPAddresses:   ep.IPAddresses,
		}

		st.ContainerInterfaces[id] = info
	}

	return &st, nil
}

// Stops the plugin.
func (plugin *NetPlugin) Stop() {
	plugin.nm.Uninitialize()
	plugin.Uninitialize()
	log.Printf("[cni-net] Plugin stopped.")
}

// FindMasterInterface returns the name of the master interface.
func (plugin *NetPlugin) findMasterInterface(nwCfg *cni.NetworkConfig, subnetPrefix *net.IPNet) string {
	// An explicit master configuration wins. Explicitly specifying a master is
	// useful if host has multiple interfaces with addresses in the same subnet.
	if nwCfg.Master != "" {
		return nwCfg.Master
	}

	// Otherwise, pick the first interface with an IP address in the given subnet.
	subnetPrefixString := subnetPrefix.String()
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			_, ipnet, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if subnetPrefixString == ipnet.String() {
				return iface.Name
			}
		}
	}

	// Failed to find a suitable interface.
	return ""
}

// GetEndpointID returns a unique endpoint ID based on the CNI args.
func GetEndpointID(args *cniSkel.CmdArgs) string {
	infraEpId, _ := network.ConstructEndpointID(args.ContainerID, args.Netns, args.IfName)
	return infraEpId
}

// getPodInfo returns POD info by parsing the CNI args.
func (plugin *NetPlugin) getPodInfo(args string) (name, ns string, err error) {
	podCfg, err := cni.ParseCniArgs(args)
	if err != nil {
		log.Printf("Error while parsing CNI Args %v", err)
		return "", "", err
	}

	k8sNamespace := string(podCfg.K8S_POD_NAMESPACE)
	if len(k8sNamespace) == 0 {
		errMsg := "Pod Namespace not specified in CNI Args"
		log.Printf(errMsg)
		return "", "", plugin.Errorf(errMsg)
	}

	k8sPodName := string(podCfg.K8S_POD_NAME)
	if len(k8sPodName) == 0 {
		errMsg := "Pod Name not specified in CNI Args"
		log.Printf(errMsg)
		return "", "", plugin.Errorf(errMsg)
	}

	return k8sPodName, k8sNamespace, nil
}

func SetCustomDimensions(cniMetric *telemetry.AIMetric, nwCfg *cni.NetworkConfig, err error) {
	if cniMetric == nil {
		log.Errorf("[CNI] Unable to set custom dimension. Report is nil")
		return
	}

	if err != nil {
		cniMetric.Metric.CustomDimensions[telemetry.StatusStr] = telemetry.FailedStr
	} else {
		cniMetric.Metric.CustomDimensions[telemetry.StatusStr] = telemetry.SucceededStr
	}

	if nwCfg != nil {
		if nwCfg.MultiTenancy {
			cniMetric.Metric.CustomDimensions[telemetry.CNIModeStr] = telemetry.MultiTenancyStr
		} else {
			cniMetric.Metric.CustomDimensions[telemetry.CNIModeStr] = telemetry.SingleTenancyStr
		}

		cniMetric.Metric.CustomDimensions[telemetry.CNINetworkModeStr] = nwCfg.Mode
	}
}

func (plugin *NetPlugin) setCNIReportDetails(nwCfg *cni.NetworkConfig, opType, msg string) {
	plugin.report.OperationType = opType
	plugin.report.SubContext = fmt.Sprintf("%+v", nwCfg)
	plugin.report.EventMessage = msg
	plugin.report.BridgeDetails.NetworkMode = nwCfg.Mode
	plugin.report.InterfaceDetails.SecondaryCAUsedCount = plugin.nm.GetNumberOfEndpoints("", nwCfg.Name)
}

func addNatIPV6SubnetInfo(nwCfg *cni.NetworkConfig,
	resultV6 *cniTypesCurr.Result,
	nwInfo *network.NetworkInfo,
) {
	if nwCfg.IPV6Mode == network.IPV6Nat {
		ipv6Subnet := resultV6.IPs[0].Address
		ipv6Subnet.IP = ipv6Subnet.IP.Mask(ipv6Subnet.Mask)
		ipv6SubnetInfo := network.SubnetInfo{
			Family:  platform.AfINET6,
			Prefix:  ipv6Subnet,
			Gateway: resultV6.IPs[0].Gateway,
		}
		log.Printf("[net] ipv6 subnet info:%+v", ipv6SubnetInfo)
		nwInfo.Subnets = append(nwInfo.Subnets, ipv6SubnetInfo)
	}
}

//
// CNI implementation
// https://github.com/containernetworking/cni/blob/master/SPEC.md
//

// Add handles CNI add commands.
func (plugin *NetPlugin) Add(args *cniSkel.CmdArgs) error {
	var (
		ipamAddResult    IPAMAddResult
		azIpamResult     *cniTypesCurr.Result
		enableInfraVnet  bool
		enableSnatForDNS bool
		k8sPodName       string
		cniMetric        telemetry.AIMetric
	)

	startTime := time.Now()

	logAndSendEvent(plugin, fmt.Sprintf("[cni-net] Processing ADD command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v StdinData:%s}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, args.StdinData))

	// Parse network configuration from stdin.
	nwCfg, err := cni.ParseNetworkConfig(args.StdinData)
	if err != nil {
		err = plugin.Errorf("Failed to parse network configuration: %v.", err)
		return err
	}

	iptables.DisableIPTableLock = nwCfg.DisableIPTableLock
	plugin.setCNIReportDetails(nwCfg, CNI_ADD, "")

	defer func() {
		operationTimeMs := time.Since(startTime).Milliseconds()
		cniMetric.Metric = aitelemetry.Metric{
			Name:             telemetry.CNIAddTimeMetricStr,
			Value:            float64(operationTimeMs),
			AppVersion:       plugin.Version,
			CustomDimensions: make(map[string]string),
		}
		SetCustomDimensions(&cniMetric, nwCfg, err)
		telemetry.SendCNIMetric(&cniMetric, plugin.tb)

		// Add Interfaces to result.
		if ipamAddResult.ipv4Result == nil {
			ipamAddResult.ipv4Result = &cniTypesCurr.Result{}
		}

		iface := &cniTypesCurr.Interface{
			Name: args.IfName,
		}
		ipamAddResult.ipv4Result.Interfaces = append(ipamAddResult.ipv4Result.Interfaces, iface)

		if ipamAddResult.ipv6Result != nil {
			ipamAddResult.ipv4Result.IPs = append(ipamAddResult.ipv4Result.IPs, ipamAddResult.ipv6Result.IPs...)
		}

		addSnatInterface(nwCfg, ipamAddResult.ipv4Result)
		// Convert result to the requested CNI version.
		res, vererr := ipamAddResult.ipv4Result.GetAsVersion(nwCfg.CNIVersion)
		if vererr != nil {
			log.Printf("GetAsVersion failed with error %v", vererr)
			plugin.Error(vererr)
		}

		if err == nil && res != nil {
			// Output the result to stdout.
			res.Print()
		}

		log.Printf("[cni-net] ADD command completed for pod %v with IPs:%+v err:%v.", k8sPodName, ipamAddResult.ipv4Result.IPs, err)
	}()

	// Parse Pod arguments.
	k8sPodName, k8sNamespace, err := plugin.getPodInfo(args.Args)
	if err != nil {
		return err
	}

	plugin.report.ContainerName = k8sPodName + ":" + k8sNamespace

	k8sContainerID := args.ContainerID
	if len(k8sContainerID) == 0 {
		errMsg := "Container ID not specified in CNI Args"
		log.Printf(errMsg)
		return plugin.Errorf(errMsg)
	}

	k8sIfName := args.IfName
	if len(k8sIfName) == 0 {
		errMsg := "Interfacename not specified in CNI Args"
		log.Printf(errMsg)
		return plugin.Errorf(errMsg)
	}

	platformInit(nwCfg)
	if nwCfg.ExecutionMode == string(util.Baremetal) {
		var res *nnscontracts.ConfigureContainerNetworkingResponse
		log.Printf("Baremetal mode. Calling vnet agent for ADD")
		res, err = plugin.nnsClient.AddContainerNetworking(context.Background(), k8sPodName, args.Netns)

		if err == nil {
			ipamAddResult.ipv4Result = convertNnsToCniResult(res, args.IfName, k8sPodName, "AddContainerNetworking")
		}

		return err
	}

	for _, ns := range nwCfg.PodNamespaceForDualNetwork {
		if k8sNamespace == ns {
			log.Printf("Enable infravnet for this pod %v in namespace %v", k8sPodName, k8sNamespace)
			enableInfraVnet = true
			break
		}
	}

	cnsClient, er := cnscli.New(nwCfg.CNSUrl, defaultRequestTimeout)
	if er != nil {
		return fmt.Errorf("failed to create cns client with error: %w", er)
	}

	if nwCfg.MultiTenancy {
		plugin.report.Context = "AzureCNIMultitenancy"
		plugin.multitenancyClient.Init(cnsClient, AzureNetIOShim{})

		// Temporary if block to determining whether we disable SNAT on host (for multi-tenant scenario only)
		if enableSnatForDNS, nwCfg.EnableSnatOnHost, err = plugin.multitenancyClient.DetermineSnatFeatureOnHost(
			snatConfigFileName, nmAgentSupportedApisURL); err != nil {
			return fmt.Errorf("%w", err)
		}

		ipamAddResult.ncResponse, ipamAddResult.hostSubnetPrefix, er = plugin.multitenancyClient.GetContainerNetworkConfiguration(
			context.TODO(), nwCfg, k8sPodName, k8sNamespace)
		if er != nil {
			er = errors.Wrapf(er, "GetContainerNetworkConfiguration failed for podname %v namespace %v", k8sPodName, k8sNamespace)
			log.Printf("%+v", er)
			return er
		}

		ipamAddResult.ipv4Result = convertToCniResult(ipamAddResult.ncResponse, args.IfName)

		log.Printf("PrimaryInterfaceIdentifier: %v", ipamAddResult.hostSubnetPrefix.IP.String())
	}

	// Initialize values from network config.
	networkID, err := plugin.getNetworkName(args.Netns, &ipamAddResult, nwCfg)
	if err != nil {
		log.Printf("[cni-net] Failed to extract network name from network config. error: %v", err)
		return err
	}

	endpointID := GetEndpointID(args)
	policies := cni.GetPoliciesFromNwCfg(nwCfg.AdditionalArgs)

	options := make(map[string]interface{})
	// Check whether the network already exists.
	nwInfo, nwInfoErr := plugin.nm.GetNetworkInfo(networkID)
	/* Handle consecutive ADD calls for infrastructure containers.
	 * This is a temporary work around for issue #57253 of Kubernetes.
	 * We can delete this if statement once they fix it.
	 * Issue link: https://github.com/kubernetes/kubernetes/issues/57253
	 */
	if nwInfoErr == nil {
		log.Printf("[cni-net] Found network %v with subnet %v.", networkID, nwInfo.Subnets[0].Prefix.String())
		nwInfo.IPAMType = nwCfg.IPAM.Type
		options = nwInfo.Options

		var resultSecondAdd *cniTypesCurr.Result
		resultSecondAdd, err = plugin.handleConsecutiveAdd(args, endpointID, networkID, &nwInfo, nwCfg)
		if err != nil {
			log.Printf("handleConsecutiveAdd failed with error %v", err)
			return err
		}

		if resultSecondAdd != nil {
			ipamAddResult.ipv4Result = resultSecondAdd
			return nil
		}
	}

	// Initialize azureipam/cns ipam
	if plugin.ipamInvoker == nil {
		switch nwCfg.IPAM.Type {
		case network.AzureCNS:
			plugin.ipamInvoker = NewCNSInvoker(k8sPodName, k8sNamespace, cnsClient, util.ExecutionMode(nwCfg.ExecutionMode), util.IpamMode(nwCfg.IPAM.Mode))

		default:
			plugin.ipamInvoker = NewAzureIpamInvoker(plugin, &nwInfo)
		}
	}

	ipamAddConfig := IPAMAddConfig{nwCfg: nwCfg, args: args, options: options}
	// No need to call Add if we already got IPAMAddResult in multitenancy section via GetContainerNetworkConfiguration
	if !nwCfg.MultiTenancy {
		ipamAddResult, err = plugin.ipamInvoker.Add(ipamAddConfig)
		if err != nil {
			return fmt.Errorf("IPAM Invoker Add failed with error: %w", err)
		}
	}

	sendEvent(plugin, fmt.Sprintf("Allocated IPAddress from ipam:%+v v6:%+v", ipamAddResult.ipv4Result, ipamAddResult.ipv6Result))

	defer func() {
		if err != nil {
			plugin.cleanupAllocationOnError(ipamAddResult.ipv4Result, ipamAddResult.ipv6Result, nwCfg, args, options)
		}
	}()

	// Create network
	if nwInfoErr != nil {
		// Network does not exist.
		logAndSendEvent(plugin, fmt.Sprintf("[cni-net] Creating network %v.", networkID))
		// opts map needs to get passed in here
		if nwInfo, err = plugin.createNetworkInternal(networkID, policies, ipamAddConfig, ipamAddResult); err != nil {
			log.Errorf("Create network failed: %w", err)
			return err
		}

		logAndSendEvent(plugin, fmt.Sprintf("[cni-net] Created network %v with subnet %v.", networkID, ipamAddResult.hostSubnetPrefix.String()))
	}

	natInfo := getNATInfo(nwCfg.ExecutionMode, options[network.SNATIPKey], nwCfg.MultiTenancy, enableSnatForDNS)

	createEndpointInternalOpt := createEndpointInternalOpt{
		nwCfg:            nwCfg,
		cnsNetworkConfig: ipamAddResult.ncResponse,
		result:           ipamAddResult.ipv4Result,
		resultV6:         ipamAddResult.ipv6Result,
		azIpamResult:     azIpamResult,
		args:             args,
		nwInfo:           &nwInfo,
		policies:         policies,
		endpointID:       endpointID,
		k8sPodName:       k8sPodName,
		k8sNamespace:     k8sNamespace,
		enableInfraVnet:  enableInfraVnet,
		enableSnatForDNS: enableSnatForDNS,
		natInfo:          natInfo,
	}
	epInfo, err := plugin.createEndpointInternal(&createEndpointInternalOpt)
	if err != nil {
		log.Errorf("Endpoint creation failed:%w", err)
		return err
	}

	sendEvent(plugin, fmt.Sprintf("CNI ADD succeeded : IP:%+v, VlanID: %v, podname %v, namespace %v numendpoints:%d",
		ipamAddResult.ipv4Result.IPs, epInfo.Data[network.VlanIDKey], k8sPodName, k8sNamespace, plugin.nm.GetNumberOfEndpoints("", nwCfg.Name)))

	return nil
}

func (plugin *NetPlugin) cleanupAllocationOnError(
	result, resultV6 *cniTypesCurr.Result,
	nwCfg *cni.NetworkConfig,
	args *cniSkel.CmdArgs,
	options map[string]interface{},
) {
	if result != nil && len(result.IPs) > 0 {
		addresses := []*net.IPNet{}
		for _, IP := range result.IPs {
			addresses = append(addresses, &IP.Address)
		}
		if er := plugin.ipamInvoker.Delete(addresses, nwCfg, args, options); er != nil {
			log.Errorf("Failed to cleanup ip allocation on failure: %v", er)
		}
	}
	if resultV6 != nil && len(resultV6.IPs) > 0 {
		addressesV6 := []*net.IPNet{}
		for _, IP := range resultV6.IPs {
			addressesV6 = append(addressesV6, &IP.Address)
		}
		if er := plugin.ipamInvoker.Delete(addressesV6, nwCfg, args, options); er != nil {
			log.Errorf("Failed to cleanup ipv6 allocation on failure: %v", er)
		}
	}
}

func (plugin *NetPlugin) createNetworkInternal(
	networkID string,
	policies []policy.Policy,
	ipamAddConfig IPAMAddConfig,
	ipamAddResult IPAMAddResult,
) (network.NetworkInfo, error) {
	nwInfo := network.NetworkInfo{}
	ipamAddResult.hostSubnetPrefix.IP = ipamAddResult.hostSubnetPrefix.IP.Mask(ipamAddResult.hostSubnetPrefix.Mask)
	ipamAddConfig.nwCfg.IPAM.Subnet = ipamAddResult.hostSubnetPrefix.String()
	// Find the master interface.
	masterIfName := plugin.findMasterInterface(ipamAddConfig.nwCfg, &ipamAddResult.hostSubnetPrefix)
	if masterIfName == "" {
		err := plugin.Errorf("Failed to find the master interface")
		return nwInfo, err
	}
	log.Printf("[cni-net] Found master interface %v.", masterIfName)

	// Add the master as an external interface.
	err := plugin.nm.AddExternalInterface(masterIfName, ipamAddResult.hostSubnetPrefix.String())
	if err != nil {
		err = plugin.Errorf("Failed to add external interface: %v", err)
		return nwInfo, err
	}

	nwDNSInfo, err := getNetworkDNSSettings(ipamAddConfig.nwCfg, ipamAddResult.ipv4Result)
	if err != nil {
		err = plugin.Errorf("Failed to getDNSSettings: %v", err)
		return nwInfo, err
	}

	log.Printf("[cni-net] nwDNSInfo: %v", nwDNSInfo)

	var podSubnetPrefix *net.IPNet
	_, podSubnetPrefix, err = net.ParseCIDR(ipamAddResult.ipv4Result.IPs[0].Address.String())
	if err != nil {
		return nwInfo, fmt.Errorf("Failed to ParseCIDR for pod subnet prefix: %w", err)
	}

	// Create the network.
	nwInfo = network.NetworkInfo{
		Id:           networkID,
		Mode:         ipamAddConfig.nwCfg.Mode,
		MasterIfName: masterIfName,
		AdapterName:  ipamAddConfig.nwCfg.AdapterName,
		Subnets: []network.SubnetInfo{
			{
				Family:  platform.AfINET,
				Prefix:  *podSubnetPrefix,
				Gateway: ipamAddResult.ipv4Result.IPs[0].Gateway,
			},
		},
		BridgeName:                    ipamAddConfig.nwCfg.Bridge,
		EnableSnatOnHost:              ipamAddConfig.nwCfg.EnableSnatOnHost,
		DNS:                           nwDNSInfo,
		Policies:                      policies,
		NetNs:                         ipamAddConfig.args.Netns,
		Options:                       ipamAddConfig.options,
		DisableHairpinOnHostInterface: ipamAddConfig.nwCfg.DisableHairpinOnHostInterface,
		IPV6Mode:                      ipamAddConfig.nwCfg.IPV6Mode,
		IPAMType:                      ipamAddConfig.nwCfg.IPAM.Type,
		ServiceCidrs:                  ipamAddConfig.nwCfg.ServiceCidrs,
	}

	setNetworkOptions(ipamAddResult.ncResponse, &nwInfo)

	addNatIPV6SubnetInfo(ipamAddConfig.nwCfg, ipamAddResult.ipv6Result, &nwInfo)

	err = plugin.nm.CreateNetwork(&nwInfo)
	if err != nil {
		err = plugin.Errorf("createNetworkInternal: Failed to create network: %v", err)
	}

	return nwInfo, err
}

type createEndpointInternalOpt struct {
	nwCfg            *cni.NetworkConfig
	cnsNetworkConfig *cns.GetNetworkContainerResponse
	result           *cniTypesCurr.Result
	resultV6         *cniTypesCurr.Result
	azIpamResult     *cniTypesCurr.Result
	args             *cniSkel.CmdArgs
	nwInfo           *network.NetworkInfo
	policies         []policy.Policy
	endpointID       string
	k8sPodName       string
	k8sNamespace     string
	enableInfraVnet  bool
	enableSnatForDNS bool
	natInfo          []policy.NATInfo
}

func (plugin *NetPlugin) createEndpointInternal(opt *createEndpointInternalOpt) (network.EndpointInfo, error) {
	epInfo := network.EndpointInfo{}

	epDNSInfo, err := getEndpointDNSSettings(opt.nwCfg, opt.result, opt.k8sNamespace)
	if err != nil {
		err = plugin.Errorf("Failed to getEndpointDNSSettings: %v", err)
		return epInfo, err
	}
	policyArgs := PolicyArgs{
		nwInfo:    opt.nwInfo,
		nwCfg:     opt.nwCfg,
		ipconfigs: opt.result.IPs,
	}
	endpointPolicies, err := getEndpointPolicies(policyArgs)
	if err != nil {
		log.Errorf("Failed to get endpoint policies:%v", err)
		return epInfo, err
	}

	opt.policies = append(opt.policies, endpointPolicies...)

	vethName := fmt.Sprintf("%s.%s", opt.k8sNamespace, opt.k8sPodName)
	if opt.nwCfg.Mode != OpModeTransparent {
		// this mechanism of using only namespace and name is not unique for different incarnations of POD/container.
		// IT will result in unpredictable behavior if API server decides to
		// reorder DELETE and ADD call for new incarnation of same POD.
		vethName = fmt.Sprintf("%s%s%s", opt.nwInfo.Id, opt.args.ContainerID, opt.args.IfName)
	}

	epInfo = network.EndpointInfo{
		Id:                 opt.endpointID,
		ContainerID:        opt.args.ContainerID,
		NetNsPath:          opt.args.Netns,
		IfName:             opt.args.IfName,
		Data:               make(map[string]interface{}),
		DNS:                epDNSInfo,
		Policies:           opt.policies,
		IPsToRouteViaHost:  opt.nwCfg.IPsToRouteViaHost,
		EnableSnatOnHost:   opt.nwCfg.EnableSnatOnHost,
		EnableMultiTenancy: opt.nwCfg.MultiTenancy,
		EnableInfraVnet:    opt.enableInfraVnet,
		EnableSnatForDns:   opt.enableSnatForDNS,
		PODName:            opt.k8sPodName,
		PODNameSpace:       opt.k8sNamespace,
		SkipHotAttachEp:    false, // Hot attach at the time of endpoint creation
		IPV6Mode:           opt.nwCfg.IPV6Mode,
		VnetCidrs:          opt.nwCfg.VnetCidrs,
		ServiceCidrs:       opt.nwCfg.ServiceCidrs,
		NATInfo:            opt.natInfo,
	}

	epPolicies := getPoliciesFromRuntimeCfg(opt.nwCfg)
	epInfo.Policies = append(epInfo.Policies, epPolicies...)

	// Populate addresses.
	for _, ipconfig := range opt.result.IPs {
		epInfo.IPAddresses = append(epInfo.IPAddresses, ipconfig.Address)
	}

	if opt.resultV6 != nil {
		for _, ipconfig := range opt.resultV6.IPs {
			epInfo.IPAddresses = append(epInfo.IPAddresses, ipconfig.Address)
		}
	}

	// Populate routes.
	for _, route := range opt.result.Routes {
		epInfo.Routes = append(epInfo.Routes, network.RouteInfo{Dst: route.Dst, Gw: route.GW})
	}

	if opt.azIpamResult != nil && opt.azIpamResult.IPs != nil {
		epInfo.InfraVnetIP = opt.azIpamResult.IPs[0].Address
	}

	if opt.nwCfg.MultiTenancy {
		plugin.multitenancyClient.SetupRoutingForMultitenancy(opt.nwCfg, opt.cnsNetworkConfig, opt.azIpamResult, &epInfo, opt.result)
	}

	setEndpointOptions(opt.cnsNetworkConfig, &epInfo, vethName)

	cnsclient, err := cnscli.New(opt.nwCfg.CNSUrl, defaultRequestTimeout)
	if err != nil {
		log.Printf("failed to initialized cns client with URL %s: %v", opt.nwCfg.CNSUrl, err.Error())
		return epInfo, plugin.Errorf(err.Error())
	}

	// Create the endpoint.
	logAndSendEvent(plugin, fmt.Sprintf("[cni-net] Creating endpoint %s.", epInfo.PrettyString()))
	err = plugin.nm.CreateEndpoint(cnsclient, opt.nwInfo.Id, &epInfo)
	if err != nil {
		err = plugin.Errorf("Failed to create endpoint: %v", err)
	}

	return epInfo, err
}

// Get handles CNI Get commands.
func (plugin *NetPlugin) Get(args *cniSkel.CmdArgs) error {
	var (
		result    cniTypesCurr.Result
		err       error
		nwCfg     *cni.NetworkConfig
		epInfo    *network.EndpointInfo
		iface     *cniTypesCurr.Interface
		networkID string
	)

	log.Printf("[cni-net] Processing GET command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path)

	defer func() {
		// Add Interfaces to result.
		iface = &cniTypesCurr.Interface{
			Name: args.IfName,
		}
		result.Interfaces = append(result.Interfaces, iface)

		// Convert result to the requested CNI version.
		res, vererr := result.GetAsVersion(nwCfg.CNIVersion)
		if vererr != nil {
			log.Printf("GetAsVersion failed with error %v", vererr)
			plugin.Error(vererr)
		}

		if err == nil && res != nil {
			// Output the result to stdout.
			res.Print()
		}

		log.Printf("[cni-net] GET command completed with result:%+v err:%v.", result, err)
	}()

	// Parse network configuration from stdin.
	if nwCfg, err = cni.ParseNetworkConfig(args.StdinData); err != nil {
		err = plugin.Errorf("Failed to parse network configuration: %v.", err)
		return err
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	iptables.DisableIPTableLock = nwCfg.DisableIPTableLock

	// Initialize values from network config.
	if networkID, err = plugin.getNetworkName(args.Netns, nil, nwCfg); err != nil {
		// TODO: Ideally we should return from here only.
		log.Printf("[cni-net] Failed to extract network name from network config. error: %v", err)
	}

	endpointID := GetEndpointID(args)

	// Query the network.
	if _, err = plugin.nm.GetNetworkInfo(networkID); err != nil {
		plugin.Errorf("Failed to query network: %v", err)
		return err
	}

	// Query the endpoint.
	if epInfo, err = plugin.nm.GetEndpointInfo(networkID, endpointID); err != nil {
		plugin.Errorf("Failed to query endpoint: %v", err)
		return err
	}

	for _, ipAddresses := range epInfo.IPAddresses {
		ipConfig := &cniTypesCurr.IPConfig{
			Interface: &epInfo.IfIndex,
			Address:   ipAddresses,
		}

		if epInfo.Gateways != nil {
			ipConfig.Gateway = epInfo.Gateways[0]
		}

		result.IPs = append(result.IPs, ipConfig)
	}

	for _, route := range epInfo.Routes {
		result.Routes = append(result.Routes, &cniTypes.Route{Dst: route.Dst, GW: route.Gw})
	}

	result.DNS.Nameservers = epInfo.DNS.Servers
	result.DNS.Domain = epInfo.DNS.Suffix

	return nil
}

// Delete handles CNI delete commands.
func (plugin *NetPlugin) Delete(args *cniSkel.CmdArgs) error {
	var (
		err          error
		nwCfg        *cni.NetworkConfig
		k8sPodName   string
		k8sNamespace string
		networkID    string
		nwInfo       network.NetworkInfo
		epInfo       *network.EndpointInfo
		cniMetric    telemetry.AIMetric
	)

	startTime := time.Now()

	logAndSendEvent(plugin, fmt.Sprintf("[cni-net] Processing DEL command with args {ContainerID:%v Netns:%v IfName:%v Args:%v Path:%v, StdinData:%s}.",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, args.StdinData))

	defer func() {
		log.Printf("[cni-net] DEL command completed for pod %v with err:%v.", k8sPodName, err)
	}()

	// Parse network configuration from stdin.
	if nwCfg, err = cni.ParseNetworkConfig(args.StdinData); err != nil {
		err = plugin.Errorf("[cni-net] Failed to parse network configuration: %v", err)
		return err
	}

	// Parse Pod arguments.
	if k8sPodName, k8sNamespace, err = plugin.getPodInfo(args.Args); err != nil {
		log.Printf("[cni-net] Failed to get POD info due to error: %v", err)
	}

	plugin.setCNIReportDetails(nwCfg, CNI_DEL, "")
	plugin.report.ContainerName = k8sPodName + ":" + k8sNamespace

	iptables.DisableIPTableLock = nwCfg.DisableIPTableLock

	sendMetricFunc := func() {
		operationTimeMs := time.Since(startTime).Milliseconds()
		cniMetric.Metric = aitelemetry.Metric{
			Name:             telemetry.CNIDelTimeMetricStr,
			Value:            float64(operationTimeMs),
			AppVersion:       plugin.Version,
			CustomDimensions: make(map[string]string),
		}
		SetCustomDimensions(&cniMetric, nwCfg, err)
		telemetry.SendCNIMetric(&cniMetric, plugin.tb)
	}

	platformInit(nwCfg)

	log.Printf("Execution mode :%s", nwCfg.ExecutionMode)
	if nwCfg.ExecutionMode == string(util.Baremetal) {

		log.Printf("Baremetal mode. Calling vnet agent for delete container")

		// schedule send metric before attempting delete
		defer sendMetricFunc()
		_, err = plugin.nnsClient.DeleteContainerNetworking(context.Background(), k8sPodName, args.Netns)
		if err != nil {
			return fmt.Errorf("nnsClient.DeleteContainerNetworking failed with err %w", err)
		}
	}

	if plugin.ipamInvoker == nil {
		switch nwCfg.IPAM.Type {
		case network.AzureCNS:
			cnsClient, cnsErr := cnscli.New("", defaultRequestTimeout)
			if cnsErr != nil {
				log.Printf("[cni-net] failed to create cns client:%v", cnsErr)
				return errors.Wrap(cnsErr, "failed to create cns client")
			}
			plugin.ipamInvoker = NewCNSInvoker(k8sPodName, k8sNamespace, cnsClient, util.ExecutionMode(nwCfg.ExecutionMode), util.IpamMode(nwCfg.IPAM.Mode))

		default:
			plugin.ipamInvoker = NewAzureIpamInvoker(plugin, &nwInfo)
		}
	}

	// Initialize values from network config.
	networkID, err = plugin.getNetworkName(args.Netns, nil, nwCfg)
	if err != nil {
		log.Printf("[cni-net] Failed to extract network name from network config. error: %v", err)
		// If error is not found error, then we ignore it, to comply with CNI SPEC.
		if !network.IsNetworkNotFoundError(err) {
			err = plugin.Errorf("Failed to extract network name from network config. error: %v", err)
			return err
		}
	}

	// Query the network.
	if nwInfo, err = plugin.nm.GetNetworkInfo(networkID); err != nil {
		if !nwCfg.MultiTenancy {
			log.Printf("[cni-net] Failed to query network:%s: %v", networkID, err)
			// Log the error but return success if the network is not found.
			// if cni hits this, mostly state file would be missing and it can be reboot scenario where
			// container runtime tries to delete and create pods which existed before reboot.
			err = nil
			return err
		}
	}

	endpointID := GetEndpointID(args)
	// Query the endpoint.
	if epInfo, err = plugin.nm.GetEndpointInfo(networkID, endpointID); err != nil {

		if !nwCfg.MultiTenancy {
			// attempt to release address associated with this Endpoint id
			// This is to ensure clean up is done even in failure cases
			log.Printf("[cni-net] Failed to query endpoint %s: %v", endpointID, err)
			logAndSendEvent(plugin, fmt.Sprintf("Release ip by ContainerID (endpoint not found):%v", args.ContainerID))
			if err = plugin.ipamInvoker.Delete(nil, nwCfg, args, nwInfo.Options); err != nil {
				return plugin.RetriableError(fmt.Errorf("failed to release address(no endpoint): %w", err))
			}
		}

		// Log the error but return success if the endpoint being deleted is not found.
		err = nil
		return err
	}

	// schedule send metric before attempting delete
	defer sendMetricFunc()
	logAndSendEvent(plugin, fmt.Sprintf("Deleting endpoint:%v", endpointID))
	// Delete the endpoint.
	if err = plugin.nm.DeleteEndpoint(networkID, endpointID); err != nil {
		// return a retriable error so the container runtime will retry this DEL later
		// the implementation of this function returns nil if the endpoint doens't exist, so
		// we don't have to check that here
		return plugin.RetriableError(fmt.Errorf("failed to delete endpoint: %w", err))
	}

	if !nwCfg.MultiTenancy {
		// Call into IPAM plugin to release the endpoint's addresses.
		addresses := []*net.IPNet{}
		for _, address := range epInfo.IPAddresses {
			addresses = append(addresses, &address)
			logAndSendEvent(plugin, fmt.Sprintf("Release ip:%s", address.IP.String()))
		}
		err = plugin.ipamInvoker.Delete(addresses, nwCfg, args, nwInfo.Options)
		if err != nil {
			return plugin.RetriableError(fmt.Errorf("failed to release addresses: %w", err))
		}
	} else if epInfo.EnableInfraVnet {
		nwCfg.IPAM.Subnet = nwInfo.Subnets[0].Prefix.String()
		nwCfg.IPAM.Address = epInfo.InfraVnetIP.IP.String()
		err = plugin.ipamInvoker.Delete(nil, nwCfg, args, nwInfo.Options)
		if err != nil {
			return plugin.RetriableError(fmt.Errorf("failed to release address: %w", err))
		}
	}

	sendEvent(plugin, fmt.Sprintf("CNI DEL succeeded : Released ip %+v podname %v namespace %v", nwCfg.IPAM.Address, k8sPodName, k8sNamespace))

	return err
}

// Update handles CNI update commands.
// Update is only supported for multitenancy and to update routes.
func (plugin *NetPlugin) Update(args *cniSkel.CmdArgs) error {
	var (
		result              *cniTypesCurr.Result
		err                 error
		nwCfg               *cni.NetworkConfig
		existingEpInfo      *network.EndpointInfo
		podCfg              *cni.K8SPodEnvArgs
		orchestratorContext []byte
		targetNetworkConfig *cns.GetNetworkContainerResponse
		cniMetric           telemetry.AIMetric
	)

	startTime := time.Now()

	log.Printf("[cni-net] Processing UPDATE command with args {Netns:%v Args:%v Path:%v}.",
		args.Netns, args.Args, args.Path)

	// Parse network configuration from stdin.
	if nwCfg, err = cni.ParseNetworkConfig(args.StdinData); err != nil {
		err = plugin.Errorf("Failed to parse network configuration: %v.", err)
		return err
	}

	log.Printf("[cni-net] Read network configuration %+v.", nwCfg)

	iptables.DisableIPTableLock = nwCfg.DisableIPTableLock
	plugin.setCNIReportDetails(nwCfg, CNI_UPDATE, "")

	defer func() {
		operationTimeMs := time.Since(startTime).Milliseconds()
		cniMetric.Metric = aitelemetry.Metric{
			Name:             telemetry.CNIUpdateTimeMetricStr,
			Value:            float64(operationTimeMs),
			AppVersion:       plugin.Version,
			CustomDimensions: make(map[string]string),
		}
		SetCustomDimensions(&cniMetric, nwCfg, err)
		telemetry.SendCNIMetric(&cniMetric, plugin.tb)

		if result == nil {
			result = &cniTypesCurr.Result{}
		}

		// Convert result to the requested CNI version.
		res, vererr := result.GetAsVersion(nwCfg.CNIVersion)
		if vererr != nil {
			log.Printf("GetAsVersion failed with error %v", vererr)
			plugin.Error(vererr)
		}

		if err == nil && res != nil {
			// Output the result to stdout.
			res.Print()
		}

		log.Printf("[cni-net] UPDATE command completed with result:%+v err:%v.", result, err)
	}()

	// Parse Pod arguments.
	if podCfg, err = cni.ParseCniArgs(args.Args); err != nil {
		log.Printf("[cni-net] Error while parsing CNI Args during UPDATE %v", err)
		return err
	}

	k8sNamespace := string(podCfg.K8S_POD_NAMESPACE)
	if len(k8sNamespace) == 0 {
		errMsg := "Required parameter Pod Namespace not specified in CNI Args during UPDATE"
		log.Printf(errMsg)
		return plugin.Errorf(errMsg)
	}

	k8sPodName := string(podCfg.K8S_POD_NAME)
	if len(k8sPodName) == 0 {
		errMsg := "Required parameter Pod Name not specified in CNI Args during UPDATE"
		log.Printf(errMsg)
		return plugin.Errorf(errMsg)
	}

	// Initialize values from network config.
	networkID := nwCfg.Name

	// Query the network.
	if _, err = plugin.nm.GetNetworkInfo(networkID); err != nil {
		errMsg := fmt.Sprintf("Failed to query network during CNI UPDATE: %v", err)
		log.Printf(errMsg)
		return plugin.Errorf(errMsg)
	}

	// Query the existing endpoint since this is an update.
	// Right now, we do not support updating pods that have multiple endpoints.
	existingEpInfo, err = plugin.nm.GetEndpointInfoBasedOnPODDetails(networkID, k8sPodName, k8sNamespace, nwCfg.EnableExactMatchForPodName)
	if err != nil {
		plugin.Errorf("Failed to retrieve target endpoint for CNI UPDATE [name=%v, namespace=%v]: %v", k8sPodName, k8sNamespace, err)
		return err
	}

	log.Printf("Retrieved existing endpoint from state that may get update: %+v", existingEpInfo)

	// now query CNS to get the target routes that should be there in the networknamespace (as a result of update)
	log.Printf("Going to collect target routes for [name=%v, namespace=%v] from CNS.", k8sPodName, k8sNamespace)

	// create struct with info for target POD
	podInfo := cns.KubernetesPodInfo{
		PodName:      k8sPodName,
		PodNamespace: k8sNamespace,
	}
	if orchestratorContext, err = json.Marshal(podInfo); err != nil {
		log.Printf("Marshalling KubernetesPodInfo failed with %v", err)
		return plugin.Errorf(err.Error())
	}

	cnsclient, err := cnscli.New(nwCfg.CNSUrl, defaultRequestTimeout)
	if err != nil {
		log.Printf("failed to initialized cns client with URL %s: %v", nwCfg.CNSUrl, err.Error())
		return plugin.Errorf(err.Error())
	}

	if targetNetworkConfig, err = cnsclient.GetNetworkConfiguration(context.TODO(), orchestratorContext); err != nil {
		log.Printf("GetNetworkConfiguration failed with %v", err)
		return plugin.Errorf(err.Error())
	}

	log.Printf("Network config received from cns for [name=%v, namespace=%v] is as follows -> %+v", k8sPodName, k8sNamespace, targetNetworkConfig)
	targetEpInfo := &network.EndpointInfo{}

	// get the target routes that should replace existingEpInfo.Routes inside the network namespace
	log.Printf("Going to collect target routes for [name=%v, namespace=%v] from targetNetworkConfig.", k8sPodName, k8sNamespace)
	if targetNetworkConfig.Routes != nil && len(targetNetworkConfig.Routes) > 0 {
		for _, route := range targetNetworkConfig.Routes {
			log.Printf("Adding route from routes to targetEpInfo %+v", route)
			_, dstIPNet, _ := net.ParseCIDR(route.IPAddress)
			gwIP := net.ParseIP(route.GatewayIPAddress)
			targetEpInfo.Routes = append(targetEpInfo.Routes, network.RouteInfo{Dst: *dstIPNet, Gw: gwIP, DevName: existingEpInfo.IfName})
			log.Printf("Successfully added route from routes to targetEpInfo %+v", route)
		}
	}

	log.Printf("Going to collect target routes based on Cnetaddressspace for [name=%v, namespace=%v] from targetNetworkConfig.", k8sPodName, k8sNamespace)
	ipconfig := targetNetworkConfig.IPConfiguration
	for _, ipRouteSubnet := range targetNetworkConfig.CnetAddressSpace {
		log.Printf("Adding route from cnetAddressspace to targetEpInfo %+v", ipRouteSubnet)
		dstIPNet := net.IPNet{IP: net.ParseIP(ipRouteSubnet.IPAddress), Mask: net.CIDRMask(int(ipRouteSubnet.PrefixLength), 32)}
		gwIP := net.ParseIP(ipconfig.GatewayIPAddress)
		route := network.RouteInfo{Dst: dstIPNet, Gw: gwIP, DevName: existingEpInfo.IfName}
		targetEpInfo.Routes = append(targetEpInfo.Routes, route)
		log.Printf("Successfully added route from cnetAddressspace to targetEpInfo %+v", ipRouteSubnet)
	}

	log.Printf("Finished collecting new routes in targetEpInfo as follows: %+v", targetEpInfo.Routes)
	log.Printf("Now saving existing infravnetaddress space if needed.")
	for _, ns := range nwCfg.PodNamespaceForDualNetwork {
		if k8sNamespace == ns {
			targetEpInfo.EnableInfraVnet = true
			targetEpInfo.InfraVnetAddressSpace = nwCfg.InfraVnetAddressSpace
			log.Printf("Saving infravnet address space %s for [%s-%s]",
				targetEpInfo.InfraVnetAddressSpace, existingEpInfo.PODNameSpace, existingEpInfo.PODName)
			break
		}
	}

	// Update the endpoint.
	log.Printf("Now updating existing endpoint %v with targetNetworkConfig %+v.", existingEpInfo.Id, targetNetworkConfig)
	if err = plugin.nm.UpdateEndpoint(networkID, existingEpInfo, targetEpInfo); err != nil {
		err = plugin.Errorf("Failed to update endpoint: %v", err)
		return err
	}

	msg := fmt.Sprintf("CNI UPDATE succeeded : Updated %+v podname %v namespace %v", targetNetworkConfig, k8sPodName, k8sNamespace)
	plugin.setCNIReportDetails(nwCfg, CNI_UPDATE, msg)

	return nil
}

func convertNnsToCniResult(
	netRes *nnscontracts.ConfigureContainerNetworkingResponse,
	ifName string,
	podName string,
	operationName string,
) *cniTypesCurr.Result {
	// This function does not add interfaces to CNI result. Reason being CRI (containerD in baremetal case)
	// only looks for default interface named "eth0" and this default interface is added in the defer
	// method of ADD method
	result := &cniTypesCurr.Result{}
	var resultIpconfigs []*cniTypesCurr.IPConfig

	if netRes.Interfaces != nil {
		for i, ni := range netRes.Interfaces {

			intIndex := i
			for _, ip := range ni.Ipaddresses {

				ipWithPrefix := fmt.Sprintf("%s/%s", ip.Ip, ip.PrefixLength)
				_, ipNet, err := net.ParseCIDR(ipWithPrefix)
				if err != nil {
					log.Printf("Error while converting to cni result for %s operation on pod %s. %s",
						operationName, podName, err)
					continue
				}

				gateway := net.ParseIP(ip.DefaultGateway)
				ipConfig := &cniTypesCurr.IPConfig{
					Address:   *ipNet,
					Gateway:   gateway,
					Interface: &intIndex,
				}

				resultIpconfigs = append(resultIpconfigs, ipConfig)
			}
		}
	}

	result.IPs = resultIpconfigs

	return result
}
