// Package modeltime implements the Simulator-owned model clock
// (design-specification.md section 4.3, ADR-003). Model time is distinct
// from wall-clock time and is never inferred by another component from its
// own local clock.
package modeltime

import (
	"sync"
	"time"
)

// Layout is the RFC3339 representation used for model_time on the wire
// (design-specification.md section 4.3 example).
const Layout = time.RFC3339

// Format renders t as a model_time string.
func Format(t time.Time) string {
	return t.UTC().Format(Layout)
}

// Parse parses a model_time string.
func Parse(s string) (time.Time, error) {
	return time.Parse(Layout, s)
}

// Clock is the Simulator's authoritative model clock. It supports
// deterministic advancement so tests do not need to wait on real time for
// model-time-based behavior such as the REQ-FUNC-003 relock timeout
// (design-specification.md section 4.3.3).
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock creates a Clock starting at start (the Scenario's initial model
// time, design-specification.md section 3.5).
func NewClock(start time.Time) *Clock {
	return &Clock{now: start}
}

// Now returns the current model time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the model clock forward by d. It is a test/scenario-control
// operation, not something a device process does to itself.
func (c *Clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}

// Set positions the model clock at t. Like Advance, this is a
// test/scenario-control operation: a named Scenario defines the initial
// model time it runs from (design-specification.md section 3.5), so
// starting one must be able to (re)position the clock there deterministically
// rather than only moving it forward from wherever it already was.
func (c *Clock) Set(t time.Time) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
	return c.now
}
