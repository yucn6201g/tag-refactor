package testutil

import (
	"sync"
	"time"
)

// FakeClock is a deterministic clock. Every call of Now returns the current
// fake time and then moves it forward by the step, so measured durations are
// predictable without real waiting.
type FakeClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

// NewFakeClock returns a clock that starts at start and advances by step on
// every call of Now.
func NewFakeClock(start time.Time, step time.Duration) *FakeClock {
	return &FakeClock{now: start, step: step}
}

// Now returns the fake time and advances it by the step.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

// Advance moves the fake time forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
