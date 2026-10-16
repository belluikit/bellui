package analytics

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBelluiStore_SaveCreatesThenOverwrites is the regression for pre-rebrand tracker #699: the very
// first Save must CREATE the spool file. bellui.Storage.Save only reopens an
// existing document (ErrNotExists otherwise), so before the fix the spool was
// never written and the error was swallowed — crash-safe spooling silently
// never happened. The test app's storage embeds the same internal.Docs as a
// real app, so it reproduces the exact Save semantics.
func TestBelluiStore_SaveCreatesThenOverwrites(t *testing.T) {
	test.NewTempApp(t)
	s := NewBelluiStore(bellui.CurrentApp())

	// The underlying bellui.Storage.Save refuses a not-yet-created document — this
	// is the quirk BelluiStore.Save works around, and the direct cause of pre-rebrand tracker #699.
	_, rawErr := bellui.CurrentApp().Storage().Save("nonexistent-" + spoolFileName)
	assert.ErrorIs(t, rawErr, storage.ErrNotExists)

	require.NoError(t, s.Clear()) // storage sandbox can persist across runs
	got, err := s.Load()
	require.NoError(t, err)
	assert.Nil(t, got)

	// First Save must create the file (was: ErrNotExists, wrote nothing).
	require.NoError(t, s.Save([]byte("first")))
	got, err = s.Load()
	require.NoError(t, err)
	assert.Equal(t, []byte("first"), got)

	// Subsequent Save overwrites.
	require.NoError(t, s.Save([]byte("second-and-longer")))
	got, err = s.Load()
	require.NoError(t, err)
	assert.Equal(t, []byte("second-and-longer"), got)

	// Clear removes it; Load then reports empty.
	require.NoError(t, s.Clear())
	got, err = s.Load()
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestSpoolRoundTrip_BelluiStore is TestSpoolRoundTrip over the real bellui-storage
// spool rather than the in-memory memStore — the path the pre-rebrand tracker #699 device run
// exercised, and the one the memStore test could not have caught.
func TestSpoolRoundTrip_BelluiStore(t *testing.T) {
	test.NewTempApp(t)
	store := NewBelluiStore(bellui.CurrentApp())
	require.NoError(t, store.Clear()) // storage sandbox can persist across runs
	t.Cleanup(func() { _ = store.Clear() })
	clk := newFakeClock()

	// Session 1: enqueue, then "die" (Close without flushing).
	flushCh1 := make(chan error, 8)
	tr1 := New(newFakeSink(), WithClock(clk), WithStore(store), WithBatchSize(1000), withFlushNotify(flushCh1)).(*tracker)
	tr1.SetEnabled(true)
	waitFlush(t, flushCh1)
	for i := 0; i < 5; i++ {
		tr1.Track("evt", map[string]any{"seq": i})
	}
	require.NoError(t, tr1.Close()) // simulated kill: no flush

	data, err := store.Load()
	require.NoError(t, err)
	require.NotEmpty(t, data, "events must survive on disk")

	// Session 2: relaunch against the same on-disk spool, drain oldest-first.
	sink2 := newFakeSink()
	flushCh := make(chan error, 8)
	tr2 := New(sink2, WithClock(clk), WithStore(store), WithBatchSize(1000), withFlushNotify(flushCh)).(*tracker)
	t.Cleanup(func() { _ = tr2.Close() })
	tr2.SetEnabled(true)
	waitFlush(t, flushCh)

	got := sink2.allEvents()
	require.Len(t, got, 5)
	for i, e := range got {
		assert.EqualValues(t, i, e.Props["seq"], "drained oldest-first, in order")
	}
}
