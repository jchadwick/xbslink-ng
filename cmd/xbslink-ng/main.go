// xbslink-ng is a P2P Xbox System Link bridge.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/xbslink/xbslink-ng/internal/bridge"
	"github.com/xbslink/xbslink-ng/internal/capture"
	"github.com/xbslink/xbslink-ng/internal/discovery"
	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
	"github.com/xbslink/xbslink-ng/internal/transport"
)

// Version is set at build time via -ldflags.
var Version = "dev"

const (
	defaultPort          = 31415
	defaultStatsInterval = 30
	defaultLogLevel      = "info"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "listen":
		runListen(args)
	case "connect":
		runConnect(args)
	case "interfaces":
		runInterfaces()
	case "version", "--version", "-v":
		fmt.Printf("xbslink-ng %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`xbslink-ng - P2P Xbox System Link Bridge

Usage:
  xbslink-ng <command> [flags]

Commands:
  listen      Listen for incoming peer connection (requires port forwarding)
  connect     Connect to a listening peer
  interfaces  List available network interfaces
  version     Print version information

Flags for listen/connect:
  --port            UDP port (listen: port to bind, connect: optional local port)
  --address         Peer's IP:port (connect mode only, required)
  --interface       Network interface name (required)
  --xbox-mac        Xbox MAC address (auto-detected if omitted)
  --key             Pre-shared key for authentication (strongly recommended)
  --log             Log level: error|warn|info|debug|trace (default: info)
  --stats-interval  Seconds between stats output, 0 to disable (default: 30)

Examples:
  # List network interfaces
  xbslink-ng interfaces

  # Listen for incoming connection (port forward UDP 31415)
  xbslink-ng listen --port 31415 --interface "Ethernet" --xbox-mac 00:50:F2:1A:2B:3C

  # Connect to a listening peer
  xbslink-ng connect --address 203.0.113.50:31415 --interface "Ethernet" --xbox-mac 00:50:F2:4D:5E:6F

  # With authentication (recommended)
  xbslink-ng listen --port 31415 --interface "Ethernet" --xbox-mac 00:50:F2:1A:2B:3C --key "mysecretkey"

Press Enter at any time to see current statistics.
`)
}

func runInterfaces() {
	// Check for Npcap on Windows before listing
	if err := capture.CheckNpcapInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s\n", err, capture.NpcapInstallHelp())
		os.Exit(1)
	}

	interfaces, err := capture.ListInterfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing interfaces: %v\n", err)
		os.Exit(1)
	}

	if len(interfaces) == 0 {
		fmt.Println("No network interfaces found.")
		fmt.Println()
		fmt.Println(capture.NpcapInstallHelp())
		os.Exit(1)
	}

	fmt.Print(capture.FormatInterfaceList(interfaces))
}

func runListen(args []string) {
	fs := flag.NewFlagSet("listen", flag.ExitOnError)

	port := fs.Uint("port", defaultPort, "UDP port to listen on")
	ifaceName := fs.String("interface", "", "Network interface name (required)")
	xboxMAC := fs.String("xbox-mac", "", "Xbox MAC address (auto-detected if omitted)")
	key := fs.String("key", "", "Pre-shared key for authentication")
	logLevel := fs.String("log", defaultLogLevel, "Log level: error|warn|info|debug|trace")
	statsInterval := fs.Uint("stats-interval", defaultStatsInterval, "Seconds between stats output (0 to disable)")

	fs.Parse(args)

	// Validate required flags
	if *ifaceName == "" {
		fmt.Fprintln(os.Stderr, "Error: --interface is required")
		fmt.Fprintln(os.Stderr, "\nRun 'xbslink-ng interfaces' to list available interfaces.")
		os.Exit(1)
	}
	if *port == 0 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "Error: --port must be between 1 and 65535")
		os.Exit(1)
	}

	runBridge(transport.ModeListen, uint16(*port), "", *ifaceName, *xboxMAC, *key, *logLevel, time.Duration(*statsInterval)*time.Second)
}

func runConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)

	address := fs.String("address", "", "Peer address in IP:port format (required)")
	port := fs.Uint("port", 0, "Local UDP port (0 = auto-assign)")
	ifaceName := fs.String("interface", "", "Network interface name (required)")
	xboxMAC := fs.String("xbox-mac", "", "Xbox MAC address (auto-detected if omitted)")
	key := fs.String("key", "", "Pre-shared key for authentication")
	logLevel := fs.String("log", defaultLogLevel, "Log level: error|warn|info|debug|trace")
	statsInterval := fs.Uint("stats-interval", defaultStatsInterval, "Seconds between stats output (0 to disable)")

	fs.Parse(args)

	// Validate required flags
	if *address == "" {
		fmt.Fprintln(os.Stderr, "Error: --address is required")
		os.Exit(1)
	}
	if *ifaceName == "" {
		fmt.Fprintln(os.Stderr, "Error: --interface is required")
		fmt.Fprintln(os.Stderr, "\nRun 'xbslink-ng interfaces' to list available interfaces.")
		os.Exit(1)
	}

	// Validate address format
	if !strings.Contains(*address, ":") {
		fmt.Fprintln(os.Stderr, "Error: --address must be in IP:port format (e.g., 192.168.1.100:31415)")
		os.Exit(1)
	}

	runBridge(transport.ModeConnect, uint16(*port), *address, *ifaceName, *xboxMAC, *key, *logLevel, time.Duration(*statsInterval)*time.Second)
}

func runBridge(mode transport.Mode, port uint16, peerAddr, ifaceName, xboxMACStr, key, logLevelStr string, statsInterval time.Duration) {
	// Parse log level
	level, err := logging.ParseLevel(logLevelStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logger := logging.NewLogger(level)

	// Print banner
	logger.Info("xbslink-ng %s starting", Version)

	// Check Npcap on Windows
	if runtime.GOOS == "windows" {
		if err := capture.CheckNpcapInstalled(); err != nil {
			logger.Error("Npcap not found: %v", err)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, capture.NpcapInstallHelp())
			os.Exit(1)
		}
	}

	// Warn about insecure mode
	var keyBytes []byte
	if key == "" {
		logger.Warn("*************************************************************")
		logger.Warn("* WARNING: Running without --key (insecure mode)            *")
		logger.Warn("* Anyone who discovers your port can inject traffic into    *")
		logger.Warn("* your LAN. Use --key with a shared secret for security.    *")
		logger.Warn("*************************************************************")
	} else {
		keyBytes = []byte(key)
		logger.Info("Authentication enabled (HMAC-SHA256)")
	}

	// Get Xbox MAC address - either from flag or via auto-discovery
	var mac net.HardwareAddr
	if xboxMACStr != "" {
		// Parse provided MAC address
		var err error
		mac, err = capture.ParseMAC(xboxMACStr)
		if err != nil {
			logger.Error("Invalid Xbox MAC address: %v", err)
			os.Exit(1)
		}
	} else {
		// Auto-discover Xbox MAC by listening for System Link traffic
		logger.Info("No --xbox-mac specified, listening for System Link traffic (UDP port 3074)...")
		logger.Info("Start a System Link game on your Xbox to detect it automatically.")

		// Create a cancellable context for discovery
		ctx, cancel := context.WithCancel(context.Background())

		// Handle Ctrl+C during discovery
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		result, err := discovery.Discover(ctx, discovery.Config{
			Interface: ifaceName,
			Logger:    logger,
		})

		signal.Stop(sigCh)

		if err != nil {
			if err == discovery.ErrDiscoveryCancelled {
				logger.Info("Discovery cancelled")
				os.Exit(0)
			}
			logger.Error("Discovery failed: %v", err)
			os.Exit(1)
		}

		mac = result.MAC
		logger.Info("Found Xbox: %s", mac)
	}

	// Find and display interface info
	iface, err := capture.FindInterface(ifaceName)
	if err != nil {
		logger.Error("Interface not found: %v", err)
		fmt.Fprintln(os.Stderr, "\nRun 'xbslink-ng interfaces' to list available interfaces.")
		os.Exit(1)
	}

	addrStr := "no IP"
	if len(iface.Addresses) > 0 {
		addrStr = iface.Addresses[0]
	}
	logger.Info("Interface: %s (%s)", iface.Name, addrStr)
	logger.Info("Xbox MAC: %s", mac)

	// Create protocol codec
	codec := protocol.NewCodec(keyBytes)

	// Create capture
	cap, err := capture.New(capture.Config{
		Interface: ifaceName,
		XboxMAC:   mac,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("Failed to open capture: %v", err)
		os.Exit(1)
	}

	// Create transport
	trans, err := transport.New(transport.Config{
		Mode:      mode,
		LocalPort: port,
		PeerAddr:  peerAddr,
		Codec:     codec,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("Failed to create transport: %v", err)
		cap.Close()
		os.Exit(1)
	}

	// Create and run bridge
	br, err := bridge.New(bridge.Config{
		Capture:       cap,
		Transport:     trans,
		Codec:         codec,
		Logger:        logger,
		Mode:          mode,
		StatsInterval: statsInterval,
	})
	if err != nil {
		logger.Error("Failed to create bridge: %v", err)
		trans.Close()
		cap.Close()
		os.Exit(1)
	}

	// Run until shutdown
	ctx := context.Background()
	if err := br.Run(ctx); err != nil {
		logger.Error("Bridge error: %v", err)
		os.Exit(1)
	}
}
