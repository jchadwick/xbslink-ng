package testutil

import (
	"bytes"
	"sync"
)

// MockLogger captures log output for testing.
type MockLogger struct {
	mu       sync.Mutex
	Messages []LogMessage
	Output   bytes.Buffer
}

// LogMessage represents a captured log message.
type LogMessage struct {
	Level   string
	Message string
}

// NewMockLogger creates a new mock logger.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		Messages: make([]LogMessage, 0),
	}
}

// Log captures a log message.
func (m *MockLogger) Log(level, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, LogMessage{Level: level, Message: message})
}

// GetMessages returns all captured messages.
func (m *MockLogger) GetMessages() []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]LogMessage, len(m.Messages))
	copy(result, m.Messages)
	return result
}

// Clear clears all captured messages.
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = m.Messages[:0]
}

// HasMessage checks if a message with the given level and substring exists.
func (m *MockLogger) HasMessage(level, substr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.Messages {
		if msg.Level == level && bytes.Contains([]byte(msg.Message), []byte(substr)) {
			return true
		}
	}
	return false
}
