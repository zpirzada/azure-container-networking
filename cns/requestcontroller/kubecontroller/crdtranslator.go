package kubecontroller

import (
	"fmt"
	"net"

	"github.com/Azure/azure-container-networking/cns"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

// CRDStatusToNCRequest translates a crd status to createnetworkcontainer request
func CRDStatusToNCRequest(crdStatus nnc.NodeNetworkConfigStatus) (cns.CreateNetworkContainerRequest, error) {
	var (
		ncRequest         cns.CreateNetworkContainerRequest
		nc                nnc.NetworkContainer
		secondaryIPConfig cns.SecondaryIPConfig
		ipSubnet          cns.IPSubnet
		ipAssignment      nnc.IPAssignment
		err               error
		ip                net.IP
		ipNet             *net.IPNet
		size              int
		numNCsSupported   int
		numNCs            int
	)

	numNCsSupported = 1
	numNCs = len(crdStatus.NetworkContainers)

	// Right now we're only supporing one NC per node, but in the future we will support multiple NCs per node
	if numNCs > numNCsSupported {
		return ncRequest, fmt.Errorf("Number of network containers is not supported. Got %v number of ncs, supports %v", numNCs, numNCsSupported)
	}

	for _, nc = range crdStatus.NetworkContainers {
		ncRequest.SecondaryIPConfigs = make(map[string]cns.SecondaryIPConfig)
		ncRequest.NetworkContainerid = nc.ID
		ncRequest.NetworkContainerType = cns.Docker

		if ip = net.ParseIP(nc.PrimaryIP); ip == nil {
			return ncRequest, fmt.Errorf("Invalid PrimaryIP %s:", nc.PrimaryIP)
		}

		if _, ipNet, err = net.ParseCIDR(nc.SubnetAddressSpace); err != nil {
			return ncRequest, fmt.Errorf("Invalid SubnetAddressSpace %s:, err:%s", nc.SubnetAddressSpace, err)
		}

		size, _ = ipNet.Mask.Size()
		ipSubnet.IPAddress = ip.String()
		ipSubnet.PrefixLength = uint8(size)
		ncRequest.IPConfiguration.IPSubnet = ipSubnet
		ncRequest.IPConfiguration.GatewayIPAddress = nc.DefaultGateway

		for _, ipAssignment = range nc.IPAssignments {
			if ip = net.ParseIP(ipAssignment.IP); ip == nil {
				return ncRequest, fmt.Errorf("Invalid SecondaryIP %s:", ipAssignment.IP)
			}

			secondaryIPConfig = cns.SecondaryIPConfig{
				IPAddress: ip.String(),
			}
			ncRequest.SecondaryIPConfigs[ipAssignment.Name] = secondaryIPConfig
		}
	}

	//Only returning the first network container for now, later we will return a list
	return ncRequest, nil
}

// CNSToCRDSpec translates CNS's map of Ips to be released and requested ip count into a CRD Spec
func CNSToCRDSpec(toBeDeletedSecondaryIPConfigs map[string]cns.SecondaryIPConfig, ipCount int) (nnc.NodeNetworkConfigSpec, error) {
	var (
		spec nnc.NodeNetworkConfigSpec
		uuid string
	)

	if toBeDeletedSecondaryIPConfigs == nil {
		return spec, fmt.Errorf("Error when translating toBeDeletedSecondaryIPConfigs to CRD spec, map is nil")
	}

	spec.RequestedIPCount = int64(ipCount)

	for uuid = range toBeDeletedSecondaryIPConfigs {
		spec.IPsNotInUse = append(spec.IPsNotInUse, uuid)
	}

	return spec, nil
}
