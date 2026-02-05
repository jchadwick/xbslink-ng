# xbslink-ng Implementation Plan

## Overview

Build a cross-platform (Windows, macOS, Linux) P2P System Link bridge in Go. Single binary, minimal dependencies, direct peer-to-peer UDP connection.

## Architecture

```
xbslink-ng/
├── cmd/
│   └── xbslink-ng/
│       └── main.go           # CLI entry point, flag parsing, command dispatch
├── internal/
│   ├── capture/
│   │   └── capture.go        # pcap operations: open, filter, capture, inject
│   ├── transport/
│   │   └── transport.go      # UDP socket: listen, connect, send, recv
│   ├── protocol/
│   │   └── protocol.go       # Wire format: message types, serialization
│   ├── bridge/
│   │   └── bridge.go         # Main loop: goroutine coordination, channels
│   └── logging/
│       └── logging.go        # Leveled logging with colors and timestamps
├── go.mod
├── go.sum
└── README.md
```

## Dependencies

```go
require (
    github.com/google/gopacket v1.1.19
)
```

## Implementation Phases

### Phase 1: Project Scaffold

**Files**: `go.mod`, directory structure, `main.go` skeleton

**Tasks**:

1. Initialize Go module: `go mod init github.com/xbslink/xbslink-ng`
2. Create directory structure
3. Create minimal `main.go` that prints version and exits
4. Verify `go build` works

**Acceptance**: `go build ./cmd/xbslink-ng` produces a binary

---

### Phase 2: Logging Module

**File**: `internal/logging/logging.go`

**Tasks**:

1. Define log levels: ERROR, WARN, INFO, DEBUG, TRACE
2. Create Logger struct with configurable level
3. Implement level-specific methods: Error(), Warn(), Info(), Debug(), Trace()
4. Add timestamp formatting: `2006-01-02 15:04:05`
5. Add colored output for terminals (optional, detect TTY)
6. Add special format for STATS lines

**Interface**:

```go
type Logger struct {
    level Level
    // ...
}

func NewLogger(level Level) *Logger
func (l *Logger) Error(format string, args ...interface{})
func (l *Logger) Warn(format string, args ...interface{})
func (l *Logger) Info(format string, args ...interface{})
func (l *Logger) Debug(format string, args ...interface{})
func (l *Logger) Trace(format string, args ...interface{})
func (l *Logger) Stats(format string, args ...interface{})
func ParseLevel(s string) (Level, error)
```

**Acceptance**: Logger outputs correctly formatted messages at appropriate levels

---

### Phase 3: CLI Parsing

**File**: `cmd/xbslink-ng/main.go`

**Tasks**:

1. Implement subcommands: `listen`, `connect`, `interfaces`
2. Parse flags:
   - `--port` (uint16)
   - `--address` (string, IP:port format)
   - `--interface` (string)
   - `--xbox-mac` (string, validate XX:XX:XX:XX:XX:XX format)
   - `--log` (string, default "info")
   - `--stats-interval` (int, default 30)
3. Validate required flags per subcommand
4. Parse MAC address string to `[6]byte`
5. Display help text

**Acceptance**:

- `xbslink-ng interfaces` runs (even if not implemented yet)
- `xbslink-ng listen --port 31415 --interface eth0 --xbox-mac 00:11:22:33:44:55` parses correctly
- Missing required flags show helpful error

---

### Phase 4: Protocol Module

**File**: `internal/protocol/protocol.go`

**Tasks**:

1. Define message type constants:
   ```go
   const (
       MsgFrame    = 0x00
       MsgHello    = 0x01
       MsgHelloAck = 0x02
       MsgPing     = 0x03
       MsgPong     = 0x04
       MsgBye      = 0x05
   )
   ```
2. Define protocol version: `const ProtocolVersion = 1`
3. Implement serialization functions:
   ```go
   func EncodeFrame(frame []byte) []byte
   func EncodeHello() []byte
   func EncodeHelloAck() []byte
   func EncodePing(timestamp int64) []byte
   func EncodePong(timestamp int64) []byte
   func EncodeBye() []byte
   ```
4. Implement parsing function:
   ```go
   func ParseMessage(data []byte) (msgType byte, payload []byte, err error)
   ```

**Acceptance**: Round-trip encode/decode works for all message types

---

### Phase 5: Transport Module

**File**: `internal/transport/transport.go`

**Tasks**:

1. Define Transport interface/struct:
   ```go
   type Transport struct {
       conn     *net.UDPConn
       peerAddr *net.UDPAddr
       mode     Mode // Listen or Connect
   }
   ```
2. Implement Listen mode:
   - Bind to specified port
   - Wait for first HELLO message
   - Store peer address
   - Send HELLO_ACK
3. Implement Connect mode:
   - Dial to peer address
   - Send HELLO
   - Wait for HELLO_ACK with timeout
4. Implement Send/Recv methods:
   ```go
   func (t *Transport) Send(data []byte) error
   func (t *Transport) Recv() ([]byte, error)
   ```
5. Implement Close() for graceful shutdown (send BYE)

**Acceptance**: Two instances can connect and exchange messages

---

### Phase 6: Capture Module

**File**: `internal/capture/capture.go`

**Tasks**:

1. Implement interface listing:
   ```go
   func ListInterfaces() ([]InterfaceInfo, error)
   ```
2. Implement Capture struct:
   ```go
   type Capture struct {
       handle  *pcap.Handle
       xboxMAC net.HardwareAddr
   }
   ```
3. Implement Open():
   - Open pcap handle on interface
   - Set promiscuous mode
   - Set BPF filter: `ether src XX:XX:XX:XX:XX:XX`
   - Set reasonable timeout (10ms)
4. Implement capture loop:
   ```go
   func (c *Capture) ReadPacket() ([]byte, error)
   ```
5. Implement injection:
   ```go
   func (c *Capture) WritePacket(frame []byte) error
   ```
6. Handle platform differences (Windows needs different buffer sizes, etc.)

**Acceptance**:

- `interfaces` command lists NICs correctly
- Can capture packets from specified MAC
- Can inject packets that appear on the network

---

### Phase 7: Bridge Module

**File**: `internal/bridge/bridge.go`

**Tasks**:

1. Define Bridge struct:
   ```go
   type Bridge struct {
       capture   *capture.Capture
       transport *transport.Transport
       logger    *logging.Logger
       stats     *Stats
       // channels, etc.
   }
   ```
2. Implement main coordination loop:
   - Goroutine 1: pcap capture → channel → UDP send
   - Goroutine 2: UDP recv → parse → dispatch (frames to inject, control to handle)
   - Goroutine 3: Ping/pong loop (every 5s)
   - Goroutine 4: Stats output (every N seconds)
   - Goroutine 5: Stdin monitor (for on-demand stats)
3. Implement graceful shutdown on SIGINT/SIGTERM
4. Implement connection state machine:
   - CONNECTING → CONNECTED → DISCONNECTED
   - Handle HELLO/HELLO_ACK handshake
   - Track missed pings (3 = disconnect)

**Acceptance**: Full bridge works - frames flow bidirectionally

---

### Phase 8: Stats & Alerts

**File**: `internal/bridge/bridge.go` (or separate `stats.go`)

**Tasks**:

1. Define Stats struct:
   ```go
   type Stats struct {
       txPackets  uint64
       txBytes    uint64
       rxPackets  uint64
       rxBytes    uint64
       rttCurrent time.Duration
       rttAvg     time.Duration
       rttSamples []time.Duration
   }
   ```
2. Implement RTT tracking from PING/PONG
3. Implement periodic stats output (configurable interval)
4. Implement on-demand stats (stdin Enter key)
5. Implement RTT alerts:
   - Warn on >50% spike from average
   - Persistent warning when >30ms (Xbox 360 threshold)
   - Recovery message when RTT drops back down
6. Format stats nicely:
   ```
   [STATS] TX: 1,247 pkts (328 KB) | RX: 1,302 pkts (351 KB) | RTT: 8ms
   ```

**Acceptance**: Stats display correctly, alerts fire appropriately

---

### Phase 9: Integration & Testing

**Tasks**:

1. Wire everything together in main.go
2. Test `interfaces` command on all platforms
3. Test listen/connect handshake
4. Test packet forwarding (use two VMs or machines)
5. Test stats and alerts
6. Test graceful shutdown (Ctrl+C)
7. Test error cases:
   - Invalid interface
   - Port already in use
   - Peer unreachable
   - Connection timeout

**Acceptance**: End-to-end test passes - two Xboxes can see each other via System Link

---

### Phase 10: Cross-Compilation & Release

**Tasks**:

1. Test cross-compilation:
   ```bash
   GOOS=windows GOARCH=amd64 go build -o xbslink-ng.exe ./cmd/xbslink-ng
   GOOS=darwin GOARCH=amd64 go build -o xbslink-ng-darwin-amd64 ./cmd/xbslink-ng
   GOOS=darwin GOARCH=arm64 go build -o xbslink-ng-darwin-arm64 ./cmd/xbslink-ng
   GOOS=linux GOARCH=amd64 go build -o xbslink-ng-linux-amd64 ./cmd/xbslink-ng
   ```
2. Test Windows binary (requires npcap)
3. Test macOS binary (may need code signing for distribution)
4. Test Linux binary
5. Add version flag: `--version`
6. Strip debug symbols for smaller binaries: `-ldflags="-s -w"`

**Acceptance**: Binaries work on all three platforms

---

## Wire Protocol Specification

### Message Format

```
┌──────────┬─────────────────────────────────────────┐
│ Type (1B)│ Payload (variable)                      │
└──────────┴─────────────────────────────────────────┘
```

### Message Types

| Type | Name      | Payload              | Description               |
| ---- | --------- | -------------------- | ------------------------- |
| 0x00 | FRAME     | Raw Ethernet frame   | L2 frame to forward       |
| 0x01 | HELLO     | uint16 version (BE)  | Initiate connection       |
| 0x02 | HELLO_ACK | uint16 version (BE)  | Accept connection         |
| 0x03 | PING      | int64 timestamp (BE) | Latency probe (unix nano) |
| 0x04 | PONG      | int64 timestamp (BE) | Latency response (echo)   |
| 0x05 | BYE       | (empty)              | Graceful disconnect       |

### Connection State Machine

```
LISTEN MODE:                         CONNECT MODE:

┌─────────────┐                      ┌─────────────┐
│  LISTENING  │                      │ CONNECTING  │
└──────┬──────┘                      └──────┬──────┘
       │ recv HELLO                         │ send HELLO
       │ send HELLO_ACK                     │
       ▼                                    ▼
       │                             ┌─────────────┐
       │                             │  wait for   │
       │                             │  HELLO_ACK  │
       │                             └──────┬──────┘
       │                                    │ recv HELLO_ACK (timeout: 5s)
       ▼                                    ▼
┌─────────────────────────────────────────────────┐
│                   CONNECTED                      │
│  - Forward FRAME messages bidirectionally       │
│  - Send PING every 5s                           │
│  - Expect PONG within 2s                        │
│  - 3 missed PONGs → DISCONNECTED                │
└─────────────────────────────────────────────────┘
```

---

## Error Handling Strategy

| Error                    | Handling                                      |
| ------------------------ | --------------------------------------------- |
| Interface not found      | Log ERROR, exit 1                             |
| Permission denied (pcap) | Log ERROR with platform-specific help, exit 1 |
| Port already in use      | Log ERROR, exit 1                             |
| Invalid MAC format       | Log ERROR, exit 1                             |
| Peer unreachable         | Log WARN, retry connection                    |
| Connection timeout       | Log ERROR, exit 1                             |
| Peer disconnected        | Log INFO, exit 0                              |
| Malformed packet         | Log DEBUG, ignore packet                      |
| Injection failed         | Log WARN, continue                            |

---

## Performance Considerations

1. **Minimize allocations in hot path**: Pre-allocate buffers for packet capture/send
2. **Use buffered channels**: Prevent goroutine blocking
3. **Batch stats updates**: Use atomic operations, compute display values periodically
4. **pcap timeout**: Use 10ms timeout to balance latency vs CPU usage
5. **UDP buffer sizes**: Set appropriate SO_RCVBUF/SO_SNDBUF for high throughput

---

## Phase 11: GitHub Actions CI/CD

**Files**: `.github/workflows/ci.yml`, `.github/workflows/release.yml`

### CI Workflow (ci.yml)

**Triggers**: Pull requests, pushes to any branch

**Jobs**:

1. **lint**: Run `go vet` and `staticcheck`
2. **test**: Run `go test ./...`
3. **build**: Verify cross-compilation works for all platforms

### Release Workflow (release.yml)

**Triggers**: Push to `main` branch (with version tag) or manual dispatch

**Jobs**:

1. **build**: Matrix build for all platforms:

   - `windows/amd64` → `xbslink-ng-windows-amd64.zip`
   - `darwin/amd64` → `xbslink-ng-darwin-amd64.tar.gz`
   - `darwin/arm64` → `xbslink-ng-darwin-arm64.tar.gz`
   - `linux/amd64` → `xbslink-ng-linux-amd64.tar.gz`

2. **release**: Create GitHub Release with all archives attached

**Build Matrix**:

```yaml
strategy:
  matrix:
    include:
      - os: ubuntu-latest
        goos: linux
        goarch: amd64
        ext: ""
        archive: tar.gz
      - os: ubuntu-latest
        goos: windows
        goarch: amd64
        ext: ".exe"
        archive: zip
      - os: macos-latest
        goos: darwin
        goarch: amd64
        ext: ""
        archive: tar.gz
      - os: macos-latest
        goos: darwin
        goarch: arm64
        ext: ""
        archive: tar.gz
```

**Build Commands**:

```bash
# Build with version and stripped symbols
CGO_ENABLED=1 GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
  go build -ldflags="-s -w -X main.Version=${{ github.ref_name }}" \
  -o xbslink-ng${{ matrix.ext }} ./cmd/xbslink-ng

# Create archive
# For tar.gz:
tar -czvf xbslink-ng-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz xbslink-ng${{ matrix.ext }} README.md

# For zip (Windows):
zip xbslink-ng-${{ matrix.goos }}-${{ matrix.goarch }}.zip xbslink-ng${{ matrix.ext }} README.md
```

**CGO Note**:

- gopacket/pcap requires CGO
- Linux build: Install `libpcap-dev`
- macOS build: pcap is built-in
- Windows cross-compile: Complex - may need separate Windows runner or use `xgo`

**Alternative (simpler)**: Use separate runners per OS:

```yaml
strategy:
  matrix:
    include:
      - os: ubuntu-latest
        goos: linux
        goarch: amd64
      - os: windows-latest
        goos: windows
        goarch: amd64
      - os: macos-latest
        goos: darwin
        goarch: amd64
      - os: macos-latest
        goos: darwin
        goarch: arm64
```

---

## Future Enhancements (Out of Scope for v1)

- [ ] NAT traversal (UDP hole punching, STUN)
- [ ] Config file support
- [ ] Multiple peer mesh networking
- [ ] GUI wrapper
- [ ] Auto-discovery of Xbox MAC
- [ ] Packet compression (unlikely to help - frames are small)
- [ ] Encryption (not needed for trusted friends)
