package analytics

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitFlush waits for one flush attempt to be reported on ch, failing the test
// on timeout.
func waitFlush(t *testing.T, ch chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for flush")
		return nil
	}
}

func newTestTracker(t *testing.T, sink Sink, opts ...Option) (*tracker, *fakeClock, *memStore, chan error) {
	t.Helper()
	clk := newFakeClock()
	store := &memStore{}
	flushCh := make(chan error, 16)
	base := []Option{WithClock(clk), WithStore(store), withFlushNotify(flushCh), WithJitterSource(func() float64 { return 0.5 })}
	tr := New(sink, append(base, opts...)...).(*tracker)
	t.Cleanup(func() { _ = tr.Close() })
	return tr, clk, store, flushCh
}

// TestConsentGate_NothingBeforeEnable proves the consent invariant: while
// disabled, Track/Identify/Flush cause zero sink calls and nothing is spooled.
func TestConsentGate_NothingBeforeEnable(t *testing.T) {
	sink := newFakeSink()
	tr, _, store, _ := newTestTracker(t, sink)

	for i := 0; i < 100; i++ {
		tr.Track("click", map[string]any{"i": i})
	}
	tr.Identify("user-1", nil)
	require.NoError(t, tr.Flush())

	assert.Equal(t, 0, sink.callCount(), "no sink calls while disabled")

	tr.mu.Lock()
	assert.Empty(t, tr.queue, "nothing queued while disabled")
	tr.mu.Unlock()

	data, _ := store.Load()
	assert.Empty(t, data, "nothing spooled to disk while disabled")
}

// TestTrack_NeverSendsSynchronously proves Track never performs a synchronous
// send: with a blocking sink, a batch-threshold-crossing Track still returns
// promptly (it would block if it sent inline).
func TestTrack_NeverSendsSynchronously(t *testing.T) {
	sink := newFakeSink()
	sink.block = make(chan struct{})
	sink.started = make(chan struct{})
	defer close(sink.block)

	tr, _, _, _ := newTestTracker(t, sink, WithBatchSize(5))
	tr.SetEnabled(true)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5; i++ {
			tr.Track("evt", nil) // crossing the threshold must not block
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Track blocked — it performed a synchronous send")
	}

	// The worker did enter Send (asynchronously), proving the flush happened
	// off the Track path.
	select {
	case <-sink.started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never reached the sink")
	}
}

// TestConcurrentTrack_RaceClean hammers Track from many goroutines.
func TestConcurrentTrack_RaceClean(t *testing.T) {
	sink := newFakeSink()
	tr, _, _, _ := newTestTracker(t, sink, WithBatchSize(50), WithMaxSpool(100000))
	tr.SetEnabled(true)

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				tr.Track("evt", map[string]any{"g": g, "i": i})
			}
		}(g)
	}
	wg.Wait()
	require.NoError(t, tr.Flush())
}

// TestBatchSizeFlush verifies the size-triggered flush fires at the threshold.
func TestBatchSizeFlush(t *testing.T) {
	sink := newFakeSink()
	tr, _, _, flushCh := newTestTracker(t, sink, WithBatchSize(3))
	tr.SetEnabled(true)
	waitFlush(t, flushCh) // the enable-time flush (queue empty → no-op)

	tr.Track("a", nil)
	tr.Track("b", nil)
	assert.Equal(t, 0, sink.callCount(), "no flush below threshold")
	tr.Track("c", nil) // crosses threshold
	waitFlush(t, flushCh)
	assert.Equal(t, 1, sink.callCount())
	assert.Len(t, sink.allEvents(), 3)
}

// TestIntervalFlush verifies the interval timer flushes, deterministically via
// the fake clock (no real sleeps).
func TestIntervalFlush(t *testing.T) {
	sink := newFakeSink()
	tr, clk, _, flushCh := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)
	waitFlush(t, flushCh)

	tr.Track("a", nil)
	assert.Equal(t, 0, sink.callCount(), "no flush before interval")

	clk.Advance(DefaultInterval)
	clk.Tick()
	waitFlush(t, flushCh)
	assert.Equal(t, 1, sink.callCount(), "interval flush delivered")
}

// TestIntervalSuspendedWhileBackground verifies the ticker is stopped on
// background so a tick does not flush.
func TestIntervalSuspendedWhileBackground(t *testing.T) {
	sink := newFakeSink()
	tr, clk, _, flushCh := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)
	waitFlush(t, flushCh)

	tr.Track("a", nil)
	tr.signal(ctrlSuspend)
	// Observe the suspend deterministically: the ticker is stopped.
	require.Eventually(t, clk.firstTickerStopped, time.Second, time.Millisecond)

	clk.Advance(DefaultInterval)
	clk.Tick() // stopped ticker does not fire

	select {
	case <-flushCh:
		t.Fatal("interval fired while backgrounded")
	case <-time.After(200 * time.Millisecond):
	}
	assert.Equal(t, 0, sink.callCount(), "queued event stays unsent while backgrounded")
	tr.mu.Lock()
	assert.Len(t, tr.queue, 1, "event retained for next foreground/launch")
	tr.mu.Unlock()
}

func drain(ch chan error) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// TestDisableClearsSpool verifies disabling drops the queue and clears disk.
func TestDisableClearsSpool(t *testing.T) {
	sink := newFakeSink()
	// Every send fails, so the flush SetEnabled(true) asks for cannot take the
	// events off the spool. It ran after the Track calls at times, and the
	// spool then held no events: the JSON spool still read as "null", so the
	// check below passed without anything spooled.
	sink.failNext = 1000
	tr, _, store, _ := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)
	tr.Track("a", nil)
	tr.Track("b", nil)

	data, _ := store.Load()
	spooled, err := unmarshalSpool(data)
	require.NoError(t, err)
	require.Len(t, spooled, 2, "events spooled while enabled")

	tr.SetEnabled(false)

	tr.mu.Lock()
	assert.Empty(t, tr.queue)
	tr.mu.Unlock()
	data, _ = store.Load()
	assert.Empty(t, data, "spool cleared on disable")
}

// TestDisableDuringSend_ClearsSpool is the deterministic regression for the
// drain-race behind the flaky TestDisableClearsSpool: if consent is withdrawn
// while a flush's send is in flight, the post-send bookkeeping must not
// re-persist the (now-dropped) queue — which marshalled the nil queue to "null"
// and landed after SetEnabled(false)'s store.Clear(), resurrecting a spool the
// user opted out of. The blocking sink pins the send open across SetEnabled(false).
func TestDisableDuringSend_ClearsSpool(t *testing.T) {
	sink := newFakeSink()
	sink.block = make(chan struct{})
	sink.started = make(chan struct{})

	tr, _, store, flushCh := newTestTracker(t, sink, WithBatchSize(2))
	tr.SetEnabled(true)
	waitFlush(t, flushCh) // enable-time flush (empty queue → no-op send)

	tr.Track("a", nil)
	tr.Track("b", nil) // crosses the threshold → worker flush enters Send

	// Wait until the worker is inside the (blocked) send: the batch is taken and
	// the queue drained, but the disk spool still holds [a, b].
	select {
	case <-sink.started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never reached the sink")
	}

	// Withdraw consent mid-send: drops the queue and clears the spool.
	tr.SetEnabled(false)

	// Complete the send; doFlush re-locks and must observe the disabled state
	// rather than persisting the nil queue as "null".
	close(sink.block)
	waitFlush(t, flushCh)

	data, _ := store.Load()
	assert.Empty(t, data, "spool must stay cleared when disabled mid-send")

	tr.mu.Lock()
	assert.Empty(t, tr.queue, "queue must stay empty when disabled mid-send")
	tr.mu.Unlock()
}

// TestSpoolRoundTrip simulates a killed session draining on next launch, in
// order. The store is shared between the two trackers to model persistence.
func TestSpoolRoundTrip(t *testing.T) {
	store := &memStore{}
	clk := newFakeClock()

	// Session 1: enqueue, then "die" (Close without flushing).
	flushCh1 := make(chan error, 8)
	tr1 := New(newFakeSink(), WithClock(clk), WithStore(store), WithBatchSize(1000), withFlushNotify(flushCh1)).(*tracker)
	tr1.SetEnabled(true)
	waitFlush(t, flushCh1) // enable-time flush runs on an empty queue first
	for i := 0; i < 5; i++ {
		tr1.Track("evt", map[string]any{"seq": i})
	}
	require.NoError(t, tr1.Close()) // simulated kill: no flush

	require.NotEmpty(t, store.data, "events survived on disk")

	// Session 2: relaunch, drain in order.
	sink2 := newFakeSink()
	flushCh := make(chan error, 8)
	tr2 := New(sink2, WithClock(clk), WithStore(store), WithBatchSize(1000), withFlushNotify(flushCh)).(*tracker)
	t.Cleanup(func() { _ = tr2.Close() })
	tr2.SetEnabled(true) // drains spool + triggers flush
	waitFlush(t, flushCh)

	got := sink2.allEvents()
	require.Len(t, got, 5)
	for i, e := range got {
		assert.EqualValues(t, i, e.Props["seq"], "drained oldest-first, in order")
	}
}

// TestBackoffRetainsAndGates verifies a failed flush keeps events and gates the
// next non-forced flush until the backoff elapses (deterministic clock).
func TestBackoffRetainsAndGates(t *testing.T) {
	sink := newFakeSink()
	tr, clk, store, flushCh := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)
	waitFlush(t, flushCh) // drain the enable-time flush (empty queue, no send)

	sink.mu.Lock()
	sink.failNext = 1
	sink.mu.Unlock()
	tr.Track("a", nil)

	// First flush fails; events retained, backoff armed.
	err := tr.Flush()
	require.Error(t, err)
	tr.mu.Lock()
	assert.Len(t, tr.queue, 1, "failed events stay in the spool")
	assert.False(t, tr.nextRetryAt.IsZero(), "backoff armed")
	tr.mu.Unlock()
	data, _ := store.Load()
	assert.NotEmpty(t, data, "still spooled after failure")

	// A non-forced flush before backoff elapses is gated (no sink call).
	before := sink.callCount()
	assert.NoError(t, tr.doFlush(false))
	assert.Equal(t, before, sink.callCount(), "gated by backoff")

	// After the backoff window, a non-forced flush succeeds.
	clk.Advance(DefaultBackoffMax)
	require.NoError(t, tr.doFlush(false))
	assert.Len(t, sink.allEvents(), 1)
	tr.mu.Lock()
	assert.Empty(t, tr.queue, "queue cleared after successful flush")
	assert.True(t, tr.nextRetryAt.IsZero(), "backoff reset")
	tr.mu.Unlock()
}

// TestMaxSpoolEviction verifies oldest-first eviction at the cap.
func TestMaxSpoolEviction(t *testing.T) {
	sink := newFakeSink()
	tr, _, _, _ := newTestTracker(t, sink, WithBatchSize(100000), WithMaxSpool(10))
	// Enable directly rather than via SetEnabled: this isolates the eviction
	// invariant (a synchronous enqueue/evictLocked property) from the worker,
	// whose flush would otherwise race the loop and drain the successful sink.
	// (Worker flush behaviour is covered by TestSpoolRoundTrip / backoff tests.)
	tr.mu.Lock()
	tr.enabled = true
	tr.mu.Unlock()
	for i := 0; i < 25; i++ {
		tr.Track("evt", map[string]any{"seq": i})
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	require.Len(t, tr.queue, 10)
	assert.EqualValues(t, 15, tr.queue[0].Props["seq"], "oldest evicted first")
	assert.EqualValues(t, 24, tr.queue[9].Props["seq"])
}

// TestIdentifyDisabledDrops verifies Identify is gated by consent too.
func TestIdentifyDisabledDrops(t *testing.T) {
	sink := newFakeSink()
	tr, _, store, _ := newTestTracker(t, sink)
	tr.Identify("user-1", map[string]any{"plan": "pro"})
	tr.mu.Lock()
	assert.Empty(t, tr.queue)
	assert.Empty(t, tr.userID, "no identity retained while disabled")
	tr.mu.Unlock()
	data, _ := store.Load()
	assert.Empty(t, data)
}

// TestNoDeviceIdentifierGenerated verifies session ids differ per session and
// the package never fabricates a stable id: events carry only the ephemeral
// session id (and an app-supplied user id if Identify was called).
func TestNoDeviceIdentifierGenerated(t *testing.T) {
	sink := newFakeSink()
	tr, _, _, flushCh := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)

	s1 := tr.sessionID
	tr.startSession()
	s2 := tr.sessionID
	assert.NotEqual(t, s1, s2, "session id rotates per session")

	tr.Track("evt", nil)
	require.NoError(t, tr.Flush())
	drain(flushCh)
	for _, e := range sink.allEvents() {
		assert.Empty(t, e.UserID, "no app identity → no user id")
		assert.NotEmpty(t, e.SessionID, "ephemeral session id only")
	}
}
