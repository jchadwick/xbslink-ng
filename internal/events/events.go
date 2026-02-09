// Package events provides structured event emission for diagnostics.
package events

import "time"

// EventType identifies the kind of event.
type EventType string

const (
	EventStateChanged EventType = "state_changed"
	EventStats        EventType = "stats"
	EventLatency      EventType = "latency"
	EventDiscovery    EventType = "discovery"
	EventError        EventType = "error"
)

// Envelope wraps every emitted event with type and timestamp.
type Envelope struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// StateChangedData is the payload for state_changed events.
type StateChangedData struct {
	State    string `json:"state"`
	PeerAddr string `json:"peer_addr,omitempty"`
}

// StatsData is the payload for stats events.
type StatsData struct {
	TxPackets    uint64  `json:"tx_packets"`
	TxBytes      uint64  `json:"tx_bytes"`
	RxPackets    uint64  `json:"rx_packets"`
	RxBytes      uint64  `json:"rx_bytes"`
	RTTCurrentMs float64 `json:"rtt_current_ms"`
	RTTAvgMs     float64 `json:"rtt_avg_ms"`
}

// LatencyData is the payload for latency events.
type LatencyData struct {
	RTTMs            float64 `json:"rtt_ms"`
	IsSpike          bool    `json:"is_spike"`
	ExceedsThreshold bool    `json:"exceeds_threshold"`
}

// DiscoveryData is the payload for discovery events.
type DiscoveryData struct {
	MAC string `json:"mac"`
}

// ErrorData is the payload for error events.
type ErrorData struct {
	Message string `json:"message"`
}

// Emitter is the interface for emitting structured events.
type Emitter interface {
	Emit(eventType EventType, data interface{})
	Close() error
}
