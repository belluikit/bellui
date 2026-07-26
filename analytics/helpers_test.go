package analytics

import (
	"errors"
	"sync"
	"time"
)

// fakeClock is a deterministic [Clock]. Now advances only via Advance, and its
// tickers fire only via Tick — no wall-clock, no sleeps.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *fakeClock) NewTicker(time.Duration) Ticker {
	t := &fakeTicker{ch: make(chan time.Time, 1)}
	c.mu.Lock()
	c.tickers = append(c.tickers, t)
	c.mu.Unlock()
	return t
}

// Tick fires one tick on every live ticker.
func (c *fakeClock) Tick() {
	c.mu.Lock()
	now := c.now
	live := make([]*fakeTicker, 0, len(c.tickers))
	for _, t := range c.tickers {
		if !t.stopped() {
			live = append(live, t)
		}
	}
	c.mu.Unlock()
	for _, t := range live {
		select {
		case t.ch <- now:
		default:
		}
	}
}

// firstTickerStopped reports whether the earliest-created ticker has been
// stopped (used to observe a suspend deterministically).
func (c *fakeClock) firstTickerStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.tickers) == 0 {
		return false
	}
	return c.tickers[0].stopped()
}

type fakeTicker struct {
	ch   chan time.Time
	mu   sync.Mutex
	done bool
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }

func (t *fakeTicker) Stop() {
	t.mu.Lock()
	t.done = true
	t.mu.Unlock()
}

func (t *fakeTicker) stopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done
}

// fakeSink records batches and counts Send calls; optionally fails or blocks.
type fakeSink struct {
	mu       sync.Mutex
	batches  [][]Event
	calls    int
	failNext int           // number of upcoming Send calls to fail
	block    chan struct{} // when non-nil, Send blocks until closed
	started  chan struct{} // closed on first Send entry
}

func newFakeSink() *fakeSink { return &fakeSink{} }

func (s *fakeSink) Send(batch []Event) error {
	s.mu.Lock()
	s.calls++
	if s.started != nil {
		select {
		case <-s.started:
		default:
			close(s.started)
		}
	}
	block := s.block
	fail := s.failNext > 0
	if fail {
		s.failNext--
	}
	s.mu.Unlock()

	if block != nil {
		<-block
	}
	if fail {
		return errors.New("fake sink failure")
	}

	cp := make([]Event, len(batch))
	copy(cp, batch)
	s.mu.Lock()
	s.batches = append(s.batches, cp)
	s.mu.Unlock()
	return nil
}

func (s *fakeSink) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *fakeSink) allEvents() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, b := range s.batches {
		out = append(out, b...)
	}
	return out
}
