package events

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// JSONLineWriter writes JSON Lines (one JSON object per line) to an io.Writer.
// It is safe for concurrent use.
type JSONLineWriter struct {
	mu  sync.Mutex
	enc *json.Encoder
	w   io.Writer
}

// NewJSONLineWriter creates a new JSONLineWriter that writes to w.
func NewJSONLineWriter(w io.Writer) *JSONLineWriter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONLineWriter{enc: enc, w: w}
}

// Emit writes a JSON line with the event envelope.
// Encoding errors are silently dropped; events must never block the bridge.
func (j *JSONLineWriter) Emit(eventType EventType, data interface{}) {
	env := Envelope{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	// Silently drop errors — events are diagnostic, not critical
	_ = j.enc.Encode(env)
}

// Close closes the underlying writer if it implements io.Closer.
func (j *JSONLineWriter) Close() error {
	if c, ok := j.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
