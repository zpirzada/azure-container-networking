// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"fmt"
	"math/rand"
	"net"

	"golang.org/x/sys/unix"
)

// CreateBridge creates a bridge device
func CreateBridge(bridgeName string) error {

	deviceTypeData := "bridge"
	deviceNameData := bridgeName + "\000"

	netlinkSocketFd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE)
	if err != nil {
		fmt.Printf("Error happenned while creating netlink socket %s \n", err)
		return err
	}
	defer unix.Close(netlinkSocketFd)

	var sockAddrNetlink unix.SockaddrNetlink
	sockAddrNetlink.Family = unix.AF_NETLINK // Address family of socket
	sockAddrNetlink.Pad = 0                  // should always be zero
	sockAddrNetlink.Pid = 0                  // have the kernel process message

	if err := unix.Bind(netlinkSocketFd, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while binding netlikn socket " + err.Error())
	}

	// Create netlink message header
	var netlinkMsgHeader unix.NlMsghdr
	netlinkMsgHeader.Type = unix.RTM_NEWLINK
	netlinkMsgHeader.Flags = unix.NLM_F_REQUEST | unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	// Seq is a 4 byte arbitrary number which is used to correlate request with response
	netlinkMsgHeader.Seq = rand.Uint32()
	netlinkMsgHeader.Pid = uint32(unix.Getpid())
	netlinkMsgHeader.Len = uint32(unix.SizeofNlMsghdr) +
		uint32(unix.SizeofIfInfomsg) +
		uint32(rtaAlign(unix.SizeofRtAttr)+rtaAlign(unix.SizeofRtAttr+len(deviceTypeData))) +
		uint32(rtaAlign(unix.SizeofRtAttr+len(deviceNameData)))

	// Message of type NewLINK is required to contain an ifinfomsg structure
	// after the header (ancilliary data)
	var ifInfomsg unix.IfInfomsg
	ifInfomsg.Family = unix.AF_UNSPEC // it has to be this Value

	attrLen := unix.SizeofRtAttr

	// this will contain the link type
	rtAttrLinkInfo := createRtAttr(unix.IFLA_LINKINFO, rtaAlign(attrLen+attrLen+len(deviceTypeData)))
	rtAttrLinkType := createRtAttr(1, rtaAlign(attrLen+len(deviceTypeData)))
	rtAttrName := createRtAttr(unix.IFLA_IFNAME, rtaAlign(attrLen+len(deviceNameData)))

	data := SerializeNetLinkMessageHeader(&netlinkMsgHeader)
	data = append(data, SerializeAncilliaryMessageHeader(&ifInfomsg)...)

	rtAttrLinkInfoSerialized := SerializeRoutingAttribute(&rtAttrLinkInfo)
	rtAttrLinkInfoWithPadding := make([]byte, rtaAlign(unix.SizeofRtAttr))
	copy(rtAttrLinkInfoWithPadding[0:rtaAlign(unix.SizeofRtAttr)], rtAttrLinkInfoSerialized)

	data = append(data, rtAttrLinkInfoWithPadding...)

	rtAttrLinkTypeSerialized := SerializeRoutingAttribute(&rtAttrLinkType)
	rtAttrLinkTypeLength := rtaAlign(unix.SizeofRtAttr + len(deviceTypeData))
	rtAttrLinkTypeWithPadding := make([]byte, rtAttrLinkTypeLength)
	copy(rtAttrLinkTypeWithPadding[0:unix.SizeofRtAttr], rtAttrLinkTypeSerialized)
	copy(rtAttrLinkTypeWithPadding[unix.SizeofRtAttr:rtAttrLinkTypeLength], []byte(deviceTypeData))
	data = append(data, rtAttrLinkTypeWithPadding...)

	rtAttrLinkNameSerialized := SerializeRoutingAttribute(&rtAttrName)
	rtAttrLinkNameLength := rtaAlign(unix.SizeofRtAttr + len(deviceNameData))
	rtAttrLinkNameWithPadding := make([]byte, rtAttrLinkNameLength)
	copy(rtAttrLinkNameWithPadding[0:unix.SizeofRtAttr], rtAttrLinkNameSerialized)
	copy(rtAttrLinkNameWithPadding[unix.SizeofRtAttr:rtAttrLinkNameLength], []byte(deviceNameData))
	data = append(data, rtAttrLinkNameWithPadding...)

	flags := 0

	// the only way to communicate with netlink sockets is via sendto
	if err := unix.Sendto(netlinkSocketFd, data, flags, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while sending message to kernel " + err.Error())
		return err
	}
	fmt.Println("Bridge creation command given to kernel successfully")

	if err := SetupKernelAcknowledgement(netlinkSocketFd, netlinkMsgHeader.Seq); err != nil {
		fmt.Printf("Error received from kernel -> %s\n", err.Error())
		return err
	}
	fmt.Println("Bridge created successfully")
	return nil

}

// AddInterfaceToBridge adds an interface to bridge
func AddInterfaceToBridge(linkName string, bridgeName string) error {
	bridge, err := net.InterfaceByName(bridgeName)
	if err != nil {
		return err
	}
	fmt.Printf("Going to add %s to %s\n", linkName, bridgeName)
	return addInterfaceToBridgeInternal(linkName, bridge.Index)
}

// RemoveInterfaceFromBridge removes an interface from bridge
func RemoveInterfaceFromBridge(linkName string) error {
	return addInterfaceToBridgeInternal(linkName, 0)
}

func addInterfaceToBridgeInternal(linkName string, bridgeIndex int) error {

	iface, err := net.InterfaceByName(linkName)
	if err != nil {
		return err
	}

	bindex := uint32(bridgeIndex)
	bindexByte := make([]byte, 4)
	getNativeType().PutUint32(bindexByte, bindex)

	netlinkSocketFd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE)
	if err != nil {
		fmt.Printf("Error happenned while creating netlink socket %s \n", err)
		return err
	}

	defer unix.Close(netlinkSocketFd)

	var sockAddrNetlink unix.SockaddrNetlink
	sockAddrNetlink.Family = unix.AF_NETLINK // Address family of socket
	sockAddrNetlink.Pad = 0                  // should always be zero
	sockAddrNetlink.Pid = 0                  // have the kernel process message
	// sockAddrNetlink.Groups // Not yet sure what to do with this

	if err := unix.Bind(netlinkSocketFd, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while binding netlikn socket " + err.Error())
	}

	// Create netlink message header
	var netlinkMsgHeader unix.NlMsghdr
	netlinkMsgHeader.Type = unix.RTM_SETLINK
	netlinkMsgHeader.Flags = unix.NLM_F_REQUEST | unix.NLM_F_ACK
	// Seq is a 4 byte arbitrary number which is used to correlate request with response
	netlinkMsgHeader.Seq = rand.Uint32()
	netlinkMsgHeader.Pid = uint32(unix.Getpid())
	netlinkMsgHeader.Len =
		uint32(unix.SizeofNlMsghdr) +
			uint32(unix.SizeofIfInfomsg) +
			uint32(unix.SizeofRtAttr) +
			uint32(len(bindexByte))

	// Message of type NewLINK is required to contain an ifinfomsg structure
	// after the header (ancilliary data)
	var ifInfomsg unix.IfInfomsg
	ifInfomsg.Family = unix.AF_UNSPEC // it has to be this Value
	ifInfomsg.Type = unix.RTM_SETLINK
	ifInfomsg.Flags = unix.NLM_F_REQUEST
	ifInfomsg.Index = int32(iface.Index)
	ifInfomsg.Change = 0xFFFFFFFF

	attrLen := unix.SizeofRtAttr
	rtAttrLinkInfo := createRtAttr(unix.IFLA_MASTER, rtaAlign(attrLen+len(bindexByte)))

	data := SerializeNetLinkMessageHeader(&netlinkMsgHeader)
	data = append(data, SerializeAncilliaryMessageHeader(&ifInfomsg)...)

	rtAttrLinkInfoSerialized := SerializeRoutingAttribute(&rtAttrLinkInfo)
	rtAttrLinkInfoLength := rtaAlign(unix.SizeofRtAttr + len(bindexByte))
	rtAttrLinkInfoWithPadding := make([]byte, rtAttrLinkInfoLength)
	copy(rtAttrLinkInfoWithPadding[0:unix.SizeofRtAttr], rtAttrLinkInfoSerialized)
	copy(rtAttrLinkInfoWithPadding[unix.SizeofRtAttr:rtAttrLinkInfoLength], []byte(bindexByte))
	data = append(data, rtAttrLinkInfoWithPadding...)

	flags := 0
	if err := unix.Sendto(netlinkSocketFd, data, flags, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while sending message to kernel " + err.Error())
	}

	fmt.Println("Add interface to bridge command given to kernel successfully")

	if err := SetupKernelAcknowledgement(netlinkSocketFd, netlinkMsgHeader.Seq); err != nil {
		fmt.Printf("Error received from kernel -> %s\n", err.Error())
		return err
	}

	return nil

}
