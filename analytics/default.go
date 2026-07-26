package analytics

import (
	"sync"

	"github.com/belluikit/bellui"
)

var (
	defaultMu sync.RWMutex
	// defaultTracker starts as a no-op so the package functions are always safe
	// to call, and consent-preserving until an app installs a real tracker.
	defaultTracker Tracker = nopTracker{}
)

// SetDefault installs the package-level default [Tracker] used by the package
// functions [Track], [Identify], [Flush] and [SetEnabled]. Passing nil restores
// the safe no-op default.
func SetDefault(t Tracker) {
	defaultMu.Lock()
	if t == nil {
		t = nopTracker{}
	}
	defaultTracker = t
	defaultMu.Unlock()
}

// Default returns the package-level default [Tracker].
func Default() Tracker {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultTracker
}

// Track calls Track on the default tracker.
func Track(event string, props map[string]any) { Default().Track(event, props) }

// Identify calls Identify on the default tracker.
func Identify(id string, traits map[string]any) { Default().Identify(id, traits) }

// Flush calls Flush on the default tracker.
func Flush() error { return Default().Flush() }

// SetEnabled calls SetEnabled on the default tracker.
func SetEnabled(enabled bool) { Default().SetEnabled(enabled) }

// Attach wires a tracker's session lifecycle (session_start / session_end and
// the background-edge flush) to a [bellui.Lifecycle], obtained from
// bellui.App.Lifecycle(). It is the only toolkit integration point. Trackers that
// were not created by [New] are ignored.
func Attach(t Tracker, lc bellui.Lifecycle) {
	if a, ok := t.(interface{ attach(bellui.Lifecycle) }); ok {
		a.attach(lc)
	}
}

// Close stops a tracker's background worker (see [New]). It does not flush;
// Flush first if a final delivery is wanted. Trackers not created by [New] are
// ignored.
func Close(t Tracker) error {
	if c, ok := t.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

// nopTracker is the safe no-op default. Every method drops its input.
type nopTracker struct{}

func (nopTracker) Track(string, map[string]any)    {}
func (nopTracker) Identify(string, map[string]any) {}
func (nopTracker) Flush() error                    { return nil }
func (nopTracker) SetEnabled(bool)                 {}
