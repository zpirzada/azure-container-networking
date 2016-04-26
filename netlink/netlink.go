// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// Initializes netlink module.
func init() {
	initEncoder()
}

// Sends a netlink echo request message.
func Echo(text string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.NLMSG_NOOP, unix.NLM_F_ECHO | unix.NLM_F_ACK)
	if req == nil {
		return unix.ENOMEM
	} 

	req.addPayload(newAttributeString(0, text))

	return s.sendAndComplete(req)
}

// Adds a new network link of a specified type.
func AddLink(name string, linkType string) error {
	if name == "" || linkType == "" {
		return fmt.Errorf("Invalid link name or type")
	}

	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	req.addPayload(ifInfo)

	attrLinkInfo := newAttribute(unix.IFLA_LINKINFO, nil)
	attrLinkInfo.addNested(newAttributeString(IFLA_INFO_KIND, linkType))
	req.addPayload(attrLinkInfo)

	attrIfName := newAttributeStringZ(unix.IFLA_IFNAME, name)
	req.addPayload(attrIfName)

	return s.sendAndComplete(req)
}

// Deletes a network link.
func DeleteLink(name string) error {
	if name == "" {
		return fmt.Errorf("Invalid link name")
	}

	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_DELLINK, unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	ifInfo.Index = int32(iface.Index)
	req.addPayload(ifInfo)

	return s.sendAndComplete(req)
}

// Sets the operational state of a network interface.
func SetLinkState(name string, up bool) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	ifInfo.Type = unix.RTM_SETLINK
	ifInfo.Index = int32(iface.Index)

	if up {
		ifInfo.Flags = unix.IFF_UP
		ifInfo.Change = unix.IFF_UP
	} else {
		ifInfo.Flags = 0 & ^unix.IFF_UP
		ifInfo.Change = DEFAULT_CHANGE
	}

	req.addPayload(ifInfo)

	return s.sendAndComplete(req)
}

// Sets the master (upper) device of a network interface.
func SetLinkMaster(name string, master string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}

	var masterIndex uint32
	if master != "" {
		masterIface, err := net.InterfaceByName(master)
		if err != nil {
			return err
		}
		masterIndex = uint32(masterIface.Index)
	}

	req := newRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	ifInfo.Type = unix.RTM_SETLINK
	ifInfo.Index = int32(iface.Index)
	ifInfo.Flags = unix.NLM_F_REQUEST
	ifInfo.Change = DEFAULT_CHANGE
	req.addPayload(ifInfo)

	attrMaster := newAttributeUint32(unix.IFLA_MASTER, masterIndex)
	req.addPayload(attrMaster)

	return s.sendAndComplete(req)
}

// Sets the link layer hardware address of a network interface.
func SetLinkAddress(ifName string, hwAddress net.HardwareAddr) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	ifInfo.Type = unix.RTM_SETLINK
	ifInfo.Index = int32(iface.Index)
	ifInfo.Flags = unix.NLM_F_REQUEST
	ifInfo.Change = DEFAULT_CHANGE
	req.addPayload(ifInfo)

	req.addPayload(newAttribute(unix.IFLA_ADDRESS, hwAddress))

	return s.sendAndComplete(req)
}

// Adds a new veth pair.
func AddVethPair(name1 string, name2 string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	req.addPayload(ifInfo)

	attrIfName := newAttributeStringZ(unix.IFLA_IFNAME, name1)
	req.addPayload(attrIfName)

	attrLinkInfo := newAttribute(unix.IFLA_LINKINFO, nil)
	attrLinkInfo.addNested(newAttributeStringZ(IFLA_INFO_KIND, "veth"))

	attrData := newAttribute(IFLA_INFO_DATA, nil)

	attrPeer := newAttribute(VETH_INFO_PEER, nil)
	attrPeer.addNested(newIfInfoMsg())
	attrPeer.addNested(newAttributeStringZ(unix.IFLA_IFNAME, name2))

	attrLinkInfo.addNested(attrData)
	attrData.addNested(attrPeer)

	req.addPayload(attrLinkInfo)

	return s.sendAndComplete(req)
}

// Returns the address family of an IP address.
func getIpAddressFamily(ip net.IP) int {
	if len(ip) <= net.IPv4len {
		return unix.AF_INET
	}
	if ip.To4() != nil {
		return unix.AF_INET
	}
	return unix.AF_INET6
}

// Sends an IP address set request.
func setIpAddress(ifName string, ipAddress net.IP, ipNet *net.IPNet, add bool) error {
	var msgType, flags int
	
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return err
	}

	if add {
		msgType = unix.RTM_NEWADDR
		flags = unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	} else {
		msgType = unix.RTM_DELADDR
		flags = unix.NLM_F_EXCL | unix.NLM_F_ACK
	}

	req := newRequest(msgType, flags)

	family := getIpAddressFamily(ipAddress)

	ifAddr := newIfAddrMsg(family)
	ifAddr.Index = uint32(iface.Index)
	prefixLen, _ := ipNet.Mask.Size()
	ifAddr.Prefixlen = uint8(prefixLen)
	req.addPayload(ifAddr)

	var ipAddrValue []byte
	if family == unix.AF_INET {
		ipAddrValue = ipAddress.To4()
	} else {
		ipAddrValue = ipAddress.To16()
	}

	req.addPayload(newAttribute(unix.IFA_LOCAL, ipAddrValue))
	req.addPayload(newAttribute(unix.IFA_ADDRESS, ipAddrValue))

	return s.sendAndComplete(req)
}

// Adds an IP address to an interface.
func AddIpAddress(ifName string, ipAddress net.IP, ipNet *net.IPNet) error {
	return setIpAddress(ifName, ipAddress, ipNet, true)
}

// Deletes an IP address from an interface.
func DeleteIpAddress(ifName string, ipAddress net.IP, ipNet *net.IPNet) error {
	return setIpAddress(ifName, ipAddress, ipNet, false)
}
