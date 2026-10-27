package source

import (
	"bytes"
	"image"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

const reconnectMedia = "testdata/sample_stream.mp4" // index at the front

// decodeAll decodes s to its end and counts what it delivered.
func decodeAll(t *testing.T, s *FFmpegSource, within time.Duration) (frames, audioBytes int) {
	t.Helper()
	s.SetVideoCallback(func(*image.RGBA, time.Duration) { frames++ })
	s.SetAudioCallback(func(b []byte) { audioBytes += len(b) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !s.HasEnded() {
			s.Decode(0)
		}
	}()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("decoding did not finish within %v", within)
	}
	return frames, audioBytes
}

// failFirst serves data honouring Range, except that the first request sends
// the first half and then fails as fail says.
func failFirst(t *testing.T, data []byte, fail func(http.ResponseWriter, *http.Request)) (*httptest.Server, func() int) {
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		first := n == 1
		mu.Unlock()
		if !first {
			http.ServeContent(w, r, "s.mp4", time.Time{}, bytes.NewReader(data))
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		fail(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return n }
}

// TestURLSourceRidesOutANetworkFailure pins the http options of #839: a
// connection that drops, or goes silent without closing, is picked up again
// from where it stopped, and the whole file decodes. Without them a drop ended
// the stream partway and a stall blocked the read forever.
func TestURLSourceRidesOutANetworkFailure(t *testing.T) {
	data, err := os.ReadFile(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewFFmpegSourceURL(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	wantFrames, wantAudio := decodeAll(t, control, 10*time.Second)
	control.Close()

	for _, tc := range []struct {
		name string
		fail func(http.ResponseWriter, *http.Request)
	}{
		{"dropped", func(w http.ResponseWriter, _ *http.Request) {
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.(*net.TCPConn).Close()
			}
		}},
		{"stalled", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, requests := failFirst(t, data, tc.fail)
			s, err := NewFFmpegSourceURL(srv.URL + "/s.mp4")
			if err != nil {
				t.Fatalf("NewFFmpegSourceURL: %v", err)
			}
			defer s.Close()
			frames, audio := decodeAll(t, s, 20*time.Second)
			t.Logf("%s: %d frames, %d audio bytes over %d requests; the file has %d and %d",
				tc.name, frames, audio, requests(), wantFrames, wantAudio)
			if err := s.Err(); err != nil {
				t.Fatalf("ended on %v; want it to reconnect and finish", err)
			}
			if requests() < 2 {
				t.Errorf("%d request(s); want a reconnect after the failure", requests())
			}
			if frames != wantFrames || audio != wantAudio {
				t.Errorf("decoded %d frames and %d audio bytes; want the file's %d and %d",
					frames, audio, wantFrames, wantAudio)
			}
		})
	}
}
