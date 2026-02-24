package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// Test key for secure mode tests
var testKey = []byte("test-secret-key!")

func TestNewCodec_WithKey(t *testing.T) {
	codec := NewCodec(testKey)
	if !codec.IsSecure() {
		t.Error("expected codec to be secure with key")
	}
}

func TestNewCodec_WithoutKey(t *testing.T) {
	codec := NewCodec(nil)
	if codec.IsSecure() {
		t.Error("expected codec to be insecure with nil key")
	}
}

func TestNewCodec_EmptyKey(t *testing.T) {
	codec := NewCodec([]byte{})
	if codec.IsSecure() {
		t.Error("expected codec to be insecure with empty key")
	}
}

func TestEncodeFrame_Roundtrip_Insecure(t *testing.T) {
	codec := NewCodec(nil)
	frame := makeTestFrame(100)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgFrame {
		t.Errorf("expected type FRAME, got %s", MessageTypeName(msg.Type))
	}
	if !bytes.Equal(msg.Frame, frame) {
		t.Error("frame content mismatch")
	}
}

func TestEncodeFrame_Roundtrip_Secure(t *testing.T) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(100)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgFrame {
		t.Errorf("expected type FRAME, got %s", MessageTypeName(msg.Type))
	}
	if !bytes.Equal(msg.Frame, frame) {
		t.Error("frame content mismatch")
	}
}

func TestEncodeFrame_MinSize(t *testing.T) {
	codec := NewCodec(nil)
	frame := makeTestFrame(MinEthernetFrame)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(msg.Frame, frame) {
		t.Error("frame content mismatch")
	}
}

func TestEncodeFrame_MaxSize(t *testing.T) {
	codec := NewCodec(nil)
	frame := makeTestFrame(MaxFrameSize)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(msg.Frame, frame) {
		t.Error("frame content mismatch")
	}
}

func TestEncodeFrame_TooSmall(t *testing.T) {
	codec := NewCodec(nil)
	frame := makeTestFrame(10) // Less than MinEthernetFrame

	_, err := codec.EncodeFrame(frame)
	if err == nil {
		t.Error("expected error for too-small frame")
	}
}

func TestEncodeFrame_TooLarge(t *testing.T) {
	codec := NewCodec(nil)
	frame := makeTestFrame(MaxFrameSize + 1)

	_, err := codec.EncodeFrame(frame)
	if err == nil {
		t.Error("expected error for too-large frame")
	}
}

func TestEncodeHello_Format(t *testing.T) {
	codec := NewCodec(nil)

	encoded, challenge, err := codec.EncodeHello()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if len(challenge) != ChallengeSize {
		t.Errorf("expected challenge size %d, got %d", ChallengeSize, len(challenge))
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgHello {
		t.Errorf("expected type HELLO, got %s", MessageTypeName(msg.Type))
	}
	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %d, got %d", ProtocolVersion, msg.Version)
	}
	if !bytes.Equal(msg.Challenge, challenge) {
		t.Error("challenge mismatch")
	}
}

func TestEncodeHelloAck_Format(t *testing.T) {
	codec := NewCodec(testKey)
	challenge := make([]byte, ChallengeSize)
	for i := range challenge {
		challenge[i] = byte(i)
	}

	encoded := codec.EncodeHelloAck(challenge)

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgHelloAck {
		t.Errorf("expected type HELLO_ACK, got %s", MessageTypeName(msg.Type))
	}
	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %d, got %d", ProtocolVersion, msg.Version)
	}
	if len(msg.Response) != ChallengeRespLen {
		t.Errorf("expected response length %d, got %d", ChallengeRespLen, len(msg.Response))
	}
}

func TestHandshake_ChallengeResponse(t *testing.T) {
	codec := NewCodec(testKey)

	// Simulate HELLO
	_, challenge, err := codec.EncodeHello()
	if err != nil {
		t.Fatalf("encode hello failed: %v", err)
	}

	// Simulate HELLO_ACK with same codec (same key)
	codec2 := NewCodec(testKey)
	ackEncoded := codec2.EncodeHelloAck(challenge)

	// Decode the ACK
	msg, err := codec.Decode(ackEncoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Verify challenge response
	if !codec.VerifyChallengeResponse(challenge, msg.Response) {
		t.Error("challenge response verification failed")
	}
}

func TestHandshake_WrongKey(t *testing.T) {
	codec1 := NewCodec(testKey)
	codec2 := NewCodec([]byte("different-key!!"))

	// Simulate HELLO
	_, challenge, err := codec1.EncodeHello()
	if err != nil {
		t.Fatalf("encode hello failed: %v", err)
	}

	// Simulate HELLO_ACK with different key
	ackEncoded := codec2.EncodeHelloAck(challenge)

	// Decode will fail due to HMAC mismatch
	_, err = codec1.Decode(ackEncoded)
	if err != ErrInvalidHMAC {
		t.Errorf("expected ErrInvalidHMAC, got %v", err)
	}
}

func TestHandshake_InsecureMode(t *testing.T) {
	codec := NewCodec(nil)

	// In insecure mode, challenge response should always verify
	challenge := make([]byte, ChallengeSize)
	response := make([]byte, ChallengeRespLen)

	if !codec.VerifyChallengeResponse(challenge, response) {
		t.Error("insecure mode should always verify")
	}
}

func TestEncodePing_Roundtrip(t *testing.T) {
	codec := NewCodec(nil)
	timestamp := time.Now().UnixNano()

	encoded := codec.EncodePing(timestamp)

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgPing {
		t.Errorf("expected type PING, got %s", MessageTypeName(msg.Type))
	}
	if msg.Timestamp != timestamp {
		t.Errorf("expected timestamp %d, got %d", timestamp, msg.Timestamp)
	}
}

func TestEncodePong_Roundtrip(t *testing.T) {
	codec := NewCodec(nil)
	timestamp := time.Now().UnixNano()

	encoded := codec.EncodePong(timestamp)

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgPong {
		t.Errorf("expected type PONG, got %s", MessageTypeName(msg.Type))
	}
	if msg.Timestamp != timestamp {
		t.Errorf("expected timestamp %d, got %d", timestamp, msg.Timestamp)
	}
}

func TestEncodeBye_Format(t *testing.T) {
	codec := NewCodec(nil)

	encoded := codec.EncodeBye()

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if msg.Type != MsgBye {
		t.Errorf("expected type BYE, got %s", MessageTypeName(msg.Type))
	}
}

func TestDecode_ValidHMAC(t *testing.T) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(100)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(msg.Frame, frame) {
		t.Error("frame content mismatch")
	}
}

func TestDecode_InvalidHMAC(t *testing.T) {
	codec1 := NewCodec(testKey)
	codec2 := NewCodec([]byte("different-key!!"))

	frame := makeTestFrame(100)
	encoded, err := codec1.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	_, err = codec2.Decode(encoded)
	if err != ErrInvalidHMAC {
		t.Errorf("expected ErrInvalidHMAC, got %v", err)
	}
}

func TestDecode_TamperedPayload(t *testing.T) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(100)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Tamper with the payload
	encoded[15] ^= 0xFF

	_, err = codec.Decode(encoded)
	if err != ErrInvalidHMAC {
		t.Errorf("expected ErrInvalidHMAC, got %v", err)
	}
}

func TestDecode_TruncatedHMAC(t *testing.T) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(100)

	encoded, err := codec.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Truncate the HMAC - this corrupts the HMAC so verification should fail
	truncated := encoded[:len(encoded)-10]

	_, err = codec.Decode(truncated)
	// Either ErrMessageTooShort (if length check fails) or ErrInvalidHMAC (if HMAC check fails)
	if err != ErrMessageTooShort && err != ErrInvalidHMAC {
		t.Errorf("expected ErrMessageTooShort or ErrInvalidHMAC, got %v", err)
	}
}

func TestDecode_MessageTooShort_Secure(t *testing.T) {
	codec := NewCodec(testKey)

	// Message shorter than minimum secure header (Type + Nonce + HMAC = 41 bytes)
	tooShort := make([]byte, 30)
	tooShort[0] = MsgFrame

	_, err := codec.Decode(tooShort)
	if err != ErrMessageTooShort {
		t.Errorf("expected ErrMessageTooShort, got %v", err)
	}
}

func TestDecode_ReplayProtection(t *testing.T) {
	codec := NewCodec(testKey)

	// Send two frames
	frame1 := makeTestFrame(50)
	frame2 := makeTestFrame(60)

	encoded1, _ := codec.EncodeFrame(frame1)
	encoded2, _ := codec.EncodeFrame(frame2)

	// Decode in order should work
	_, err := codec.Decode(encoded1)
	if err != nil {
		t.Fatalf("first decode failed: %v", err)
	}

	_, err = codec.Decode(encoded2)
	if err != nil {
		t.Fatalf("second decode failed: %v", err)
	}

	// Replay first message should fail
	_, err = codec.Decode(encoded1)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected, got %v", err)
	}
}

func TestDecode_NonceOutOfOrder(t *testing.T) {
	codec := NewCodec(testKey)

	// Create a message with high nonce
	frame := makeTestFrame(50)
	encoded, _ := codec.EncodeFrame(frame)
	codec.Decode(encoded) // This sets recvNonce to 1

	// Second frame increases nonce to 2
	frame2 := makeTestFrame(50)
	encoded2, _ := codec.EncodeFrame(frame2)
	codec.Decode(encoded2) // recvNonce is now 2

	// Third frame increases nonce to 3
	frame3 := makeTestFrame(50)
	encoded3, _ := codec.EncodeFrame(frame3)
	codec.Decode(encoded3) // recvNonce is now 3

	// Replaying encoded2 (nonce 2) should fail
	_, err := codec.Decode(encoded2)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected, got %v", err)
	}
}

func TestResetRecvNonce(t *testing.T) {
	codec := NewCodec(testKey)

	frame := makeTestFrame(50)
	encoded, _ := codec.EncodeFrame(frame)

	// First decode
	_, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("first decode failed: %v", err)
	}

	// Replay should fail
	_, err = codec.Decode(encoded)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected, got %v", err)
	}

	// Reset nonce
	codec.ResetRecvNonce()

	// Now create a new codec to encode (simulate reconnect)
	codec2 := NewCodec(testKey)
	frame2 := makeTestFrame(50)
	encoded2, _ := codec2.EncodeFrame(frame2)

	// Should succeed after reset
	_, err = codec.Decode(encoded2)
	if err != nil {
		t.Errorf("decode after reset failed: %v", err)
	}
}

func TestDecode_HelloAllowsSessionRestartedNonce(t *testing.T) {
	listener := NewCodec(testKey)

	peer1 := NewCodec(testKey)
	hello1, _, err := peer1.EncodeHello()
	if err != nil {
		t.Fatalf("first hello encode failed: %v", err)
	}
	if _, err := listener.Decode(hello1); err != nil {
		t.Fatalf("first hello decode failed: %v", err)
	}

	// Simulate peer restart: sender nonce returns to 1.
	peer2 := NewCodec(testKey)
	hello2, _, err := peer2.EncodeHello()
	if err != nil {
		t.Fatalf("second hello encode failed: %v", err)
	}
	if _, err := listener.Decode(hello2); err != nil {
		t.Fatalf("second hello decode failed after peer restart: %v", err)
	}
}

func TestDecode_HelloAckAllowsSessionRestartedNonce(t *testing.T) {
	client := NewCodec(testKey)

	server1 := NewCodec(testKey)
	challenge := make([]byte, ChallengeSize)
	ack1 := server1.EncodeHelloAck(challenge)
	if _, err := client.Decode(ack1); err != nil {
		t.Fatalf("first hello_ack decode failed: %v", err)
	}

	// Simulate peer restart: sender nonce returns to 1.
	server2 := NewCodec(testKey)
	ack2 := server2.EncodeHelloAck(challenge)
	if _, err := client.Decode(ack2); err != nil {
		t.Fatalf("second hello_ack decode failed after peer restart: %v", err)
	}
}

func TestDecode_EmptyMessage(t *testing.T) {
	codec := NewCodec(nil)

	_, err := codec.Decode([]byte{})
	if err != ErrMessageTooShort {
		t.Errorf("expected ErrMessageTooShort, got %v", err)
	}
}

func TestDecode_UnknownType(t *testing.T) {
	codec := NewCodec(nil)

	// Create message with unknown type
	msg := []byte{0xFF}

	_, err := codec.Decode(msg)
	if err == nil || err == ErrMessageTooShort {
		t.Errorf("expected ErrUnknownMsgType, got %v", err)
	}
}

func TestDecode_InvalidFramePayload_TooSmall(t *testing.T) {
	codec := NewCodec(nil)

	// Create a FRAME message with too-small payload
	msg := make([]byte, 1+10) // Type + 10 bytes (less than MinEthernetFrame)
	msg[0] = MsgFrame

	_, err := codec.Decode(msg)
	if err == nil {
		t.Error("expected error for too-small frame payload")
	}
}

func TestDecode_InvalidPingPayload(t *testing.T) {
	codec := NewCodec(nil)

	// Create a PING message with too-small payload
	msg := make([]byte, 1+4) // Type + 4 bytes (less than 8)
	msg[0] = MsgPing

	_, err := codec.Decode(msg)
	if err == nil {
		t.Error("expected error for too-small ping payload")
	}
}

func TestMessageTypeName_AllTypes(t *testing.T) {
	tests := []struct {
		msgType  byte
		expected string
	}{
		{MsgFrame, "FRAME"},
		{MsgHello, "HELLO"},
		{MsgHelloAck, "HELLO_ACK"},
		{MsgPing, "PING"},
		{MsgPong, "PONG"},
		{MsgBye, "BYE"},
	}

	for _, tt := range tests {
		result := MessageTypeName(tt.msgType)
		if result != tt.expected {
			t.Errorf("MessageTypeName(%d) = %s, want %s", tt.msgType, result, tt.expected)
		}
	}
}

func TestMessageTypeName_Unknown(t *testing.T) {
	result := MessageTypeName(0xFF)
	expected := "UNKNOWN(0xff)"
	if result != expected {
		t.Errorf("MessageTypeName(0xFF) = %s, want %s", result, expected)
	}
}

func TestVerifyChallengeResponse_InvalidLengths(t *testing.T) {
	codec := NewCodec(testKey)

	// Wrong challenge length
	if codec.VerifyChallengeResponse(make([]byte, 5), make([]byte, ChallengeRespLen)) {
		t.Error("should reject wrong challenge length")
	}

	// Wrong response length
	if codec.VerifyChallengeResponse(make([]byte, ChallengeSize), make([]byte, 5)) {
		t.Error("should reject wrong response length")
	}
}

// Helper function to create a test frame
func makeTestFrame(size int) []byte {
	frame := make([]byte, size)
	// Set a valid EtherType (IPv4)
	if size >= 14 {
		binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	}
	// Fill with pattern
	for i := 14; i < size; i++ {
		frame[i] = byte(i)
	}
	return frame
}
