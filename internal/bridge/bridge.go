// Package bridge coordinates packet capture, transport, and statistics.
package bridge

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/xbslink/xbslink-ng/internal/capture"
	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
	"github.com/xbslink/xbslink-ng/internal/transport"
)

// Configuration constants.
const (
	// PingInterval is how often to send ping messages.
	PingInterval = 5 * time.Second
	// PongTimeout is how long to wait for a pong response.
	PongTimeout = 2 * time.Second
	// MaxMissedPongs is the number of missed pongs before disconnect.
	MaxMissedPongs = 3
	// RTTAlertThreshold is the RTT above which we warn users.
	RTTAlertThreshold = 30 * time.Millisecond
	// RTTSpikeThreshold is the percentage increase to trigger a spike warning.
	RTTSpikeThreshold = 0.5 // 50%
	// ChannelBufferSize is the buffer size for internal channels.
	ChannelBufferSize = 256
)

// State represents the bridge connection state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "DISCONNECTED"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	default:
		return "UNKNOWN"
	}
}

// Stats holds bridge statistics.
type Stats struct {
	TxPackets  uint64
	TxBytes    uint64
	RxPackets  uint64
	RxBytes    uint64
	RTTCurrent time.Duration
	RTTAvg     time.Duration

	// Internal tracking
	rttSamples []time.Duration
	rttSum     time.Duration
	lastRTT    time.Duration
	rttMu      sync.RWMutex
}

// AddRTTSample adds a new RTT sample.
func (s *Stats) AddRTTSample(rtt time.Duration) {
	s.rttMu.Lock()
	defer s.rttMu.Unlock()

	s.RTTCurrent = rtt
	s.rttSamples = append(s.rttSamples, rtt)
	s.rttSum += rtt

	// Keep only last 20 samples for averaging
	if len(s.rttSamples) > 20 {
		s.rttSum -= s.rttSamples[0]
		s.rttSamples = s.rttSamples[1:]
	}

	s.RTTAvg = s.rttSum / time.Duration(len(s.rttSamples))
}

// CheckRTTSpike checks if the current RTT is a significant spike.
func (s *Stats) CheckRTTSpike() (bool, time.Duration, time.Duration) {
	s.rttMu.RLock()
	defer s.rttMu.RUnlock()

	if len(s.rttSamples) < 2 || s.lastRTT == 0 {
		return false, 0, 0
	}

	current := s.RTTCurrent
	previous := s.lastRTT

	if previous > 0 && float64(current-previous) > float64(previous)*RTTSpikeThreshold {
		return true, previous, current
	}

	return false, 0, 0
}

// SetLastRTT stores the previous RTT for spike detection.
func (s *Stats) SetLastRTT(rtt time.Duration) {
	s.rttMu.Lock()
	defer s.rttMu.Unlock()
	s.lastRTT = rtt
}

// GetRTTCurrent returns the current RTT.
func (s *Stats) GetRTTCurrent() time.Duration {
	s.rttMu.RLock()
	defer s.rttMu.RUnlock()
	return s.RTTCurrent
}

// Bridge coordinates all components for the xbslink-ng tunnel.
type Bridge struct {
	capture   *capture.Capture
	captureMu sync.RWMutex // protects capture field
	transport *transport.Transport
	codec     *protocol.Codec
	logger    *logging.Logger
	stats     *Stats

	mode          transport.Mode
	statsInterval time.Duration

	state   State
	stateMu sync.RWMutex

	// Channels for goroutine communication
	framesToSend   chan []byte
	framesToInject chan []byte
	done           chan struct{}

	// Ping tracking
	pendingPing int64 // timestamp of pending ping (0 if none)
	missedPongs int32 // counter for missed pongs
	pingMu      sync.Mutex

	// For stdin monitoring
	stdinCh chan struct{}

	// For triggering shutdown from within the bridge
	cancelFunc context.CancelFunc

	// For capture lifecycle management
	captureReady chan struct{} // closed when capture is set
}

// Config holds bridge configuration.
type Config struct {
	Capture       *capture.Capture // Optional: can be nil and set later via SetCapture()
	Transport     *transport.Transport
	Codec         *protocol.Codec
	Logger        *logging.Logger
	Mode          transport.Mode
	StatsInterval time.Duration // 0 to disable periodic stats
}

// New creates a new Bridge instance.
func New(cfg Config) (*Bridge, error) {
	if cfg.Transport == nil {
		return nil, fmt.Errorf("transport is required")
	}
	if cfg.Codec == nil {
		return nil, fmt.Errorf("codec is required")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	b := &Bridge{
		capture:        cfg.Capture,
		transport:      cfg.Transport,
		codec:          cfg.Codec,
		logger:         cfg.Logger,
		stats:          &Stats{},
		mode:           cfg.Mode,
		statsInterval:  cfg.StatsInterval,
		state:          StateDisconnected,
		framesToSend:   make(chan []byte, ChannelBufferSize),
		framesToInject: make(chan []byte, ChannelBufferSize),
		done:           make(chan struct{}),
		stdinCh:        make(chan struct{}),
		captureReady:   make(chan struct{}),
	}

	// If capture is provided initially, mark it as ready
	if cfg.Capture != nil {
		close(b.captureReady)
	}

	return b, nil
}

// SetCapture sets the capture after bridge initialization.
// This allows starting the bridge without capture and adding it later.
// Can only be called once, before or during Run().
func (b *Bridge) SetCapture(cap *capture.Capture) error {
	b.captureMu.Lock()
	defer b.captureMu.Unlock()

	if b.capture != nil {
		return fmt.Errorf("capture already set")
	}

	b.capture = cap
	close(b.captureReady) // signal that capture is now available
	b.logger.Info("Capture activated, now forwarding Xbox packets")
	return nil
}

// HasCapture returns true if capture is set.
func (b *Bridge) HasCapture() bool {
	b.captureMu.RLock()
	defer b.captureMu.RUnlock()
	return b.capture != nil
}

// Run starts the bridge and blocks until shutdown.
func (b *Bridge) Run(ctx context.Context) error {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Store cancel func so other methods can trigger shutdown
	b.cancelFunc = cancel

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			b.logger.Info("Received signal %v, shutting down...", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	// Establish connection based on mode
	b.setState(StateConnecting)

	var err error
	if b.mode == transport.ModeListen {
		err = b.transport.WaitForPeer(ctx)
	} else {
		err = b.transport.Connect(ctx)
	}

	if err != nil {
		if ctx.Err() != nil {
			return nil // Graceful shutdown
		}
		return fmt.Errorf("connection failed: %w", err)
	}

	b.setState(StateConnected)
	b.logger.Info("Bridge active! Forwarding packets...")

	// Start all goroutines
	var wg sync.WaitGroup

	// Goroutine 1: pcap capture -> channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.captureLoop(ctx)
	}()

	// Goroutine 2: channel -> UDP send
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.sendLoop(ctx)
	}()

	// Goroutine 3: UDP recv -> parse -> dispatch
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.recvLoop(ctx)
	}()

	// Goroutine 4: channel -> pcap inject
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.injectLoop(ctx)
	}()

	// Goroutine 5: Ping/pong loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.pingLoop(ctx)
	}()

	// Goroutine 6: Stats output
	if b.statsInterval > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.statsLoop(ctx)
		}()
	}

	// Goroutine 7: Stdin monitor for on-demand stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.stdinLoop(ctx)
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Send BYE for graceful disconnect
	b.logger.Debug("Sending BYE to peer")
	if err := b.transport.SendBye(); err != nil {
		b.logger.Debug("Failed to send BYE: %v", err)
	}

	// Close resources
	close(b.done)
	b.transport.Close()

	b.captureMu.RLock()
	if b.capture != nil {
		b.capture.Close()
	}
	b.captureMu.RUnlock()

	// Wait for goroutines to finish
	wg.Wait()

	b.setState(StateDisconnected)
	b.logger.Info("Bridge stopped")

	return nil
}

// setState updates the connection state.
func (b *Bridge) setState(state State) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.state = state
}

// captureLoop reads packets from pcap and sends them to the send channel.
func (b *Bridge) captureLoop(ctx context.Context) {
	b.logger.Debug("Capture loop started")
	defer b.logger.Debug("Capture loop stopped")

	// Wait for capture to be ready
	select {
	case <-ctx.Done():
		b.logger.Debug("Capture loop cancelled before capture ready")
		return
	case <-b.captureReady:
		// Capture is now available, proceed
	}

	b.logger.Debug("Capture is ready, beginning packet capture")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.captureMu.RLock()
		cap := b.capture
		b.captureMu.RUnlock()

		if cap == nil {
			// Capture was removed (shouldn't happen in normal flow)
			b.logger.Warn("Capture is nil, stopping capture loop")
			return
		}

		frame, err := cap.ReadPacket()
		if err != nil {
			b.logger.Warn("Capture error: %v", err)
			continue
		}

		if frame == nil {
			continue // No packet available (timeout)
		}

		// Log at trace level
		if b.logger.GetLevel() >= logging.LevelTrace {
			srcMAC, dstMAC, etherType := capture.DecodeEthernetFrame(frame)
			b.logger.Trace("Captured frame: %s -> %s (%s, %d bytes)",
				srcMAC, dstMAC, capture.EtherTypeName(etherType), len(frame))
		}

		// Send to channel (non-blocking with drop on full)
		select {
		case b.framesToSend <- frame:
		default:
			b.logger.Debug("Frame send channel full, dropping packet")
		}
	}
}

// sendLoop reads frames from channel and sends them over UDP.
func (b *Bridge) sendLoop(ctx context.Context) {
	b.logger.Debug("Send loop started")
	defer b.logger.Debug("Send loop stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-b.framesToSend:
			encoded, err := b.codec.EncodeFrame(frame)
			if err != nil {
				b.logger.Debug("Failed to encode frame: %v", err)
				continue
			}

			if err := b.transport.Send(encoded); err != nil {
				b.logger.Warn("Failed to send frame: %v", err)
				continue
			}

			// Update stats
			atomic.AddUint64(&b.stats.TxPackets, 1)
			atomic.AddUint64(&b.stats.TxBytes, uint64(len(frame)))
		}
	}
}

// recvLoop reads from UDP and dispatches messages.
func (b *Bridge) recvLoop(ctx context.Context) {
	b.logger.Debug("Recv loop started")
	defer b.logger.Debug("Recv loop stopped")

	buf := make([]byte, 65536)
	peerAddr := b.transport.PeerAddr()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set read deadline
		b.transport.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, addr, err := b.transport.Recv(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			b.logger.Warn("Recv error: %v", err)
			continue
		}

		// Verify sender (ignore packets from unexpected sources)
		if peerAddr != nil && !addrEqual(addr, peerAddr) {
			b.logger.Debug("Ignoring packet from unexpected source: %s", addr)
			continue
		}

		// Decode message
		msg, err := b.codec.Decode(buf[:n])
		if err != nil {
			b.logger.Debug("Failed to decode message: %v", err)
			continue
		}

		// Dispatch based on message type
		switch msg.Type {
		case protocol.MsgFrame:
			b.handleFrame(msg.Frame)
		case protocol.MsgPing:
			b.handlePing(msg.Timestamp)
		case protocol.MsgPong:
			b.handlePong(msg.Timestamp)
		case protocol.MsgBye:
			b.handleBye()
		default:
			b.logger.Debug("Unexpected message type: %s", protocol.MessageTypeName(msg.Type))
		}
	}
}

// handleFrame processes a received frame.
func (b *Bridge) handleFrame(frame []byte) {
	// Log at trace level
	if b.logger.GetLevel() >= logging.LevelTrace {
		srcMAC, dstMAC, etherType := capture.DecodeEthernetFrame(frame)
		b.logger.Trace("Received frame: %s -> %s (%s, %d bytes)",
			srcMAC, dstMAC, capture.EtherTypeName(etherType), len(frame))
	}

	// Update stats
	atomic.AddUint64(&b.stats.RxPackets, 1)
	atomic.AddUint64(&b.stats.RxBytes, uint64(len(frame)))

	// Send to inject channel (non-blocking)
	select {
	case b.framesToInject <- frame:
	default:
		b.logger.Debug("Frame inject channel full, dropping packet")
	}
}

// handlePing responds to a ping message.
func (b *Bridge) handlePing(timestamp int64) {
	b.logger.Trace("Received PING (ts=%d)", timestamp)

	pong := b.codec.EncodePong(timestamp)
	if err := b.transport.Send(pong); err != nil {
		b.logger.Debug("Failed to send PONG: %v", err)
	}
}

// handlePong processes a pong response.
func (b *Bridge) handlePong(timestamp int64) {
	b.pingMu.Lock()
	defer b.pingMu.Unlock()

	if b.pendingPing == 0 {
		b.logger.Debug("Received unexpected PONG")
		return
	}

	if timestamp != b.pendingPing {
		b.logger.Debug("PONG timestamp mismatch: expected %d, got %d", b.pendingPing, timestamp)
		return
	}

	// Calculate RTT
	rtt := time.Duration(time.Now().UnixNano() - timestamp)
	b.pendingPing = 0
	atomic.StoreInt32(&b.missedPongs, 0)

	// Check for spike before updating
	previousRTT := b.stats.GetRTTCurrent()
	b.stats.SetLastRTT(previousRTT)
	b.stats.AddRTTSample(rtt)

	// Check for RTT spike
	if spiked, oldRTT, newRTT := b.stats.CheckRTTSpike(); spiked {
		b.logger.Warn("RTT spike: %v -> %v", oldRTT.Round(time.Millisecond), newRTT.Round(time.Millisecond))
	}

	// Check against threshold
	if rtt > RTTAlertThreshold {
		b.logger.Warn("[!] RTT %v exceeds Xbox 360 System Link threshold (%v)",
			rtt.Round(time.Millisecond), RTTAlertThreshold)
	}

	b.logger.Trace("PONG received: RTT=%v", rtt.Round(time.Millisecond))
}

// handleBye processes a graceful disconnect.
func (b *Bridge) handleBye() {
	b.logger.Info("Peer disconnected gracefully")
	b.setState(StateDisconnected)
	// Trigger shutdown
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

// injectLoop reads frames from channel and injects them to the network.
func (b *Bridge) injectLoop(ctx context.Context) {
	b.logger.Debug("Inject loop started")
	defer b.logger.Debug("Inject loop stopped")

	// Wait for capture to be ready
	select {
	case <-ctx.Done():
		b.logger.Debug("Inject loop cancelled before capture ready")
		return
	case <-b.captureReady:
		// Capture is now available, proceed
	}

	b.logger.Debug("Capture is ready, beginning packet injection")

	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-b.framesToInject:
			b.captureMu.RLock()
			cap := b.capture
			b.captureMu.RUnlock()

			if cap == nil {
				// Capture was removed (shouldn't happen in normal flow)
				b.logger.Warn("Capture is nil, dropping frame")
				continue
			}

			if err := cap.WritePacket(frame); err != nil {
				b.logger.Warn("Injection failed: %v", err)
				continue
			}
		}
	}
}

// pingLoop sends periodic ping messages.
func (b *Bridge) pingLoop(ctx context.Context) {
	b.logger.Debug("Ping loop started")
	defer b.logger.Debug("Ping loop stopped")

	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.sendPing()
		}
	}
}

// sendPing sends a ping message and tracks it.
func (b *Bridge) sendPing() {
	b.pingMu.Lock()

	// Check for missed pong
	if b.pendingPing != 0 {
		missed := atomic.AddInt32(&b.missedPongs, 1)
		b.logger.Debug("Missed PONG response (count: %d)", missed)

		if missed >= MaxMissedPongs {
			b.pingMu.Unlock()
			b.logger.Warn("Peer unresponsive (missed %d pongs), disconnecting...", missed)
			b.setState(StateDisconnected)
			if b.cancelFunc != nil {
				b.cancelFunc()
			}
			return
		}
	}

	// Send new ping
	timestamp := time.Now().UnixNano()
	b.pendingPing = timestamp
	b.pingMu.Unlock()

	ping := b.codec.EncodePing(timestamp)
	if err := b.transport.Send(ping); err != nil {
		b.logger.Debug("Failed to send PING: %v", err)
	}
}

// statsLoop outputs periodic statistics.
func (b *Bridge) statsLoop(ctx context.Context) {
	b.logger.Debug("Stats loop started")
	defer b.logger.Debug("Stats loop stopped")

	ticker := time.NewTicker(b.statsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.printStats()
		case <-b.stdinCh:
			b.printStats()
		}
	}
}

// stdinLoop monitors stdin for Enter key presses.
func (b *Bridge) stdinLoop(ctx context.Context) {
	b.logger.Debug("Stdin monitor started")
	defer b.logger.Debug("Stdin monitor stopped")

	// Read from stdin in a separate goroutine
	inputCh := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		for {
			_, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if buf[0] == '\n' || buf[0] == '\r' {
				select {
				case inputCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-inputCh:
			// Signal stats output
			select {
			case b.stdinCh <- struct{}{}:
			default:
			}
		}
	}
}

// printStats outputs the current statistics.
func (b *Bridge) printStats() {
	txPkts := atomic.LoadUint64(&b.stats.TxPackets)
	txBytes := atomic.LoadUint64(&b.stats.TxBytes)
	rxPkts := atomic.LoadUint64(&b.stats.RxPackets)
	rxBytes := atomic.LoadUint64(&b.stats.RxBytes)
	rtt := b.stats.GetRTTCurrent()

	b.logger.Stats("TX: %s pkts (%s) | RX: %s pkts (%s) | RTT: %v",
		formatNumber(txPkts), formatBytes(txBytes),
		formatNumber(rxPkts), formatBytes(rxBytes),
		rtt.Round(time.Millisecond))
}

// GetStats returns the current statistics.
func (b *Bridge) GetStats() *Stats {
	return b.stats
}

// formatNumber formats a number with comma separators.
func formatNumber(n uint64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

// formatBytes formats bytes in human-readable form.
func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%d KB", b/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// addrEqual compares two UDP addresses.
func addrEqual(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.IP.Equal(b.IP) && a.Port == b.Port
}
