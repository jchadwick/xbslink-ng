package events

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestJSONLineWriter_Emit(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLineWriter(&buf)

	w.Emit(EventStateChanged, StateChangedData{State: "connected", PeerAddr: "1.2.3.4:31415"})

	line := strings.TrimSpace(buf.String())
	var env Envelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		t.Fatalf("failed to parse JSON line: %v", err)
	}

	if env.Type != EventStateChanged {
		t.Errorf("type = %q, want %q", env.Type, EventStateChanged)
	}
	if env.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}

	// Data is decoded as map[string]interface{} by default
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map, got %T", env.Data)
	}
	if data["state"] != "connected" {
		t.Errorf("data.state = %v, want connected", data["state"])
	}
	if data["peer_addr"] != "1.2.3.4:31415" {
		t.Errorf("data.peer_addr = %v, want 1.2.3.4:31415", data["peer_addr"])
	}
}

func TestJSONLineWriter_MultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLineWriter(&buf)

	w.Emit(EventStats, StatsData{TxPackets: 100, RxPackets: 200})
	w.Emit(EventLatency, LatencyData{RTTMs: 8.5, IsSpike: false})
	w.Emit(EventDiscovery, DiscoveryData{MAC: "00:50:F2:1A:2B:3C"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var env Envelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Errorf("line %d: failed to parse: %v", i, err)
		}
	}
}

func TestJSONLineWriter_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLineWriter(&buf)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Emit(EventLatency, LatencyData{RTTMs: 5.0})
		}()
	}

	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 50 {
		t.Errorf("got %d lines, want 50", len(lines))
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var env Envelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestJSONLineWriter_ErrorEventPayload(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLineWriter(&buf)

	w.Emit(EventError, ErrorData{Message: "peer unresponsive"})

	var env Envelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &env); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if env.Type != EventError {
		t.Errorf("type = %q, want %q", env.Type, EventError)
	}
}

func TestJSONLineWriter_Close_WithCloser(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONLineWriter(&buf)

	// bytes.Buffer doesn't implement io.Closer, so Close returns nil
	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestNopEmitter_Emit(t *testing.T) {
	var nop NopEmitter
	// Should not panic
	nop.Emit(EventStateChanged, StateChangedData{State: "connected"})
	nop.Emit(EventStats, nil)
}

func TestNopEmitter_Close(t *testing.T) {
	var nop NopEmitter
	if err := nop.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// Verify interface compliance at compile time.
var _ Emitter = (*JSONLineWriter)(nil)
var _ Emitter = NopEmitter{}
