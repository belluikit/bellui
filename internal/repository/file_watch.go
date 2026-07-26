package repository

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage/repository"
)

// Declare conformity to the optional watch interface (#454).
var _ repository.WatchableRepository = (*FileRepository)(nil)

type fileWatchSub struct {
	path   string
	fn     func()
	notify chan struct{}
	done   chan struct{}
}

// run drains change signals for one subscription and invokes its callback,
// coalescing bursts (the cap-1 notify channel collapses rapid successive
// changes into a single call). Running on a per-subscription goroutine means a
// slow or blocking callback can never stall the shared fsnotify reader or any
// other subscriber — the flaw behind the #663 dispatch-stall flake.
func (s *fileWatchSub) run() {
	for {
		select {
		case <-s.done:
			return
		case <-s.notify:
			s.fn()
		}
	}
}

// WatchURI registers fn to be called whenever the item the given URI
// references changes; for a directory that includes its children being
// created, removed or renamed. fn is invoked on a dedicated goroutine per
// subscription; rapid successive changes may be coalesced into a single call.
// Implements repository.WatchableRepository using fsnotify; platforms without
// file notification support (WebAssembly) return an error, so callers can
// degrade gracefully.
func (r *FileRepository) WatchURI(u bellui.URI, fn func()) (cancel func(), err error) {
	path := filepath.Clean(u.Path())

	r.watchOps.Lock()
	defer r.watchOps.Unlock()

	r.watchLock.Lock()
	w := r.watcher
	r.watchLock.Unlock()
	if w == nil {
		w, err = fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		r.watchLock.Lock()
		r.watcher = w
		r.watchSubs = make(map[string][]*fileWatchSub)
		r.watchLock.Unlock()
		go r.dispatchWatchEvents(w)
	}

	r.watchLock.Lock()
	needAdd := len(r.watchSubs[path]) == 0
	r.watchLock.Unlock()
	if needAdd {
		// Outside watchLock: on Windows, Add round-trips through the fsnotify
		// reader goroutine, which may be waiting on watchLock (#957). watchOps
		// keeps the check-then-Add race-free against other control ops.
		if err := w.Add(path); err != nil {
			return nil, err
		}
	}

	sub := &fileWatchSub{path: path, fn: fn, notify: make(chan struct{}, 1), done: make(chan struct{})}
	r.watchLock.Lock()
	r.watchSubs[path] = append(r.watchSubs[path], sub)
	r.watchLock.Unlock()
	go sub.run()

	return func() {
		r.watchOps.Lock()
		defer r.watchOps.Unlock()

		r.watchLock.Lock()
		subs := r.watchSubs[path]
		for i, s := range subs {
			if s == sub {
				r.watchSubs[path] = append(subs[:i], subs[i+1:]...)
				close(sub.done)
				break
			}
		}
		var w *fsnotify.Watcher
		if len(r.watchSubs[path]) == 0 {
			delete(r.watchSubs, path) // no-op after closeWatcher (nil map)
			w = r.watcher
		}
		r.watchLock.Unlock()

		if w != nil {
			// Outside watchLock, like Add above (#957). The path may already
			// be gone (deleted directory) — the watch died with it, so the
			// error is not actionable.
			_ = w.Remove(path)
		}
	}, nil
}

// dispatchWatchEvents fans watcher events out to the subscriptions of the
// changed path and of its parent directory (a directory watch reports its
// children in the event name). It exits when the watcher is closed.
func (r *FileRepository) dispatchWatchEvents(w *fsnotify.Watcher) {
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			r.notifyWatchers(filepath.Clean(ev.Name))
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

func (r *FileRepository) notifyWatchers(name string) {
	r.watchLock.Lock()
	var subs []*fileWatchSub
	subs = append(subs, r.watchSubs[name]...)
	if dir := filepath.Dir(name); dir != name {
		subs = append(subs, r.watchSubs[dir]...)
	}
	r.watchLock.Unlock()

	// Signal each subscription without blocking: the cap-1 channel coalesces a
	// burst into a single pending call, and a full channel means a call is
	// already queued — so a slow subscriber can never stall this reader.
	for _, s := range subs {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
}

// closeWatcher shuts down the shared fsnotify watcher, releasing all watches.
func (r *FileRepository) closeWatcher() {
	r.watchOps.Lock()
	defer r.watchOps.Unlock()

	r.watchLock.Lock()
	w := r.watcher
	subs := r.watchSubs
	r.watcher = nil
	r.watchSubs = nil
	r.watchLock.Unlock()

	if w == nil {
		return
	}
	// Close() must run WITHOUT watchLock held: on Windows it blocks until the
	// dispatchWatchEvents goroutine drains w.Events and exits, but that goroutine
	// takes watchLock in notifyWatchers — so closing under the lock deadlocks
	// (Close waits for the reader, the reader waits for the lock). With the lock
	// released, notifyWatchers sees the now-nil subs and returns, the reader
	// drains and exits, and Close returns (#663). The same rule now covers
	// Add/Remove in WatchURI and its cancel (#957).
	//
	// The wait is bounded: fsnotify's Windows reader can miss Close's IOCP
	// wakeup entirely (captured on the windows/arm64 CI VM: Close parked on
	// its reply while the reader sat in GetQueuedCompletionStatus forever —
	// the completion packet was posted but never woke the thread, #963).
	// Once Close has marked the watcher closed there is no public way to
	// poke the reader again, so after the timeout Destroy proceeds: the
	// subscriptions are detached and their goroutines exit below; the
	// watcher's reader goroutine and OS handles leak until process exit,
	// which beats wedging the caller for good.
	closed := make(chan struct{})
	go func() {
		_ = w.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		bellui.LogError("timed out closing the file watcher; leaking its reader", nil)
	}
	for _, ss := range subs {
		for _, s := range ss {
			close(s.done)
		}
	}
}
