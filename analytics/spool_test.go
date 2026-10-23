package analytics

import (
	"encoding/json"
	"errors"
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
	assertSpoolMatchesJSON(t, spoolEvents())
}

// Zero values, empty containers and bytes that are not valid UTF-8 read back as
// JSON gave them too. PXF writes invalid UTF-8 in a string field but refuses to
// read it back, so one such event name, user ID or property key used to make the
// whole spool unreadable at the next launch.
func TestSpool_MatchesJSON_EdgeValues(t *testing.T) {
	ts := time.Date(2026, 10, 22, 9, 30, 15, 0, time.UTC)
	assertSpoolMatchesJSON(t, []Event{
		{
			Kind: KindTrack, Name: "opened\xff", SessionID: "s\xfe", Timestamp: ts,
			Props: map[string]any{
				"k\xff": "v\xfe", "nested": map[string]any{"k\xc3": 1},
				"false": false, "zero": 0, "empty": "", "emptyMap": map[string]any{}, "emptyList": []any{},
			},
		},
		{Kind: KindIdentify, UserID: "u\xe9", Traits: map[string]any{"plan\xff": "pro"}},
	})
}

func assertSpoolMatchesJSON(t *testing.T, events []Event) {
	t.Helper()
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

// flakyStorage is app storage whose writers can fail to close and whose
// removals can fail.
type flakyStorage struct {
	bellui.Storage
	closeErr  error
	removeErr error
}

type failingCloser struct {
	bellui.URIWriteCloser
	err error
}

func (w failingCloser) Close() error {
	_ = w.URIWriteCloser.Close()
	return w.err
}

func (f *flakyStorage) Create(name string) (bellui.URIWriteCloser, error) {
	w, err := f.Storage.Create(name)
	return f.wrap(w, err)
}

func (f *flakyStorage) Save(name string) (bellui.URIWriteCloser, error) {
	w, err := f.Storage.Save(name)
	return f.wrap(w, err)
}

func (f *flakyStorage) wrap(w bellui.URIWriteCloser, err error) (bellui.URIWriteCloser, error) {
	if err != nil || f.closeErr == nil {
		return w, err
	}
	return failingCloser{w, f.closeErr}, nil
}

func (f *flakyStorage) Remove(name string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	return f.Storage.Remove(name)
}

// The legacy spool is removed only once the new one is saved, and a removal
// that fails is tried again: either way, its events are neither lost nor left
// to be read again.
func TestBelluiStore_LegacySpoolOutlivesFailedSave(t *testing.T) {
	test.NewTempApp(t)
	fs := &flakyStorage{Storage: bellui.CurrentApp().Storage()}
	s := newBelluiStore(fs)
	require.NoError(t, s.Clear())
	t.Cleanup(func() { _ = s.Clear() })

	w, err := fs.Create(legacySpoolFileName)
	require.NoError(t, err)
	_, err = w.Write([]byte(`[{"kind":"track","name":"old"}]`))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	fs.closeErr = errors.New("close failed")
	assert.Error(t, s.Save([]byte("events = []")), "a failed close fails the save")
	assert.True(t, docExists(fs, legacySpoolFileName), "the legacy spool outlives a save that did not complete")

	fs.closeErr = nil
	fs.removeErr = errors.New("remove failed")
	require.NoError(t, s.Save([]byte("events = []")))
	assert.True(t, docExists(fs, legacySpoolFileName))

	fs.removeErr = nil
	require.NoError(t, s.Save([]byte("events = []")))
	assert.False(t, docExists(fs, legacySpoolFileName), "a failed removal is tried again")
}

func docExists(s bellui.Storage, name string) bool {
	r, err := s.Open(name)
	if err != nil {
		return false
	}
	_ = r.Close()
	return true
}

// TestSpoolPiecesMatchMarshal pins what the per-event spool cache relies on
// (#657): joining the pieces spoolPiece encodes gives exactly the bytes
// marshalSpool writes for the same events, so a spool saved from the cache is
// the same file, in the same format.
func TestSpoolPiecesMatchMarshal(t *testing.T) {
	ts := time.Date(2026, 10, 22, 9, 30, 15, 123, time.UTC)
	odd := []Event{
		{Kind: KindTrack, Name: "a\nb\t\"q\" \\ ü 😀", Timestamp: ts, Props: map[string]any{
			"list":    []any{1, "two", map[string]any{"k": nil}, []any{}},
			"empty":   map[string]any{},
			"when":    ts, // goes through JSON (jsonValue)
			"big":     1 << 53,
			"neg":     -0.5,
			"bad\xff": "x\xfey",
		}},
		{Kind: KindIdentify, UserID: "u\x00", SessionID: "s", Timestamp: ts, Traits: map[string]any{"plan": "pro"}},
		{Kind: KindTrack, Timestamp: time.Time{}},
		{Kind: KindTrack, Name: "no props", Props: map[string]any{}, Timestamp: ts},
	}
	for name, events := range map[string][]Event{
		"none":   nil,
		"one":    odd[:1],
		"odd":    odd,
		"golden": {{Kind: KindTrack, Name: "opened", SessionID: "s1", Timestamp: ts, Props: map[string]any{"screen": "home", "count": 3, "dark": true}}},
		"bench":  benchQueue(DefaultMaxSpool),
	} {
		t.Run(name, func(t *testing.T) {
			want, err := marshalSpool(events)
			require.NoError(t, err)
			pieces := make([][]byte, len(events))
			for i, e := range events {
				pieces[i], err = spoolPiece(e)
				require.NoError(t, err)
			}
			got := joinSpool(pieces)
			require.Equal(t, string(want), string(got))
			back, err := unmarshalSpool(got)
			require.NoError(t, err)
			assert.Len(t, back, len(events))
		})
	}
}
