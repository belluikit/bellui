// Package analytics is an opt-in, consent-first product-analytics package that
// lives beside the toolkit (the theme/cjk placement pattern): it is never
// imported by Bellui core, and costs nothing unless an app imports it. bellui.App,
// the drivers and the widgets stay 100% analytics-unaware; the only toolkit
// integration point is the public [bellui.Lifecycle] (see [Attach]).
//
// # Consent invariant
//
// A [Tracker] starts DISABLED. Until [Tracker.SetEnabled](true) is called,
// [Tracker.Track] and [Tracker.Identify] drop events on the floor — nothing is
// queued to disk and nothing leaves the process. The package generates NO
// device identifiers (no advertising ID, no fingerprint, no random install ID);
// an app supplies an identity only via [Tracker.Identify], and only when it has
// consent. Disabling again (SetEnabled(false)) clears the on-disk spool. These
// properties are enforced by construction and covered by tests, which keeps the
// Google Play Data-safety / GDPR story honest: with consent withheld there is
// no data collection and no identifier to disclose.
//
// # Battery model
//
// Per-event HTTP on cellular is the classic telemetry battery killer — the LTE
// radio cost is per-wake-up, not per-byte. This package is built so an app can
// instrument every interaction without pinning the radio:
//
//   - Track is fire-into-queue, NEVER fire-into-socket. No code path performs
//     synchronous per-event network I/O.
//   - Batches flush when (defaults, all tunable via [Option]) the queue reaches
//     30 events, a 60s foreground interval elapses, the app enters the
//     background, or [Tracker.Flush] is called explicitly.
//   - The interval timer is suspended while backgrounded (mobile processes get
//     frozen); the background-edge flush is the workhorse.
//   - The queue is checkpointed to a disk spool so it survives process death and
//     drains oldest-first at the next launch; the spool is bounded with
//     oldest-first eviction so a dead sink cannot grow it without limit.
//   - Batch bodies are gzipped by the [Sink]; one request per flush.
//   - A failed flush returns the events to the spool and backs off exponentially
//     with jitter — offline is a normal state, not an error.
//
// Pure-Go apps cannot use Android WorkManager / iOS BGTaskScheduler (no JNI
// surface for OS-scheduled deferred upload); the compensation is bigger batches,
// longer intervals, a background-edge flush and a next-launch drain. A one-
// session analytics latency is acceptable by design.
package analytics

import "time"

// EventKind distinguishes an explicit event from an identity association.
type EventKind string

const (
	// KindTrack is a plain event recorded by [Tracker.Track].
	KindTrack EventKind = "track"
	// KindIdentify associates a stable user identity, recorded by
	// [Tracker.Identify].
	KindIdentify EventKind = "identify"
)

// Event is a single queued analytics record. Sinks translate it to their
// vendor wire format.
type Event struct {
	// Kind is KindTrack or KindIdentify.
	Kind EventKind `json:"kind"`
	// Name is the event name (empty for KindIdentify).
	Name string `json:"name,omitempty"`
	// Props are the event properties (KindTrack).
	Props map[string]any `json:"props,omitempty"`
	// Traits are the identity traits (KindIdentify).
	Traits map[string]any `json:"traits,omitempty"`
	// UserID is the app-supplied identity, if any. The package never
	// synthesises this value.
	UserID string `json:"user_id,omitempty"`
	// SessionID is the ephemeral, per-run session identifier. It is
	// regenerated every launch and never persisted as an identity, so it is
	// not a device or install identifier.
	SessionID string `json:"session_id,omitempty"`
	// Timestamp is the wall-clock time the event was recorded.
	Timestamp time.Time `json:"timestamp"`
}

// Sink delivers a batch of events to a backend. Implementations should gzip the
// body and perform exactly one request per Send. A Sink is invoked only from the
// tracker's background worker or from an explicit [Tracker.Flush]; it is never
// called on the [Tracker.Track] hot path.
//
// Vendor adapters live in dumb single-file subpackages (analytics/ga4,
// analytics/posthog); apps can bring their own.
type Sink interface {
	// Send delivers batch. A returned error causes the events to be retained
	// in the spool and retried later with backoff; Send must not block
	// indefinitely.
	Send(batch []Event) error
}

// Tracker is the consent-gated event queue. Obtain one from [New].
type Tracker interface {
	// Track queues an event. It is fire-into-queue: it never performs network
	// I/O and never blocks on a socket. When the tracker is disabled the call
	// is a no-op (the event is dropped, not spooled).
	Track(event string, props map[string]any)
	// Identify associates a stable, app-supplied identity with subsequent
	// events. Like Track it is queue-only and a no-op while disabled. Pass an
	// identity only when the app has consent; the package never generates one.
	Identify(id string, traits map[string]any)
	// Flush attempts to deliver all queued events now and returns the sink
	// error, if any. It is the one explicitly synchronous path (the caller
	// asked for it); it still performs a single batched request.
	Flush() error
	// SetEnabled toggles consent. Enabling drains any spool left by a previous
	// run (oldest-first). Disabling drops the in-memory queue and clears the
	// on-disk spool. A tracker is created disabled.
	SetEnabled(enabled bool)
}
