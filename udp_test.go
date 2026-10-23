package main

import (
	"bytes"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"go.uber.org/zap"
)

func TestCreateSerializedUDPPackets(t *testing.T) {
	slogger = zap.NewNop().Sugar()
	srcIP := net.ParseIP("10.0.0.1")
	dstIP := net.ParseIP("10.0.0.2")

	lastID := -1
	for _, size := range []int{0, 1, 1472, 1473, 2951, 2952, 2953, 4432, 10000, 65507} {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i * 7)
		}

		packets, err := createSerializedUDPPackets(UdpFrameOptions{
			sourceIP:     srcIP,
			destIP:       dstIP,
			sourcePort:   1111,
			destPort:     2222,
			payloadBytes: payload,
		})
		if err != nil {
			t.Fatalf("size %d: %s", size, err)
		}
		if want := (size + 8 + MaxFragmentPayload - 1) / MaxFragmentPayload; len(packets) != want {
			t.Errorf("size %d: got %d packets, want %d", size, len(packets), want)
		}

		// reassemble, checking every fragment like a receiver would
		var datagram []byte
		for i, p := range packets {
			if len(p) > MTU {
				t.Errorf("size %d: packet %d has %d bytes, bigger than MTU", size, i, len(p))
			}
			ip := gopacket.NewPacket(p, layers.LayerTypeIPv4, gopacket.Default).Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !bytes.Equal(ip.SrcIP, srcIP.To4()) || !bytes.Equal(ip.DstIP, dstIP.To4()) || int(ip.Length) != len(p) {
				t.Errorf("size %d: packet %d bad IP header %+v", size, i, ip)
			}
			if len(ip.Payload) == 0 {
				t.Errorf("size %d: packet %d is an empty fragment", size, i)
			}
			if i == 0 {
				if int(ip.Id) == lastID {
					t.Errorf("size %d: IP id %d reused from previous datagram", size, ip.Id)
				}
				lastID = int(ip.Id)
			} else if int(ip.Id) != lastID {
				t.Errorf("size %d: packet %d has IP id %d, want %d", size, i, ip.Id, lastID)
			}
			isLast := i == len(packets)-1
			if (ip.Flags&layers.IPv4MoreFragments != 0) == isLast {
				t.Errorf("size %d: packet %d has wrong MF flag", size, i)
			}
			if !isLast && len(ip.Payload)%8 != 0 {
				t.Errorf("size %d: packet %d payload %d not multiple of 8", size, i, len(ip.Payload))
			}
			if int(ip.FragOffset)*8 != len(datagram) {
				t.Errorf("size %d: packet %d offset %d, want %d", size, i, int(ip.FragOffset)*8, len(datagram))
			}
			datagram = append(datagram, ip.Payload...)
		}
		if !udpChecksumValid(srcIP, dstIP, datagram) {
			t.Errorf("size %d: bad UDP checksum", size)
		}

		udp := gopacket.NewPacket(datagram, layers.LayerTypeUDP, gopacket.Default).Layer(layers.LayerTypeUDP).(*layers.UDP)
		if udp.SrcPort != 1111 || udp.DstPort != 2222 || int(udp.Length) != size+8 {
			t.Errorf("size %d: bad UDP header %+v", size, udp)
		}
		if !bytes.Equal(udp.Payload, payload) {
			t.Errorf("size %d: reassembled payload differs", size)
		}
	}
}

// udpChecksumValid checks the one's complement sum over pseudo header and datagram
func udpChecksumValid(src, dst net.IP, datagram []byte) bool {
	var sum uint32
	add := func(b []byte) {
		for i := 0; i+1 < len(b); i += 2 {
			sum += uint32(b[i])<<8 | uint32(b[i+1])
		}
		if len(b)%2 == 1 {
			sum += uint32(b[len(b)-1]) << 8
		}
	}
	add(src.To4())
	add(dst.To4())
	add([]byte{0, byte(layers.IPProtocolUDP), byte(len(datagram) >> 8), byte(len(datagram))})
	add(datagram)
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return sum == 0xffff
}
