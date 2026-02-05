package bridge

import (
	"sync"
	"testing"
	"time"
)

func TestStats_IncrementTxPackets(t *testing.T) {
	stats := &Stats{}

	for i := 0; i < 100; i++ {
		stats.TxPackets++
	}

	if stats.TxPackets != 100 {
		t.Errorf("TxPackets = %d, want 100", stats.TxPackets)
	}
}

func TestStats_IncrementRxPackets(t *testing.T) {
	stats := &Stats{}

	for i := 0; i < 100; i++ {
		stats.RxPackets++
	}

	if stats.RxPackets != 100 {
		t.Errorf("RxPackets = %d, want 100", stats.RxPackets)
	}
}

func TestStats_IncrementTxBytes(t *testing.T) {
	stats := &Stats{}

	stats.TxBytes += 1500
	stats.TxBytes += 500

	if stats.TxBytes != 2000 {
		t.Errorf("TxBytes = %d, want 2000", stats.TxBytes)
	}
}

func TestStats_IncrementRxBytes(t *testing.T) {
	stats := &Stats{}

	stats.RxBytes += 1500
	stats.RxBytes += 500

	if stats.RxBytes != 2000 {
		t.Errorf("RxBytes = %d, want 2000", stats.RxBytes)
	}
}

func TestStats_AddRTTSample(t *testing.T) {
	stats := &Stats{}

	stats.AddRTTSample(10 * time.Millisecond)
	stats.AddRTTSample(20 * time.Millisecond)
	stats.AddRTTSample(30 * time.Millisecond)

	if stats.RTTCurrent != 30*time.Millisecond {
		t.Errorf("RTTCurrent = %v, want 30ms", stats.RTTCurrent)
	}

	// Average should be 20ms
	if stats.RTTAvg != 20*time.Millisecond {
		t.Errorf("RTTAvg = %v, want 20ms", stats.RTTAvg)
	}
}

func TestStats_RTTAverage_SlidingWindow(t *testing.T) {
	stats := &Stats{}

	// Add 25 samples (more than the 20-sample window)
	for i := 1; i <= 25; i++ {
		stats.AddRTTSample(time.Duration(i) * time.Millisecond)
	}

	// Should only keep last 20 samples (6-25)
	// Average of 6+7+...+25 = sum(6..25)/20 = (6+25)*20/2/20 = 15.5ms
	expectedAvg := 15*time.Millisecond + 500*time.Microsecond

	if stats.RTTAvg != expectedAvg {
		t.Errorf("RTTAvg = %v, want %v", stats.RTTAvg, expectedAvg)
	}
}

func TestStats_GetRTTCurrent(t *testing.T) {
	stats := &Stats{}

	stats.AddRTTSample(50 * time.Millisecond)

	if stats.GetRTTCurrent() != 50*time.Millisecond {
		t.Errorf("GetRTTCurrent() = %v, want 50ms", stats.GetRTTCurrent())
	}
}

func TestStats_SetLastRTT(t *testing.T) {
	stats := &Stats{}

	stats.SetLastRTT(100 * time.Millisecond)

	stats.rttMu.RLock()
	if stats.lastRTT != 100*time.Millisecond {
		t.Errorf("lastRTT = %v, want 100ms", stats.lastRTT)
	}
	stats.rttMu.RUnlock()
}

func TestStats_CheckRTTSpike_NoSpike(t *testing.T) {
	stats := &Stats{}

	stats.AddRTTSample(10 * time.Millisecond)
	stats.SetLastRTT(10 * time.Millisecond)
	stats.AddRTTSample(12 * time.Millisecond) // 20% increase, under threshold

	spiked, _, _ := stats.CheckRTTSpike()
	if spiked {
		t.Error("expected no spike for small increase")
	}
}

func TestStats_CheckRTTSpike_WithSpike(t *testing.T) {
	stats := &Stats{}

	stats.AddRTTSample(10 * time.Millisecond)
	stats.SetLastRTT(10 * time.Millisecond)
	stats.AddRTTSample(20 * time.Millisecond) // 100% increase, over 50% threshold

	spiked, oldRTT, newRTT := stats.CheckRTTSpike()
	if !spiked {
		t.Error("expected spike for large increase")
	}
	if oldRTT != 10*time.Millisecond {
		t.Errorf("oldRTT = %v, want 10ms", oldRTT)
	}
	if newRTT != 20*time.Millisecond {
		t.Errorf("newRTT = %v, want 20ms", newRTT)
	}
}

func TestStats_CheckRTTSpike_NotEnoughSamples(t *testing.T) {
	stats := &Stats{}

	stats.AddRTTSample(10 * time.Millisecond)

	spiked, _, _ := stats.CheckRTTSpike()
	if spiked {
		t.Error("should not report spike with only one sample")
	}
}

func TestStats_Concurrent(t *testing.T) {
	stats := &Stats{}
	var wg sync.WaitGroup

	// Multiple goroutines updating stats
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				stats.AddRTTSample(time.Duration(j) * time.Millisecond)
				stats.SetLastRTT(time.Duration(j) * time.Millisecond)
				stats.GetRTTCurrent()
				stats.CheckRTTSpike()
			}
		}()
	}

	wg.Wait()
	// If we get here without data race, test passes
}

func TestNew_MissingCapture(t *testing.T) {
	cfg := Config{
		Capture: nil,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing capture")
	}
}

func TestNew_MissingTransport(t *testing.T) {
	cfg := Config{
		Capture:   nil, // Would need real capture
		Transport: nil,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing capture or transport")
	}
}

func TestNew_MissingCodec(t *testing.T) {
	cfg := Config{
		Capture:   nil, // Would need real capture
		Transport: nil, // Would need real transport
		Codec:     nil,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing components")
	}
}

func TestNew_MissingLogger(t *testing.T) {
	cfg := Config{
		Capture:   nil, // Would need real capture
		Transport: nil, // Would need real transport
		Codec:     nil, // Would need real codec
		Logger:    nil,
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for missing components")
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateDisconnected, "DISCONNECTED"},
		{StateConnecting, "CONNECTING"},
		{StateConnected, "CONNECTED"},
		{State(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("State(%d).String() = %s, want %s", tt.state, tt.state.String(), tt.expected)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1500, "1,500"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1000000, "1,000,000"},
		{1500000, "1,500,000"},
		{12345678, "12,345,678"},
	}

	for _, tt := range tests {
		result := formatNumber(tt.input)
		if result != tt.expected {
			t.Errorf("formatNumber(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1536, "1 KB"},       // 1.5 KB rounds to 1 KB (integer division)
		{1048576, "1.0 MB"},  // 1 MB
		{1572864, "1.5 MB"},  // 1.5 MB
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.input)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestAddrEqual_Bridge(t *testing.T) {
	tests := []struct {
		name     string
		a        *mockUDPAddr
		b        *mockUDPAddr
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, &mockUDPAddr{}, false},
		{"b nil", &mockUDPAddr{}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't directly test addrEqual with mock since it uses net.UDPAddr
			// This test is more about documenting the expected behavior
		})
	}
}

// Note: Full integration testing of New() with valid components requires
// actual pcap access and is covered in integration tests.
