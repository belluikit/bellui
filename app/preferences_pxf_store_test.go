// The JSON→PXF migration cases that drive the on-disk store: loadFromFile,
// storagePath and legacyStoragePath are all `!wasm` (wasm keeps a single
// localStorage key, with nothing to migrate — see cleanupLegacyStore in
// preferences_wasm.go). The format-level tests they were split from stay
// untagged in preferences_pxf_test.go (#77).
//go:build !wasm

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

// TestPreferences_LoadLegacyJSONWithBOM guards the read-both window against a
// catastrophic misclassification: a legacy JSON store carrying a UTF-8 BOM must be
// read as JSON, not routed to the PXF decoder (which would fail and silently empty
// the store, then delete the recoverable file on the next save).
func TestPreferences_LoadLegacyJSONWithBOM(t *testing.T) {
	p := loadPreferences(t, "bom")
	path := filepath.Join(t.TempDir(), "preferences.json")
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"keyString":"hi","keyInt":9}`)...)
	assert.NoError(t, os.WriteFile(path, content, 0o600))

	assert.NoError(t, p.loadFromFile(path))
	assert.Equal(t, "hi", bellui.PrefOr(p, "keyString", ""))
	assert.Equal(t, 9, bellui.PrefOr[int](p, "keyInt", 0))
}

// TestPreferences_MigrateFromLegacyJSON drives the read-both migration window
// end-to-end: a legacy preferences.json with no PXF sibling is read on load, then
// the next save writes preferences.pxf and removes the superseded JSON file.
func TestPreferences_MigrateFromLegacyJSON(t *testing.T) {
	p := loadPreferences(t, "migrate")

	pxfPath := p.storagePath()
	legacy := p.legacyStoragePath()
	assert.Equal(t, filepath.Ext(pxfPath), ".pxf")
	assert.Equal(t, filepath.Ext(legacy), ".json")

	// Seed the legacy JSON store, ensure no PXF store exists yet.
	assert.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o700))
	assert.NoError(t, os.WriteFile(legacy, []byte(`{"keyString":"hello","keyInt":7}`), 0o600))
	_ = os.Remove(pxfPath)

	// Load must fall back to (and decode) the legacy JSON store.
	p.load()
	assert.Equal(t, "hello", bellui.PrefOr(p, "keyString", ""))
	assert.Equal(t, 7, bellui.PrefOr[int](p, "keyInt", 0))

	// Saving migrates: the PXF store appears and the legacy JSON file is removed.
	assert.NoError(t, p.save())
	defer os.Remove(pxfPath)

	content, err := os.ReadFile(pxfPath)
	assert.NoError(t, err, "PXF store should be written on save")
	assert.False(t, isLegacyJSONPreferences(content))

	_, statErr := os.Stat(legacy)
	assert.True(t, os.IsNotExist(statErr), "legacy JSON store should be removed after migration")

	// And a fresh load reads the migrated PXF store.
	p2 := loadPreferences(t, "migrate")
	assert.Equal(t, "hello", bellui.PrefOr(p2, "keyString", ""))
	assert.Equal(t, 7, bellui.PrefOr[int](p2, "keyInt", 0))
}
