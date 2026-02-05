// Package testutil provides test helpers and utilities for xbslink-ng tests.
package testutil

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"time"
)

// RandomBytes generates cryptographically random bytes.
func RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// RandomMAC generates a random MAC address.
func RandomMAC() net.HardwareAddr {
	mac := make([]byte, 6)
	_, _ = rand.Read(mac)
	// Set unicast bit (clear multicast bit)
	mac[0] &= 0xFE
	return mac
}

// RandomXboxMAC generates a random MAC with Xbox OUI (00:50:F2).
func RandomXboxMAC() net.HardwareAddr {
	mac := make([]byte, 6)
	mac[0] = 0x00
	mac[1] = 0x50
	mac[2] = 0xF2
	_, _ = rand.Read(mac[3:])
	return mac
}

// RandomFrame generates a valid Ethernet frame with random content.
// Size must be at least 14 (Ethernet header only).
func RandomFrame(size int) []byte {
	if size < 14 {
		size = 14
	}
	frame := make([]byte, size)
	_, _ = rand.Read(frame)

	// Set proper EtherType (IPv4 = 0x0800)
	frame[12] = 0x08
	frame[13] = 0x00

	return frame
}

// RandomEthernetFrame generates a valid Ethernet frame with specified MACs.
func RandomEthernetFrame(srcMAC, dstMAC net.HardwareAddr, etherType uint16, payloadSize int) []byte {
	frame := make([]byte, 14+payloadSize)

	// Destination MAC (bytes 0-5)
	copy(frame[0:6], dstMAC)
	// Source MAC (bytes 6-11)
	copy(frame[6:12], srcMAC)
	// EtherType (bytes 12-13)
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	// Random payload
	if payloadSize > 0 {
		_, _ = rand.Read(frame[14:])
	}

	return frame
}

// FreePort finds an available UDP port.
func FreePort() int {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

// WaitFor polls until condition is true or timeout.
func WaitFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// BroadcastMAC returns the Ethernet broadcast address.
func BroadcastMAC() net.HardwareAddr {
	return net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
}

// EtherTypeIPv4 is the EtherType for IPv4.
const EtherTypeIPv4 = 0x0800

// EtherTypeIPv6 is the EtherType for IPv6.
const EtherTypeIPv6 = 0x86DD

// EtherTypeARP is the EtherType for ARP.
const EtherTypeARP = 0x0806
