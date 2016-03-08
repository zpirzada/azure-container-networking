// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"fmt"
	"math/rand"
	"net"

	"golang.org/x/sys/unix"
)

// AddLinkIPAddress adds an ip address to a link
func AddLinkIPAddress(linkName string, ip net.IP, ipNet *net.IPNet) error {
	action := unix.RTM_NEWADDR
	flags := unix.NLM_F_REQUEST | unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	return configureLinkIPAddress(linkName, ip, ipNet, action, flags)
}

// RemoveLinkIPAddress removes ip address configured on a link
func RemoveLinkIPAddress(linkName string, ip net.IP, ipNet *net.IPNet) error {
	action := unix.RTM_DELADDR
	flags := unix.NLM_F_REQUEST | unix.NLM_F_ACK
	return configureLinkIPAddress(linkName, ip, ipNet, action, flags)
}

// CreateVethPair creates a pair of veth devices
func CreateVethPair(veth1 string, veth2 string) error {

	deviceTypeData := "veth" + "\000"
	rtAttrLen := unix.SizeofRtAttr

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
	netlinkMsgHeader.Type = unix.RTM_NEWLINK
	netlinkMsgHeader.Flags = unix.NLM_F_REQUEST | unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	netlinkMsgHeader.Seq = rand.Uint32()
	netlinkMsgHeader.Pid = uint32(unix.Getpid())
	netlinkMsgHeader.Len =
		uint32(unix.SizeofNlMsghdr +
			unix.SizeofIfInfomsg +
			rtaAlign(unix.SizeofRtAttr+len(veth1+"\000")) +
			rtaAlign(unix.SizeofRtAttr) + // IFLA_LINKINFO
			rtaAlign(unix.SizeofRtAttr+len(deviceTypeData)) + // veth
			rtaAlign(unix.SizeofRtAttr) + // data
			rtaAlign(unix.SizeofRtAttr) + //peer2
			unix.SizeofIfInfomsg + // AF_UNSPEC
			rtaAlign(unix.SizeofRtAttr+len(veth2+"\000")))

	// Message of type NewLINK is required to contain an ifinfomsg structure
	// after the header (ancilliary data)
	var ifInfomsg unix.IfInfomsg
	ifInfomsg.Family = unix.AF_UNSPEC // it has to be this Value

	rtAttrName := createRtAttr(unix.IFLA_IFNAME, rtAttrLen+len(veth1+"\000"))

	rtAttrLinkInfoLen := rtaAlign(unix.SizeofRtAttr) + // IFLA_LINKINFO
		rtaAlign(unix.SizeofRtAttr+len(deviceTypeData)) + // veth
		rtaAlign(unix.SizeofRtAttr) + // data
		rtaAlign(unix.SizeofRtAttr) + //peer2
		unix.SizeofIfInfomsg + // AF_UNSPEC
		rtaAlign(unix.SizeofRtAttr+len(veth2+"\000"))
	rtAttrLinkInfo := createRtAttr(unix.IFLA_LINKINFO, rtAttrLinkInfoLen)

	rtAttrLinkType := createRtAttr(1, rtaAlign(unix.SizeofRtAttr+len(deviceTypeData)))

	// we need a peer
	dataLen := rtaAlign(unix.SizeofRtAttr) + // data
		rtaAlign(unix.SizeofRtAttr) + //peer2
		unix.SizeofIfInfomsg + // AF_UNSPEC
		rtaAlign(unix.SizeofRtAttr+len(veth2+"\000"))
	rtAttrLinkTypeData := createRtAttr(2, dataLen)

	peerLength := rtaAlign(unix.SizeofRtAttr) + //peer2
		unix.SizeofIfInfomsg + // AF_UNSPEC
		rtaAlign(unix.SizeofRtAttr+len(veth2+"\000"))
	rtAttrLinkPeer := createRtAttr(1, peerLength)

	var ifInfomsgForPeer unix.IfInfomsg
	ifInfomsgForPeer.Family = unix.AF_UNSPEC // it has to be this Value

	rtAtttrLinkPeerName := createRtAttr(unix.IFLA_IFNAME, rtaAlign(rtAttrLen+len(veth2+"\000")))

	data := SerializeNetLinkMessageHeader(&netlinkMsgHeader)
	data = append(data, SerializeAncilliaryMessageHeader(&ifInfomsg)...)

	rtAttrLinkNameSerialized := SerializeRoutingAttribute(&rtAttrName)
	rtAttrLinkNameLength := rtaAlign(unix.SizeofRtAttr + len(veth1+"\000"))
	rtAttrLinkNameWithPadding := make([]byte, rtAttrLinkNameLength)
	copy(rtAttrLinkNameWithPadding[0:unix.SizeofRtAttr], rtAttrLinkNameSerialized)
	copy(rtAttrLinkNameWithPadding[unix.SizeofRtAttr:rtAttrLinkNameLength], []byte(veth1+"\000"))
	data = append(data, rtAttrLinkNameWithPadding...)

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

	rtAttrLinkTypeDataSerialized := SerializeRoutingAttribute(&rtAttrLinkTypeData)
	rtAttrLinkTypeDataWithPadding := make([]byte, rtaAlign(unix.SizeofRtAttr))
	copy(rtAttrLinkTypeDataWithPadding[0:rtaAlign(unix.SizeofRtAttr)], rtAttrLinkTypeDataSerialized)
	data = append(data, rtAttrLinkTypeDataWithPadding...)

	rtAttrLinkPeerSerialized := SerializeRoutingAttribute(&rtAttrLinkPeer)
	rtAttrLinkPeerWithPadding := make([]byte, rtaAlign(unix.SizeofRtAttr))
	copy(rtAttrLinkPeerWithPadding[0:rtaAlign(unix.SizeofRtAttr)], rtAttrLinkPeerSerialized)
	data = append(data, rtAttrLinkPeerWithPadding...)

	data = append(data, SerializeAncilliaryMessageHeader(&ifInfomsgForPeer)...)

	rtAtttrLinkPeerNameSerialized := SerializeRoutingAttribute(&rtAtttrLinkPeerName)
	rtAtttrLinkPeerNameLength := rtaAlign(unix.SizeofRtAttr + len(veth2+"\000"))
	rtAtttrLinkPeerNameWithPadding := make([]byte, rtAtttrLinkPeerNameLength)
	copy(rtAtttrLinkPeerNameWithPadding[0:unix.SizeofRtAttr], rtAtttrLinkPeerNameSerialized)
	copy(rtAtttrLinkPeerNameWithPadding[unix.SizeofRtAttr:rtAtttrLinkPeerNameLength], []byte(veth2+"\000"))
	data = append(data, rtAtttrLinkPeerNameWithPadding...)

	flags := 0

	// the only way to communicate with netlink sockets is via sendto
	if err := unix.Sendto(netlinkSocketFd, data, flags, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while sending message to kernel " + err.Error())
		return err
	}
	fmt.Println("Veth pair creation command given to kernel successfully")

	if err := SetupKernelAcknowledgement(netlinkSocketFd, netlinkMsgHeader.Seq); err != nil {
		fmt.Printf("Error received from kernel in response to veth pair -> %s\n", err.Error())
		return err
	}

	return nil
}

// DeleteNetworkLink deletes a network link
func DeleteNetworkLink(linkName string) error {

	iface, err := net.InterfaceByName(linkName)
	if err != nil {
		return err
	}

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
	// DELLINK is used to delete devices (ethernet, bridge etc.)
	netlinkMsgHeader.Type = unix.RTM_DELLINK
	// REQUEST: indicates a request messages
	netlinkMsgHeader.Flags = unix.NLM_F_REQUEST | unix.NLM_F_ACK
	// Seq is a 4 byte arbitrary number which is used to correlate request with response
	netlinkMsgHeader.Seq = rand.Uint32()
	netlinkMsgHeader.Pid = uint32(unix.Getpid())
	netlinkMsgHeader.Len = uint32(unix.SizeofNlMsghdr) + uint32(unix.SizeofIfInfomsg)

	// Message of type NewLINK is required to contain an ifinfomsg structure
	// after the header (ancilliary data)
	var ifInfomsg unix.IfInfomsg
	ifInfomsg.Family = unix.AF_UNSPEC // it has to be this Value
	ifInfomsg.Index = int32(iface.Index)

	data := SerializeNetLinkMessageHeader(&netlinkMsgHeader)
	data = append(data, SerializeAncilliaryMessageHeader(&ifInfomsg)...)

	flags := 0
	if err := unix.Sendto(netlinkSocketFd, data, flags, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while sending message to kernel " + err.Error())
	}

	fmt.Println("Delete link command given to kernel successfully")

	if err := SetupKernelAcknowledgement(netlinkSocketFd, netlinkMsgHeader.Seq); err != nil {
		fmt.Printf("Error received from kernel -> %s\n", err.Error())
		return err
	}

	return nil

}

func configureLinkIPAddress(linkName string, ip net.IP, ipNet *net.IPNet, action int, headerFlags int) error {

	iface, err := net.InterfaceByName(linkName)
	if err != nil {
		return err
	}

	fmt.Printf("Got interface %s\n", iface.Name)

	ifaceIndex := uint32(iface.Index)
	ifaceIndexByte := make([]byte, 4)
	getNativeType().PutUint32(ifaceIndexByte, ifaceIndex)

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
		fmt.Println("Got an error while binding netlink socket " + err.Error())
	}

	// Create netlink message header
	var netlinkMsgHeader unix.NlMsghdr
	netlinkMsgHeader.Type = uint16(action)
	netlinkMsgHeader.Flags = uint16(headerFlags)
	netlinkMsgHeader.Seq = rand.Uint32()
	netlinkMsgHeader.Pid = uint32(unix.Getpid())

	var ifAddrmsg unix.IfAddrmsg
	ifAddrmsg.Family = unix.AF_INET // ipv4

	//ifAddrmsg.Flags = unix.NLM_F_REQUEST
	ifAddrmsg.Index = uint32(iface.Index)
	prefixlen, _ := ipNet.Mask.Size()
	fmt.Printf("prefixlen: %d\n", prefixlen)
	ifAddrmsg.Prefixlen = uint8(prefixlen)

	var payLoad []byte
	if ifAddrmsg.Family == unix.AF_INET {
		payLoad = ip.To4()
		fmt.Println(payLoad)
	} else {
		// not supported
		fmt.Println(payLoad)
		payLoad = ip.To16()
	}
	fmt.Printf("Payload: %s \n", ip.To4())
	netlinkMsgHeader.Len =
		uint32(unix.SizeofNlMsghdr) +
			uint32(rtaAlign(unix.SizeofIfAddrmsg)) +
			uint32(rtaAlign(unix.SizeofRtAttr+len(payLoad))) +
			uint32(rtaAlign(unix.SizeofRtAttr+len(payLoad)))

	attrLen := unix.SizeofRtAttr
	var rtAttrLocal unix.RtAttr
	rtAttrLocal.Type = unix.IFA_LOCAL
	rtAttrLocal.Len = uint16(rtaAlign(attrLen + len(payLoad)))

	var rtAttrAddress unix.RtAttr
	rtAttrAddress.Type = unix.IFA_ADDRESS
	rtAttrAddress.Len = uint16(rtaAlign(attrLen + len(payLoad)))

	data := SerializeNetLinkMessageHeader(&netlinkMsgHeader)

	ifAddrmsgSerialized := SerializeAddressMessageHeader(&ifAddrmsg)
	ifAddrmsgLength := rtaAlign(unix.SizeofIfAddrmsg)
	ifAddrmsgWithPadding := make([]byte, ifAddrmsgLength)
	copy(ifAddrmsgWithPadding[0:unix.SizeofIfAddrmsg], ifAddrmsgSerialized)
	data = append(data, ifAddrmsgWithPadding...)

	rtAttrLocalSerialized := SerializeRoutingAttribute(&rtAttrLocal)
	rtAttrLocalLength := rtaAlign(unix.SizeofRtAttr + len(payLoad))
	rtAttrLocalWithPadding := make([]byte, rtAttrLocalLength)
	copy(rtAttrLocalWithPadding[0:unix.SizeofRtAttr], rtAttrLocalSerialized)
	copy(rtAttrLocalWithPadding[unix.SizeofRtAttr:rtAttrLocalLength], []byte(payLoad))
	data = append(data, rtAttrLocalWithPadding...)

	rtAttrAddressSerialized := SerializeRoutingAttribute(&rtAttrAddress)
	rtAttrAddressLength := rtaAlign(unix.SizeofRtAttr + len(payLoad))
	rtAttrAddressWithPadding := make([]byte, rtAttrAddressLength)
	copy(rtAttrAddressWithPadding[0:unix.SizeofRtAttr], rtAttrAddressSerialized)
	copy(rtAttrAddressWithPadding[unix.SizeofRtAttr:rtAttrAddressLength], []byte(payLoad))
	data = append(data, rtAttrAddressWithPadding...)

	flags := 0
	if err := unix.Sendto(netlinkSocketFd, data, flags, &sockAddrNetlink); err != nil {
		fmt.Println("Got an error while sending message to kernel " + err.Error())
	}

	fmt.Println("Configure link ipaddress command given to kernel successfully")

	if err := SetupKernelAcknowledgement(netlinkSocketFd, netlinkMsgHeader.Seq); err != nil {
		fmt.Printf("Error received from kernel -> %s\n", err.Error())
		return err
	}

	return nil

}
