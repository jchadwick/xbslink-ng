# xbslink-ng

A lightweight, cross-platform P2P bridge for Xbox 360 System Link traffic. Connect two LANs over the internet for local multiplayer gaming without relying on centralized servers.

## Why xbslink-ng?

Existing solutions like XLink Kai route traffic through centralized servers, often hundreds of miles away. Even if your friend lives a few miles down the road, you might see 30ms+ latency due to server routing.

xbslink-ng establishes a **direct peer-to-peer connection** between two friends, giving you the lowest possible latency for your geography.

## How It Works

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              YOUR HOUSE                                         │
│  ┌──────────┐      ┌─────────────────────────────────────────┐                  │
│  │  Xbox    │      │              Your PC                    │                  │
│  │  360     │──────│  [NIC] ◄──► xbslink-ng ◄──► [UDP]       │──────────────────┤
│  │          │  L2  │       capture/inject    encap/decap     │   UDP over       │
│  └──────────┘      └─────────────────────────────────────────┘   Internet       │
└─────────────────────────────────────────────────────────────────────────────────┘
                                                                        │
                                                                        │
┌───────────────────────────────────────────────────────────────────────┼─────────┐
│                           FRIEND'S HOUSE                              │         │
│  ┌──────────┐      ┌─────────────────────────────────────────┐        │         │
│  │  Xbox    │      │            Friend's PC                  │        │         │
│  │  360     │──────│  [NIC] ◄──► xbslink-ng ◄──► [UDP]       │────────┘         │
│  │          │  L2  │       capture/inject    encap/decap     │   :31415         │
│  └──────────┘      └─────────────────────────────────────────┘   (port fwd)     │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Xbox System Link operates at Layer 2 (Ethernet). xbslink-ng:

1. Captures raw Ethernet frames from your Xbox using pcap
2. Encapsulates them in UDP packets
3. Sends them directly to your friend's IP address
4. Your friend's xbslink-ng extracts the frames and injects them into their local network
5. Their Xbox sees the traffic as if your Xbox was on the same LAN

## Requirements

- **Windows**: [Npcap](https://npcap.com/) (install with "WinPcap API-compatible mode")
- **macOS**: No additional software (uses built-in BPF)
- **Linux**: libpcap (`apt install libpcap-dev` or equivalent)

One side must be able to **port forward** a UDP port through their router.

## Installation

### Option 1: Docker (Recommended)

The easiest way to run xbslink-ng is via Docker. No installation required except Docker itself:

```bash
docker pull ghcr.io/xbslink/xbslink-ng:latest
```

See the [Docker Usage](#docker-usage) section below for examples.

### Option 2: Binary Download

Download the latest release for your platform from the [Releases](https://github.com/your-repo/xbslink-ng/releases) page.

### Option 3: Build from Source

```bash
go build -o xbslink-ng ./cmd/xbslink-ng
```

## Quick Start

### Step 1: Find your network interface

```bash
xbslink-ng interfaces
```

Note the interface name where your Xbox is connected (e.g., `Ethernet`, `en0`, `eth0`).

### Step 2: Find your Xbox's MAC address

- On Xbox 360: Settings → System → Network Settings → Configure Network → Additional Settings → Advanced Settings
- Or check your router's DHCP client list

### Step 3: Set up the connection

**Person A** (has port forwarding):

1. Forward UDP port 31415 on your router to your PC's local IP
2. Run:

```bash
xbslink-ng listen --port 31415 --interface "Ethernet" --xbox-mac 00:50:F2:XX:XX:XX
```

**Person B**:

1. Get Person A's public IP address
2. Run:

```bash
xbslink-ng connect --address <Person-A-IP>:31415 --interface "Ethernet" --xbox-mac 00:50:F2:YY:YY:YY
```

### Step 4: Play!

Once connected, start a System Link game on both Xboxes. They should see each other!

## Usage

```
xbslink-ng [command] [flags]

Commands:
  listen      Listen for incoming peer connection (requires port forwarding)
  connect     Connect to a listening peer
  interfaces  List available network interfaces

Flags for listen/connect:
  --port            UDP port (listen: port to bind, connect: optional local port)
  --address         Peer's IP:port (connect mode only)
  --interface       Network interface name (required)
  --xbox-mac        Xbox MAC address in XX:XX:XX:XX:XX:XX format (required)
  --key             Pre-shared key for authentication (strongly recommended)
  --log             Log level: error|warn|info|debug|trace (default: info)
  --stats-interval  Seconds between stats output, 0 to disable (default: 30)
```

**Security Note:** Always use `--key` with the same secret on both sides. Without it, anyone who discovers your port can inject traffic into your LAN.

## Example Output

```
$ xbslink-ng listen --port 31415 --interface "Ethernet" --xbox-mac 00:50:F2:1A:2B:3C

2024-01-15 14:30:01 [INFO]  xbslink-ng v0.1.0 starting
2024-01-15 14:30:01 [INFO]  Interface: Ethernet (192.168.1.100)
2024-01-15 14:30:01 [INFO]  Xbox MAC: 00:50:F2:1A:2B:3C
2024-01-15 14:30:01 [INFO]  Listening on UDP :31415
2024-01-15 14:30:01 [INFO]  Waiting for peer connection...
2024-01-15 14:30:05 [INFO]  Peer connected: 203.0.113.50:54321
2024-01-15 14:30:05 [INFO]  Bridge active! Forwarding packets...
2024-01-15 14:30:35 [STATS] TX: 1,247 pkts (328 KB) | RX: 1,302 pkts (351 KB) | RTT: 8ms
```

Press **Enter** at any time for instant stats.

### RTT Alerts

xbslink-ng monitors latency and warns you about potential issues:

```
2024-01-15 14:32:15 [WARN]  RTT spike: 8ms → 45ms
2024-01-15 14:32:20 [WARN]  [!] RTT 45ms exceeds Xbox 360 System Link threshold (30ms)
```

Xbox 360 System Link requires <30ms latency. If you see this warning, the Xboxes may fail to connect or disconnect during play.

## Architecture

### Wire Protocol

All UDP packets use authenticated message format (when `--key` is provided):

| Offset | Size | Field   | Description                           |
| ------ | ---- | ------- | ------------------------------------- |
| 0      | 1    | Type    | Message type (0x00-0x05)              |
| 1      | 8    | Nonce   | Monotonic counter (replay protection) |
| 9      | var  | Payload | Message-specific data                 |
| -32    | 32   | HMAC    | HMAC-SHA256 of Type+Nonce+Payload     |

When no key is provided, Nonce and HMAC fields are omitted (insecure mode).

| Type | Name      | Payload                                          |
| ---- | --------- | ------------------------------------------------ |
| 0x00 | FRAME     | Raw Ethernet frame (14-1514 bytes)               |
| 0x01 | HELLO     | Protocol version (2B) + challenge (16B)          |
| 0x02 | HELLO_ACK | Protocol version (2B) + challenge response (32B) |
| 0x03 | PING      | Timestamp in unix nanoseconds (8 bytes)          |
| 0x04 | PONG      | Echoed timestamp (8 bytes)                       |
| 0x05 | BYE       | Graceful disconnect (0 bytes)                    |

### Packet Flow

```
Xbox A                    xbslink-ng A              xbslink-ng B                    Xbox B
   │                           │                          │                           │
   │── L2 broadcast ──────────►│                          │                           │
   │                           │── UDP [FRAME] ──────────►│                           │
   │                           │                          │── L2 inject ─────────────►│
   │                           │                          │                           │
   │                           │                          │◄── L2 response ───────────│
   │                           │◄── UDP [FRAME] ──────────│                           │
   │◄── L2 inject ─────────────│                          │                           │
```

## Docker Usage

### Running with Docker

Docker containers require special flags to access host network interfaces and perform packet capture:

- `--net=host` - Access host network interfaces
- `--cap-add=NET_RAW` - Raw packet capture capability
- `--cap-add=NET_ADMIN` - Network interface control

**Listen mode example:**

```bash
docker run --rm \
  --net=host \
  --cap-add=NET_RAW \
  --cap-add=NET_ADMIN \
  ghcr.io/xbslink/xbslink-ng:latest \
  listen --port 31415 --interface eth0 --xbox-mac 00:50:F2:XX:XX:XX --key "mysecretkey"
```

**Connect mode example:**

```bash
docker run --rm \
  --net=host \
  --cap-add=NET_RAW \
  --cap-add=NET_ADMIN \
  ghcr.io/xbslink/xbslink-ng:latest \
  connect --address 1.2.3.4:31415 --interface eth0 --xbox-mac 00:50:F2:YY:YY:YY --key "mysecretkey"
```

**List network interfaces:**

```bash
docker run --rm \
  --net=host \
  --cap-add=NET_RAW \
  --cap-add=NET_ADMIN \
  ghcr.io/xbslink/xbslink-ng:latest \
  interfaces
```

### Docker Compose

For easier management, use docker-compose. See [docker-compose.yml](docker-compose.yml) for a full example.

**Listen mode:**

```yaml
version: '3.8'
services:
  xbslink-ng:
    image: ghcr.io/xbslink/xbslink-ng:latest
    container_name: xbslink-ng
    network_mode: host
    cap_add:
      - NET_RAW
      - NET_ADMIN
    command: >
      listen
      --port 31415
      --interface eth0
      --xbox-mac 00:50:F2:XX:XX:XX
      --key "mysecretkey"
    restart: unless-stopped
```

Then run with:

```bash
docker-compose up -d
```

### Home Assistant Integration

xbslink-ng can run as a Home Assistant add-on or container. The Docker image is fully compatible with Home Assistant OS.

**Using as a Home Assistant Add-on:**

A dedicated Home Assistant add-on will be published separately. Until then, you can run xbslink-ng as a regular Docker container on the same host as Home Assistant.

**Network Configuration:**

Since the container needs `--net=host`, it will share the host's network stack. Make sure your Xbox is connected to the same network as your Home Assistant instance.

**Configuration:**

You can configure xbslink-ng via environment variables or command-line arguments. For Home Assistant, create an automation or script to start/stop the container as needed.

## Troubleshooting

### "No interfaces found" or permission errors

- **Windows**: Run as Administrator, ensure Npcap is installed with WinPcap compatibility
- **macOS**: Run with `sudo`
- **Linux**: Run with `sudo` or add your user to the `pcap` group

### Xboxes don't see each other

1. Check both xbslink-ng instances show "Bridge active"
2. Verify Xbox MAC addresses are correct
3. Enable `--log debug` to see if packets are being captured/forwarded
4. Ensure both Xboxes are on the same game version

### High latency / disconnections

- Xbox 360 System Link requires <30ms RTT
- Check your internet connection
- Ensure no bandwidth-heavy applications are running
- Try switching who does port forwarding (route may be asymmetric)

## Known Limitations

### MTU and Large Frames

Xbox 360 System Link uses standard 1500-byte Ethernet frames. With xbslink-ng's
protocol overhead (1 byte type + 8 byte nonce + 32 byte HMAC = 41 bytes) plus
UDP/IP headers (28 bytes), the total packet size can reach **1569 bytes**.

This exceeds the standard internet MTU of 1500 bytes, which may cause:

- Packet fragmentation (adds latency)
- Packet drops on networks that block fragments

**If you experience connection issues or high latency:**

1. Try reducing your Xbox's MTU to 1400 in Network Settings
2. Or configure your router's MTU if possible

A future version may add compression to mitigate this.

## Building from Source

```bash
# Clone the repo
git clone https://github.com/your-repo/xbslink-ng.git
cd xbslink-ng

# Build for current platform
go build -o xbslink-ng ./cmd/xbslink-ng

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o xbslink-ng.exe ./cmd/xbslink-ng
GOOS=darwin GOARCH=amd64 go build -o xbslink-ng-mac ./cmd/xbslink-ng
GOOS=linux GOARCH=amd64 go build -o xbslink-ng-linux ./cmd/xbslink-ng
```

## Development

### Getting Started

After cloning the repository, run the setup command to install git hooks and dependencies:

```bash
make setup
```

This will:
- Install [Lefthook](https://github.com/evilmartians/lefthook) git hooks manager
- Set up pre-commit hooks (format checks, linting)
- Set up pre-push hooks (unit tests)
- Download project dependencies

### Git Hooks

This project uses [Lefthook](https://github.com/evilmartians/lefthook) to manage git hooks, similar to Husky in the Node.js ecosystem.

**Hooks are automatically enforced:**
- ✅ **pre-commit**: Runs `gofmt` and `go vet` on staged Go files
- ✅ **pre-push**: Runs all unit tests before pushing

**Manual installation** (if needed):

```bash
make install-hooks
# or directly:
lefthook install
```

**Run hooks manually:**

```bash
lefthook run pre-commit   # Run pre-commit checks
lefthook run pre-push     # Run pre-push checks
```

**Bypass hooks** (not recommended):

```bash
git commit --no-verify
git push --no-verify
# or set LEFTHOOK=0:
LEFTHOOK=0 git commit -m "skip hooks"
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage
make test-cover

# Run integration tests
make test-int

# Run benchmarks
make test-bench

# Run all tests (unit + integration + benchmarks)
make test-all
```

### Code Quality

**Format code:**

```bash
make fmt
```

**Run linters:**

```bash
make lint
```

**CI checks locally:**

```bash
make ci
```

### Available Make Targets

Run `make help` to see all available targets:

```bash
make help
```

## License

MIT License - See [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by the original [XBSlink](https://www.seuffert.biz/xbslink/) project
- [XLink Kai](https://www.teamxlink.co.uk/) for pioneering System Link tunneling
- [l2tunnel](https://github.com/mborgerson/l2tunnel) for the simple reference implementation
