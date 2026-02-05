//go:build integration
// +build integration

package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
)

func TestIntegration_Handshake_Loopback(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec1 := protocol.NewCodec(nil)
	codec2 := protocol.NewCodec(nil)

	// Find free port
	port := freePort()

	// Create listener
	listener, err := New(Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec1,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Create connector
	connector, err := New(Config{
		Mode:     ModeConnect,
		PeerAddr: net.JoinHostPort("127.0.0.1", string(rune(port))),
		Codec:    codec2,
		Logger:   logger,
	})
	if err != nil {
		// Try with formatted port
		connector, err = New(Config{
			Mode:     ModeConnect,
			PeerAddr: "127.0.0.1:" + itoa(port),
			Codec:    codec2,
			Logger:   logger,
		})
		if err != nil {
			t.Fatalf("failed to create connector: %v", err)
		}
	}
	defer connector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start listener in goroutine
	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- listener.WaitForPeer(ctx)
	}()

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// Connect
	connectorDone := make(chan error, 1)
	go func() {
		connectorDone <- connector.Connect(ctx)
	}()

	// Wait for both
	select {
	case err := <-listenerDone:
		if err != nil {
			t.Errorf("listener error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("listener timeout")
	}

	select {
	case err := <-connectorDone:
		if err != nil {
			t.Errorf("connector error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("connector timeout")
	}

	// Verify both are connected
	if !listener.IsConnected() {
		t.Error("listener should be connected")
	}
	if !connector.IsConnected() {
		t.Error("connector should be connected")
	}
}

func TestIntegration_Handshake_Secure(t *testing.T) {
	key := []byte("shared-secret-16")
	logger := logging.NewLogger(logging.LevelError)
	codec1 := protocol.NewCodec(key)
	codec2 := protocol.NewCodec(key)

	port := freePort()

	listener, err := New(Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec1,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	connector, err := New(Config{
		Mode:     ModeConnect,
		PeerAddr: "127.0.0.1:" + itoa(port),
		Codec:    codec2,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}
	defer connector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- listener.WaitForPeer(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	connectorDone := make(chan error, 1)
	go func() {
		connectorDone <- connector.Connect(ctx)
	}()

	select {
	case err := <-listenerDone:
		if err != nil {
			t.Errorf("listener error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("listener timeout")
	}

	select {
	case err := <-connectorDone:
		if err != nil {
			t.Errorf("connector error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("connector timeout")
	}

	if !listener.IsConnected() || !connector.IsConnected() {
		t.Error("both should be connected")
	}
}

func TestIntegration_Handshake_KeyMismatch(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec1 := protocol.NewCodec([]byte("key-for-listener"))
	codec2 := protocol.NewCodec([]byte("key-for-connect!"))

	port := freePort()

	listener, err := New(Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec1,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	connector, err := New(Config{
		Mode:     ModeConnect,
		PeerAddr: "127.0.0.1:" + itoa(port),
		Codec:    codec2,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}
	defer connector.Close()

	// Short timeout since we expect failure
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- listener.WaitForPeer(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connector should fail due to HMAC mismatch when decoding HELLO_ACK
	err = connector.Connect(ctx)
	
	// We expect an error (either timeout or HMAC failure)
	// The specific error depends on timing
	if err == nil && connector.IsConnected() {
		t.Error("expected connection to fail with mismatched keys")
	}
}

func TestIntegration_SendReceive(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec1 := protocol.NewCodec(nil)
	codec2 := protocol.NewCodec(nil)

	port := freePort()

	listener, err := New(Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec1,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	connector, err := New(Config{
		Mode:     ModeConnect,
		PeerAddr: "127.0.0.1:" + itoa(port),
		Codec:    codec2,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("failed to create connector: %v", err)
	}
	defer connector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Establish connection
	go listener.WaitForPeer(ctx)
	time.Sleep(100 * time.Millisecond)
	go connector.Connect(ctx)

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	if !listener.IsConnected() || !connector.IsConnected() {
		t.Fatal("connection not established")
	}

	// Send data from connector to listener
	testData := []byte("hello from connector")
	err = connector.Send(testData)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Receive on listener
	buf := make([]byte, 1024)
	listener.SetReadDeadline(time.Now().Add(time.Second))
	n, addr, err := listener.Recv(buf)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}

	if string(buf[:n]) != string(testData) {
		t.Errorf("received %q, want %q", buf[:n], testData)
	}

	if addr == nil {
		t.Error("expected non-nil sender address")
	}
}

// Helper to convert int to string (avoid importing strconv)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
