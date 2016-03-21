// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/azure/aqua/netfilter"
	"github.com/azure/aqua/netlink"
)

type interfaceDetails struct {
	Address     net.IPNet
	AddressIPV6 net.IPNet
	MacAddress  net.HardwareAddr
	ID          int
	SrcName     string
	DstPrefix   string
	GatewayIPv4 net.IP
}

type ca struct {
	ip     *net.IP
	ipNet  *net.IPNet
	caName string
	caType string
}

type enslavedInterface struct {
	rnmAllocatedMacAddress net.HardwareAddr
	modifiedMacAddress     net.HardwareAddr
	nicName                string
	bridgeName             string
	provisionedCas         map[string]ca
}

type vethPair struct {
	used                    bool
	peer1                   int
	peer2                   int
	ifaceNameCaWasTakenFrom string
	ip                      *net.IP
	ipNet                   *net.IPNet
}

var mapEnslavedInterfaces map[string]enslavedInterface

var vethPairCollection map[int]vethPair

var vethPrefix = "azveth"

// GetInterfaceToAttach is a function that contains the logic to create/select
// the interface that will be attached to the container
// It is deprecated now
func GetInterfaceToAttach(interfaceNameToAttach string, ipAddressToAttach string) (net.IPNet, net.IPNet, net.HardwareAddr, int, string, string, net.IP, string) {

	printHostInterfaces()

	fmt.Println("Request came for", ipAddressToAttach)
	var selectedInterface net.Interface
	selected := false

	hostInterfaces, err := net.Interfaces()
	if err != nil {
		ermsg := "Azure: Got error while retrieving interfaces"
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, ermsg
	}

	fmt.Println("Azure: Going to select an interface for container")
	for _, hostInterface := range hostInterfaces {

		addresses, ok := hostInterface.Addrs()
		flag := hostInterface.Flags & net.FlagBroadcast
		loopbackFlag := hostInterface.Flags & net.FlagLoopback
		canBeSelected := ok == nil &&
			// interface is configured with some ip address
			len(addresses) > 0 &&
			// interface supports broadcast access capability
			flag == net.FlagBroadcast &&
			// interface is not a loopback interface
			loopbackFlag != net.FlagLoopback // &&
			//strings.Contains(hostInterface.Name, "veth")

		if ipAddressToAttach == "" {
			if canBeSelected && interfaceNameToAttach != "" {
				isThisSameAsRequested := hostInterface.Name == interfaceNameToAttach
				canBeSelected = canBeSelected && isThisSameAsRequested
			}
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		} else {
			if canBeSelected {
				doesThisInterfaceHaveSameIPAsRequested := false
				addrs, _ := hostInterface.Addrs()
				for _, addr := range addrs {
					address := addr.String()
					if strings.Split(address, "/")[0] == ipAddressToAttach {
						doesThisInterfaceHaveSameIPAsRequested = true
						break
					}
				}
				canBeSelected = canBeSelected && doesThisInterfaceHaveSameIPAsRequested
			}
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		}
	}

	if !selected {
		ermsg := "Azure: Interface Not Found Error. " +
			"It is possible that none of the interfaces is configured properly, " +
			"or none of configured interfaces match the selection criteria."
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, ermsg
	}

	fmt.Println("Selected interface: ", selectedInterface.Name)

	addresses, _ := selectedInterface.Addrs()
	address := addresses[0].String()
	ipv4, ipv4Net, _ := net.ParseCIDR(address)
	ipv4Net.IP = ipv4
	bytes := strings.Split(address, ".")
	gateway := bytes[0] + "." + bytes[1] + "." + bytes[2] + ".1"
	gatewayIpv4 := net.ParseIP(gateway)
	srcName := selectedInterface.Name
	macAddress, _ := net.ParseMAC(selectedInterface.HardwareAddr.String())

	fmt.Println("Azure: Interface ip/netmask: ",
		ipv4Net.IP.String(), "/", ipv4Net.Mask.String())
	fmt.Println("Azure: Gateway IP: ", gatewayIpv4.String())

	retval := &interfaceDetails{
		Address:     *ipv4Net,
		MacAddress:  macAddress,
		SrcName:     srcName,
		DstPrefix:   srcName + "eth",
		GatewayIPv4: gatewayIpv4,
	}
	fmt.Println("Azure: Successfully selected interface ", retval)
	return *ipv4Net, net.IPNet{}, macAddress, -1, srcName, srcName + "eth", gatewayIpv4, ""
}

// CleanupAfterContainerDeletion cleans up
func CleanupAfterContainerDeletion(ifaceName string, macAddress net.HardwareAddr) error {
	// ifaceName should be of the form veth followed by an even number
	fmt.Println("Going to cleanup for " + ifaceName + " -- " + macAddress.String())
	seq := strings.SplitAfter(ifaceName, vethPrefix)
	val, err := strconv.ParseUint(seq[1], 10, 32)
	if err != nil {
		return err
	}
	fmt.Println("Got index of veth pair as " + fmt.Sprintf("%d", val))
	targetVeth, ok := vethPairCollection[int(val)]
	if ok {
		fmt.Println("The object contains " + targetVeth.ifaceNameCaWasTakenFrom + " " + string(targetVeth.peer1))
	} else {
		fmt.Println("Received null target veth pair")
		return errors.New("received null veth pair for cleanup")
	}

	netlink.DeleteNetworkLink(ifaceName)
	err = ebtables.RemoveDnatBasedOnIPV4Address(targetVeth.ip.String(), macAddress.String())
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	a := targetVeth.ip
	fmt.Println("going to add " + a.String() + "-- to " + targetVeth.ifaceNameCaWasTakenFrom)
	netlink.AddLinkIPAddress(targetVeth.ifaceNameCaWasTakenFrom, *(targetVeth.ip), targetVeth.ipNet)
	delete(vethPairCollection, int(val))
	return nil
}

// GetTargetInterface returns the interface to be moved to container name space
func GetTargetInterface(interfaceNameToAttach string, ipAddressToAttach string) (
	net.IPNet, net.IPNet, net.HardwareAddr, int, string, string, net.IP, string) {

	targetNic, errmsg := getInterfaceWithMultipleConfiguredCAs(ipAddressToAttach)
	if errmsg != "" {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, errmsg
	}

	err := enslaveInterfaceIfRequired(targetNic, "penguin")
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}

	ip, ipNet, err := getAvailableCaAndRemoveFromHostInterface(targetNic, ipAddressToAttach)
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}

	pair, err := generateVethPair(targetNic.Name, ip, ipNet)
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}

	name1 := fmt.Sprintf("%s%d", vethPrefix, pair.peer1)
	name2 := fmt.Sprintf("%s%d", vethPrefix, pair.peer2)
	fmt.Println("Received veth pair names as ", name1, "-", name2, ". Now creating these.")
	err = netlink.CreateVethPair(name1, name2)
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}
	fmt.Println("Successfully generated veth pair.")

	fmt.Println("Going to add ip address ", *ip, ipNet, " to ", name1)
	err = netlink.AddLinkIPAddress(name1, *ip, ipNet)
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}
	fmt.Println("Successfully added ip address ", *ip, ipNet, " to ", name1)

	fmt.Println("Updating veth pair state")

	fmt.Println("Going to set ", name2, " as up.")
	command := fmt.Sprintf("ip link set %s up", name2)
	err = ExecuteShellCommand(command)
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}
	fmt.Println("successfully ifupped ", name2, ".")

	fmt.Println("Going to add ", name2, " to penguin.")
	err = netlink.AddInterfaceToBridge(name2, "penguin")
	if err != nil {
		fmt.Println(err.Error())
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}

	fmt.Println("Selected interface for container: ", name1)
	selectedInterface, _ := net.InterfaceByName(name1)

	addresses, _ := selectedInterface.Addrs()
	address := addresses[0].String()
	ipv4, ipv4Net, _ := net.ParseCIDR(address)
	ipv4Net.IP = ipv4
	bytes := strings.Split(address, ".")
	// bug: this needs fixing
	gateway := bytes[0] + "." + bytes[1] + "." + bytes[2] + ".1"
	gatewayIpv4 := net.ParseIP(gateway)
	macAddress, _ := net.ParseMAC(selectedInterface.HardwareAddr.String())

	err = ebtables.SetupDnatBasedOnIPV4Address(ipv4.String(), macAddress.String())
	if err != nil {
		return net.IPNet{}, net.IPNet{}, nil, -1, "", "", net.IP{}, err.Error()
	}

	fmt.Println("Azure: Interface ip/netmask: ",
		ipv4Net.IP.String(), "/", ipv4Net.Mask.String())
	fmt.Println("Azure: Gateway IP: ", gatewayIpv4.String())

	return *ipv4Net, net.IPNet{}, macAddress, -1, name1, name1 + "eth", gatewayIpv4, ""
}

// FreeSlaves will free slaves and cleans up stuff
func FreeSlaves() error {
	for ifaceName, ifaceDetails := range mapEnslavedInterfaces {
		fmt.Println("Going to remove " + ifaceName + " from bridge")
		err := netlink.RemoveInterfaceFromBridge(ifaceName)

		fmt.Println("Going to if down the interface so that mac address can be fixed")
		command := fmt.Sprintf("ip link set %s down", ifaceName)
		err = ExecuteShellCommand(command)
		if err != nil {
			return err
		}

		macAddress := ifaceDetails.rnmAllocatedMacAddress
		fmt.Println("Going to revert hardware address of " + ifaceName + " to " + macAddress.String())
		command = fmt.Sprintf("ip link set %s address %s", ifaceName, macAddress)
		err = ExecuteShellCommand(command)
		if err != nil {
			return err
		}

		fmt.Println("Going to revert hardware address")
		command = fmt.Sprintf("ip link set %s up", ifaceName)
		err = ExecuteShellCommand(command)
		if err != nil {
			return err
		}

		// cleanup dnat for arp and snat for outgoing
		fmt.Println("Going to clean up dnat for arp replies")
		ebtables.CleanupDnatForArpReplies(ifaceName)

		fmt.Println("Going to clean up snat for outgoing packets")
		ebtables.CleanupSnatForOutgoingPackets(ifaceName, macAddress.String())
		fmt.Println("Clean up finished...")

		fmt.Println("Going to add ip addresses back to interface " + ifaceName)
		for _, caDetails := range ifaceDetails.provisionedCas {
			netlink.AddLinkIPAddress(ifaceName, *(caDetails.ip), caDetails.ipNet)
		}
	}

	return nil
}

func generateVethPair(ifaceNameCaWasTakenFrom string, ip *net.IP, ipNet *net.IPNet) (vethPair, error) {
	var vethpair vethPair
	fmt.Println("Going to generate veth pair names")
	if vethPairCollection == nil {
		vethPairCollection = make(map[int]vethPair)
		vethpair.used = true
		vethpair.peer1 = 0
		vethpair.peer2 = 1
		vethpair.ifaceNameCaWasTakenFrom = ifaceNameCaWasTakenFrom
		vethpair.ip = ip
		vethpair.ipNet = ipNet
		vethPairCollection[0] = vethpair
		return vethpair, nil
	}
	for i := uint32(0); i < ^uint32(0); i += 2 {
		_, ok := vethPairCollection[int(i)]
		if !ok {
			vethpair.used = true
			vethpair.peer1 = int(i)
			vethpair.peer2 = int(i + 1)
			vethpair.ifaceNameCaWasTakenFrom = ifaceNameCaWasTakenFrom
			vethpair.ip = ip
			vethpair.ipNet = ipNet
			vethPairCollection[int(i)] = vethpair
			return vethpair, nil
		}
	}
	fmt.Println("Unable to generate veth pair")
	return vethpair, errors.New("unable to generate veth pair")
}

func getInterfaceWithMultipleConfiguredCAs(ipAddressToAttach string) (*net.Interface, string) {

	var selectedInterface net.Interface
	selected := false

	hostInterfaces, err := net.Interfaces()
	if err != nil {
		ermsg := "Azure: Got error while retrieving interfaces."
		return nil, ermsg
	}

	fmt.Println("Azure: Going to select an interface that has multiple CAs.")
	for _, hostInterface := range hostInterfaces {

		// We need to get ip addresses that are available to be used from this
		// interface.
		// We can get CAs provisioned on this interface from metadata server.
		// The current allocation either lives in memory, or we go through current
		// containers and see what has already been allocated.
		// For now, we assume that all CAs are configured on the interface.
		// Whenever our driver picks up a CA, it removes it from the interface.
		// When container is destriyed, we add CA back on the interface.
		// So our state lives on the interface and in docker containers.
		addresses, ok := hostInterface.Addrs()
		flag := hostInterface.Flags & net.FlagBroadcast
		loopbackFlag := hostInterface.Flags & net.FlagLoopback
		canBeSelected := ok == nil &&
			// interface is configured with some ip address
			len(addresses) > 1 && // only multi CA interfaces for now
			// interface supports broadcast access capability
			flag == net.FlagBroadcast &&
			// interface is not a loopback interface
			loopbackFlag != net.FlagLoopback &&
			// temporary hack until we have metadata server
			!strings.Contains(hostInterface.Name, "veth") &&
			!strings.Contains(hostInterface.Name, "vth") &&
			!strings.Contains(hostInterface.Name, "eth0") &&
			!strings.Contains(hostInterface.Name, "docker")

		if ipAddressToAttach == "" {
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		} else {
			if canBeSelected {
				doesThisInterfaceHaveSameIPAsRequested := false
				addrs, _ := hostInterface.Addrs()
				for _, addr := range addrs {
					address := addr.String()
					if strings.Split(address, "/")[0] == ipAddressToAttach {
						doesThisInterfaceHaveSameIPAsRequested = true
						break
					}
				}
				canBeSelected = canBeSelected && doesThisInterfaceHaveSameIPAsRequested
			}
			if canBeSelected {
				selectedInterface = hostInterface
				selected = true
			}
		}

		if canBeSelected {
			selectedInterface = hostInterface
			selected = true
		}
	}

	// it may be already enslaved
	// interface loose cas once it is added to bridge
	if !selected && mapEnslavedInterfaces != nil {
		fmt.Println("Was unable to find an interface that is not already slave")
		for ifaceName, ifaceDetails := range mapEnslavedInterfaces {
			fmt.Println("Searching through " + ifaceName)
			for caName, caDetails := range ifaceDetails.provisionedCas {
				fmt.Println("Going to check " + caName + " " + caDetails.ip.String())
				if !isCaAlreadyAssigned(caName, caDetails) {
					ipAddress := caDetails.ip.String()
					fmt.Println("going to compare " + ipAddress + " with " + ipAddressToAttach)
					if ipAddress == ipAddressToAttach || ipAddressToAttach == "" {
						iface, err := net.InterfaceByName(ifaceName)
						if err == nil {
							selectedInterface = *iface
							selected = true
							break
						} else {
							fmt.Println(err.Error())
						}
					}
				} else {
					fmt.Println("Already assigned " + caName + " " + caDetails.ip.String())
				}
			}
		}
	}

	if !selected {
		ermsg := "Azure: Interface Not Found Error. " +
			"It is possible that none of the interfaces is configured properly, " +
			"or none of configured interfaces match the selection criteria."
		fmt.Println(ermsg)
		return nil, ermsg
	}

	fmt.Println("Azure: Successfully selected interface ", selectedInterface)
	return &selectedInterface, ""
}

func isCaAlreadyAssigned(caName string, caDetails ca) bool {
	for _, pairDetails := range vethPairCollection {
		ipAddress := pairDetails.ip.String()
		if ipAddress == caDetails.ip.String() {
			return true
		}
	}
	return false
}

func enslaveInterfaceIfRequired(iface *net.Interface, bridge string) error {
	if mapEnslavedInterfaces == nil {
		mapEnslavedInterfaces = make(map[string]enslavedInterface)
	}
	if _, ok := mapEnslavedInterfaces[iface.Name]; ok {
		// already enslaved
		return nil
	}

	_, err := net.InterfaceByName(bridge)
	if err != nil {
		// bridge does not exist
		if err := netlink.CreateBridge(bridge); err != nil {
			return err
		}
	}

	fmt.Println("Going to iff up the bridge " + bridge)
	command := fmt.Sprintf("ip link set %s up", bridge)
	err = ExecuteShellCommand(command)
	if err != nil {
		return err
	}

	fmt.Println("Going to SetupSnatForOutgoingPackets " + iface.Name + " " + iface.HardwareAddr.String())
	err = ebtables.SetupSnatForOutgoingPackets(iface.Name, iface.HardwareAddr.String())
	if err != nil {
		return err
	}

	fmt.Println("Going to SetupDnatForArpReplies")
	err = ebtables.SetupDnatForArpReplies(iface.Name)
	if err != nil {
		return err
	}

	fmt.Println("Going to iff down " + iface.Name)
	command = fmt.Sprintf("ip link set %s down", iface.Name)
	err = ExecuteShellCommand(command)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	newMac, err := generateHardwareAddress()
	if err != nil {
		fmt.Println("Error happenned while generating a new mac address " + err.Error())
		return err
	}
	fmt.Println("Generated hardware address as " + newMac.String())
	var slave enslavedInterface
	slave.nicName = iface.Name
	slave.bridgeName = bridge
	slave.rnmAllocatedMacAddress = iface.HardwareAddr
	slave.modifiedMacAddress = newMac
	slave.provisionedCas = make(map[string]ca)
	addrs, _ := iface.Addrs()
	for _, addr := range addrs {
		ipl, ipNetl, _ := net.ParseCIDR(addr.String())
		var caDetails ca
		caDetails.ip = &ipl
		caDetails.ipNet = ipNetl
		slave.provisionedCas[ipl.String()] = caDetails
		fmt.Println("Adding provisioned CA " + caDetails.ip.String() + " for " + iface.Name)
	}

	fmt.Println("Going to set " + newMac.String() + " on " + iface.Name)
	command = fmt.Sprintf("ip link set %s address %s", iface.Name, newMac.String())
	err = ExecuteShellCommand(command)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	fmt.Println("Going to iff up the link " + iface.Name)
	command = fmt.Sprintf("ip link set %s up", iface.Name)
	err = ExecuteShellCommand(command)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	fmt.Println("Going to add link " + iface.Name + " to " + bridge)
	err = netlink.AddInterfaceToBridge(iface.Name, bridge)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	mapEnslavedInterfaces[iface.Name] = slave
	return nil
}

// From Go playground: http://play.golang.org/p/1eND0es4Nf
func generateHardwareAddress() (net.HardwareAddr, error) {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		fmt.Println("error:", err)
		return nil, err
	}
	// Set the local bit
	buf[0] &= 2
	macInString := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
	fmt.Println("Generated mac as " + macInString)
	hwAddr, err := net.ParseMAC(macInString)
	if err != nil {
		return nil, err
	}
	return hwAddr, nil
}

func getAvailableCaAndRemoveFromHostInterface(iface *net.Interface, ipAddressToAttach string) (*net.IP, *net.IPNet, error) {

	ensalvedIface, found := mapEnslavedInterfaces[iface.Name]
	if !found {
		erMsg := fmt.Sprintf("The interface %s was never enslaved (not possible). returning error", iface.Name)
		log.Printf(fmt.Sprintf("getAvailableCaAndRemoveFromHostInterface %s", erMsg))
		return nil, nil, errors.New(erMsg)
	}

	if ipAddressToAttach != "" {
		targetCa, found := ensalvedIface.provisionedCas[ipAddressToAttach]
		if !found {
			erMsg := "Azure Critical Core: requested CA not found on interface " + ipAddressToAttach
			fmt.Println(erMsg)
			return nil, nil, errors.New(erMsg)
		}

		if isCaAlreadyAssigned(targetCa.caName, targetCa) {
			erMsg := "Azure Critical Core: requested CA found but already in use with a container " + ipAddressToAttach
			fmt.Println(erMsg)
			return nil, nil, errors.New(erMsg)
		}

		netlink.RemoveLinkIPAddress(iface.Name, *targetCa.ip, targetCa.ipNet)
		return targetCa.ip, targetCa.ipNet, nil
	}

	// if ip address is not requested, then use any ca that is unused
	// we have no way to tell what is primary right now so we cannot avoid removing primary
	for caName, caDetails := range ensalvedIface.provisionedCas {
		if !isCaAlreadyAssigned(caName, caDetails) {
			fmt.Println("Found an unused CA " + caName)
			netlink.RemoveLinkIPAddress(iface.Name, *caDetails.ip, caDetails.ipNet)
			return caDetails.ip, caDetails.ipNet, nil
		}
	}

	erMsg := "Azure Critical Core: no unused ca found on interface " + iface.Name
	fmt.Println(erMsg)
	return nil, nil, errors.New(erMsg)
}
