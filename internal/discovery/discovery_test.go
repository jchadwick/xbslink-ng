package discovery

import (
	"testing"
)

func TestXboxSystemLinkPortConstant(t *testing.T) {
	// Xbox System Link uses UDP port 3074 (IANA registered)
	if XboxSystemLinkPort != 3074 {
		t.Errorf("XboxSystemLinkPort = %d, want 3074", XboxSystemLinkPort)
	}
}

func TestSnapLenSufficient(t *testing.T) {
	// SnapLen must be at least 14 (Ethernet) + 20 (IP) + 8 (UDP) = 42 bytes
	// We use 128 to capture some payload for potential future use
	minRequired := 42
	if SnapLen < minRequired {
		t.Errorf("SnapLen = %d, want at least %d", SnapLen, minRequired)
	}
}
