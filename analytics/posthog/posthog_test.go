package posthog

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
	var got phPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		gz, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		raw, _ := io.ReadAll(gz)
		require.NoError(t, json.Unmarshal(raw, &got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New("phc_test")
	s.url = srv.URL // redirect to the test server

	batch := []analytics.Event{
		{Kind: analytics.KindTrack, Name: "opened", SessionID: "s1", Timestamp: time.Unix(1, 0)},
		{Kind: analytics.KindIdentify, UserID: "u1", Traits: map[string]any{"plan": "pro"}, SessionID: "s1", Timestamp: time.Unix(2, 0)},
	}
	require.NoError(t, s.Send(batch))

	assert.Equal(t, "phc_test", got.APIKey)
	require.Len(t, got.Batch, 2)
	assert.Equal(t, "opened", got.Batch[0].Event)
	assert.Equal(t, "s1", got.Batch[0].DistinctID, "session id used when no user id")
	assert.Equal(t, "$identify", got.Batch[1].Event)
	assert.Equal(t, "u1", got.Batch[1].DistinctID, "app identity used as distinct_id")
	assert.Equal(t, map[string]any{"plan": "pro"}, got.Batch[1].Properties["$set"])
}

func TestNewWithHost(t *testing.T) {
	s := NewWithHost("k", "https://eu.i.posthog.com/")
	assert.Equal(t, "https://eu.i.posthog.com/batch/", s.url)
}
