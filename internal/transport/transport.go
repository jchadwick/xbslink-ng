// Package transport provides UDP transport for xbslink-ng with connection handling.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
)

// Mode represents the transport operating mode.
type Mode int

const (
	// ModeListen binds to a port and waits for incoming connections.
	ModeListen Mode = iota
	// ModeConnect actively connects to a remote peer.
	ModeConnect
)

// Configuration constants.
const (
	// DefaultReadBuffer is the default UDP read buffer size.
	DefaultReadBuffer = 65536
	// DefaultWriteBuffer is the default UDP write buffer size.
	DefaultWriteBuffer = 65536
	// HandshakeTimeout is the timeout for the initial handshake.
	HandshakeTimeout = 10 * time.Second
	// ReadTimeout is the timeout for individual read operations.
	ReadTimeout = 100 * time.Millisecond
)

// Retry backoff intervals for connect mode.
// Connect retries forever with exponential backoff: 1s, 2s, 5s, 10s (then stays at 10s).
var connectBackoff = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

// Errors returned by transport operations.
var (
	ErrNotConnected     = errors.New("transport not connected")
	ErrAlreadyConnected = errors.New("transport already connected")
	ErrHandshakeFailed  = errors.New("handshake failed")
	ErrChallengeInvalid = errors.New("challenge response invalid")
	ErrClosed           = errors.New("transport closed")
)

// Transport manages UDP communication with a peer.
type Transport struct {
	conn      *net.UDPConn
	peerAddr  *net.UDPAddr
	mode      Mode
	codec     *protocol.Codec
	logger    *logging.Logger
	challenge []byte // Challenge sent in HELLO (for verifying HELLO_ACK)

	mu        sync.RWMutex
	connected bool
	closed    bool

	// Buffer pool for reads
	readBuf []byte
}

// Config holds transport configuration.
type Config struct {
	Mode      Mode
	LocalPort uint16 // Port to bind (listen mode) or local port (connect mode, 0 = auto)
	PeerAddr  string // Peer address in "host:port" format (connect mode only)
	Codec     *protocol.Codec
	Logger    *logging.Logger
}

// New creates a new transport with the given configuration.
func New(cfg Config) (*Transport, error) {
	if cfg.Codec == nil {
		return nil, errors.New("codec is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("logger is required")
	}

	t := &Transport{
		mode:    cfg.Mode,
		codec:   cfg.Codec,
		logger:  cfg.Logger,
		readBuf: make([]byte, DefaultReadBuffer),
	}

	// Set up the UDP connection based on mode
	var err error
	switch cfg.Mode {
	case ModeListen:
		err = t.setupListen(cfg.LocalPort)
	case ModeConnect:
		err = t.setupConnect(cfg.LocalPort, cfg.PeerAddr)
	default:
		return nil, fmt.Errorf("unknown mode: %d", cfg.Mode)
	}

	if err != nil {
		return nil, err
	}

	return t, nil
}

// setupListen binds to the specified port for incoming connections.
func (t *Transport) setupListen(port uint16) error {
	addr := &net.UDPAddr{Port: int(port)}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to port %d: %w", port, err)
	}

	// Set socket buffer sizes
	if err := conn.SetReadBuffer(DefaultReadBuffer); err != nil {
		t.logger.Warn("Failed to set read buffer size: %v", err)
	}
	if err := conn.SetWriteBuffer(DefaultWriteBuffer); err != nil {
		t.logger.Warn("Failed to set write buffer size: %v", err)
	}

	t.conn = conn
	t.logger.Info("Listening on UDP :%d", port)
	return nil
}

// setupConnect prepares to connect to the specified peer.
func (t *Transport) setupConnect(localPort uint16, peerAddr string) error {
	// Resolve peer address
	addr, err := net.ResolveUDPAddr("udp", peerAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve peer address %q: %w", peerAddr, err)
	}
	t.peerAddr = addr

	// Bind to local port (0 = system-assigned)
	localAddr := &net.UDPAddr{Port: int(localPort)}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("failed to bind to local port: %w", err)
	}

	// Set socket buffer sizes
	if err := conn.SetReadBuffer(DefaultReadBuffer); err != nil {
		t.logger.Warn("Failed to set read buffer size: %v", err)
	}
	if err := conn.SetWriteBuffer(DefaultWriteBuffer); err != nil {
		t.logger.Warn("Failed to set write buffer size: %v", err)
	}

	t.conn = conn
	t.logger.Info("Connecting to peer %s", peerAddr)
	return nil
}

// WaitForPeer waits for an incoming connection (listen mode).
// Returns when a valid HELLO is received and HELLO_ACK is sent.
func (t *Transport) WaitForPeer(ctx context.Context) error {
	if t.mode != ModeListen {
		return errors.New("WaitForPeer only valid in listen mode")
	}

	t.logger.Info("Waiting for peer connection...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Set read deadline
		t.conn.SetReadDeadline(time.Now().Add(ReadTimeout))

		n, addr, err := t.conn.ReadFromUDP(t.readBuf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Timeout, check context and try again
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Try to decode as HELLO
		msg, err := t.codec.Decode(t.readBuf[:n])
		if err != nil {
			if errors.Is(err, protocol.ErrMessageTooShort) && t.codec.IsSecure() {
				t.logger.Warn("Received unreadable message from %s (pre-shared key mismatch? peer may not be using encryption)", addr)
			} else {
				t.logger.Debug("Received invalid message from %s: %v", addr, err)
			}
			continue
		}

		if msg.Type != protocol.MsgHello {
			// Send BYE to signal we need fresh handshake (enables sub-second session reset detection)
			bye := t.codec.EncodeBye()
			t.conn.WriteToUDP(bye, addr)
			t.logger.Debug("Expected HELLO from %s, got %s, sent BYE", addr, protocol.MessageTypeName(msg.Type))
			continue
		}

		t.logger.Info("Received HELLO from %s (version %d)", addr, msg.Version)

		// Store peer address and challenge
		t.peerAddr = addr
		t.challenge = msg.Challenge

		// Reset nonce state for new session (prevents "replay attack detected" on reconnection)
		t.codec.ResetRecvNonce()

		// Send HELLO_ACK with challenge response
		ack := t.codec.EncodeHelloAck(msg.Challenge)
		if _, err := t.conn.WriteToUDP(ack, addr); err != nil {
			return fmt.Errorf("failed to send HELLO_ACK: %w", err)
		}

		t.mu.Lock()
		t.connected = true
		t.mu.Unlock()

		t.logger.Info("Peer connected: %s", addr)
		return nil
	}
}

// Connect establishes a connection to the peer (connect mode).
// Retries forever with exponential backoff: 1s, 2s, 5s, 10s (then repeats 10s).
func (t *Transport) Connect(ctx context.Context) error {
	if t.mode != ModeConnect {
		return errors.New("Connect only valid in connect mode")
	}

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := t.attemptHandshake(ctx)
		if err == nil {
			return nil // Success
		}

		// Determine backoff delay
		backoffIdx := attempt
		if backoffIdx >= len(connectBackoff) {
			backoffIdx = len(connectBackoff) - 1
		}
		delay := connectBackoff[backoffIdx]

		t.logger.Warn("Connection attempt %d failed: %v. Retrying in %v...", attempt+1, err, delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		attempt++
		// Reset codec nonce on retry
		t.codec.ResetRecvNonce()
	}
}

// attemptHandshake performs a single handshake attempt.
func (t *Transport) attemptHandshake(ctx context.Context) error {
	// Send HELLO with challenge
	hello, challenge, err := t.codec.EncodeHello()
	if err != nil {
		return fmt.Errorf("failed to encode HELLO: %w", err)
	}
	t.challenge = challenge

	t.logger.Debug("Sending HELLO to %s", t.peerAddr)
	if _, err := t.conn.WriteToUDP(hello, t.peerAddr); err != nil {
		return fmt.Errorf("failed to send HELLO: %w", err)
	}

	// Wait for HELLO_ACK with timeout
	deadline := time.Now().Add(HandshakeTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		n, addr, err := t.conn.ReadFromUDP(t.readBuf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Verify it's from our expected peer
		if !addrEqual(addr, t.peerAddr) {
			t.logger.Debug("Received packet from unexpected source %s", addr)
			continue
		}

		// Decode message
		msg, err := t.codec.Decode(t.readBuf[:n])
		if err != nil {
			if errors.Is(err, protocol.ErrMessageTooShort) && t.codec.IsSecure() {
				t.logger.Warn("Invalid message from peer (pre-shared key mismatch? server may not be using encryption)")
			} else {
				t.logger.Debug("Invalid message from peer: %v", err)
			}
			continue
		}

		if msg.Type != protocol.MsgHelloAck {
			t.logger.Debug("Expected HELLO_ACK, got %s", protocol.MessageTypeName(msg.Type))
			continue
		}

		// Verify challenge response
		if t.codec.IsSecure() {
			if !t.codec.VerifyChallengeResponse(t.challenge, msg.Response) {
				return ErrChallengeInvalid
			}
			t.logger.Debug("Challenge-response verified")
		}

		// Reset nonce state for new session (prevents "replay attack detected" on reconnection)
		t.codec.ResetRecvNonce()

		t.mu.Lock()
		t.connected = true
		t.mu.Unlock()

		t.logger.Info("Connected to peer: %s", t.peerAddr)
		return nil
	}

	return fmt.Errorf("handshake timeout after %v", HandshakeTimeout)
}

// Send sends data to the connected peer.
func (t *Transport) Send(data []byte) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return ErrClosed
	}
	if !t.connected {
		t.mu.RUnlock()
		return ErrNotConnected
	}
	peerAddr := t.peerAddr
	t.mu.RUnlock()

	_, err := t.conn.WriteToUDP(data, peerAddr)
	return err
}

// Recv receives data from the peer.
// Returns the raw bytes, sender address, and any error.
func (t *Transport) Recv(buf []byte) (int, *net.UDPAddr, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return 0, nil, ErrClosed
	}
	t.mu.RUnlock()

	n, addr, err := t.conn.ReadFromUDP(buf)
	return n, addr, err
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (t *Transport) SetReadDeadline(deadline time.Time) error {
	return t.conn.SetReadDeadline(deadline)
}

// SendBye sends a graceful disconnect message.
func (t *Transport) SendBye() error {
	t.mu.RLock()
	if !t.connected || t.closed {
		t.mu.RUnlock()
		return nil
	}
	peerAddr := t.peerAddr
	t.mu.RUnlock()

	bye := t.codec.EncodeBye()
	_, err := t.conn.WriteToUDP(bye, peerAddr)
	return err
}

// Close closes the transport.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.connected = false

	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// IsConnected returns true if the transport is connected to a peer.
func (t *Transport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// PeerAddr returns the connected peer's address.
func (t *Transport) PeerAddr() *net.UDPAddr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peerAddr
}

// LocalAddr returns the local address.
func (t *Transport) LocalAddr() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

// addrEqual compares two UDP addresses.
func addrEqual(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.IP.Equal(b.IP) && a.Port == b.Port
}
