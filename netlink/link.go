// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package netlink

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// AddLink adds a new network interface of a specified type.
func AddLink(name string, linkType string) error {
	if name == "" || linkType == "" {
		return fmt.Errorf("Invalid link name or type")
	}

	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)

	ifInfo := newIfInfoMsg()
	req.addPayload(ifInfo)

	attrLinkInfo := newAttribute(unix.IFLA_LINKINFO, nil)
	attrLinkInfo.addNested(newAttributeString(IFLA_INFO_KIND, linkType))
	req.addPayload(attrLinkInfo)

	attrIfName := newAttributeStringZ(unix.IFLA_IFNAME, name)
	req.addPayload(attrIfName)

	return s.sendAndWaitForAck(req)
}

// DeleteLink deletes a network interface.
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

	return s.sendAndWaitForAck(req)
}

// SetLinkName sets the name of a network interface.
func SetLinkName(name string, newName string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(name)
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

	attrName := newAttributeString(unix.IFLA_IFNAME, newName)
	req.addPayload(attrName)

	return s.sendAndWaitForAck(req)
}

// SetLinkState sets the operational state of a network interface.
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

	return s.sendAndWaitForAck(req)
}

// SetLinkMaster sets the master (upper) device of a network interface.
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

	return s.sendAndWaitForAck(req)
}

// SetLinkNetNs sets the network namespace of a network interface.
func SetLinkNetNs(name string, fd uintptr) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	iface, err := net.InterfaceByName(name)
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

	attrNetNs := newAttributeUint32(IFLA_NET_NS_FD, uint32(fd))
	req.addPayload(attrNetNs)

	return s.sendAndWaitForAck(req)
}

// SetLinkAddress sets the link layer hardware address of a network interface.
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

	return s.sendAndWaitForAck(req)
}

// AddVethPair adds a new veth pair.
func AddVethPair(name1 string, name2 string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)

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

	return s.sendAndWaitForAck(req)
}
