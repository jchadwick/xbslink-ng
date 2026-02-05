package protocol

import (
	"testing"
)

func BenchmarkEncodeFrame_64(b *testing.B) {
	codec := NewCodec(nil)
	frame := makeTestFrame(64)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.EncodeFrame(frame)
	}
}

func BenchmarkEncodeFrame_1500(b *testing.B) {
	codec := NewCodec(nil)
	frame := makeTestFrame(1500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.EncodeFrame(frame)
	}
}

func BenchmarkEncodeFrame_Secure_64(b *testing.B) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(64)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.EncodeFrame(frame)
	}
}

func BenchmarkEncodeFrame_Secure_1500(b *testing.B) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(1500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.EncodeFrame(frame)
	}
}

func BenchmarkDecodeFrame_64(b *testing.B) {
	codec := NewCodec(nil)
	frame := makeTestFrame(64)
	encoded, _ := codec.EncodeFrame(frame)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Decode(encoded)
	}
}

func BenchmarkDecodeFrame_1500(b *testing.B) {
	codec := NewCodec(nil)
	frame := makeTestFrame(1500)
	encoded, _ := codec.EncodeFrame(frame)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Decode(encoded)
	}
}

func BenchmarkDecodeFrame_Secure_64(b *testing.B) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(64)
	encoded, _ := codec.EncodeFrame(frame)

	// Reset nonce for each iteration
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.ResetRecvNonce()
		_, _ = codec.Decode(encoded)
	}
}

func BenchmarkDecodeFrame_Secure_1500(b *testing.B) {
	codec := NewCodec(testKey)
	frame := makeTestFrame(1500)
	encoded, _ := codec.EncodeFrame(frame)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.ResetRecvNonce()
		_, _ = codec.Decode(encoded)
	}
}

func BenchmarkEncodePing(b *testing.B) {
	codec := NewCodec(nil)
	timestamp := int64(1234567890)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = codec.EncodePing(timestamp)
	}
}

func BenchmarkEncodePong(b *testing.B) {
	codec := NewCodec(nil)
	timestamp := int64(1234567890)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = codec.EncodePong(timestamp)
	}
}

func BenchmarkHMAC_Compute(b *testing.B) {
	codec := NewCodec(testKey)
	data := makeTestFrame(1500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = codec.computeHMAC(data)
	}
}
