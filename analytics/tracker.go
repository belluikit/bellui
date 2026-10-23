package analytics

import (
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/belluikit/bellui"
)

// worker control commands.
type ctrl int

const (
	ctrlFlush   ctrl = iota // size-triggered flush request
	ctrlSuspend             // suspend the interval ticker (backgrounded)
	ctrlResume              // (re)start the interval ticker (foregrounded)
	ctrlStop                // stop the worker
)

// tracker is the concrete [Tracker].
type tracker struct {
	sink Sink
	cfg  config

	mu      sync.Mutex // guards the fields below
	enabled bool
	loaded  bool // spool has been drained from disk since enabling
	queue   []Event
	// inflight is the batch a send has taken out of queue, until the send's
	// outcome is known. The spool keeps it ahead of queue meanwhile, so a crash
	// mid-send still finds it on the next launch (#670).
	inflight []Event
	// queuePieces and inflightPieces hold each event's spool encoding, index
	// for index with queue and inflight, so a save joins them rather than
	// encoding every queued event again (#657). A nil piece (its encoding
	// failed) makes the save encode the whole spool instead.
	queuePieces    [][]byte
	inflightPieces [][]byte
	// backoff state
	attempt     int
	nextRetryAt time.Time
	sessionID   string
	userID      string

	flushMu sync.Mutex // serializes sink.Send across all flush paths

	ctrlCh       chan ctrl
	closeOnce    sync.Once
	persistErrer sync.Once // log a spool-persist failure at most once
}

var _ Tracker = (*tracker)(nil)

// New returns a consent-gated [Tracker] delivering batches to sink. The tracker
// starts DISABLED (see the package doc's consent invariant): call SetEnabled
// (true) once the app has consent.
//
// When no [WithStore] option is given, the spool is backed by the current app's
// storage sandbox if a [bellui.App] exists, otherwise by a non-persistent in-
// memory store.
func New(sink Sink, opts ...Option) Tracker {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.store == nil {
		cfg.store = defaultStore()
	}
	if cfg.rand == nil {
		cfg.rand = rand.Float64
	}
	t := &tracker{
		sink:   sink,
		cfg:    cfg,
		ctrlCh: make(chan ctrl, 8),
	}
	t.sessionID = t.newSessionID()
	go t.run()
	return t
}

// defaultStore backs the spool with app storage when an app exists, else memory.
func defaultStore() Store {
	if app := bellui.CurrentApp(); app != nil {
		if s := app.Storage(); s != nil {
			return newBelluiStore(s)
		}
	}
	return &memStore{}
}

// newSessionID returns an ephemeral per-run identifier. It is NOT persisted as
// an identity and is regenerated every launch, so it is not a device/install ID.
func (t *tracker) newSessionID() string {
	return strconv.FormatInt(t.cfg.clock.Now().UnixNano(), 36) + "-" + strconv.FormatUint(rand.Uint64(), 36)
}

// Track implements [Tracker]. It never performs network I/O.
func (t *tracker) Track(event string, props map[string]any) {
	t.enqueue(Event{
		Kind:      KindTrack,
		Name:      event,
		Props:     props,
		Timestamp: t.cfg.clock.Now(),
	})
}

// Identify implements [Tracker]. It never performs network I/O.
func (t *tracker) Identify(id string, traits map[string]any) {
	t.mu.Lock()
	if !t.enabled {
		t.mu.Unlock()
		return
	}
	t.userID = id
	t.mu.Unlock()
	t.enqueue(Event{
		Kind:      KindIdentify,
		UserID:    id,
		Traits:    traits,
		Timestamp: t.cfg.clock.Now(),
	})
}

// enqueue appends an event to the queue (dropping it when disabled), persists
// the spool, and — never synchronously sending — asks the worker to flush when
// the batch threshold is reached.
func (t *tracker) enqueue(e Event) {
	t.mu.Lock()
	if !t.enabled {
		t.mu.Unlock()
		return // consent gate: drop, nothing spooled, nothing sent
	}
	e.SessionID = t.sessionID
	if e.UserID == "" {
		e.UserID = t.userID
	}
	piece, _ := spoolPiece(e)
	t.queue = append(t.queue, e)
	t.queuePieces = append(t.queuePieces, piece)
	t.evictLocked()
	t.persistLocked()
	trigger := len(t.queue) >= t.cfg.batchSize
	t.mu.Unlock()

	if trigger {
		t.signal(ctrlFlush)
	}
}

// evictLocked enforces the spool cap, oldest-first.
func (t *tracker) evictLocked() {
	if over := len(t.queue) - t.cfg.maxSpool; over > 0 {
		t.queue = t.queue[over:]
		t.queuePieces = t.queuePieces[over:]
	}
}

// signal sends a worker command without blocking (drops if the buffer is full,
// which is safe: a flush is already pending in that case).
func (t *tracker) signal(c ctrl) {
	select {
	case t.ctrlCh <- c:
	default:
	}
}

// Flush implements [Tracker]. It is the one explicitly synchronous path.
func (t *tracker) Flush() error {
	return t.doFlush(true)
}

// SetEnabled implements [Tracker].
func (t *tracker) SetEnabled(enabled bool) {
	t.mu.Lock()
	if enabled == t.enabled {
		t.mu.Unlock()
		return
	}
	t.enabled = enabled
	if enabled {
		// Drain any spool left by a previous run (oldest-first).
		if !t.loaded {
			t.loadSpoolLocked()
			t.loaded = true
		}
		t.mu.Unlock()
		t.signal(ctrlResume)
		t.signal(ctrlFlush)
		return
	}
	// Disabling: drop everything and clear the disk spool.
	t.queue, t.queuePieces = nil, nil
	t.inflight, t.inflightPieces = nil, nil
	t.attempt = 0
	t.nextRetryAt = time.Time{}
	t.loaded = false
	store := t.cfg.store
	t.mu.Unlock()
	_ = store.Clear()
	t.signal(ctrlSuspend)
}

// loadSpoolLocked reads the persisted spool into the front of the queue.
func (t *tracker) loadSpoolLocked() {
	data, err := t.cfg.store.Load()
	if err != nil || len(data) == 0 {
		return
	}
	spooled, err := unmarshalSpool(data)
	if err != nil {
		return
	}
	// Spooled events are older, so they lead.
	pieces := make([][]byte, len(spooled), len(spooled)+len(t.queuePieces))
	for i, e := range spooled {
		pieces[i], _ = spoolPiece(e)
	}
	t.queue = append(spooled, t.queue...)
	t.queuePieces = append(pieces, t.queuePieces...)
	t.evictLocked()
}

// persistLocked checkpoints the disk spool: any in-flight batch, then the
// queue, newest maxSpool events at most (the cap the queue's own eviction
// keeps, oldest first).
func (t *tracker) persistLocked() {
	events, pieces := t.queue, t.queuePieces
	if len(t.inflight) > 0 {
		events = append(append(make([]Event, 0, len(t.inflight)+len(t.queue)), t.inflight...), t.queue...)
		pieces = append(append(make([][]byte, 0, len(events)), t.inflightPieces...), t.queuePieces...)
	}
	if over := len(events) - t.cfg.maxSpool; over > 0 {
		events, pieces = events[over:], pieces[over:]
	}
	data, err := spoolData(events, pieces)
	if err != nil {
		return
	}
	// Best-effort checkpoint. A failure here means the spool is not durable
	// (crash-safety / next-launch drain won't work), so surface it once rather
	// than swallowing it silently — a silent swallow hid the store creation bug
	// in pre-rebrand tracker #699.
	if err := t.cfg.store.Save(data); err != nil {
		t.persistErrer.Do(func() {
			bellui.LogError("analytics: failed to persist spool (offline durability disabled)", err)
		})
	}
}

// spoolData is the spool for events: joined from their cached pieces, or
// encoded whole when a piece is missing.
func spoolData(events []Event, pieces [][]byte) ([]byte, error) {
	for _, p := range pieces {
		if p == nil {
			return marshalSpool(events)
		}
	}
	if len(pieces) != len(events) {
		return marshalSpool(events) // never expected; the slices move together
	}
	return joinSpool(pieces), nil
}

// backoff returns the retry delay for the given attempt (1-based), capped and
// full-jittered.
func (t *tracker) backoff(attempt int) time.Duration {
	d := t.cfg.backoffBase
	for i := 1; i < attempt && d < t.cfg.backoffMax; i++ {
		d *= 2
	}
	if d > t.cfg.backoffMax {
		d = t.cfg.backoffMax
	}
	// Full jitter: random in [0, d].
	return time.Duration(t.cfg.rand() * float64(d))
}

// doFlush attempts one batched send. It performs exactly one sink.Send call and
// returns the sink error. When force is false it respects the backoff gate.
func (t *tracker) doFlush(force bool) error {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()

	t.mu.Lock()
	if !t.enabled || len(t.queue) == 0 {
		t.mu.Unlock()
		return nil
	}
	now := t.cfg.clock.Now()
	if !force && !t.nextRetryAt.IsZero() && now.Before(t.nextRetryAt) {
		t.mu.Unlock()
		return nil // radio backoff: do not retry into a dead radio
	}
	// Take the batch out of the queue up front so events appended during the
	// (unlocked) send are preserved, and concurrent eviction — which trims the
	// front, i.e. the very events being sent — cannot corrupt the post-send
	// accounting. On failure the batch is returned to the front of the queue.
	// The batch stays on disk until the outcome is known: it is kept as
	// inflight, which persistLocked writes ahead of the queue, so a Track during
	// the send cannot save a spool without it, and a crash mid-send still
	// recovers it on the next launch (at-least-once, #670).
	batch, batchPieces := t.queue, t.queuePieces
	t.queue, t.queuePieces = nil, nil
	t.inflight, t.inflightPieces = batch, batchPieces
	t.mu.Unlock()

	// Network happens with mu released so Track never blocks behind a send.
	err := t.sink.Send(batch)

	t.mu.Lock()
	t.inflight, t.inflightPieces = nil, nil
	if !t.enabled {
		// Consent was withdrawn during the send: SetEnabled(false) already
		// dropped the queue and cleared the spool. Do not re-queue the in-flight
		// batch or persist — a persistLocked here would save the now-nil queue
		// as an empty spool *after* SetEnabled's store.Clear(), resurrecting a
		// spool the user opted out of (a rare drain-race, but real).
		t.queue, t.queuePieces = nil, nil
		t.mu.Unlock()
		return err
	}
	if err != nil {
		// Return the events to the front (oldest lead), re-bound the spool, and
		// schedule a backed-off retry.
		t.queue = append(batch, t.queue...)
		t.queuePieces = append(batchPieces, t.queuePieces...)
		t.evictLocked()
		t.attempt++
		t.nextRetryAt = now.Add(t.backoff(t.attempt))
		t.persistLocked()
		t.mu.Unlock()
		return err
	}
	// Success: the batch is already removed; only events appended during the
	// send remain. Reset backoff and checkpoint the (now shorter) spool.
	t.attempt = 0
	t.nextRetryAt = time.Time{}
	t.persistLocked()
	t.mu.Unlock()
	return nil
}

// run is the background worker: it owns the interval ticker and performs all
// non-explicit flushes. Track never runs here-synchronous network I/O — every
// send is on this goroutine or an explicit Flush caller.
func (t *tracker) run() {
	var ticker Ticker
	var tick <-chan time.Time
	stopTicker := func() {
		if ticker != nil {
			ticker.Stop()
			ticker = nil
			tick = nil
		}
	}
	defer stopTicker()

	for {
		select {
		case c := <-t.ctrlCh:
			switch c {
			case ctrlStop:
				return
			case ctrlSuspend:
				stopTicker()
			case ctrlResume:
				if ticker == nil {
					ticker = t.cfg.clock.NewTicker(t.cfg.interval)
					tick = ticker.C()
				}
			case ctrlFlush:
				t.notify(t.doFlush(false))
			}
		case <-tick:
			t.notify(t.doFlush(false))
		}
	}
}

// notify feeds the test-only flush hook, if configured.
func (t *tracker) notify(err error) {
	if t.cfg.flushNotify != nil {
		select {
		case t.cfg.flushNotify <- err:
		default:
		}
	}
}

// Close stops the background worker. It does not flush; callers that want a
// final delivery should Flush first. Safe to call multiple times.
func (t *tracker) Close() error {
	t.closeOnce.Do(func() {
		t.signal(ctrlStop)
	})
	return nil
}

// attach wires the tracker's session lifecycle to a [bellui.Lifecycle].
func (t *tracker) attach(lc bellui.Lifecycle) {
	lc.SetOnStarted(func() {
		t.startSession()
		t.signal(ctrlResume)
	})
	lc.SetOnEnteredForeground(func() {
		t.signal(ctrlResume)
	})
	lc.SetOnExitedForeground(func() {
		// Background edge: track the session end, flush now (the workhorse),
		// then suspend the interval timer — the process may be frozen next.
		t.Track("session_end", nil)
		_ = t.doFlush(true)
		t.signal(ctrlSuspend)
	})
	lc.SetOnStopped(func() {
		_ = t.doFlush(true)
	})
}

// startSession rotates the ephemeral session id and records a session_start.
func (t *tracker) startSession() {
	t.mu.Lock()
	t.sessionID = t.newSessionID()
	t.mu.Unlock()
	t.Track("session_start", nil)
}
