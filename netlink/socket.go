// Copyright Microsoft Corp.
// All rights reserved.

package netlink

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
	"github.com/Azure/Aqua/log"
)

// Represents a netlink socket.
type socket struct {
	fd  int
	sa  unix.SockaddrNetlink
	pid uint32
	seq uint32
	sync.Mutex
}

// Default netlink socket.
var s *socket
var once sync.Once

// Returns a reference to the default netlink socket.
func getSocket() (*socket, error) {
	var err error
	once.Do(func() { s, err = newSocket() })
	return s, err
}

// Creates a new netlink socket object.
func newSocket() (*socket, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE)
	defer log.Printf("[netlink] Socket created, err=%v\n", err)
	if err != nil {
		return nil, err
	}

	s := &socket{
		fd:  fd,
		pid: uint32(unix.Getpid()),
		seq: 0,
	}

	s.sa.Family = unix.AF_NETLINK

	err = unix.Bind(fd, &s.sa)
	if err != nil {
		unix.Close(fd)
		return nil, err
	}

	return s, nil
}

// Closes the socket.
func (s *socket) close() {
	err := unix.Close(s.fd)
	log.Printf("[netlink] Socket closed, err=%v\n", err)
}

// Sends a netlink message.
func (s *socket) send(msg *message) error {
	msg.Seq = atomic.AddUint32(&s.seq, 1)
	err := unix.Sendto(s.fd, msg.serialize(), 0, &s.sa)
	log.Printf("[netlink] Sent %+v, err=%v\n", *msg, err)
	return err
}

// Sends a netlink message and blocks until its completion.
func (s *socket) sendAndComplete(msg *message) error {
	s.Lock()
	defer s.Unlock()

	err := s.send(msg)
	if err != nil {
		return err
	}

	return s.waitForAck(msg)
}

// Receives a netlink message.
func (s *socket) receive() ([]syscall.NetlinkMessage, error) {
	buffer := make([]byte, unix.Getpagesize())
	n, _, err := unix.Recvfrom(s.fd, buffer, 0)

	if err != nil {
		return nil, err
	}

	if n < unix.NLMSG_HDRLEN {
		return nil, fmt.Errorf("Invalid netlink message")
	}

	buffer = buffer[:n]
	return syscall.ParseNetlinkMessage(buffer)
}

// Waits for the acknowledgement for the given sent message.
func (s *socket) waitForAck(sent *message) error {
	for {
		received, err := s.receive()
		if err != nil {
			log.Printf("[netlink] Receive err=%v\n", err)
			return err
		}

		for _, msg := range received {
			// An acknowledgement is an error message with error code set to
			// zero, followed by the original request message header.
			if msg.Header.Type == unix.NLMSG_ERROR &&
			   msg.Header.Seq == sent.Seq &&
			   msg.Header.Pid == sent.Pid {

				errCode := int32(encoder.Uint32(msg.Data[0:4]))
				if errCode != 0 {
					// Request failed.
					err = syscall.Errno(-errCode)
				}

				log.Printf("[netlink] Received %+v, err=%v\n", msg, err)

				return err
			} else {
				log.Printf("[netlink] Ignoring unexpected message %+v\n", msg)
			}
		}
	}
}
