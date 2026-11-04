package source

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/goffmpeg"
)

// TestDroppedThenRefusedEndsWithTheStatus (player#237): a song over FFmpeg's
// own http whose connection drops mid-song after its signed link expired.
// FFmpeg reconnects, the server refuses that with 403, and FFmpeg calls it
// the end of the file. The source must stop with an error naming the 403,
// which the pipeline reports through OnError, not as the end that made an
// app take a song heard for a minute as finished. Decoded here without an
// audio device, so it runs on any CI host.
func TestDroppedThenRefusedEndsWithTheStatus(t *testing.T) {
	data, err := os.ReadFile("testdata/long.mp3")
	if err != nil {
		t.Fatal(err)
	}
	var served atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Swap(true) {
			http.Error(w, "Request has expired", http.StatusForbidden)
			return
		}
		// About 3 s of a 60 s song, then the connection drops.
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Write(data[:3000])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	t.Cleanup(srv.Close)

	s, err := NewFFmpegSourceURL(srv.URL + "/s.mp3?X-Amz-Signature=secret")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	deadline := time.Now().Add(20 * time.Second)
	for !s.ended && time.Now().Before(deadline) {
		s.Decode(0)
	}
	if !s.ended {
		t.Fatal("the source neither ended nor failed within 20s")
	}
	err = s.Err()
	if err == nil {
		t.Fatal("the stream ended with no error after the reconnect was refused with 403: the app would take the song as finished")
	}
	var se *statusError
	if !errors.As(err, &se) || se.code != http.StatusForbidden || !errors.Is(err, goffmpeg.ErrStreamTruncated) {
		t.Errorf("error %q; want the 403, wrapping goffmpeg.ErrStreamTruncated", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q quotes the link's signature", err)
	}
}
