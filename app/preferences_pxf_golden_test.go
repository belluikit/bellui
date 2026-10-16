// The fixture is loaded through the real on-disk path (`!wasm`: loadFromFile),
// deliberately, so the format sniff that runs on app start is exercised — and a
// browser has no testdata directory to read it from anyway (#77). The format's
// marshal/unmarshal round-trip is covered platform-independently in
// preferences_pxf_test.go.
//go:build !wasm

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenPreferences is the exact set of values encoded in the committed
// testdata/preferences.pxf fixture: one of every prefspb.Value kind, including
// the int-vs-float scalar distinction (and []int-vs-[]float64 list distinction)
// that the PXF store exists to preserve — the legacy JSON store collapsed both
// to a number. Keep this in sync with the fixture via the regeneration guard in
// the test below.
var goldenPreferences = map[string]any{
	"keyBool":       true,
	"keyInt":        42,
	"keyFloat":      3.5,
	"keyString":     "hello world",
	"keyBoolList":   []bool{true, false, true},
	"keyIntList":    []int{1, 2, 3},
	"keyFloatList":  []float64{1.5, 2.5, 3.5},
	"keyStringList": []string{"a", "b", "c"},
}

// TestPreferencesPXF_GoldenDecode is a persisted-format regression guard (pre-rebrand tracker #931).
// It loads a committed testdata/preferences.pxf — frozen from an earlier run of
// the writer, i.e. a file an older app version would have written to disk — and
// asserts preferences_pxf.go still decodes it back to the exact typed map. The
// buf breaking check (pre-rebrand tracker #930) guards the .proto schema against wire-incompatible
// changes, but it cannot see the conversion code (preferenceValueFromProto);
// this test closes that gap, so a refactor that silently breaks reading an old
// on-disk store fails CI instead of corrupting users' saved preferences.
//
// The fixture is deliberately frozen — do NOT regenerate it to make a failing
// test pass, since that would mask exactly the regression this guards. Only when
// the on-disk format changes on purpose, regenerate with:
//
//	BELLUI_REGEN_GOLDEN=1 go test -run TestPreferencesPXF_GoldenDecode ./app/
func TestPreferencesPXF_GoldenDecode(t *testing.T) {
	path := filepath.Join("testdata", "preferences.pxf")

	if os.Getenv("BELLUI_REGEN_GOLDEN") != "" {
		data, err := marshalPreferencesPXF(goldenPreferences)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		t.Logf("regenerated %s (%d bytes)", path, len(data))
	}

	// Load through the real file path so the format sniff + PXF decode that runs
	// on every app start is exercised, not just the raw unmarshal helper.
	p := newPreferences(&belluiApp{uniqueID: "golden-decode"})
	require.NoError(t, p.loadFromFile(path))

	got := map[string]any{}
	p.ReadValues(func(v map[string]any) {
		for k, vv := range v {
			got[k] = vv
		}
	})

	// Exact typed equality: keyInt must be int (not float64/int64), keyFloat must
	// be float64, and each list must keep its element type — the whole point of
	// the PXF format over the legacy JSON store.
	assert.Equal(t, goldenPreferences, got)
}
