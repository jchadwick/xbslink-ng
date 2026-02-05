package capture

import (
	"net"
	"strings"
	"testing"
)

func TestParseMAC_Colons(t *testing.T) {
	mac, err := ParseMAC("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestParseMAC_Dashes(t *testing.T) {
	mac, err := ParseMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestParseMAC_Lowercase(t *testing.T) {
	mac, err := ParseMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestParseMAC_MixedCase(t *testing.T) {
	mac, err := ParseMAC("Aa:Bb:Cc:Dd:Ee:Ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestParseMAC_TooShort(t *testing.T) {
	_, err := ParseMAC("AA:BB:CC")
	if err == nil {
		t.Error("expected error for too-short MAC")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid MAC") {
		t.Errorf("expected 'invalid MAC' error, got: %v", err)
	}
}

func TestParseMAC_TooLong(t *testing.T) {
	_, err := ParseMAC("AA:BB:CC:DD:EE:FF:00")
	if err == nil {
		t.Error("expected error for too-long MAC")
	}
}

func TestParseMAC_InvalidChars(t *testing.T) {
	_, err := ParseMAC("GG:HH:II:JJ:KK:LL")
	if err == nil {
		t.Error("expected error for invalid hex chars")
	}
}

func TestParseMAC_Empty(t *testing.T) {
	_, err := ParseMAC("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParseMAC_Broadcast(t *testing.T) {
	mac, err := ParseMAC("FF:FF:FF:FF:FF:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestParseMAC_XboxOUI(t *testing.T) {
	mac, err := ParseMAC("00:50:F2:12:34:56")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := net.HardwareAddr{0x00, 0x50, 0xF2, 0x12, 0x34, 0x56}
	if !macEqual(mac, expected) {
		t.Errorf("ParseMAC = %v, want %v", mac, expected)
	}
}

func TestListInterfaces(t *testing.T) {
	// This test requires pcap to be available
	interfaces, err := ListInterfaces()
	if err != nil {
		// Skip if pcap not available (no admin rights, etc.)
		t.Skipf("ListInterfaces not available: %v", err)
	}

	// Should have at least one interface
	if len(interfaces) == 0 {
		t.Error("expected at least one interface")
	}
}

func TestFindInterface_NotFound(t *testing.T) {
	_, err := FindInterface("definitely-not-a-real-interface-name-12345")
	if err == nil {
		t.Error("expected error for non-existent interface")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		// Might also fail due to pcap not being available
		if !strings.Contains(err.Error(), "npcap") && !strings.Contains(err.Error(), "permission") {
			t.Logf("got error: %v", err)
		}
	}
}

func TestFormatInterfaceList(t *testing.T) {
	interfaces := []InterfaceInfo{
		{
			Name:        "eth0",
			Description: "Ethernet adapter",
			Addresses:   []string{"192.168.1.100", "fe80::1"},
			Flags:       "UP",
		},
		{
			Name:        "lo",
			Description: "Loopback",
			Addresses:   []string{"127.0.0.1"},
			Flags:       "UP",
		},
	}

	output := FormatInterfaceList(interfaces)

	if !strings.Contains(output, "eth0") {
		t.Error("expected eth0 in output")
	}
	if !strings.Contains(output, "Ethernet adapter") {
		t.Error("expected description in output")
	}
	if !strings.Contains(output, "192.168.1.100") {
		t.Error("expected IP address in output")
	}
	if !strings.Contains(output, "lo") {
		t.Error("expected lo in output")
	}
}

func TestDecodeEthernetFrame_Valid(t *testing.T) {
	frame := make([]byte, 64)
	// Destination MAC
	copy(frame[0:6], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	// Source MAC
	copy(frame[6:12], []byte{0x00, 0x50, 0xF2, 0x12, 0x34, 0x56})
	// EtherType (IPv4 = 0x0800)
	frame[12] = 0x08
	frame[13] = 0x00

	srcMAC, dstMAC, etherType := DecodeEthernetFrame(frame)

	expectedSrc := net.HardwareAddr{0x00, 0x50, 0xF2, 0x12, 0x34, 0x56}
	expectedDst := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	if !macEqual(srcMAC, expectedSrc) {
		t.Errorf("srcMAC = %v, want %v", srcMAC, expectedSrc)
	}
	if !macEqual(dstMAC, expectedDst) {
		t.Errorf("dstMAC = %v, want %v", dstMAC, expectedDst)
	}
	if etherType != 0x0800 {
		t.Errorf("etherType = 0x%04X, want 0x0800", etherType)
	}
}

func TestDecodeEthernetFrame_TooShort(t *testing.T) {
	frame := make([]byte, 10) // Less than 14 bytes

	srcMAC, dstMAC, etherType := DecodeEthernetFrame(frame)

	if srcMAC != nil || dstMAC != nil || etherType != 0 {
		t.Error("expected nil/zero values for too-short frame")
	}
}

func TestDecodeEthernetFrame_IPv4(t *testing.T) {
	frame := make([]byte, 14)
	frame[12] = 0x08
	frame[13] = 0x00

	_, _, etherType := DecodeEthernetFrame(frame)
	if etherType != 0x0800 {
		t.Errorf("etherType = 0x%04X, want 0x0800", etherType)
	}
}

func TestDecodeEthernetFrame_IPv6(t *testing.T) {
	frame := make([]byte, 14)
	frame[12] = 0x86
	frame[13] = 0xDD

	_, _, etherType := DecodeEthernetFrame(frame)
	if etherType != 0x86DD {
		t.Errorf("etherType = 0x%04X, want 0x86DD", etherType)
	}
}

func TestDecodeEthernetFrame_ARP(t *testing.T) {
	frame := make([]byte, 14)
	frame[12] = 0x08
	frame[13] = 0x06

	_, _, etherType := DecodeEthernetFrame(frame)
	if etherType != 0x0806 {
		t.Errorf("etherType = 0x%04X, want 0x0806", etherType)
	}
}

func TestEtherTypeName(t *testing.T) {
	tests := []struct {
		etherType uint16
		expected  string
	}{
		{0x0800, "IPv4"},
		{0x86DD, "IPv6"},
		{0x0806, "ARP"},
		{0x1234, "0x1234"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := EtherTypeName(tt.etherType)
			if result != tt.expected {
				t.Errorf("EtherTypeName(0x%04X) = %s, want %s", tt.etherType, result, tt.expected)
			}
		})
	}
}

func TestNpcapInstallHelp(t *testing.T) {
	help := NpcapInstallHelp()
	if help == "" {
		t.Error("expected non-empty help text")
	}
}

// Helper function to compare MAC addresses
func macEqual(a, b net.HardwareAddr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
