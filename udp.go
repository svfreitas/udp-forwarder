package main

import (
	"net"
	"sync/atomic"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const MTU = 1500
const IPv4HeaderSize = 20

// IP payload carried by each fragment, must be a multiple of 8 (1480)
const MaxFragmentPayload = (MTU - IPv4HeaderSize) &^ 7

// IP identification, must be unique per datagram so the receiver does not mix fragments
var ipID atomic.Uint32

type UdpFrameOptions struct {
	sourceIP, destIP     net.IP
	sourcePort, destPort uint16
	payloadBytes         []byte
}

// createSerializedUDPPackets creates the IPv4 packets (header included) carrying
// our UDP datagram, fragmented when it does not fit in the MTU.
// Routing and link layer are left to the kernel.
func createSerializedUDPPackets(opts UdpFrameOptions) ([][]byte, error) {
	// IP  Header = 20  <--+
	// UDP header = 8   <--+-- 1500 == MTU
	// Payload 0-1472   <--+
	// Next fragments carry 1480 bytes of the UDP datagram each
	serializeOpts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	// Serialize the whole UDP datagram once, so the checksum covers the full payload
	pseudoHeader := &layers.IPv4{
		SrcIP:    opts.sourceIP,
		DstIP:    opts.destIP,
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(opts.sourcePort),
		DstPort: layers.UDPPort(opts.destPort),
	}
	udp.SetNetworkLayerForChecksum(pseudoHeader)
	udpBuf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(udpBuf, serializeOpts, udp, gopacket.Payload(opts.payloadBytes)); err != nil {
		return nil, err
	}
	datagram := udpBuf.Bytes()

	id := uint16(ipID.Add(1))
	numberOfFragments := (len(datagram) + MaxFragmentPayload - 1) / MaxFragmentPayload
	slogger.Debugf("payloadSize=%d, numberOfFragments=%d, id=%d", len(opts.payloadBytes), numberOfFragments, id)

	slicePackets := make([][]byte, 0, numberOfFragments)
	for offset := 0; offset < len(datagram); offset += MaxFragmentPayload {
		end := offset + MaxFragmentPayload
		var flags layers.IPv4Flag
		if end < len(datagram) {
			flags = layers.IPv4MoreFragments
		} else {
			end = len(datagram)
		}

		ip := &layers.IPv4{
			Version:    4,
			IHL:        5,
			TTL:        64,
			Id:         id,
			Flags:      flags,
			FragOffset: uint16(offset / 8),
			Protocol:   layers.IPProtocolUDP,
			SrcIP:      opts.sourceIP,
			DstIP:      opts.destIP,
		}

		buf := gopacket.NewSerializeBuffer()
		if err := gopacket.SerializeLayers(buf, serializeOpts, ip, gopacket.Payload(datagram[offset:end])); err != nil {
			return nil, err
		}
		slicePackets = append(slicePackets, buf.Bytes())
	}

	return slicePackets, nil
}
