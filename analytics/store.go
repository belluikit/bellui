package analytics

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

// spoolFileName is the name used inside the app storage sandbox for the spool.
const spoolFileName = "bellui-analytics-spool.pxf"

// legacySpoolFileName is the spool's name before it moved to PXF (#523). Load
// reads it while there is no spool by the new name, and Save removes it.
const legacySpoolFileName = "bellui-analytics-spool.json"

// Store is the persistence backend for the disk spool. It stores a single
// opaque blob (the serialized event ring) that survives process death.
//
// The default implementation is backed by [bellui.Storage] (see [BelluiStore]);
// tests inject an in-memory implementation.
type Store interface {
	// Save overwrites the spool with data.
	Save(data []byte) error
	// Load returns the last saved spool, or (nil, nil) when none exists.
	Load() ([]byte, error)
	// Clear removes the spool.
	Clear() error
}

// BelluiStore persists the spool inside an app's storage sandbox via
// [bellui.Storage]. It is the default when a current app is available.
type BelluiStore struct {
	storage bellui.Storage
	name    string

	legacyName    string
	legacyRemoved atomic.Bool // there is no spool by legacyName any more
}

// NewBelluiStore returns a [Store] backed by the given app's storage sandbox.
func NewBelluiStore(app bellui.App) *BelluiStore {
	return newBelluiStore(app.Storage())
}

func newBelluiStore(s bellui.Storage) *BelluiStore {
	return &BelluiStore{storage: s, name: spoolFileName, legacyName: legacySpoolFileName}
}

// Save implements [Store].
func (s *BelluiStore) Save(data []byte) error {
	// bellui.Storage.Save only reopens an *existing* document (it returns
	// ErrNotExists otherwise), so the very first checkpoint must Create the
	// spool file. Without this the spool is never written and crash-safe
	// spooling / next-launch drain silently never happen (pre-rebrand tracker #699).
	w, err := s.storage.Save(s.name)
	if errors.Is(err, storage.ErrNotExists) {
		w, err = s.storage.Create(s.name)
	}
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	// A save completes when its writer closes (internal.Docs.Save), so the
	// spool is not known to be saved before then, and the legacy spool, whose
	// events it now holds, must outlive it until it is.
	if err := w.Close(); err != nil {
		return err
	}
	s.removeLegacy()
	return nil
}

// removeLegacy removes the spool saved under legacyName, once it is gone: a
// removal that fails is tried again at the next Save, so the old spool is not
// left to be read again after the new one has been cleared.
func (s *BelluiStore) removeLegacy() {
	if s.legacyName == "" || s.legacyRemoved.Load() {
		return
	}
	if s.storage.Remove(s.legacyName) == nil || !s.exists(s.legacyName) {
		s.legacyRemoved.Store(true)
	}
}

// exists reports whether the document name can be opened. Storage backends do
// not agree on the error a missing document gives, so this does not ask.
func (s *BelluiStore) exists(name string) bool {
	r, err := s.storage.Open(name)
	if err != nil {
		return false
	}
	_ = r.Close()
	return true
}

// Load implements [Store].
func (s *BelluiStore) Load() ([]byte, error) {
	r, err := s.storage.Open(s.name)
	if err != nil && s.legacyName != "" {
		r, err = s.storage.Open(s.legacyName)
	}
	if err != nil {
		// A missing spool is not an error.
		return nil, nil //nolint:nilerr
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Clear implements [Store].
func (s *BelluiStore) Clear() error {
	if s.legacyName != "" {
		_ = s.storage.Remove(s.legacyName)
	}
	err := s.storage.Remove(s.name)
	if err != nil && !errors.Is(err, io.EOF) {
		// Removing an absent spool is fine.
		return nil //nolint:nilerr
	}
	return nil
}

// memStore is an in-memory [Store]. It is used as the fallback when no app
// storage is available, and by tests to simulate persistence across a process
// restart (share one instance between two trackers).
//
// A [Store] is accessed concurrently — the background worker checkpoints via
// Save while the API goroutine may Clear (on disable) and callers Load — so the
// in-memory backing is mutex-guarded and Load returns a copy (never the live
// backing slice, which a later Save would overwrite in place).
type memStore struct {
	mu   sync.Mutex
	data []byte
	ok   bool
}

func (m *memStore) Save(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = append([]byte(nil), data...)
	m.ok = true
	return nil
}

func (m *memStore) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ok {
		return nil, nil
	}
	return append([]byte(nil), m.data...), nil
}

func (m *memStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = nil
	m.ok = false
	return nil
}
