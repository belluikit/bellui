package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

// syncFailWriter wraps the real atomic file writer but forces Sync to fail, to
// exercise the discard-on-error path in saveToStorage. abort/Write/Close are
// promoted from the embedded writer.
type syncFailWriter struct {
	*atomicFileWriter
}

func (w *syncFailWriter) Sync() error {
	return errors.New("forced sync failure")
}

func loadPreferences(t *testing.T, id string) *preferences {
	// Namespace the storage id by the test name so concurrent or sequential
	// tests never share a preferences file. Under -tags ci the config root is a
	// single process-wide temp dir (os.TempDir()/bellui-test), so every test using
	// the bare id "dummy" otherwise resolves to the same path — letting one
	// test's stray save clobber another's data (#94).
	uniqueID := strings.ReplaceAll(t.Name(), "/", "_") + "-" + id
	p := newPreferences(&belluiApp{uniqueID: uniqueID})
	p.load()
	t.Cleanup(func() {
		// Stop the file watcher so it does not leak a handle on the storage
		// directory and race a later test's atomic save (the rename fails on
		// Windows while a watcher has the target open).
		p.stopWatching()
		// Cancel the debounce save timer too: stopWatching only closes the
		// watcher, but resetSavedRecently re-arms a 100ms time.AfterFunc on every
		// save. A leaked timer fires during a later test and writes this test's
		// (possibly stale) data to storage, racing it — the root cause of #94.
		p.prefLock.Lock()
		if p.debounceTimer != nil {
			p.debounceTimer.Stop()
			p.debounceTimer = nil
		}
		p.prefLock.Unlock()
	})

	return p
}

func TestPreferences_Remove(t *testing.T) {
	p := loadPreferences(t, "dummy")
	p.WriteValues(func(val map[string]any) {
		val["keyString"] = "value"
		val["keyInt"] = 4
	})

	p.RemoveValue("keyString")
	err := p.saveToFile(p.storagePath())
	assert.NoError(t, err)

	// check it doesn't write values that were removed
	p = loadPreferences(t, "dummy")
	assert.Equal(t, 4, bellui.PrefOr[int](p, "keyInt", 0))
	assert.Equal(t, "missing", bellui.PrefOr(p, "keyString", "missing"))
}

func TestPreferences_Save(t *testing.T) {
	p := loadPreferences(t, "dummy")
	p.WriteValues(func(val map[string]any) {
		val["keyString"] = "value"
		val["keyStringList"] = []string{"1", "2", "3"}
		val["keyInt"] = 4
		val["keyIntList"] = []int{1, 2, 3}
		val["keyFloat"] = 3.5
		val["keyFloatList"] = []float64{1.1, 2.2, 3.3}
		val["keyBool"] = true
		val["keyBoolList"] = []bool{true, false, true}
		val["keyEmptyList"] = []string{}
	})

	path := p.storagePath()
	defer os.Remove(path)
	err := p.saveToFile(path)
	assert.NoError(t, err)

	// The store is written in the textual PXF format, not JSON: it never begins
	// with '{' and carries the Preferences `values` field.
	content, err := os.ReadFile(path)
	if err != nil {
		assert.Fail(t, "Failed to load, %v", err)
	}
	assert.False(t, isLegacyJSONPreferences(content), "preferences should be saved as PXF, not JSON")
	assert.Contains(t, string(content), "values")

	// check it reads the saved output back with every type intact
	p = loadPreferences(t, "dummy")
	assert.Equal(t, "value", bellui.PrefOr(p, "keyString", ""))
	assert.Equal(t, []string{"1", "2", "3"}, bellui.PrefOr(p, "keyStringList", []string{}))
	assert.Equal(t, 4, bellui.PrefOr[int](p, "keyInt", 0))
	assert.Equal(t, []int{1, 2, 3}, bellui.PrefOr(p, "keyIntList", []int{}))
	assert.Equal(t, 3.5, bellui.PrefOr(p, "keyFloat", 0.0))
	assert.Equal(t, []float64{1.1, 2.2, 3.3}, bellui.PrefOr(p, "keyFloatList", []float64{}))
	assert.Equal(t, true, bellui.PrefOr(p, "keyBool", false))
	assert.Equal(t, []bool{true, false, true}, bellui.PrefOr(p, "keyBoolList", []bool{}))
	assert.Empty(t, bellui.PrefOr(p, "keyEmptyList", []string{}))
}

// TestPreferences_SaveDiscardsOnError verifies that a failed write does not
// clobber the existing good preferences file: the in-progress save is aborted
// (temp file removed, target untouched) rather than committed (#78).
func TestPreferences_SaveDiscardsOnError(t *testing.T) {
	p := loadPreferences(t, "dummy")
	p.WriteValues(func(val map[string]any) {
		val["key"] = "new"
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	const good = `{"existing":"data"}`
	assert.NoError(t, os.WriteFile(path, []byte(good), 0o600))

	raw, err := p.storageWriterForPath(path)
	assert.NoError(t, err)
	failing := &syncFailWriter{atomicFileWriter: raw.(*atomicFileWriter)}

	err = p.saveToStorage(failing)
	assert.Error(t, err, "a failed Sync should surface as an error")

	// The previous file must be left intact, not replaced by the new data.
	content, readErr := os.ReadFile(path)
	assert.NoError(t, readErr)
	assert.JSONEq(t, good, string(content), "existing preferences must survive a failed save")

	// The aborted temp file must not be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".preferences-"),
			"aborted write leaked a temp file: %s", e.Name())
	}
}

func TestPreferences_Save_OverwriteFast(t *testing.T) {
	p := loadPreferences(t, "dummy2")
	p.WriteValues(func(val map[string]any) {
		val["key"] = "value"
	})

	path := filepath.Join(t.TempDir(), "belluiPrefs2.json")
	p.saveToFile(path)

	p.WriteValues(func(val map[string]any) {
		val["key2"] = "value2"
	})
	p.saveToFile(path)

	p2 := loadPreferences(t, "dummy")
	p2.loadFromFile(path)
	assert.Equal(t, "value2", bellui.PrefOr(p2, "key2", ""))
}

func TestPreferences_Load(t *testing.T) {
	p := loadPreferences(t, "dummy")
	p.loadFromFile(filepath.Join("testdata", "preferences.json"))

	assert.Equal(t, "value", bellui.PrefOr(p, "keyString", ""))
	assert.Equal(t, []string{"1", "2", "3"}, bellui.PrefOr(p, "keyStringList", []string{}))
	assert.Equal(t, 4, bellui.PrefOr[int](p, "keyInt", 0))
	assert.Equal(t, []int{1, 2, 3}, bellui.PrefOr(p, "keyIntList", []int{}))
	assert.Equal(t, 3.5, bellui.PrefOr(p, "keyFloat", 0.0))
	assert.Equal(t, []float64{1.1, 2.2, 3.3}, bellui.PrefOr(p, "keyFloatList", []float64{}))
	assert.Equal(t, true, bellui.PrefOr(p, "keyBool", false))
	assert.Equal(t, []bool{true, false, true}, bellui.PrefOr(p, "keyBoolList", []bool{}))
	assert.Empty(t, bellui.PrefOr(p, "keyEmptyList", []string{}))
}

func TestPreferences_EmptyLoad(t *testing.T) {
	p := newPreferences(&belluiApp{uniqueID: ""})

	count := 0
	p.ReadValues(func(v map[string]any) {
		for range v {
			count++
		}
	})

	assert.Zero(t, count)
}
