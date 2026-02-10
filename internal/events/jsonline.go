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

// AsyncJSONLineWriter wraps JSONLineWriter with non-blocking async emission.
// Events are queued to a buffered channel and written by a background goroutine.
// If the buffer is full, events are dropped immediately (UDP mindset: performance over perfection).
type AsyncJSONLineWriter struct {
	events chan Envelope
	done   chan struct{}
	wg     sync.WaitGroup
	w      *JSONLineWriter
}

// NewAsyncJSONLineWriter creates a new AsyncJSONLineWriter that writes to w.
// Events are buffered in a channel with capacity 64.
func NewAsyncJSONLineWriter(w io.Writer) *AsyncJSONLineWriter {
	a := &AsyncJSONLineWriter{
		events: make(chan Envelope, 64),
		done:   make(chan struct{}),
		w:      NewJSONLineWriter(w),
	}
	a.wg.Add(1)
	go a.writer()
	return a
}

// Emit queues an event for async writing.
// If the buffer is full, the event is dropped immediately (non-blocking).
func (a *AsyncJSONLineWriter) Emit(eventType EventType, data interface{}) {
	env := Envelope{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case a.events <- env:
		// Queued successfully (non-blocking)
	default:
		// Buffer full, drop event (non-critical diagnostic data)
	}
}

// writer is the background goroutine that handles potentially-blocking I/O.
func (a *AsyncJSONLineWriter) writer() {
	defer a.wg.Done()
	for {
		select {
		case env := <-a.events:
			a.w.Emit(env.Type, env.Data)
		case <-a.done:
			// Drain remaining events before shutdown
			for len(a.events) > 0 {
				env := <-a.events
				a.w.Emit(env.Type, env.Data)
			}
			return
		}
	}
}

// Close stops the background writer and closes the underlying writer.
// It waits for all queued events to be written before closing.
func (a *AsyncJSONLineWriter) Close() error {
	close(a.done)
	a.wg.Wait()
	return a.w.Close()
}
