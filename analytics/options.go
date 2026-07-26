package analytics

import "time"

// Default battery-model tuning. The defaults are deliberately radio-
// conservative (see the package doc).
const (
	// DefaultBatchSize is the queue length that triggers a flush.
	DefaultBatchSize = 30
	// DefaultInterval is the foreground flush interval.
	DefaultInterval = 60 * time.Second
	// DefaultMaxSpool is the maximum number of events retained; the oldest are
	// evicted first once the cap is exceeded.
	DefaultMaxSpool = 1000
	// DefaultBackoffBase is the first retry delay after a failed flush.
	DefaultBackoffBase = 5 * time.Second
	// DefaultBackoffMax caps the exponential retry delay.
	DefaultBackoffMax = 10 * time.Minute
)

// config holds the resolved tracker settings.
type config struct {
	batchSize   int
	interval    time.Duration
	maxSpool    int
	backoffBase time.Duration
	backoffMax  time.Duration
	store       Store
	clock       Clock
	rand        func() float64 // jitter source in [0,1)

	// flushNotify, when non-nil, receives one value after every flush attempt.
	// Test-only hook (see withFlushNotify).
	flushNotify chan error
}

func defaultConfig() config {
	return config{
		batchSize:   DefaultBatchSize,
		interval:    DefaultInterval,
		maxSpool:    DefaultMaxSpool,
		backoffBase: DefaultBackoffBase,
		backoffMax:  DefaultBackoffMax,
		clock:       realClock{},
	}
}

// Option configures a [Tracker] created by [New].
type Option func(*config)

// WithBatchSize sets the queue length that triggers a flush (default 30).
// A value <= 0 leaves the default.
func WithBatchSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

// WithInterval sets the foreground flush interval (default 60s). The timer is
// suspended while the app is backgrounded. A value <= 0 leaves the default.
func WithInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithMaxSpool bounds the number of events retained across the in-memory ring
// and the disk spool (default 1000). Once exceeded, the oldest events are
// evicted first. A value <= 0 leaves the default.
func WithMaxSpool(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxSpool = n
		}
	}
}

// WithBackoff sets the base and max delays for the exponential-with-jitter
// retry after a failed flush (defaults 5s / 10m). Non-positive values leave the
// respective default.
func WithBackoff(base, max time.Duration) Option {
	return func(c *config) {
		if base > 0 {
			c.backoffBase = base
		}
		if max > 0 {
			c.backoffMax = max
		}
	}
}

// WithStore overrides the disk spool backend. By default the spool is backed by
// the current app's [bellui.Storage] when one exists, or an in-memory (non-
// persistent) store otherwise. Tests use this to inject a fake.
func WithStore(s Store) Option {
	return func(c *config) { c.store = s }
}

// WithClock overrides the time source. Used by tests for deterministic interval
// and backoff behaviour.
func WithClock(clk Clock) Option {
	return func(c *config) {
		if clk != nil {
			c.clock = clk
		}
	}
}

// WithJitterSource overrides the jitter source (a func returning a value in
// [0,1)) used by backoff. Used by tests for determinism.
func WithJitterSource(f func() float64) Option {
	return func(c *config) {
		if f != nil {
			c.rand = f
		}
	}
}

// withFlushNotify is a test-only hook: the given channel receives the result of
// every flush attempt performed by the background worker.
func withFlushNotify(ch chan error) Option {
	return func(c *config) { c.flushNotify = ch }
}
