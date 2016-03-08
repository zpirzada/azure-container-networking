// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func rtaAlign(length int) int {
	return (length + unix.RTA_ALIGNTO - 1) & ^(unix.RTA_ALIGNTO - 1)
}

func createRtAttr(attrType int, lengthInBytes int) unix.RtAttr {
	var rtAttr unix.RtAttr
	rtAttr.Type = uint16(attrType)
	rtAttr.Len = uint16(lengthInBytes)
	return rtAttr
}

// SerializeNetLinkMessageHeader serializes the netlink header
func SerializeNetLinkMessageHeader(netlinkMessageHeader *unix.NlMsghdr) []byte {

	headerInByteArray := make([]byte, unix.SizeofNlMsghdr)
	hdr := (*(*[unix.SizeofNlMsghdr]byte)(unsafe.Pointer(netlinkMessageHeader)))[:]
	next := unix.SizeofNlMsghdr
	copy(headerInByteArray[0:next], hdr)

	return headerInByteArray
}

// SerializeAncilliaryMessageHeader serializes ancilliary message header
func SerializeAncilliaryMessageHeader(ifInfomsgHdr *unix.IfInfomsg) []byte {

	headerInByteArray := make([]byte, unix.SizeofIfInfomsg)
	hdr := (*(*[unix.SizeofIfInfomsg]byte)(unsafe.Pointer(ifInfomsgHdr)))[:]
	next := unix.SizeofIfInfomsg
	copy(headerInByteArray[0:next], hdr)

	return headerInByteArray
}

// SerializeAddressMessageHeader serializes address message header
func SerializeAddressMessageHeader(ifAddrmsgHdr *unix.IfAddrmsg) []byte {

	headerInByteArray := make([]byte, unix.SizeofIfAddrmsg)
	hdr := (*(*[unix.SizeofIfAddrmsg]byte)(unsafe.Pointer(ifAddrmsgHdr)))[:]
	next := unix.SizeofIfAddrmsg
	copy(headerInByteArray[0:next], hdr)

	return headerInByteArray
}

// SerializeRoutingAttribute serializes routing attribute
func SerializeRoutingAttribute(rtAttr *unix.RtAttr) []byte {

	attrInByteArray := make([]byte, unix.SizeofRtAttr)
	attr := (*(*[unix.SizeofRtAttr]byte)(unsafe.Pointer(rtAttr)))[:]
	next := unix.SizeofRtAttr
	copy(attrInByteArray[0:next], attr)

	return attrInByteArray
}

// RecvfromKernel sets up socket ot receive response from kernel
func RecvfromKernel(netlinkSocketFd int) ([]syscall.NetlinkMessage, error) {
	rb := make([]byte, unix.Getpagesize())
	nr, _, err := unix.Recvfrom(netlinkSocketFd, rb, 0)
	if err != nil {
		return nil, err
	}
	if nr < unix.NLMSG_HDRLEN {
		return nil, errors.New("Short response from netlink")
	}
	fmt.Printf("Received %d bytes in response from kernel. ", nr)
	fmt.Printf("Header len: %d bytes\n", unix.NLMSG_HDRLEN)
	rb = rb[:nr]

	return syscall.ParseNetlinkMessage(rb)
}

// SetupKernelAcknowledgement sets up ack from kernel
func SetupKernelAcknowledgement(netlinkSocketFd int, seqNo uint32) error {
	var lsa unix.Sockaddr
	var err error
	lsa, err = unix.Getsockname(netlinkSocketFd)
	if err != nil {
		return err
	}

	pid := uint32(0)
	switch v := lsa.(type) {
	case *unix.SockaddrNetlink:
		pid = uint32(v.Pid)
	}
	if pid == 0 {
		return errors.New("Wrong socket type")
	}

outer:
	for {
		msgs, err := RecvfromKernel(netlinkSocketFd)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if err := validate(m, seqNo, pid); err != nil {
				if err == io.EOF {
					break outer
				}
				return err
			}
		}
	}

	return nil
}

func getNativeType() binary.ByteOrder {
	var a uint32 = 0x01020304
	if *(*byte)(unsafe.Pointer(&a)) == 0x01 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func validate(m syscall.NetlinkMessage, seq, pid uint32) error {
	if m.Header.Seq != seq {
		return fmt.Errorf("invalid seq no: %d, expected: %d", m.Header.Seq, seq)
	}

	fmt.Printf("Received sequence no: %d Expected: %d\n", m.Header.Seq, seq)

	if m.Header.Pid != pid {
		return fmt.Errorf("wrong pid: %d, expected: %d", m.Header.Pid, pid)
	}
	fmt.Printf("Received Pid: %d Expected: %d\n", m.Header.Pid, pid)

	if m.Header.Type == unix.NLMSG_DONE {
		fmt.Printf("Received unix.NLMSG_DONE\n")
		return io.EOF
	}

	if m.Header.Type == unix.NLMSG_ERROR {
		fmt.Printf("Received unix.NLMSG_ERROR\n")
		e := int32(getNativeType().Uint32(m.Data[0:4]))
		fmt.Printf("Received error no. as %d\n", e)
		if e == 0 {
			return io.EOF
		}
		return syscall.Errno(-e)
	}
	return nil
}
