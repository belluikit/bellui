package analytics

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostJSONGzip(t *testing.T) {
	var gotEncoding, gotType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gotType = r.Header.Get("Content-Type")
		gz, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		raw, err := io.ReadAll(gz)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := PostJSONGzip(srv.URL, map[string]string{"X-Test": "1"}, map[string]any{"hello": "world"})
	require.NoError(t, err)
	assert.Equal(t, "gzip", gotEncoding)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, "world", gotBody["hello"])
}

func TestPostJSONGzip_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	assert.Error(t, PostJSONGzip(srv.URL, nil, map[string]any{}))
}
