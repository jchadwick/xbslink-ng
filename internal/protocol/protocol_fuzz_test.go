package protocol

import (
	"bytes"
	"testing"
)

func FuzzDecode(f *testing.F) {
	// Add some seed corpus
	f.Add([]byte{MsgFrame})
	f.Add([]byte{MsgHello})
	f.Add([]byte{MsgPing, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{MsgBye})
	f.Add([]byte{0xFF}) // Unknown type

	codec := NewCodec(nil)

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		_, _ = codec.Decode(data)
	})
}

func FuzzDecodeSecure(f *testing.F) {
	codec := NewCodec(testKey)
	
	// Generate valid seeds
	frame := makeTestFrame(64)
	encoded, _ := codec.EncodeFrame(frame)
	f.Add(encoded)

	ping := codec.EncodePing(12345)
	f.Add(ping)

	bye := codec.EncodeBye()
	f.Add(bye)

	// Reset codec for fuzzing
	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzCodec := NewCodec(testKey)
		// Should not panic
		_, _ = fuzzCodec.Decode(data)
	})
}

func FuzzEncodeDecodeFrame(f *testing.F) {
	// Add frame payloads as seeds
	f.Add(makeTestFrame(14))  // Min size
	f.Add(makeTestFrame(64))
	f.Add(makeTestFrame(1500))

	f.Fuzz(func(t *testing.T, frame []byte) {
		if len(frame) < MinEthernetFrame || len(frame) > MaxFrameSize {
			return // Skip invalid sizes
		}

		codec := NewCodec(nil)
		encoded, err := codec.EncodeFrame(frame)
		if err != nil {
			return // Invalid frame
		}

		msg, err := codec.Decode(encoded)
		if err != nil {
			t.Fatalf("decode failed after successful encode: %v", err)
		}

		if !bytes.Equal(msg.Frame, frame) {
			t.Error("frame content mismatch after roundtrip")
		}
	})
}
