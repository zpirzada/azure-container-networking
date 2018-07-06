package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/cnsclient"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

func SetupRoutingForMultitenancy(nwCfg *cni.NetworkConfig, cnsNetworkConfig *cns.GetNetworkContainerResponse, epInfo *network.EndpointInfo, result *cniTypesCurr.Result) {
	// Adding default gateway
	if nwCfg.MultiTenancy {
		// if snat enabled, add 169.254.0.1 as default gateway
		if nwCfg.EnableSnatOnHost {
			log.Printf("add default route for multitenancy.snat on host enabled")
			addDefaultRoute(cnsNetworkConfig.LocalIPConfiguration.GatewayIPAddress, epInfo, result)
		} else {
			_, defaultIPNet, _ := net.ParseCIDR("0.0.0.0/0")
			dstIP := net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: defaultIPNet.Mask}
			gwIP := net.ParseIP(cnsNetworkConfig.IPConfiguration.GatewayIPAddress)
			epInfo.Routes = append(epInfo.Routes, network.RouteInfo{Dst: dstIP, Gw: gwIP})
			result.Routes = append(result.Routes, &cniTypes.Route{Dst: dstIP, GW: gwIP})
		}
	}
}

func GetContainerNetworkConfiguration(multiTenancy bool, address string, podName string, podNamespace string) (*cniTypesCurr.Result, *cns.GetNetworkContainerResponse, net.IPNet, error) {
	if multiTenancy {
		podNameWithoutSuffix := getPodNameWithoutSuffix(podName)
		log.Printf("Podname without suffix %v", podNameWithoutSuffix)
		return getContainerNetworkConfiguration(address, podNamespace, podNameWithoutSuffix)
	}

	return nil, nil, net.IPNet{}, nil
}

func getContainerNetworkConfiguration(address string, namespace string, podName string) (*cniTypesCurr.Result, *cns.GetNetworkContainerResponse, net.IPNet, error) {
	cnsClient, err := cnsclient.NewCnsClient(address)
	if err != nil {
		log.Printf("Initializing CNS client error %v", err)
		return nil, nil, net.IPNet{}, err
	}

	podInfo := cns.KubernetesPodInfo{PodName: podName, PodNamespace: namespace}
	orchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		log.Printf("Marshalling KubernetesPodInfo failed with %v", err)
		return nil, nil, net.IPNet{}, err
	}

	networkConfig, err := cnsClient.GetNetworkConfiguration(orchestratorContext)
	if err != nil {
		log.Printf("GetNetworkConfiguration failed with %v", err)
		return nil, nil, net.IPNet{}, err
	}

	log.Printf("Network config received from cns %+v", networkConfig)

	subnetPrefix := common.GetInterfaceSubnetWithSpecificIp(networkConfig.PrimaryInterfaceIdentifier)
	if subnetPrefix == nil {
		errBuf := fmt.Sprintf("Interface not found for this ip %v", networkConfig.PrimaryInterfaceIdentifier)
		log.Printf(errBuf)
		return nil, nil, net.IPNet{}, fmt.Errorf(errBuf)
	}

	return convertToCniResult(networkConfig), networkConfig, *subnetPrefix, nil
}

func convertToCniResult(networkConfig *cns.GetNetworkContainerResponse) *cniTypesCurr.Result {
	result := &cniTypesCurr.Result{}
	resultIpconfig := &cniTypesCurr.IPConfig{}

	ipconfig := networkConfig.IPConfiguration
	ipAddr := net.ParseIP(ipconfig.IPSubnet.IPAddress)

	if ipAddr.To4() != nil {
		resultIpconfig.Version = "4"
		resultIpconfig.Address = net.IPNet{IP: ipAddr, Mask: net.CIDRMask(int(ipconfig.IPSubnet.PrefixLength), 32)}
	} else {
		resultIpconfig.Version = "6"
		resultIpconfig.Address = net.IPNet{IP: ipAddr, Mask: net.CIDRMask(int(ipconfig.IPSubnet.PrefixLength), 128)}
	}

	resultIpconfig.Gateway = net.ParseIP(ipconfig.GatewayIPAddress)
	result.IPs = append(result.IPs, resultIpconfig)
	result.DNS.Nameservers = ipconfig.DNSServers

	if networkConfig.Routes != nil && len(networkConfig.Routes) > 0 {
		for _, route := range networkConfig.Routes {
			_, routeIPnet, _ := net.ParseCIDR(route.IPAddress)
			gwIP := net.ParseIP(route.GatewayIPAddress)
			result.Routes = append(result.Routes, &cniTypes.Route{Dst: *routeIPnet, GW: gwIP})
		}
	}

	for _, ipRouteSubnet := range networkConfig.CnetAddressSpace {
		log.Printf("Adding cnetAddressspace routes %v %v", ipRouteSubnet.IPAddress, ipRouteSubnet.PrefixLength)
		routeIPnet := net.IPNet{IP: net.ParseIP(ipRouteSubnet.IPAddress), Mask: net.CIDRMask(int(ipRouteSubnet.PrefixLength), 32)}
		gwIP := net.ParseIP(ipconfig.GatewayIPAddress)
		result.Routes = append(result.Routes, &cniTypes.Route{Dst: routeIPnet, GW: gwIP})
	}

	return result
}

func getPodNameWithoutSuffix(podName string) string {
	nameSplit := strings.Split(podName, "-")
	log.Printf("namesplit %v", nameSplit)
	if len(nameSplit) > 2 {
		nameSplit = nameSplit[:len(nameSplit)-2]
	} else {
		return podName
	}

	log.Printf("Pod name after splitting based on - : %v", nameSplit)
	return strings.Join(nameSplit, "-")
}
