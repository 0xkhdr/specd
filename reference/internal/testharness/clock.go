package testharness

import (
	"sync"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// Epoch is the deterministic starting instant for every FakeClock: a fixed,
// human-recognisable UTC time so golden output is stable across runs and hosts.
var Epoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// FakeClock is a deterministic, goroutine-safe time source that replaces
// core.Clock for the duration of a test. By default each call to Now() returns
// the current instant and then advances by Step, guaranteeing that successive
// timestamps (e.g. createdAt vs updatedAt) are distinct yet fully reproducible.
// Call Freeze to make Now() constant.
type FakeClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

// NewFakeClock returns a clock starting at Epoch that auto-advances one second
// per read.
func NewFakeClock() *FakeClock {
	return &FakeClock{now: Epoch, step: time.Second}
}

// Now returns the current instant, then advances by the configured step.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

// Set repositions the clock; the next Now() returns t.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the clock forward by d without consuming a read.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Freeze stops auto-advance so every Now() returns the same instant — useful
// for byte-exact golden timestamp assertions.
func (c *FakeClock) Freeze() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.step = 0
}

// Step sets the per-read auto-advance interval.
func (c *FakeClock) SetStep(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.step = d
}

// install points core.Clock at this fake and returns a restore func. Used by
// the Harness; the lock-staleness logic in core stays on the real wall clock.
func (c *FakeClock) install() func() {
	prev := core.Clock
	core.Clock = c.Now
	return func() { core.Clock = prev }
}
