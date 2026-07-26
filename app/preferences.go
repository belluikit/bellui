package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
)

type preferences struct {
	*internal.InMemoryPreferences

	prefLock            sync.RWMutex
	savedRecently       bool
	changedDuringSaving bool
	debounceTimer       *time.Timer

	app                 *belluiApp
	needsSaveBeforeExit bool
	watcher             any // normally *fsnotify.Watcher or nil - avoid import in this file
}

// Declare conformity with Preferences interface
var _ bellui.Preferences = (*preferences)(nil)

// sentinel error to signal an empty preferences storage backend was loaded
var errEmptyPreferencesStore = errors.New("empty preferences store")

// returned from storageWriter() - may be a file, browser local storage, etc
type writeSyncCloser interface {
	io.WriteCloser
	Sync() error
}

// forceImmediateSave writes preferences to storage immediately, ignoring the debouncing
// logic in the change listener. Does nothing if preferences are not backed with a persistent store.
func (p *preferences) forceImmediateSave() {
	if !p.needsSaveBeforeExit {
		return
	}
	p.prefLock.Lock()
	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}
	p.savedRecently = false
	p.changedDuringSaving = false
	p.prefLock.Unlock()

	err := p.save()
	if err != nil {
		bellui.LogError("Failed on force saving preferences", err)
	}
}

func (p *preferences) resetSavedRecently() {
	p.prefLock.Lock()
	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}
	p.debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
		// For test reasons we need to use current app not what we were initialised with as they can differ
		bellui.DoAndWait(func() {
			p.prefLock.Lock()
			p.savedRecently = false
			changedDuringSaving := p.changedDuringSaving
			p.changedDuringSaving = false
			p.prefLock.Unlock()

			if changedDuringSaving {
				p.save()
			}
		})
	})
	p.prefLock.Unlock()
}

func (p *preferences) save() error {
	storage, err := p.storageWriter()
	if err != nil {
		return err
	}
	if err := p.saveToStorage(storage); err != nil {
		return err
	}

	// Migration complete for this store: drop the superseded legacy JSON file so
	// future loads and the file watcher use the PXF store (no-op off the file
	// backend). Done here, not in saveToStorage, so a save to an explicit alternate
	// path (saveToFile) never deletes the default store's legacy file.
	p.cleanupLegacyStore()
	return nil
}

func (p *preferences) saveToStorage(writer writeSyncCloser) error {
	p.prefLock.Lock()
	p.savedRecently = true
	p.prefLock.Unlock()
	defer p.resetSavedRecently()

	// Preferences are persisted in the textual PXF format (see preferences_pxf.go).
	// The legacy JSON store is still read on load during the migration window.
	var data []byte
	var err error
	p.InMemoryPreferences.ReadValues(func(values map[string]any) {
		data, err = marshalPreferencesPXF(values)
	})
	if err == nil {
		_, err = writer.Write(data)
	}

	// Only commit once the data is fully written. The commit point differs by
	// backend — the file backend renames the temp file over the target in Close,
	// the wasm backend flushes to local storage in Sync — so on an encode failure
	// we must neither Sync nor Close-commit, or a partial/empty payload would
	// clobber the previous good data. Discard instead.
	if err != nil {
		discardWrite(writer)
		return err
	}

	if err = writer.Sync(); err != nil {
		discardWrite(writer)
		return err
	}

	// Close commits the write (atomic rename for the file backend). Capture its
	// error so a failed commit is reported rather than silently leaving the
	// previous file in place.
	return writer.Close()
}

// discardWrite rolls back an in-progress save without committing. Backends that
// can roll back (the file backend's atomic temp file) expose abort(); others
// (wasm local storage, which only commits on a successful Sync) are simply
// closed, leaving the stored value untouched.
func discardWrite(writer writeSyncCloser) {
	if a, ok := writer.(interface{ abort() error }); ok {
		_ = a.abort()
		return
	}
	_ = writer.Close()
}

// stopWatching stops the storage file watcher if one is active. Each watcher
// keeps a handle on the storage directory, so leaking them (for example across
// tests that reuse a storage path) can race file replacement — notably on
// Windows, where os.Rename fails while another handle has the target open.
func (p *preferences) stopWatching() {
	if c, ok := p.watcher.(interface{ Close() error }); ok {
		_ = c.Close()
	}
	p.watcher = nil
}

func (p *preferences) load() {
	storage, err := p.storageReader()
	if err == nil {
		err = p.loadFromStorage(storage)
	}
	if err != nil && err != errEmptyPreferencesStore {
		bellui.LogError("Preferences load error:", err)
	}
}

func (p *preferences) loadFromStorage(storage io.ReadCloser) (err error) {
	defer func() {
		if r := storage.Close(); r != nil && err == nil {
			err = r
		}
	}()
	data, err := io.ReadAll(storage)
	if err != nil {
		return err
	}
	// Tolerate a UTF-8 BOM on a hand-edited/externally-produced legacy store so the
	// format sniff below isn't thrown off by a leading non-'{' byte.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(data) == 0 {
		return nil
	}

	p.InMemoryPreferences.WriteValues(func(values map[string]any) {
		// Read-both migration window: decode the legacy JSON store where present,
		// otherwise the current PXF store (see isLegacyJSONPreferences).
		if isLegacyJSONPreferences(data) {
			if err = json.Unmarshal(data, &values); err == nil {
				convertLists(values)
			}
			return
		}
		if err = unmarshalPreferencesPXF(data, values); err != nil {
			// A payload that is neither recognizably JSON nor valid PXF is retried as
			// JSON before giving up, so a format misclassification never silently
			// empties (and, via cleanup on the next save, deletes) a recoverable store.
			if json.Unmarshal(data, &values) == nil {
				convertLists(values)
				err = nil
			}
		}
	})

	return err
}

func newPreferences(app *belluiApp) *preferences {
	p := &preferences{}
	p.app = app
	p.InMemoryPreferences = internal.NewInMemoryPreferences()

	// don't load or watch if not setup
	if app.uniqueID == "" && app.Metadata().ID == "" {
		return p
	}

	p.needsSaveBeforeExit = true
	p.AddChangeListener(func() {
		if app.prefs != bellui.Preferences(p) {
			return
		}
		p.prefLock.Lock()
		shouldIgnoreChange := p.savedRecently
		if p.savedRecently {
			p.changedDuringSaving = true
		}
		p.prefLock.Unlock()

		if shouldIgnoreChange { // callback after loading from storage, or too many updates in a row
			return
		}

		err := p.save()
		if err != nil {
			bellui.LogError("Failed on saving preferences", err)
		}
	})
	p.watch()
	return p
}

func convertLists(values map[string]any) {
	for k, v := range values {
		if items, ok := v.([]any); ok {
			if len(items) == 0 {
				continue
			}

			switch items[0].(type) {
			case bool:
				bools := make([]bool, len(items))
				for i, item := range items {
					bools[i] = item.(bool)
				}
				values[k] = bools
			case float64:
				floats := make([]float64, len(items))
				for i, item := range items {
					floats[i] = item.(float64)
				}
				values[k] = floats
			// case int: // json has no int!
			case string:
				strings := make([]string, len(items))
				for i, item := range items {
					strings[i] = item.(string)
				}
				values[k] = strings
			}
		}
	}
}
