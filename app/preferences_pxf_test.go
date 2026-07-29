package app

import (
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

func TestIsLegacyJSONPreferences(t *testing.T) {
	assert.True(t, isLegacyJSONPreferences([]byte(`{"a":1}`)))
	assert.True(t, isLegacyJSONPreferences([]byte("  \n\t{}")))
	assert.False(t, isLegacyJSONPreferences([]byte("values {\n  key: \"a\"\n}")))
	assert.False(t, isLegacyJSONPreferences(nil))
	assert.False(t, isLegacyJSONPreferences([]byte("   ")))
}
