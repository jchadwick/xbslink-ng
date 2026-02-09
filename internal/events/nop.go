package events

// NopEmitter is a no-op emitter that discards all events.
// It has zero overhead when events are disabled.
type NopEmitter struct{}

// Emit does nothing.
func (NopEmitter) Emit(EventType, interface{}) {}

// Close does nothing and returns nil.
func (NopEmitter) Close() error { return nil }
