package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// LatencyConfig provides thread-safe configurable latency for PONG replies.
type LatencyConfig struct {
	mu     sync.Mutex
	base   time.Duration
	jitter time.Duration
	step   time.Duration
	rng    *rand.Rand
}

// NewLatencyConfig creates a new LatencyConfig.
func NewLatencyConfig(base, jitter, step time.Duration) *LatencyConfig {
	return &LatencyConfig{
		base:   base,
		jitter: jitter,
		step:   step,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Delay returns a random duration: base + uniform(-jitter, +jitter), clamped >= 0.
func (lc *LatencyConfig) Delay() time.Duration {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	d := lc.base
	if lc.jitter > 0 {
		jitterRange := int64(lc.jitter) * 2
		d += time.Duration(-int64(lc.jitter) + lc.rng.Int63n(jitterRange+1))
	}
	if d < 0 {
		d = 0
	}
	return d
}

// IncreaseBase adds step to the base latency and returns the new value.
func (lc *LatencyConfig) IncreaseBase() time.Duration {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.base += lc.step
	return lc.base
}

// DecreaseBase subtracts step from the base latency (min 0) and returns the new value.
func (lc *LatencyConfig) DecreaseBase() time.Duration {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.base -= lc.step
	if lc.base < 0 {
		lc.base = 0
	}
	return lc.base
}

// String returns a human-readable summary.
func (lc *LatencyConfig) String() string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return fmt.Sprintf("base=%s jitter=±%s step=%s", lc.base, lc.jitter, lc.step)
}
