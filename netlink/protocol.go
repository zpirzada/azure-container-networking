// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Netlink protocol constants that are not already defined in unix package.
const (
	IFLA_INFO_KIND = 1
	IFLA_INFO_DATA = 2
	VETH_INFO_PEER = 1
	DEFAULT_CHANGE = 0xFFFFFFFF
)

// Serializable types are used to construct netlink messages.
type serializable interface {
	serialize() []byte
	length() int
}

// Byte encoder
var encoder binary.ByteOrder

// Initializes the byte encoder.
func initEncoder() {
	var x uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		encoder = binary.BigEndian
	} else {
		encoder = binary.LittleEndian
	}
}

//
// Netlink message
//

// Generic netlink message
type message struct {
	unix.NlMsghdr
	payload []serializable
}

// Creates a new netlink message.
func newMessage(msgType int, flags int) *message {
	return &message{
		NlMsghdr: unix.NlMsghdr{
			Len:   uint32(unix.NLMSG_HDRLEN),
			Type:  uint16(msgType),
			Flags: uint16(flags),
			Seq:   0,
			Pid:   uint32(unix.Getpid()),
		},
	}
}

// Creates a new netlink request message.
func newRequest(msgType int, flags int) *message {
	return newMessage(msgType, flags | unix.NLM_F_REQUEST)
}

// Appends protocol specific payload to a netlink message.
func (msg *message) addPayload(payload serializable) {
	if payload != nil {
		msg.payload = append(msg.payload, payload)
	}
}

// Serializes a netlink message.
func (msg *message) serialize() []byte {
	// Serialize the protocol specific payload.
	msg.Len = uint32(unix.NLMSG_HDRLEN)
	payload := make([][]byte, len(msg.payload))
	for i, p := range msg.payload {
		payload[i] = p.serialize()
		msg.Len += uint32(len(payload[i]))
	}

	// Serialize the message header.
	b := make([]byte, msg.Len)
	encoder.PutUint32(b[0:4], msg.Len)
	encoder.PutUint16(b[4:6], msg.Type)
	encoder.PutUint16(b[6:8], msg.Flags)
	encoder.PutUint32(b[8:12], msg.Seq)
	encoder.PutUint32(b[12:16], msg.Pid)

	// Append the payload.
	next := 16
	for _, p := range payload {
		copy(b[next:], p)
		next += len(p)
	}

	return b
}

//
// Netlink message attribute
//

// Generic netlink message attribute
type attribute struct {
	unix.NlAttr
	value    []byte
	children []serializable
}

// Creates a new attribute.
func newAttribute(attrType int, value []byte) *attribute {
	return &attribute{
		NlAttr: unix.NlAttr{
			Type: uint16(attrType),
		},
		value:    value,
		children: []serializable{},
	}
}

// Creates a new attribute with a string value.
func newAttributeString(attrType int, value string) *attribute {
	return newAttribute(attrType, []byte(value))
}

// Creates a new attribute with a null-terminated string value.
func newAttributeStringZ(attrType int, value string) *attribute {
	return newAttribute(attrType, []byte(value + "\000"))
}

// Creates a new attribute with a uint32 value.
func newAttributeUint32(attrType int, value uint32) *attribute {
	buf := make([]byte, 4)
	encoder.PutUint32(buf, value)
	return newAttribute(attrType, buf)
}

// Adds a nested attribute to an attribute.
func (attr *attribute) addNested(nested serializable) {
	attr.children = append(attr.children, nested)
}

// Serializes an attribute.
func (attr *attribute) serialize() []byte {
	length := attr.length()
	buf := make([]byte, length)

	// Encode length.
	if l := uint16(length); l != 0 {
		encoder.PutUint16(buf[0:2], l)
	}

	// Encode type.
	encoder.PutUint16(buf[2:4], attr.Type)

	if attr.value != nil {
		// Encode value.
		copy(buf[4:], attr.value)
	} else {
		// Serialize any nested attributes.
		offset := 4
		for _, child := range attr.children {
			childBuf := child.serialize()
			copy(buf[offset:], childBuf)
			offset += len(childBuf)
		}
	}

	return buf
}

// Returns the aligned length of an attribute.
func (attr *attribute) length() int {
	len := unix.SizeofNlAttr + len(attr.value)

	for _, child := range attr.children {
		len += child.length()
	}

	return (len + unix.NLA_ALIGNTO - 1) & ^(unix.NLA_ALIGNTO - 1)
}

//
// Network interface service module
//

// Interface info message
type ifInfoMsg struct {
	unix.IfInfomsg
}

// Creates a new interface info message.
func newIfInfoMsg() *ifInfoMsg {
	return &ifInfoMsg{
		IfInfomsg: unix.IfInfomsg{
			Family: uint8(unix.AF_UNSPEC),
		},
	}
}

// Serializes an interface info message.
func (ifInfo *ifInfoMsg) serialize() []byte {
	b := make([]byte, ifInfo.length())
	b[0] = ifInfo.Family
	b[1] = 0 // Padding.
	encoder.PutUint16(b[2:4], ifInfo.Type)
	encoder.PutUint32(b[4:8], uint32(ifInfo.Index))
	encoder.PutUint32(b[8:12], ifInfo.Flags)
	encoder.PutUint32(b[12:16], ifInfo.Change)
	return b
}

// Returns the length of an interface info message.
func (ifInfo *ifInfoMsg) length() int {
	return unix.SizeofIfInfomsg
}

//
// IP address service module
//

// Interface address message
type ifAddrMsg struct {
	unix.IfAddrmsg
}

// Creates a new interface address message.
func newIfAddrMsg(family int) *ifAddrMsg {
	return &ifAddrMsg{
		IfAddrmsg: unix.IfAddrmsg{
			Family: uint8(family),
		},
	}
}

// Serializes an interface address message.
func (ifAddr *ifAddrMsg) serialize() []byte {
	b := make([]byte, ifAddr.length())
	b[0] = ifAddr.Family
	b[1] = ifAddr.Prefixlen
	b[2] = ifAddr.Flags
	b[3] = ifAddr.Scope
	encoder.PutUint32(b[4:8], ifAddr.Index)
	return b
}

// Returns the length of an interface address message.
func (ifAddr *ifAddrMsg) length() int {
	return unix.SizeofIfAddrmsg
}
