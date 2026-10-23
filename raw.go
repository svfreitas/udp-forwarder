package main

import (
	"fmt"
	"net"
	"syscall"
)

// rawSender sends complete IPv4 packets (header included) through a raw socket.
// The kernel takes care of routing and ARP resolution.
type rawSender struct {
	fd int
}

func newRawSender(interfaceName string) (*rawSender, error) {
	// IPPROTO_RAW implies IP_HDRINCL and the socket never receives packets
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return nil, fmt.Errorf("unable to open raw socket: %w", err)
	}
	if err := syscall.BindToDevice(fd, interfaceName); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("unable to bind raw socket to interface %s: %w", interfaceName, err)
	}
	return &rawSender{fd: fd}, nil
}

func (s *rawSender) Send(destIP net.IP, packet []byte) error {
	var sa syscall.SockaddrInet4
	copy(sa.Addr[:], destIP.To4())
	return syscall.Sendto(s.fd, packet, 0, &sa)
}

func (s *rawSender) Close() error {
	return syscall.Close(s.fd)
}
