// WatchURI rides the forked fsnotify, which has no wasm implementation —
// there is no filesystem to watch in a browser. (#79 Phase B.)
//go:build !wasm

package repository

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
)

// watchTestDir returns a fresh directory under testdata: TMPDIR would be more
// normal but fsnotify cannot watch that on macOS (same as the app settings
// watcher tests).
func watchTestDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "watch-"+t.Name())
	require.NoError(t, os.MkdirAll(dir, 0o755))
	t.Cleanup(func() { os.RemoveAll(dir) })
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

func TestFileRepository_WatchURI(t *testing.T) {
	dir := watchTestDir(t)
	repo := NewFileRepository()
	defer repo.Destroy("file")

	var _ repository.WatchableRepository = repo

	changed := make(chan struct{}, 16)
	cancel, err := repo.WatchURI(storage.NewFileURI(dir), func() { changed <- struct{}{} })
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644))
	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("no notification for a created child")
	}

	cancel()
	// Drain anything already in flight before asserting silence.
	time.Sleep(50 * time.Millisecond)
	for len(changed) > 0 {
		<-changed
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "later.txt"), []byte("y"), 0o644))
	select {
	case <-changed:
		t.Fatal("cancelled watch must not notify")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestFileRepository_WatchURI_MultipleSubscribers(t *testing.T) {
	dir := watchTestDir(t)
	repo := NewFileRepository()
	defer repo.Destroy("file")

	a := make(chan struct{}, 16)
	b := make(chan struct{}, 16)
	cancelA, err := repo.WatchURI(storage.NewFileURI(dir), func() { a <- struct{}{} })
	require.NoError(t, err)
	cancelB, err := repo.WatchURI(storage.NewFileURI(dir), func() { b <- struct{}{} })
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.txt"), []byte("1"), 0o644))
	for name, ch := range map[string]chan struct{}{"a": a, "b": b} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %s missed the notification", name)
		}
	}

	// Cancelling one subscription must keep the other alive (the shared
	// fsnotify watch is released only with the last one).
	cancelA()
	time.Sleep(50 * time.Millisecond)
	for len(b) > 0 {
		<-b
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "two.txt"), []byte("2"), 0o644))
	select {
	case <-b:
	case <-time.After(2 * time.Second):
		t.Fatal("remaining subscriber missed the notification")
	}
	cancelB()
}

// TestFileRepository_WatchURI_SlowSubscriberDoesNotStallOthers guards pre-rebrand tracker #663: a
// subscriber whose callback blocks must never stall delivery to another
// subscriber (previously all callbacks ran synchronously on the single dispatch
// goroutine, so one blocked callback wedged every watch to the test timeout).
func TestFileRepository_WatchURI_SlowSubscriberDoesNotStallOthers(t *testing.T) {
	dir := watchTestDir(t)
	repo := NewFileRepository()
	defer repo.Destroy("file")

	block := make(chan struct{})
	// Registered first, so it is dispatched before the fast subscriber: its
	// blocking callback used to prevent the fast one from ever being called.
	cancelSlow, err := repo.WatchURI(storage.NewFileURI(dir), func() { <-block })
	require.NoError(t, err)

	fast := make(chan struct{}, 16)
	cancelFast, err := repo.WatchURI(storage.NewFileURI(dir), func() { fast <- struct{}{} })
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.txt"), []byte("1"), 0o644))
	select {
	case <-fast:
	case <-time.After(2 * time.Second):
		t.Fatal("fast subscriber stalled behind the blocked slow subscriber")
	}

	for len(fast) > 0 {
		<-fast
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "two.txt"), []byte("2"), 0o644))
	select {
	case <-fast:
	case <-time.After(2 * time.Second):
		t.Fatal("fast subscriber missed a later notification while another was blocked")
	}

	close(block) // release the slow callback so its goroutine can exit cleanly
	cancelSlow()
	cancelFast()
}

func TestFileRepository_WatchURI_CancelTwiceSafe(t *testing.T) {
	dir := watchTestDir(t)
	repo := NewFileRepository()
	defer repo.Destroy("file")

	cancel, err := repo.WatchURI(storage.NewFileURI(dir), func() {})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		cancel()
		cancel()
	})
}

// TestFileRepository_WatchURI_ConcurrentControlOps guards pre-rebrand tracker #957: WatchURI /
// cancel / Destroy must never hold the lock the event dispatcher needs across
// an fsnotify call (on Windows Add/Remove/Close round-trip through fsnotify's
// reader goroutine, which can itself be waiting to deliver an event into
// notifyWatchers — holding watchLock across the call deadlocks the whole watch
// subsystem). Churn subscriptions from several goroutines while events fire;
// under the old locking this wedges, under -race it also proves the two-lock
// split is sound.
func TestFileRepository_WatchURI_ConcurrentControlOps(t *testing.T) {
	dir := watchTestDir(t)
	repo := NewFileRepository()
	defer repo.Destroy("file")

	stop := make(chan struct{})
	defer close(stop)
	go func() { // steady event pressure on the watched directory
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = os.WriteFile(filepath.Join(dir, "churn.txt"), []byte{byte(i)}, 0o644)
			time.Sleep(time.Millisecond)
		}
	}()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				cancel, err := repo.WatchURI(storage.NewFileURI(dir), func() {})
				if err != nil {
					t.Error(err)
					return
				}
				cancel()
			}
		}()
	}

	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("watch control ops deadlocked under concurrent subscribe/cancel churn")
	}
}
