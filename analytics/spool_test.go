package analytics

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
)

type spoolProp struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func spoolEvents() []Event {
	ts := time.Date(2026, 10, 22, 9, 30, 15, 123456789, time.UTC)
	return []Event{
		{
			Kind: KindTrack, Name: "opened", SessionID: "s1", Timestamp: ts,
			Props: map[string]any{
				"str": "x", "int": 42, "int64": int64(1) << 40, "float": 2.5, "bool": true, "nil": nil,
				"list": []any{"a", 1}, "map": map[string]any{"k": "v"},
				"strings": []string{"p", "q"}, "time": ts, "struct": spoolProp{A: 1, B: "two"},
			},
		},
		{Kind: KindIdentify, UserID: "u1", SessionID: "s1", Timestamp: ts, Traits: map[string]any{"plan": "pro"}},
		{Kind: KindTrack, Name: "bare", Timestamp: ts},
	}
}

// The PXF spool reads back what the JSON spool did before #523: the same
// events, with property values as JSON gave them.
func TestSpool_MatchesJSON(t *testing.T) {
	events := spoolEvents()
	data, err := marshalSpool(events)
	require.NoError(t, err)
	assert.NotEqual(t, byte('['), data[0], "written as PXF")
	got, err := unmarshalSpool(data)
	require.NoError(t, err)

	legacy, err := json.Marshal(events)
	require.NoError(t, err)
	var want []Event
	require.NoError(t, json.Unmarshal(legacy, &want))

	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, want[i].Kind, got[i].Kind)
		assert.Equal(t, want[i].Name, got[i].Name)
		assert.Equal(t, want[i].Props, got[i].Props, "event %d props", i)
		assert.Equal(t, want[i].Traits, got[i].Traits, "event %d traits", i)
		assert.Equal(t, want[i].UserID, got[i].UserID)
		assert.Equal(t, want[i].SessionID, got[i].SessionID)
		assert.True(t, want[i].Timestamp.Equal(got[i].Timestamp), "event %d timestamp", i)
	}
}

// A spool written as JSON by a version before #523 still loads.
func TestSpool_ReadsLegacyJSON(t *testing.T) {
	legacy, err := json.Marshal(spoolEvents())
	require.NoError(t, err)
	got, err := unmarshalSpool(legacy)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "opened", got[0].Name)
	assert.Equal(t, map[string]any{"plan": "pro"}, got[1].Traits)
}

// BelluiStore reads a spool saved under the JSON name while there is none under
// the new name, and the first Save removes the old file (#523).
func TestBelluiStore_LegacySpoolFile(t *testing.T) {
	test.NewTempApp(t)
	st := bellui.CurrentApp().Storage()
	s := NewBelluiStore(bellui.CurrentApp())
	require.NoError(t, s.Clear())
	t.Cleanup(func() { _ = s.Clear() })

	w, err := st.Create(legacySpoolFileName)
	require.NoError(t, err)
	_, err = w.Write([]byte(`[{"kind":"track","name":"old","timestamp":"2026-10-22T09:30:15Z"}]`))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	data, err := s.Load()
	require.NoError(t, err)
	events, err := unmarshalSpool(data)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "old", events[0].Name)

	require.NoError(t, s.Save([]byte("events = []")))
	_, err = st.Open(legacySpoolFileName)
	assert.Error(t, err, "Save removed the legacy spool")
	r, err := st.Open(spoolFileName)
	require.NoError(t, err)
	got, _ := io.ReadAll(r)
	r.Close()
	assert.Equal(t, "events = []", string(got))
}
