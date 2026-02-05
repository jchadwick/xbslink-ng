package capture

import (
	"testing"
)

func FuzzParseMAC(f *testing.F) {
	// Add seeds
	f.Add("AA:BB:CC:DD:EE:FF")
	f.Add("AA-BB-CC-DD-EE-FF")
	f.Add("00:50:F2:12:34:56")
	f.Add("FF:FF:FF:FF:FF:FF")
	f.Add("")
	f.Add("invalid")
	f.Add("GG:HH:II:JJ:KK:LL")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_, _ = ParseMAC(input)
	})
}

func FuzzDecodeEthernetFrame(f *testing.F) {
	// Add seeds
	f.Add(make([]byte, 14))
	f.Add(make([]byte, 64))
	f.Add(make([]byte, 1500))
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		_, _, _ = DecodeEthernetFrame(data)
	})
}
