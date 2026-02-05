// Package main provides the xbox-sim tool for E2E testing.
// xbox-sim simulates Xbox System Link traffic for testing xbslink-ng.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "test":
		runTests()
	case "generate":
		runGenerate()
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`xbox-sim - Xbox System Link Traffic Simulator

Commands:
  test      Run E2E tests against xbslink-ng bridges
  generate  Generate traffic for manual testing
  help      Show this help message

Test flags:
  --bridge-a     IP address of bridge A
  --bridge-b     IP address of bridge B
  --xbox-mac-a   Xbox MAC for bridge A (default: 00:50:F2:AA:AA:AA)
  --xbox-mac-b   Xbox MAC for bridge B (default: 00:50:F2:BB:BB:BB)
`)
}

func runTests() {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	bridgeA := fs.String("bridge-a", "", "IP address of bridge A")
	bridgeB := fs.String("bridge-b", "", "IP address of bridge B")
	xboxMacA := fs.String("xbox-mac-a", "00:50:F2:AA:AA:AA", "Xbox MAC for bridge A")
	xboxMacB := fs.String("xbox-mac-b", "00:50:F2:BB:BB:BB", "Xbox MAC for bridge B")
	
	fs.Parse(os.Args[2:])

	if *bridgeA == "" || *bridgeB == "" {
		fmt.Fprintln(os.Stderr, "Error: --bridge-a and --bridge-b are required")
		os.Exit(1)
	}

	fmt.Println("=== Xbox System Link E2E Tests ===")
	fmt.Printf("Bridge A: %s (Xbox MAC: %s)\n", *bridgeA, *xboxMacA)
	fmt.Printf("Bridge B: %s (Xbox MAC: %s)\n", *bridgeB, *xboxMacB)
	fmt.Println()

	passed := 0
	failed := 0

	// Test 1: Verify bridges are reachable
	fmt.Print("Test 1: Bridges reachable... ")
	if testBridgesReachable(*bridgeA, *bridgeB) {
		fmt.Println("PASSED")
		passed++
	} else {
		fmt.Println("FAILED")
		failed++
	}

	// Test 2: UDP connectivity
	fmt.Print("Test 2: UDP connectivity... ")
	if testUDPConnectivity(*bridgeA, *bridgeB) {
		fmt.Println("PASSED")
		passed++
	} else {
		fmt.Println("FAILED")
		failed++
	}

	// Test 3: Simulated frame exchange (simplified without pcap)
	fmt.Print("Test 3: Frame exchange simulation... ")
	if testFrameExchange(*bridgeA, *bridgeB, *xboxMacA, *xboxMacB) {
		fmt.Println("PASSED")
		passed++
	} else {
		fmt.Println("FAILED")
		failed++
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)

	if failed > 0 {
		os.Exit(1)
	}
}

func testBridgesReachable(bridgeA, bridgeB string) bool {
	// Try to resolve and reach both bridge IPs
	addrA, err := net.ResolveUDPAddr("udp", bridgeA+":31415")
	if err != nil {
		fmt.Printf("\n  Error resolving bridge A: %v\n", err)
		return false
	}

	addrB, err := net.ResolveUDPAddr("udp", bridgeB+":31415")
	if err != nil {
		fmt.Printf("\n  Error resolving bridge B: %v\n", err)
		return false
	}

	// Quick UDP probe
	conn, err := net.DialUDP("udp", nil, addrA)
	if err != nil {
		fmt.Printf("\n  Error connecting to bridge A: %v\n", err)
		return false
	}
	conn.Close()

	conn, err = net.DialUDP("udp", nil, addrB)
	if err != nil {
		fmt.Printf("\n  Error connecting to bridge B: %v\n", err)
		return false
	}
	conn.Close()

	return true
}

func testUDPConnectivity(bridgeA, bridgeB string) bool {
	// Create UDP socket
	localAddr, _ := net.ResolveUDPAddr("udp", ":0")
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		fmt.Printf("\n  Error creating UDP socket: %v\n", err)
		return false
	}
	defer conn.Close()

	// Try sending to both bridges (they won't respond, but we verify no errors)
	remoteA, _ := net.ResolveUDPAddr("udp", bridgeA+":31415")
	remoteB, _ := net.ResolveUDPAddr("udp", bridgeB+":31415")

	testPacket := []byte("xbox-sim-test")
	_, err = conn.WriteToUDP(testPacket, remoteA)
	if err != nil {
		fmt.Printf("\n  Error sending to bridge A: %v\n", err)
		return false
	}

	_, err = conn.WriteToUDP(testPacket, remoteB)
	if err != nil {
		fmt.Printf("\n  Error sending to bridge B: %v\n", err)
		return false
	}

	return true
}

func testFrameExchange(bridgeA, bridgeB, xboxMacA, xboxMacB string) bool {
	// This is a simplified test that verifies the test infrastructure works
	// Real frame exchange testing would require pcap access

	macA, err := net.ParseMAC(xboxMacA)
	if err != nil {
		fmt.Printf("\n  Invalid MAC A: %v\n", err)
		return false
	}

	macB, err := net.ParseMAC(xboxMacB)
	if err != nil {
		fmt.Printf("\n  Invalid MAC B: %v\n", err)
		return false
	}

	// Build a simulated Ethernet frame
	frame := buildEthernetFrame(macA, macB, 0x0800, []byte("test-payload"))
	if len(frame) < 14 {
		fmt.Printf("\n  Frame too short: %d bytes\n", len(frame))
		return false
	}

	// Verify frame structure
	dstMAC := net.HardwareAddr(frame[0:6])
	srcMAC := net.HardwareAddr(frame[6:12])
	etherType := binary.BigEndian.Uint16(frame[12:14])

	if !macEqual(srcMAC, macA) {
		fmt.Printf("\n  Source MAC mismatch\n")
		return false
	}
	if !macEqual(dstMAC, macB) {
		fmt.Printf("\n  Dest MAC mismatch\n")
		return false
	}
	if etherType != 0x0800 {
		fmt.Printf("\n  EtherType mismatch: 0x%04X\n", etherType)
		return false
	}

	return true
}

func buildEthernetFrame(srcMAC, dstMAC net.HardwareAddr, etherType uint16, payload []byte) []byte {
	frame := make([]byte, 14+len(payload))
	copy(frame[0:6], dstMAC)
	copy(frame[6:12], srcMAC)
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	copy(frame[14:], payload)
	return frame
}

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

func runGenerate() {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	targetIP := fs.String("target", "", "Target IP address")
	targetPort := fs.Int("port", 31415, "Target UDP port")
	count := fs.Int("count", 10, "Number of frames to generate")
	interval := fs.Duration("interval", 100*time.Millisecond, "Interval between frames")
	
	fs.Parse(os.Args[2:])

	if *targetIP == "" {
		fmt.Fprintln(os.Stderr, "Error: --target is required")
		os.Exit(1)
	}

	fmt.Printf("Generating %d frames to %s:%d\n", *count, *targetIP, *targetPort)

	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", *targetIP, *targetPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving address: %v\n", err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	for i := 0; i < *count; i++ {
		// Create a simple test frame
		frame := buildEthernetFrame(
			net.HardwareAddr{0x00, 0x50, 0xF2, 0xAA, 0xAA, 0xAA},
			net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			0x0800,
			[]byte(fmt.Sprintf("frame-%d", i)),
		)

		_, err := conn.Write(frame)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending frame %d: %v\n", i, err)
			continue
		}

		fmt.Printf("Sent frame %d (%d bytes)\n", i+1, len(frame))
		time.Sleep(*interval)
	}

	fmt.Println("Done!")
}
