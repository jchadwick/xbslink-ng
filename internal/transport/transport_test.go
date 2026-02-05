package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
)

func TestNew_ListenMode(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	if transport.mode != ModeListen {
		t.Error("expected ModeListen")
	}
}

func TestNew_ConnectMode(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	cfg := Config{
		Mode:     ModeConnect,
		PeerAddr: "127.0.0.1:12345",
		Codec:    codec,
		Logger:   logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	if transport.mode != ModeConnect {
		t.Error("expected ModeConnect")
	}
}

func TestNew_MissingCodec(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)

	cfg := Config{
		Mode:      ModeListen,
		LocalPort: 12345,
		Codec:     nil,
		Logger:    logger,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing codec")
	}
}

func TestNew_MissingLogger(t *testing.T) {
	codec := protocol.NewCodec(nil)

	cfg := Config{
		Mode:      ModeListen,
		LocalPort: 12345,
		Codec:     codec,
		Logger:    nil,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing logger")
	}
}

func TestNew_InvalidPeerAddr(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	cfg := Config{
		Mode:     ModeConnect,
		PeerAddr: "not-a-valid-address",
		Codec:    codec,
		Logger:   logger,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for invalid peer address")
	}
}

func TestLocalAddr(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	addr := transport.LocalAddr()
	if addr == nil {
		t.Error("expected non-nil local address")
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		t.Error("expected UDP address")
	}
	if udpAddr.Port != port {
		t.Errorf("expected port %d, got %d", port, udpAddr.Port)
	}
}

func TestIsConnected_Initial(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	if transport.IsConnected() {
		t.Error("expected not connected initially")
	}
}

func TestClose_BeforeConnect(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	err = transport.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Double close should be safe
	err = transport.Close()
	if err != nil {
		t.Errorf("double close failed: %v", err)
	}
}

func TestSend_NotConnected(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	err = transport.Send([]byte("test"))
	if err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestSetReadDeadline(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	deadline := time.Now().Add(100 * time.Millisecond)
	err = transport.SetReadDeadline(deadline)
	if err != nil {
		t.Errorf("SetReadDeadline failed: %v", err)
	}
}

func TestWaitForPeer_WrongMode(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	cfg := Config{
		Mode:     ModeConnect,
		PeerAddr: "127.0.0.1:12345",
		Codec:    codec,
		Logger:   logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	ctx := context.Background()
	err = transport.WaitForPeer(ctx)
	if err == nil {
		t.Error("expected error when calling WaitForPeer in connect mode")
	}
}

func TestConnect_WrongMode(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	ctx := context.Background()
	err = transport.Connect(ctx)
	if err == nil {
		t.Error("expected error when calling Connect in listen mode")
	}
}

func TestSendBye_NotConnected(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	// SendBye when not connected should be a no-op
	err = transport.SendBye()
	if err != nil {
		t.Errorf("SendBye failed: %v", err)
	}
}

func TestRecv_Closed(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	transport.Close()

	buf := make([]byte, 1024)
	_, _, err = transport.Recv(buf)
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}
}

func TestSend_Closed(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	transport.Close()

	err = transport.Send([]byte("test"))
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}
}

func TestPeerAddr_Initial(t *testing.T) {
	logger := logging.NewLogger(logging.LevelError)
	codec := protocol.NewCodec(nil)

	port := freePort()
	cfg := Config{
		Mode:      ModeListen,
		LocalPort: uint16(port),
		Codec:     codec,
		Logger:    logger,
	}

	transport, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	// In listen mode, peer address is nil until connected
	if transport.PeerAddr() != nil {
		t.Error("expected nil peer address before connection")
	}
}

func TestAddrEqual(t *testing.T) {
	addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}
	addr2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}
	addr3 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5678}
	addr4 := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1234}

	if !addrEqual(addr1, addr2) {
		t.Error("expected addr1 == addr2")
	}
	if addrEqual(addr1, addr3) {
		t.Error("expected addr1 != addr3 (different port)")
	}
	if addrEqual(addr1, addr4) {
		t.Error("expected addr1 != addr4 (different IP)")
	}
	if !addrEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
	if addrEqual(addr1, nil) {
		t.Error("expected addr1 != nil")
	}
}

// Helper function to find a free port
func freePort() int {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}
