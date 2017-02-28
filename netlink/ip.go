// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package netlink

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// Init initializes netlink module.
func init() {
	initEncoder()
}

// Echo sends a netlink echo request message.
func Echo(text string) error {
	s, err := getSocket()
	if err != nil {
		return err
	}

	req := newRequest(unix.NLMSG_NOOP, unix.NLM_F_ECHO|unix.NLM_F_ACK)
	if req == nil {
		return unix.ENOMEM
	}

	req.addPayload(newAttributeString(0, text))

	return s.sendAndWaitForAck(req)
}

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

// GetIpAddressFamily returns the address family of an IP address.
func GetIpAddressFamily(ip net.IP) int {
	if len(ip) <= net.IPv4len {
		return unix.AF_INET
	}
	if ip.To4() != nil {
		return unix.AF_INET
	}
	return unix.AF_INET6
}

// setIpAddress sends an IP address set request.
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

	family := GetIpAddressFamily(ipAddress)

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

	return s.sendAndWaitForAck(req)
}

// AddIpAddress adds an IP address to a network interface.
func AddIpAddress(ifName string, ipAddress net.IP, ipNet *net.IPNet) error {
	return setIpAddress(ifName, ipAddress, ipNet, true)
}

// DeleteIpAddress deletes an IP address from a network interface.
func DeleteIpAddress(ifName string, ipAddress net.IP, ipNet *net.IPNet) error {
	return setIpAddress(ifName, ipAddress, ipNet, false)
}

// Route represents a netlink route.
type Route struct {
	Family     int
	Dst        *net.IPNet
	Src        net.IP
	Gw         net.IP
	Tos        int
	Table      int
	Protocol   int
	Scope      int
	Type       int
	Flags      int
	Priority   int
	LinkIndex  int
	ILinkIndex int
}

// deserializeRoute decodes a netlink message into a Route struct.
func deserializeRoute(msg *message) (*Route, error) {
	// Parse route message.
	rtmsg := deserializeRtMsg(msg.data)
	attrs := msg.getAttributes(rtmsg)

	// Initialize a new route object.
	route := Route{
		Family:   int(rtmsg.Family),
		Tos:      int(rtmsg.Tos),
		Table:    int(rtmsg.Table),
		Protocol: int(rtmsg.Protocol),
		Scope:    int(rtmsg.Scope),
		Type:     int(rtmsg.Type),
		Flags:    int(rtmsg.Flags),
	}

	// Populate route attributes.
	for _, attr := range attrs {
		switch attr.Type {
		case unix.RTA_DST:
			route.Dst = &net.IPNet{
				IP:   attr.value,
				Mask: net.CIDRMask(int(rtmsg.Dst_len), 8*len(attr.value)),
			}
		case unix.RTA_PREFSRC:
			route.Src = net.IP(attr.value)
		case unix.RTA_GATEWAY:
			route.Gw = net.IP(attr.value)
		case unix.RTA_TABLE:
			route.Table = int(encoder.Uint32(attr.value[0:4]))
		case unix.RTA_PRIORITY:
			route.Priority = int(encoder.Uint32(attr.value[0:4]))
		case unix.RTA_OIF:
			route.LinkIndex = int(encoder.Uint32(attr.value[0:4]))
		case unix.RTA_IIF:
			route.ILinkIndex = int(encoder.Uint32(attr.value[0:4]))
		}
	}

	return &route, nil
}

// GetIpRoute returns a list of IP routes matching the given filter.
func GetIpRoute(filter *Route) ([]*Route, error) {
	s, err := getSocket()
	if err != nil {
		return nil, err
	}

	req := newRequest(unix.RTM_GETROUTE, unix.NLM_F_DUMP)

	ifInfo := newIfInfoMsg()
	ifInfo.Family = uint8(filter.Family)
	req.addPayload(ifInfo)

	msgs, err := s.sendAndWaitForResponse(req)
	if err != nil {
		return nil, err
	}

	var routes []*Route

	// For each route in the list...
	for _, msg := range msgs {
		route, err := deserializeRoute(msg)
		if err != nil {
			return nil, err
		}

		// Ignore cloned routes.
		if route.Flags&unix.RTM_F_CLONED != 0 {
			continue
		}

		// Filter by table.
		if (filter.Table == 0 && route.Table != unix.RT_TABLE_MAIN) ||
			(filter.Table != 0 && filter.Table != route.Table) {
			continue
		}

		// Filter by protocol.
		if filter.Protocol != 0 && filter.Protocol != route.Protocol {
			continue
		}

		// Filter by destination prefix.
		if filter.Dst != nil {
			fMaskOnes, fMaskBits := filter.Dst.Mask.Size()

			if route.Dst == nil {
				if fMaskOnes != 0 {
					continue
				}
			} else {
				rMaskOnes, rMaskBits := route.Dst.Mask.Size()

				if !filter.Dst.IP.Equal(route.Dst.IP) ||
					fMaskOnes != rMaskOnes || fMaskBits != rMaskBits {
					continue
				}
			}
		}

		// Filter by link index.
		if filter.LinkIndex != 0 && filter.LinkIndex != route.LinkIndex {
			continue
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// setIpRoute sends an IP route set request.
func setIpRoute(route *Route, add bool) error {
	var msgType, flags int

	s, err := getSocket()
	if err != nil {
		return err
	}

	if add {
		msgType = unix.RTM_NEWROUTE
		flags = unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	} else {
		msgType = unix.RTM_DELROUTE
		flags = unix.NLM_F_EXCL | unix.NLM_F_ACK
	}

	req := newRequest(msgType, flags)

	msg := newRtMsg(route.Family)
	msg.Tos = uint8(route.Tos)
	msg.Table = uint8(route.Table)

	if route.Protocol != 0 {
		msg.Protocol = uint8(route.Protocol)
	}

	if route.Scope != 0 {
		msg.Scope = uint8(route.Scope)
	}

	if route.Type != 0 {
		msg.Type = uint8(route.Type)
	}

	msg.Flags = uint32(route.Flags)

	req.addPayload(msg)

	if route.Dst != nil {
		prefixLength, _ := route.Dst.Mask.Size()
		msg.Dst_len = uint8(prefixLength)
		req.addPayload(newAttributeIpAddress(unix.RTA_DST, route.Dst.IP))
	}

	if route.Src != nil {
		req.addPayload(newAttributeIpAddress(unix.RTA_PREFSRC, route.Src))
	}

	if route.Gw != nil {
		req.addPayload(newAttributeIpAddress(unix.RTA_GATEWAY, route.Gw))
	}

	if route.Priority != 0 {
		req.addPayload(newAttributeUint32(unix.RTA_PRIORITY, uint32(route.Priority)))
	}

	if route.LinkIndex != 0 {
		req.addPayload(newAttributeUint32(unix.RTA_OIF, uint32(route.LinkIndex)))
	}

	if route.ILinkIndex != 0 {
		req.addPayload(newAttributeUint32(unix.RTA_IIF, uint32(route.ILinkIndex)))
	}

	return s.sendAndWaitForAck(req)
}

// AddIpRoute adds an IP route to the route table.
func AddIpRoute(route *Route) error {
	return setIpRoute(route, true)
}

// DeleteIpRoute deletes an IP route from the route table.
func DeleteIpRoute(route *Route) error {
	return setIpRoute(route, false)
}
