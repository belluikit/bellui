package mmedia

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// TestExpiredLinkSeekBackIsAnError: a song over a signed https link, read
// through Go as on Android, whose link expires mid-song. A seek back needs a
// new request, which the server refuses with 403; the app hears OnError with
// that reason. (This already worked; it guards the Go reader's half of
// player#237. The drop below is the half that did not.)
func TestExpiredLinkSeekBackIsAnError(t *testing.T) {
	data, err := os.ReadFile("internal/source/testdata/long.mp3")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	expired := false
	var log []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		e := expired
		log = append(log, r.Header.Get("Range"))
		mu.Unlock()
		if e {
			http.Error(w, "Request has expired", http.StatusForbidden)
			return
		}
		http.ServeContent(w, r, "s.mp3", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(source.UseGoHTTPSForProbes(srv.Client()))

	ended := make(chan struct{}, 1)
	failed := make(chan error, 1)
	p, err := NewAudioPlaybackURL(srv.URL+"/s.mp3?X-Amz-Signature=secret",
		WithVolume(0.01),
		WithOnEnd(func() { ended <- struct{}{} }),
		WithOnError(func(err error) { failed <- err }))
	if err != nil {
		if strings.Contains(err.Error(), "audio device") {
			t.Skipf("no audio device: %v", err)
		}
		t.Fatal(err)
	}
	defer p.Close()
	// Far ahead while the link is good, so the read buffer holds the end of
	// the song and not its start.
	p.Seek(50 * time.Second)
	p.Play()
	time.Sleep(time.Second)
	mu.Lock()
	expired = true
	mu.Unlock()
	p.Seek(0)

	select {
	case err := <-failed:
		if !strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "secret") {
			t.Errorf("OnError %q; want the 403, without the signature", err)
		}
	case <-ended:
		mu.Lock()
		t.Fatalf("OnEnd after the seek back was refused with 403; want OnError (requests: %q)", log)
		mu.Unlock()
	case <-time.After(20 * time.Second):
		t.Fatal("neither OnEnd nor OnError within 20s")
	}
}

// TestDroppedThenRefusedIsAnErrorNotTheEnd (player#237): a song over FFmpeg's
// own http whose connection drops mid-song after its signed link expired.
// FFmpeg reconnects, the server refuses that with 403, and FFmpeg calls it
// the end of file. The app heard OnEnd with the clock at the song's length,
// and took a song it had heard a minute of as finished. It must hear OnError
// naming the 403, and the clock must stay where playback stopped.
func TestDroppedThenRefusedIsAnErrorNotTheEnd(t *testing.T) {
	data, err := os.ReadFile("internal/source/testdata/long.mp3")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var log []string
	served := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		first := !served
		served = true
		log = append(log, r.Header.Get("Range"))
		mu.Unlock()
		if !first {
			// The link expired: every later request is refused.
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

	ended := make(chan time.Duration, 1)
	failed := make(chan error, 1)
	var mu2 sync.Mutex
	var pos time.Duration
	p, err := NewAudioPlaybackURL(srv.URL+"/s.mp3?X-Amz-Signature=secret",
		WithVolume(0.01),
		WithOnEnd(func() { mu2.Lock(); ended <- pos; mu2.Unlock() }),
		WithOnError(func(err error) { failed <- err }),
		WithOnTimeUpdate(func(position, _ time.Duration) { mu2.Lock(); pos = position; mu2.Unlock() }))
	if err != nil {
		if strings.Contains(err.Error(), "audio device") {
			t.Skipf("no audio device: %v", err)
		}
		t.Fatal(err)
	}
	defer p.Close()
	p.Play()

	select {
	case err := <-failed:
		if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "secret") {
			t.Errorf("OnError %q; want the 403 and the expired-link hint, without the signature", err)
		}
		mu2.Lock()
		at := pos
		mu2.Unlock()
		if at > 30*time.Second {
			t.Errorf("the clock went to %v after the error; want it where playback stopped, near 3s", at)
		}
	case at := <-ended:
		mu.Lock()
		t.Fatalf("OnEnd at %v after the reconnect was refused with 403; want OnError (requests: %q)", at, log)
		mu.Unlock()
	case <-time.After(30 * time.Second):
		t.Fatal("neither OnEnd nor OnError within 30s")
	}
}
