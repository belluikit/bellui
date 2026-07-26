package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// fakeLifecycle is a minimal bellui.Lifecycle that captures the hooks so a test
// can drive the transitions.
type fakeLifecycle struct {
	started, stopped     func()
	foreground, exitedFg func()
}

var _ bellui.Lifecycle = (*fakeLifecycle)(nil)

func (l *fakeLifecycle) SetOnStarted(f func())           { l.started = f }
func (l *fakeLifecycle) SetOnStopped(f func())           { l.stopped = f }
func (l *fakeLifecycle) SetOnEnteredForeground(f func()) { l.foreground = f }
func (l *fakeLifecycle) SetOnExitedForeground(f func())  { l.exitedFg = f }

func TestAttach_SessionAndBackgroundFlush(t *testing.T) {
	sink := newFakeSink()
	tr, clk, _, flushCh := newTestTracker(t, sink, WithBatchSize(1000))
	tr.SetEnabled(true)
	waitFlush(t, flushCh)

	lc := &fakeLifecycle{}
	Attach(tr, lc)

	// Started → session_start recorded, ticker resumed.
	lc.started()
	tr.mu.Lock()
	require.Len(t, tr.queue, 1)
	assert.Equal(t, "session_start", tr.queue[0].Name)
	tr.mu.Unlock()

	// Interval works while foregrounded.
	clk.Advance(DefaultInterval)
	clk.Tick()
	waitFlush(t, flushCh)
	assert.Equal(t, 1, sink.callCount(), "foreground interval flush")

	// Background edge → session_end + immediate flush + ticker suspended.
	tr.Track("mid", nil)
	lc.exitedFg()
	events := sink.allEvents()
	require.NotEmpty(t, events)
	assert.Equal(t, "session_end", events[len(events)-1].Name, "background flush delivered session_end")

	callsAfterBg := sink.callCount()
	clk.Advance(DefaultInterval)
	clk.Tick() // suspended: must not flush
	tr.mu.Lock()
	empty := len(tr.queue) == 0
	tr.mu.Unlock()
	assert.True(t, empty, "background flush drained the queue")
	assert.Equal(t, callsAfterBg, sink.callCount(), "no interval flush while backgrounded")
}

func TestDefaultTracker_NopSafe(t *testing.T) {
	// The package default is a safe no-op until SetDefault is called.
	Track("x", nil)
	Identify("id", nil)
	SetEnabled(true)
	assert.NoError(t, Flush())

	prev := Default()
	t.Cleanup(func() { SetDefault(prev) })

	sink := newFakeSink()
	tr, _, _, _ := newTestTracker(t, sink)
	SetDefault(tr)
	assert.Same(t, tr, Default().(*tracker))

	SetDefault(nil) // restores nop
	assert.NoError(t, Flush())
}
