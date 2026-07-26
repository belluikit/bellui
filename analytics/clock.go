package analytics

import "time"

// Clock abstracts time so the interval-flush timer and backoff scheduling are
// deterministic in tests. The default [realClock] uses the standard library.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// NewTicker returns a ticker firing every d. The tracker stops and
	// recreates the ticker across background/foreground transitions.
	NewTicker(d time.Duration) Ticker
}

// Ticker mirrors the subset of *time.Ticker the tracker needs.
type Ticker interface {
	// C is the channel on which ticks are delivered.
	C() <-chan time.Time
	// Stop halts the ticker.
	Stop()
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) NewTicker(d time.Duration) Ticker { return &realTicker{t: time.NewTicker(d)} }

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }

func (r *realTicker) Stop() { r.t.Stop() }
