package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
)

// TestPreferencesPXF_RoundTrip checks every supported preference type survives a
// PXF marshal/unmarshal cycle, including the int-vs-float distinction the legacy
// JSON store could not preserve.
func TestPreferencesPXF_RoundTrip(t *testing.T) {
	in := map[string]any{
		"str":       "hello",
		"int":       7,
		"float":     3.5,
		"bool":      true,
		"strList":   []string{"a", "b"},
		"intList":   []int{1, 2, 3},
		"floatList": []float64{1.1, 2.2},
		"boolList":  []bool{true, false},
		"emptyList": []string{},
	}

	data, err := marshalPreferencesPXF(in)
	assert.NoError(t, err)
	assert.False(t, isLegacyJSONPreferences(data), "PXF output must not look like JSON")

	out := map[string]any{}
	assert.NoError(t, unmarshalPreferencesPXF(data, out))

	assert.Equal(t, "hello", out["str"])
	assert.Equal(t, 7, out["int"], "int must round-trip as int, not float64")
	assert.Equal(t, 3.5, out["float"])
	assert.Equal(t, true, out["bool"])
	assert.Equal(t, []string{"a", "b"}, out["strList"])
	assert.Equal(t, []int{1, 2, 3}, out["intList"], "[]int must round-trip as []int")
	assert.Equal(t, []float64{1.1, 2.2}, out["floatList"])
	assert.Equal(t, []bool{true, false}, out["boolList"])
	// An empty list keeps its key but is untyped on read (matching the legacy JSON
	// store), so PrefListOr[T] can adapt it to the reader's requested type.
	assert.Contains(t, out, "emptyList")
	assert.Empty(t, out["emptyList"])
}

// TestPreferencesPXF_EmptyUntypedList covers the migration edge case: a legacy JSON
// empty array decodes to []any{} (convertLists cannot type it), which must still be
// preserved — as an empty list — across a PXF save rather than dropped.
func TestPreferencesPXF_EmptyUntypedList(t *testing.T) {
	data, err := marshalPreferencesPXF(map[string]any{"emptyAny": []any{}})
	assert.NoError(t, err)

	out := map[string]any{}
	assert.NoError(t, unmarshalPreferencesPXF(data, out))

	v, ok := out["emptyAny"]
	assert.True(t, ok, "empty untyped list key must survive the PXF save")
	assert.Empty(t, v)
}

// TestPreferencesPXF_EmptyListAdaptsToReaderType pins the residual migration
// contract: an empty list read back from PXF is untyped, so a typed reader gets an
// empty slice of its own type rather than the fallback — exactly as the legacy JSON
// store behaved (a JSON [] could not carry its element type).
func TestPreferencesPXF_EmptyListAdaptsToReaderType(t *testing.T) {
	data, err := marshalPreferencesPXF(map[string]any{"list": []int{}})
	assert.NoError(t, err)

	out := map[string]any{}
	assert.NoError(t, unmarshalPreferencesPXF(data, out))

	p := internal.NewInMemoryPreferences()
	p.WriteValues(func(v map[string]any) {
		for k, vv := range out {
			v[k] = vv
		}
	})
	assert.Equal(t, []int{}, bellui.PrefListOr(p, "list", []int{99}), "empty list must adapt to reader type, not return the fallback")
}

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

func TestIsLegacyJSONPreferences(t *testing.T) {
	assert.True(t, isLegacyJSONPreferences([]byte(`{"a":1}`)))
	assert.True(t, isLegacyJSONPreferences([]byte("  \n\t{}")))
	assert.False(t, isLegacyJSONPreferences([]byte("values {\n  key: \"a\"\n}")))
	assert.False(t, isLegacyJSONPreferences(nil))
	assert.False(t, isLegacyJSONPreferences([]byte("   ")))
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
