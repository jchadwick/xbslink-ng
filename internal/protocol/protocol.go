// Package protocol implements the xbslink-ng wire protocol with optional HMAC authentication.
package protocol

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
)

// Protocol constants.
const (
	// ProtocolVersion is the current protocol version.
	ProtocolVersion uint16 = 1

	// Message types.
	MsgFrame    byte = 0x00 // Raw Ethernet frame
	MsgHello    byte = 0x01 // Initiate connection
	MsgHelloAck byte = 0x02 // Accept connection
	MsgPing     byte = 0x03 // Latency probe
	MsgPong     byte = 0x04 // Latency response
	MsgBye      byte = 0x05 // Graceful disconnect

	// Size constants.
	NonceSize        = 8  // 8-byte nonce for replay protection
	HMACSize         = 32 // HMAC-SHA256 output size
	ChallengeSize    = 16 // 16-byte challenge in HELLO
	ChallengeRespLen = 32 // HMAC response to challenge

	// Header sizes.
	MinHeaderSize       = 1                    // Type only (insecure mode)
	SecureHeaderSize    = 1 + NonceSize        // Type + Nonce
	MinPayloadSize      = 0                    // BYE has no payload
	MaxFrameSize        = 1514                 // Max Ethernet frame size
	MinEthernetFrame    = 14                   // Min Ethernet frame (header only)
	HelloPayloadSize    = 2 + ChallengeSize    // version (2) + challenge (16)
	HelloAckPayloadSize = 2 + ChallengeRespLen // version (2) + response (32)
	PingPongPayloadSize = 8                    // timestamp (8 bytes)
)

// Errors returned by protocol functions.
var (
	ErrMessageTooShort   = errors.New("message too short")
	ErrInvalidHMAC       = errors.New("invalid HMAC signature")
	ErrReplayDetected    = errors.New("replay attack detected: nonce not increasing")
	ErrUnknownMsgType    = errors.New("unknown message type")
	ErrInvalidPayload    = errors.New("invalid payload size")
	ErrVersionMismatch   = errors.New("protocol version mismatch")
	ErrChallengeRequired = errors.New("challenge required but not present")
)

// Codec handles encoding and decoding of protocol messages with optional HMAC authentication.
type Codec struct {
	key        []byte // Pre-shared key for HMAC (nil = insecure mode)
	sendNonce  uint64 // Monotonic counter for outgoing messages
	recvNonce  uint64 // Last received nonce (for replay protection)
	secureMode bool   // True if key is set
}

// NewCodec creates a new protocol codec.
// If key is nil or empty, the codec operates in insecure mode (no HMAC, no nonces).
func NewCodec(key []byte) *Codec {
	return &Codec{
		key:        key,
		sendNonce:  0,
		recvNonce:  0,
		secureMode: len(key) > 0,
	}
}

// IsSecure returns true if the codec is operating in secure mode.
func (c *Codec) IsSecure() bool {
	return c.secureMode
}

// nextNonce atomically increments and returns the next nonce.
func (c *Codec) nextNonce() uint64 {
	return atomic.AddUint64(&c.sendNonce, 1)
}

// computeHMAC computes HMAC-SHA256 over the given data.
func (c *Codec) computeHMAC(data []byte) []byte {
	h := hmac.New(sha256.New, c.key)
	h.Write(data)
	return h.Sum(nil)
}

// verifyHMAC verifies the HMAC signature.
func (c *Codec) verifyHMAC(data, sig []byte) bool {
	expected := c.computeHMAC(data)
	return hmac.Equal(expected, sig)
}

// encode creates a wire-format message with optional HMAC.
// Format (secure):  [Type(1)][Nonce(8)][Payload(var)][HMAC(32)]
// Format (insecure): [Type(1)][Payload(var)]
func (c *Codec) encode(msgType byte, payload []byte) []byte {
	if c.secureMode {
		// Secure mode: Type + Nonce + Payload + HMAC
		nonce := c.nextNonce()
		msg := make([]byte, 1+NonceSize+len(payload)+HMACSize)
		msg[0] = msgType
		binary.BigEndian.PutUint64(msg[1:9], nonce)
		copy(msg[9:9+len(payload)], payload)

		// Compute HMAC over Type+Nonce+Payload
		mac := c.computeHMAC(msg[:9+len(payload)])
		copy(msg[9+len(payload):], mac)
		return msg
	}

	// Insecure mode: Type + Payload
	msg := make([]byte, 1+len(payload))
	msg[0] = msgType
	copy(msg[1:], payload)
	return msg
}

// decode parses a wire-format message and verifies HMAC if in secure mode.
// Returns message type, payload, and any error.
func (c *Codec) decode(data []byte) (msgType byte, payload []byte, err error) {
	if len(data) < MinHeaderSize {
		return 0, nil, ErrMessageTooShort
	}

	if c.secureMode {
		// Secure mode: need at least Type + Nonce + HMAC
		minLen := 1 + NonceSize + HMACSize
		if len(data) < minLen {
			return 0, nil, ErrMessageTooShort
		}

		// Extract components
		msgType = data[0]
		nonce := binary.BigEndian.Uint64(data[1:9])
		payloadEnd := len(data) - HMACSize
		payload = data[9:payloadEnd]
		sig := data[payloadEnd:]

		// Verify HMAC
		if !c.verifyHMAC(data[:payloadEnd], sig) {
			return 0, nil, ErrInvalidHMAC
		}

		// Verify nonce is increasing (replay protection) for non-handshake traffic.
		// HELLO/HELLO_ACK are exempt so peers can reconnect even if their sender
		// nonce counter restarts from 1 (e.g. process restart).
		if msgType != MsgHello && msgType != MsgHelloAck {
			if nonce > 0 && nonce <= atomic.LoadUint64(&c.recvNonce) {
				return 0, nil, ErrReplayDetected
			}
			atomic.StoreUint64(&c.recvNonce, nonce)
		}

		return msgType, payload, nil
	}

	// Insecure mode: Type + Payload
	msgType = data[0]
	payload = data[1:]
	return msgType, payload, nil
}

// EncodeFrame encodes a raw Ethernet frame.
func (c *Codec) EncodeFrame(frame []byte) ([]byte, error) {
	if len(frame) < MinEthernetFrame || len(frame) > MaxFrameSize {
		return nil, fmt.Errorf("frame size %d out of range [%d, %d]", len(frame), MinEthernetFrame, MaxFrameSize)
	}
	return c.encode(MsgFrame, frame), nil
}

// EncodeHello encodes a HELLO message with a challenge for authentication.
func (c *Codec) EncodeHello() ([]byte, []byte, error) {
	payload := make([]byte, HelloPayloadSize)
	binary.BigEndian.PutUint16(payload[0:2], ProtocolVersion)

	// Generate random challenge
	challenge := payload[2 : 2+ChallengeSize]
	if _, err := rand.Read(challenge); err != nil {
		return nil, nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	return c.encode(MsgHello, payload), challenge, nil
}

// EncodeHelloAck encodes a HELLO_ACK message with challenge response.
// The response is HMAC-SHA256(key, challenge) if in secure mode, or zeros if insecure.
func (c *Codec) EncodeHelloAck(challenge []byte) []byte {
	payload := make([]byte, HelloAckPayloadSize)
	binary.BigEndian.PutUint16(payload[0:2], ProtocolVersion)

	// Compute challenge response
	if c.secureMode && len(challenge) == ChallengeSize {
		response := c.computeHMAC(challenge)
		copy(payload[2:], response)
	}
	// If insecure, leave response as zeros

	return c.encode(MsgHelloAck, payload)
}

// EncodePing encodes a PING message with a timestamp.
func (c *Codec) EncodePing(timestamp int64) []byte {
	payload := make([]byte, PingPongPayloadSize)
	binary.BigEndian.PutUint64(payload, uint64(timestamp))
	return c.encode(MsgPing, payload)
}

// EncodePong encodes a PONG message with the echoed timestamp.
func (c *Codec) EncodePong(timestamp int64) []byte {
	payload := make([]byte, PingPongPayloadSize)
	binary.BigEndian.PutUint64(payload, uint64(timestamp))
	return c.encode(MsgPong, payload)
}

// EncodeBye encodes a BYE message for graceful disconnect.
func (c *Codec) EncodeBye() []byte {
	return c.encode(MsgBye, nil)
}

// Message represents a decoded protocol message.
type Message struct {
	Type      byte
	Frame     []byte // For MsgFrame
	Version   uint16 // For MsgHello, MsgHelloAck
	Challenge []byte // For MsgHello (16 bytes)
	Response  []byte // For MsgHelloAck (32 bytes)
	Timestamp int64  // For MsgPing, MsgPong
}

// Decode parses a wire-format message into a structured Message.
func (c *Codec) Decode(data []byte) (*Message, error) {
	msgType, payload, err := c.decode(data)
	if err != nil {
		return nil, err
	}

	msg := &Message{Type: msgType}

	switch msgType {
	case MsgFrame:
		if len(payload) < MinEthernetFrame {
			return nil, fmt.Errorf("%w: frame too small (%d bytes)", ErrInvalidPayload, len(payload))
		}
		if len(payload) > MaxFrameSize {
			return nil, fmt.Errorf("%w: frame too large (%d bytes)", ErrInvalidPayload, len(payload))
		}
		msg.Frame = payload

	case MsgHello:
		if len(payload) < HelloPayloadSize {
			return nil, fmt.Errorf("%w: HELLO payload too small", ErrInvalidPayload)
		}
		msg.Version = binary.BigEndian.Uint16(payload[0:2])
		msg.Challenge = payload[2 : 2+ChallengeSize]
		if msg.Version != ProtocolVersion {
			return nil, fmt.Errorf("%w: expected %d, got %d", ErrVersionMismatch, ProtocolVersion, msg.Version)
		}

	case MsgHelloAck:
		if len(payload) < HelloAckPayloadSize {
			return nil, fmt.Errorf("%w: HELLO_ACK payload too small", ErrInvalidPayload)
		}
		msg.Version = binary.BigEndian.Uint16(payload[0:2])
		msg.Response = payload[2 : 2+ChallengeRespLen]
		if msg.Version != ProtocolVersion {
			return nil, fmt.Errorf("%w: expected %d, got %d", ErrVersionMismatch, ProtocolVersion, msg.Version)
		}

	case MsgPing:
		if len(payload) < PingPongPayloadSize {
			return nil, fmt.Errorf("%w: PING payload too small", ErrInvalidPayload)
		}
		msg.Timestamp = int64(binary.BigEndian.Uint64(payload))

	case MsgPong:
		if len(payload) < PingPongPayloadSize {
			return nil, fmt.Errorf("%w: PONG payload too small", ErrInvalidPayload)
		}
		msg.Timestamp = int64(binary.BigEndian.Uint64(payload))

	case MsgBye:
		// No payload expected

	default:
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnknownMsgType, msgType)
	}

	return msg, nil
}

// VerifyChallengeResponse verifies the challenge response in a HELLO_ACK.
func (c *Codec) VerifyChallengeResponse(challenge, response []byte) bool {
	if !c.secureMode {
		// In insecure mode, always accept (response should be zeros anyway)
		return true
	}
	if len(challenge) != ChallengeSize || len(response) != ChallengeRespLen {
		return false
	}
	expected := c.computeHMAC(challenge)
	return hmac.Equal(expected, response)
}

// ResetRecvNonce resets the receive nonce counter (used when reconnecting).
func (c *Codec) ResetRecvNonce() {
	atomic.StoreUint64(&c.recvNonce, 0)
}

// MessageTypeName returns a human-readable name for a message type.
func MessageTypeName(t byte) string {
	switch t {
	case MsgFrame:
		return "FRAME"
	case MsgHello:
		return "HELLO"
	case MsgHelloAck:
		return "HELLO_ACK"
	case MsgPing:
		return "PING"
	case MsgPong:
		return "PONG"
	case MsgBye:
		return "BYE"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", t)
	}
}
