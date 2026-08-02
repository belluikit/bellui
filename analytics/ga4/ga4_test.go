// Both tests POST batches to an in-process httptest server; a browser has
// no loopback listener, and the sink's transport is the !wasm half anyway.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package ga4

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/analytics"
)

func TestSink_Send(t *testing.T) {
	var payloads []ga4Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		gz, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		raw, _ := io.ReadAll(gz)
		var p ga4Payload
		require.NoError(t, json.Unmarshal(raw, &p))
		payloads = append(payloads, p)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := New("G-TEST", "secret")
	s.url = srv.URL // redirect to the test server

	batch := []analytics.Event{
		{Kind: analytics.KindIdentify, UserID: "u1", Traits: map[string]any{"plan": "pro"}, Timestamp: time.Unix(1, 0)},
		{Kind: analytics.KindTrack, Name: "click", Props: map[string]any{"btn": "ok"}, SessionID: "s1", UserID: "u1", Timestamp: time.Unix(2, 0)},
	}
	require.NoError(t, s.Send(batch))

	require.Len(t, payloads, 1)
	p := payloads[0]
	assert.Equal(t, "u1", p.ClientID, "app identity used as client_id")
	require.Len(t, p.Events, 1)
	assert.Equal(t, "click", p.Events[0].Name)
	assert.Equal(t, "ok", p.Events[0].Params["btn"])
}

func TestSink_ChunksOver25(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := New("G-TEST", "secret")
	s.url = srv.URL

	batch := make([]analytics.Event, 30)
	for i := range batch {
		batch[i] = analytics.Event{Kind: analytics.KindTrack, Name: "e", SessionID: "s", Timestamp: time.Unix(int64(i), 0)}
	}
	require.NoError(t, s.Send(batch))
	assert.Equal(t, 2, count, "30 events split into 25 + 5")
}
