// Package discovery provides passive Xbox console discovery.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket/pcap"

	"github.com/xbslink/xbslink-ng/internal/logging"
)

// Xbox System Link uses UDP port 3074 (registered with IANA for Xbox).
const XboxSystemLinkPort = 3074

// Configuration constants.
const (
	// SnapLen captures enough for Ethernet + IP + UDP headers plus some payload.
	SnapLen = 128
	// ReadTimeout is the pcap read timeout.
	ReadTimeout = 100 * time.Millisecond
)

// Errors returned by discovery operations.
var (
	ErrDiscoveryCancelled = errors.New("discovery cancelled")
	ErrInterfaceNotFound  = errors.New("interface not found")
)

// Result represents a discovered Xbox console.
type Result struct {
	MAC      net.HardwareAddr
	LastSeen time.Time
}

// Config holds discovery configuration.
type Config struct {
	Interface string          // Network interface name
	Logger    *logging.Logger // Logger (optional)
}

// Discover passively listens for Xbox System Link traffic on the specified interface.
// It detects any device sending UDP traffic on port 3074 (Xbox System Link port).
// Returns immediately when the first Xbox is detected.
// The operation can be cancelled via the context.
func Discover(ctx context.Context, cfg Config) (*Result, error) {
	// Find the interface
	iface, err := findInterface(cfg.Interface)
	if err != nil {
		return nil, err
	}

	// Open pcap handle
	inactive, err := pcap.NewInactiveHandle(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to create handle for %s: %w", cfg.Interface, err)
	}
	defer inactive.CleanUp()

	// Configure the handle
	if err := inactive.SetSnapLen(SnapLen); err != nil {
		return nil, fmt.Errorf("failed to set snap length: %w", err)
	}
	if err := inactive.SetPromisc(true); err != nil {
		return nil, fmt.Errorf("failed to set promiscuous mode: %w", err)
	}
	if err := inactive.SetTimeout(ReadTimeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}

	// Activate the handle
	handle, err := inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("failed to activate capture on %s: %w", cfg.Interface, err)
	}
	defer handle.Close()

	// BPF filter for Xbox System Link traffic:
	// - UDP port 3074 (Xbox System Link port)
	// This catches any device (Xbox, emulators) sending System Link traffic
	filter := fmt.Sprintf("udp port %d", XboxSystemLinkPort)

	if err := handle.SetBPFFilter(filter); err != nil {
		return nil, fmt.Errorf("failed to set BPF filter: %w", err)
	}

	if cfg.Logger != nil {
		cfg.Logger.Debug("Listening for Xbox System Link traffic (UDP port %d)", XboxSystemLinkPort)
	}

	// Listen for packets
	for {
		select {
		case <-ctx.Done():
			return nil, ErrDiscoveryCancelled
		default:
		}

		data, _, err := handle.ZeroCopyReadPacketData()
		if err != nil {
			if err == pcap.NextErrorTimeoutExpired {
				continue
			}
			// Other errors might be transient, continue
			continue
		}

		// Need at least 14 bytes for Ethernet header
		if len(data) < 14 {
			continue
		}

		// Extract source MAC (bytes 6-11 of Ethernet frame)
		srcMAC := net.HardwareAddr(data[6:12])

		// Skip broadcast/multicast source MACs (invalid)
		if srcMAC[0]&0x01 != 0 {
			continue
		}

		// Found a device sending System Link traffic
		mac := make(net.HardwareAddr, 6)
		copy(mac, srcMAC)

		return &Result{
			MAC:      mac,
			LastSeen: time.Now(),
		}, nil
	}
}

// findInterface finds an interface by name using pcap.
func findInterface(name string) (string, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("failed to list interfaces: %w", err)
	}

	nameLower := strings.ToLower(name)

	// Try exact match first
	for _, dev := range devices {
		if dev.Name == name {
			return dev.Name, nil
		}
	}

	// Try case-insensitive match
	for _, dev := range devices {
		if strings.ToLower(dev.Name) == nameLower {
			return dev.Name, nil
		}
	}

	// Try partial match on description (useful on Windows)
	for _, dev := range devices {
		if strings.Contains(strings.ToLower(dev.Description), nameLower) {
			return dev.Name, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrInterfaceNotFound, name)
}
