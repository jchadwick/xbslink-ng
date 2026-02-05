// Package capture provides pcap-based packet capture and injection.
package capture

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/xbslink/xbslink-ng/internal/logging"
)

// Configuration constants.
const (
	// SnapLen is the maximum number of bytes to capture per packet.
	SnapLen = 65536
	// ReadTimeout is the pcap read timeout.
	ReadTimeout = 10 * time.Millisecond
	// BufferSize is the pcap buffer size (platform-dependent defaults may apply).
	BufferSize = 2 * 1024 * 1024 // 2MB
)

// Errors returned by capture operations.
var (
	ErrNpcapNotInstalled = errors.New("npcap not installed")
	ErrInterfaceNotFound = errors.New("interface not found")
	ErrInvalidMAC        = errors.New("invalid MAC address format")
)

// InterfaceInfo contains information about a network interface.
type InterfaceInfo struct {
	Name        string   // System name (e.g., "eth0", "Ethernet")
	Description string   // Human-readable description
	Addresses   []string // IP addresses assigned to this interface
	Flags       string   // Interface flags
}

// Capture handles pcap packet capture and injection.
type Capture struct {
	handle  *pcap.Handle
	xboxMAC net.HardwareAddr
	ifName  string
	logger  *logging.Logger
}

// Config holds capture configuration.
type Config struct {
	Interface string           // Network interface name
	XboxMAC   net.HardwareAddr // Xbox MAC address to filter
	Logger    *logging.Logger
}

// CheckNpcapInstalled checks if Npcap is installed on Windows.
// Returns nil on non-Windows platforms or if Npcap is found.
func CheckNpcapInstalled() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	// On Windows, try to find wpcap.dll
	// The gopacket/pcap package will fail to initialize if Npcap isn't installed,
	// but we want to give a helpful error message first.

	// Try to get the version string - this will fail if Npcap isn't installed
	version := pcap.Version()
	if version == "" {
		return ErrNpcapNotInstalled
	}

	// Check if it looks like Npcap
	if !strings.Contains(strings.ToLower(version), "npcap") &&
		!strings.Contains(strings.ToLower(version), "winpcap") {
		// Might still work, just warn
		return nil
	}

	return nil
}

// NpcapInstallHelp returns platform-specific help for installing packet capture support.
func NpcapInstallHelp() string {
	switch runtime.GOOS {
	case "windows":
		return `Npcap is required for packet capture on Windows.

To install Npcap:
1. Download from https://npcap.com/
2. Run the installer
3. IMPORTANT: Check "Install Npcap in WinPcap API-compatible Mode"
4. Restart this application

If you have Npcap installed but still see this error:
- Make sure you installed with WinPcap compatibility mode
- Try running this application as Administrator`

	case "darwin":
		return `Packet capture requires root privileges on macOS.

Try running with sudo:
  sudo xbslink-ng [command] [flags]

If you see "Operation not permitted", ensure your terminal has 
Full Disk Access in System Preferences > Privacy & Security.`

	case "linux":
		return `Packet capture requires either root privileges or the pcap capability.

Option 1: Run with sudo:
  sudo xbslink-ng [command] [flags]

Option 2: Add pcap capability to the binary:
  sudo setcap cap_net_raw,cap_net_admin=eip /path/to/xbslink-ng

Option 3: Add your user to the pcap group:
  sudo usermod -a -G pcap $USER
  (Log out and back in for this to take effect)

If libpcap is not installed:
  Debian/Ubuntu: sudo apt install libpcap-dev
  Fedora/RHEL:   sudo dnf install libpcap-devel
  Arch:          sudo pacman -S libpcap`

	default:
		return "Ensure libpcap is installed and you have permission to capture packets."
	}
}

// ListInterfaces returns all available network interfaces.
func ListInterfaces() ([]InterfaceInfo, error) {
	// Check Npcap on Windows first
	if err := CheckNpcapInstalled(); err != nil {
		return nil, fmt.Errorf("%w\n\n%s", err, NpcapInstallHelp())
	}

	devices, err := pcap.FindAllDevs()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w\n\n%s", err, NpcapInstallHelp())
	}

	var interfaces []InterfaceInfo
	for _, dev := range devices {
		info := InterfaceInfo{
			Name:        dev.Name,
			Description: dev.Description,
		}

		// Collect IP addresses
		for _, addr := range dev.Addresses {
			if addr.IP != nil {
				info.Addresses = append(info.Addresses, addr.IP.String())
			}
		}

		// Build flags string
		var flags []string
		if len(dev.Addresses) > 0 {
			flags = append(flags, "UP")
		}
		info.Flags = strings.Join(flags, ",")

		interfaces = append(interfaces, info)
	}

	return interfaces, nil
}

// FindInterface finds an interface by name (exact or partial match).
func FindInterface(name string) (*InterfaceInfo, error) {
	interfaces, err := ListInterfaces()
	if err != nil {
		return nil, err
	}

	// Try exact match first
	for _, iface := range interfaces {
		if iface.Name == name {
			return &iface, nil
		}
	}

	// Try case-insensitive match
	nameLower := strings.ToLower(name)
	for _, iface := range interfaces {
		if strings.ToLower(iface.Name) == nameLower {
			return &iface, nil
		}
	}

	// Try partial match on description (useful on Windows)
	for _, iface := range interfaces {
		if strings.Contains(strings.ToLower(iface.Description), nameLower) {
			return &iface, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrInterfaceNotFound, name)
}

// ParseMAC parses a MAC address in XX:XX:XX:XX:XX:XX or XX-XX-XX-XX-XX-XX format.
func ParseMAC(s string) (net.HardwareAddr, error) {
	// Normalize to colon separator
	s = strings.ReplaceAll(s, "-", ":")
	mac, err := net.ParseMAC(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMAC, err)
	}
	if len(mac) != 6 {
		return nil, fmt.Errorf("%w: expected 6 bytes, got %d", ErrInvalidMAC, len(mac))
	}
	return mac, nil
}

// New creates a new Capture instance.
func New(cfg Config) (*Capture, error) {
	if cfg.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if len(cfg.XboxMAC) != 6 {
		return nil, ErrInvalidMAC
	}

	// Check Npcap on Windows
	if err := CheckNpcapInstalled(); err != nil {
		return nil, fmt.Errorf("%w\n\n%s", err, NpcapInstallHelp())
	}

	// Find the interface
	iface, err := FindInterface(cfg.Interface)
	if err != nil {
		return nil, err
	}

	cfg.Logger.Debug("Opening interface %s (%s)", iface.Name, iface.Description)

	// Open pcap handle
	inactive, err := pcap.NewInactiveHandle(iface.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create handle for %s: %w\n\n%s", iface.Name, err, NpcapInstallHelp())
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

	// Set buffer size (may fail on some platforms, ignore error)
	_ = inactive.SetBufferSize(BufferSize)

	// Activate the handle
	handle, err := inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("failed to activate capture on %s: %w\n\n%s", iface.Name, err, NpcapInstallHelp())
	}

	// Set BPF filter to capture only packets from the Xbox MAC
	// This significantly reduces CPU usage by filtering in the kernel
	filter := fmt.Sprintf("ether src %s", cfg.XboxMAC.String())
	if err := handle.SetBPFFilter(filter); err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to set BPF filter %q: %w", filter, err)
	}

	cfg.Logger.Debug("BPF filter set: %s", filter)

	c := &Capture{
		handle:  handle,
		xboxMAC: cfg.XboxMAC,
		ifName:  iface.Name,
		logger:  cfg.Logger,
	}

	return c, nil
}

// ReadPacket reads the next packet from the capture.
// Returns the raw Ethernet frame bytes, or nil if no packet is available.
func (c *Capture) ReadPacket() ([]byte, error) {
	// Use ZeroCopyReadPacketData for efficiency
	data, _, err := c.handle.ZeroCopyReadPacketData()
	if err != nil {
		if err == pcap.NextErrorTimeoutExpired {
			return nil, nil // No packet available
		}
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	// Make a copy since ZeroCopy data is only valid until next read
	frame := make([]byte, len(data))
	copy(frame, data)

	return frame, nil
}

// WritePacket injects a raw Ethernet frame onto the network.
func (c *Capture) WritePacket(frame []byte) error {
	if len(frame) < 14 {
		return fmt.Errorf("frame too small: %d bytes", len(frame))
	}

	return c.handle.WritePacketData(frame)
}

// Close closes the capture handle.
func (c *Capture) Close() error {
	if c.handle != nil {
		c.handle.Close()
		c.handle = nil
	}
	return nil
}

// Stats returns capture statistics.
func (c *Capture) Stats() (*pcap.Stats, error) {
	if c.handle == nil {
		return nil, errors.New("capture not open")
	}
	return c.handle.Stats()
}

// InterfaceName returns the name of the capture interface.
func (c *Capture) InterfaceName() string {
	return c.ifName
}

// XboxMAC returns the Xbox MAC address being filtered.
func (c *Capture) XboxMAC() net.HardwareAddr {
	return c.xboxMAC
}

// FormatInterfaceList formats the interface list for display.
func FormatInterfaceList(interfaces []InterfaceInfo) string {
	var sb strings.Builder
	sb.WriteString("Available network interfaces:\n\n")

	for i, iface := range interfaces {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, iface.Name))
		if iface.Description != "" {
			sb.WriteString(fmt.Sprintf("     Description: %s\n", iface.Description))
		}
		if len(iface.Addresses) > 0 {
			sb.WriteString(fmt.Sprintf("     Addresses:   %s\n", strings.Join(iface.Addresses, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// DecodeEthernetFrame extracts basic info from an Ethernet frame for logging.
func DecodeEthernetFrame(frame []byte) (srcMAC, dstMAC net.HardwareAddr, etherType uint16) {
	if len(frame) < 14 {
		return nil, nil, 0
	}

	dstMAC = net.HardwareAddr(frame[0:6])
	srcMAC = net.HardwareAddr(frame[6:12])
	etherType = uint16(frame[12])<<8 | uint16(frame[13])

	return srcMAC, dstMAC, etherType
}

// EtherTypeName returns a human-readable name for common EtherTypes.
func EtherTypeName(etherType uint16) string {
	switch layers.EthernetType(etherType) {
	case layers.EthernetTypeIPv4:
		return "IPv4"
	case layers.EthernetTypeIPv6:
		return "IPv6"
	case layers.EthernetTypeARP:
		return "ARP"
	default:
		return fmt.Sprintf("0x%04X", etherType)
	}
}
