package analytics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpool_GoldenDecode guards the on-disk spool format (#523): it decodes a
// committed testdata/spool.pxf, a file this version wrote, so a change that
// stops reading spools older versions left on users' machines fails here.
// Do not regenerate it to make the test pass; only when the format changes on
// purpose:
//
//	BELLUI_REGEN_GOLDEN=1 go test -run TestSpool_GoldenDecode ./analytics/
func TestSpool_GoldenDecode(t *testing.T) {
	path := filepath.Join("testdata", "spool.pxf")
	ts := time.Date(2026, 10, 22, 9, 30, 15, 0, time.UTC)
	golden := []Event{
		{Kind: KindTrack, Name: "opened", SessionID: "s1", Timestamp: ts, Props: map[string]any{"screen": "home", "count": 3, "dark": true}},
		{Kind: KindIdentify, UserID: "u1", SessionID: "s1", Timestamp: ts, Traits: map[string]any{"plan": "pro"}},
	}
	if os.Getenv("BELLUI_REGEN_GOLDEN") != "" {
		data, err := marshalSpool(golden)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0o600))
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	got, err := unmarshalSpool(data)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "opened", got[0].Name)
	assert.Equal(t, map[string]any{"screen": "home", "count": float64(3), "dark": true}, got[0].Props)
	assert.Equal(t, KindIdentify, got[1].Kind)
	assert.Equal(t, "u1", got[1].UserID)
	assert.Equal(t, map[string]any{"plan": "pro"}, got[1].Traits)
	assert.True(t, ts.Equal(got[1].Timestamp))
}
