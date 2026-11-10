package source

import (
	"bytes"
	"errors"
	"image"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/codecset"
)

const indexAtEnd = "testdata/sample.mp4" // moov after mdat: the open seeks to the end

// goHTTPS takes the Go path for https, trusting srv's certificate, until the
// test ends. The fixtures are smaller than httpSkipLimit, so it is lowered for
// a seek to make the range request it would make in a real file.
func goHTTPS(t *testing.T, srv *httptest.Server) {
	t.Helper()
	oldGo, oldClient, oldSkip := httpsInGo, httpClient, httpSkipLimit
	httpsInGo, httpClient, httpSkipLimit = true, srv.Client(), 1024
	t.Cleanup(func() { httpsInGo, httpClient, httpSkipLimit = oldGo, oldClient, oldSkip })
}

// rangeLog records the start of each request's Range, -1 for none.
type rangeLog struct {
	mu     sync.Mutex
	starts []int64
}

func (l *rangeLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	start := int64(-1)
	if v, ok := strings.CutPrefix(r.Header.Get("Range"), "bytes="); ok {
		first, _, _ := strings.Cut(v, "-")
		start, _ = strconv.ParseInt(first, 10, 64)
	}
	l.starts = append(l.starts, start)
}

func (l *rangeLog) get() []int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]int64(nil), l.starts...)
}

// serveTLS serves data over https, honouring Range unless ignoreRange.
func serveTLS(t *testing.T, data []byte, ignoreRange bool) (*httptest.Server, *rangeLog) {
	t.Helper()
	log := &rangeLog{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if ignoreRange {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			_, _ = w.Write(data)
			return
		}
		http.ServeContent(w, r, "m.mp4", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	goHTTPS(t, srv)
	return srv, log
}

func control(t *testing.T, path string) (frames, audio int) {
	t.Helper()
	s, err := NewFFmpegSourceURL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	return decodeAll(t, s, 10*time.Second)
}

// TestHTTPSInGoPlaysAndSeeks is #871: an https URL plays through Go's TLS, as
// it must on Android, where FFmpeg has none. The file has its index at the end,
// so the open has to fetch the end and come back, and must do it with ranges
// rather than by reading the whole file. A server that ignores Range still
// plays, read through from the start (belluikit/player#148 suspected this).
func TestHTTPSInGoPlaysAndSeeks(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(indexAtEnd)
	if err != nil {
		t.Fatal(err)
	}
	wantFrames, wantAudio := control(t, indexAtEnd)

	for _, ignoreRange := range []bool{false, true} {
		name := "ranges"
		if ignoreRange {
			name = "no-ranges"
		}
		t.Run(name, func(t *testing.T) {
			srv, log := serveTLS(t, data, ignoreRange)
			s, err := NewFFmpegSourceURL(srv.URL + "/m.mp4")
			if err != nil {
				t.Fatalf("NewFFmpegSourceURL: %v", err)
			}
			defer s.Close()
			if _, ok := s.dec.(*readerDecoder); !ok {
				t.Fatalf("decoder is %T; want the https URL read through Go", s.dec)
			}
			if d := s.Duration(); d < 3*time.Second {
				t.Errorf("Duration = %v; want >= 3s, which needs the index at the end", d)
			}
			if !ignoreRange {
				var reachedEnd bool
				for _, st := range log.get() {
					reachedEnd = reachedEnd || st >= 60922 // the moov box
				}
				if !reachedEnd {
					t.Errorf("requests started at %v; want one at the index, past 60922", log.get())
				}
			}

			frames, audio := decodeAll(t, s, 20*time.Second)
			if err := s.Err(); err != nil {
				t.Fatalf("ended on %v", err)
			}
			if frames != wantFrames || audio != wantAudio {
				t.Errorf("decoded %d frames and %d audio bytes; want the file's %d and %d",
					frames, audio, wantFrames, wantAudio)
			}

			// A seek straight after the first frame, as Player's onLoaded does.
			s.Rewind()
			s.SetVideoCallback(nil)
			s.Decode(0)
			var got []time.Duration
			s.SetVideoCallback(func(_ *image.RGBA, pts time.Duration) { got = append(got, pts) })
			s.Seek(3 * time.Second)
			s.Decode(0)
			if err := s.Err(); err != nil || len(got) == 0 || got[0] < 2*time.Second {
				t.Errorf("after a seek to 3s: frames at %v, error %v; want a frame near 3s", got, err)
			}
			t.Logf("%s: requests started at %v", name, log.get())
		})
	}
}

// TestHTTPSInGoRidesOutANetworkFailure is TestURLSourceRidesOutANetworkFailure
// for the Go reader: a drop and a stall are each resumed from where they
// stopped, and the whole file decodes.
func TestHTTPSInGoRidesOutANetworkFailure(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	wantFrames, wantAudio := control(t, reconnectMedia)
	oldTimeout := httpReadTimeout
	httpReadTimeout = 500 * time.Millisecond
	t.Cleanup(func() { httpReadTimeout = oldTimeout })

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
			dec, err := openHTTPSInGo(srv.URL + "/s.mp4") // the reader does not mind http
			if err != nil {
				t.Fatalf("openHTTPSInGo: %v", err)
			}
			s := newFFmpegSource(dec)
			defer s.Close()
			start := time.Now()
			frames, audio := decodeAll(t, s, 20*time.Second)
			t.Logf("%s: %d frames, %d audio bytes over %d requests in %v",
				tc.name, frames, audio, requests(), time.Since(start).Round(time.Millisecond))
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

// TestHTTPSInGoRetriesAnOpen is #873: an open that fails on the network, or on
// a server error, is tried again, as a read is. On a device two opens timed out
// together in a moment the phone's network was away, and nothing retried them.
// A failure no retry can fix is reported at once, and a link that never answers
// is reported within httpOpenTimeout.
func TestHTTPSInGoRetriesAnOpen(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	oldTimeout, oldOpen := httpReadTimeout, httpOpenTimeout
	httpReadTimeout, httpOpenTimeout = 300*time.Millisecond, 2*time.Second
	t.Cleanup(func() { httpReadTimeout, httpOpenTimeout = oldTimeout, oldOpen })

	drop := func(w http.ResponseWriter, _ *http.Request) {
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
	}
	stall := func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }
	// Headers, then nothing: the open succeeds and the read of its first bytes
	// stalls.
	stallBody := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}
	unavailable := func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}
	notFound := func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }

	for _, tc := range []struct {
		name     string
		fail     func(http.ResponseWriter, *http.Request)
		failures int // requests that fail before the server answers; -1 for all
		requests int // requests the open should make
		ok       bool
	}{
		{"dropped", drop, 2, 3, true},
		{"stalled", stall, 1, 2, true},
		{"stalled body", stallBody, 1, 2, true},
		{"server error", unavailable, 3, 4, true},
		{"not found", notFound, -1, 1, false},
		{"never answers", stall, -1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			n := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				n++
				failing := tc.failures < 0 || n <= tc.failures
				mu.Unlock()
				if failing {
					tc.fail(w, r)
					return
				}
				http.ServeContent(w, r, "s.mp4", time.Time{}, bytes.NewReader(data))
			}))
			t.Cleanup(srv.Close)
			requests := func() int { mu.Lock(); defer mu.Unlock(); return n }

			start := time.Now()
			dec, err := openHTTPSInGo(srv.URL + "/s.mp4")
			took := time.Since(start)
			if dec != nil {
				dec.Close()
			}
			t.Logf("%s: %v after %v and %d requests", tc.name, err, took.Round(time.Millisecond), requests())
			if tc.ok != (err == nil) {
				t.Fatalf("open: %v; want ok=%v", err, tc.ok)
			}
			if tc.requests > 0 && requests() < tc.requests {
				t.Errorf("%d request(s); want at least %d", requests(), tc.requests)
			}
			if !tc.ok && tc.requests == 1 && requests() != 1 {
				t.Errorf("%d requests; want one, since no retry can fix a 404", requests())
			}
			if limit := httpOpenTimeout + 200*time.Millisecond; took > limit {
				t.Errorf("open took %v; want it within httpOpenTimeout, %v", took, httpOpenTimeout)
			}
		})
	}
}

// TestHTTPSInGoReportsAnExpiredLink: a signed link that expires mid-playback
// keeps playing on the open response, and the next request, here a seek back,
// fails with a reason a person can act on, not goffmpeg's -1. The signature in
// the query stays out of the message.
func TestHTTPSInGoReportsAnExpiredLink(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	expired := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		e := expired
		mu.Unlock()
		if e {
			http.Error(w, "Request has expired", http.StatusForbidden)
			return
		}
		http.ServeContent(w, r, "s.mp4", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	goHTTPS(t, srv)

	s, err := NewFFmpegSourceURL(srv.URL + "/s.mp4?X-Amz-Signature=secret")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Decode(0)
	mu.Lock()
	expired = true
	mu.Unlock()
	s.Seek(0)

	err = s.Err()
	if err == nil {
		t.Fatal("a seek after the link expired reported no error")
	}
	var se *statusError
	if !errors.As(err, &se) || se.code != http.StatusForbidden {
		t.Errorf("error %q; want the 403", err)
	}
	if !strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q; want it to name expiry and leave out the signature", err)
	}
}

// TestHTTPSInGoRefusesHLS: a playlist cannot be read through Go, so an https
// one fails at open with a reason rather than as FFmpeg's -1.
func TestHTTPSInGoRefusesHLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.FileServer(http.Dir("testdata/hls")))
	t.Cleanup(srv.Close)
	goHTTPS(t, srv)
	_, err := NewFFmpegSourceURL(srv.URL + "/stream.m3u8")
	if !errors.Is(err, errHLSOverReader) {
		t.Fatalf("error %v; want errHLSOverReader", err)
	}
}

// TestHTTPSInGoNamesAnUnsizedServer: a server that ignores Range and sends no
// length gives a stream that cannot seek, so a file with its index at the end
// cannot play. FFmpeg's own http fails on it too; the error should say why.
// filesamples.com answered this way on 2026-11-01.
func TestHTTPSInGoNamesAnUnsizedServer(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(indexAtEnd)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush() // no Content-Length: chunked
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	goHTTPS(t, srv)

	s, err := NewFFmpegSourceURL(srv.URL + "/m.mp4")
	if err == nil {
		defer s.Close()
		decodeAll(t, s, 10*time.Second)
		err = s.Err()
	}
	if err == nil || !strings.Contains(err.Error(), "sent no length") {
		t.Fatalf("error %v; want one naming the missing length", err)
	}
}

// TestHTTPSInGoThumbnailer: scrub preview opens the URL a second time, and gets
// the Go reader too.
func TestHTTPSInGoThumbnailer(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(indexAtEnd)
	if err != nil {
		t.Fatal(err)
	}
	srv, _ := serveTLS(t, data, false)
	th := NewThumbnailer(srv.URL + "/m.mp4")
	defer th.Close()
	img, err := th.FrameAt(2 * time.Second)
	if err != nil || img == nil {
		t.Fatalf("FrameAt: %v", err)
	}
}

// TestHTTPReaderCloseUnblocksARead: Close from another goroutine ends a read
// waiting on a silent server at once, not after the read timeout.
func TestHTTPReaderCloseUnblocksARead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("0123456789"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	r, err := newHTTPReader(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(r, make([]byte, 10)); err != nil {
		t.Fatal(err) // everything the server sent; the next read waits
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.Read(make([]byte, 10))
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	r.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a read on a closed reader succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not unblock the read")
	}
}

func TestParseContentRange(t *testing.T) {
	for _, tc := range []struct {
		in           string
		start, total int64
		ok           bool
	}{
		{"bytes 0-99/100", 0, 100, true},
		{"bytes 60922-65619/65620", 60922, 65620, true},
		{"bytes 5-9/*", 5, -1, true},
		{"items 0-1/2", 0, 0, false},
		{"bytes x-1/2", 0, 0, false},
	} {
		start, total, ok := parseContentRange(tc.in)
		if start != tc.start || total != tc.total || ok != tc.ok {
			t.Errorf("parseContentRange(%q) = %d, %d, %v; want %d, %d, %v",
				tc.in, start, total, ok, tc.start, tc.total, tc.ok)
		}
	}
}
