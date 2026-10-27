package pipeline

import (
	"bytes"
	"errors"
	"fmt"
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

// failingSource is a clip that stops after frames of its total, with err as
// the reason it gives through ErrorSource. A nil err is a clean end.
type failingSource struct {
	clipSource
	total int
	err   error
}

func (s *failingSource) Duration() time.Duration { return time.Duration(s.total) * time.Second / 30 }

func (s *failingSource) Err() error {
	if s.HasEnded() {
		return s.err
	}
	return nil
}

// endReport is what a pipeline told its callbacks when its source ran out.
type endReport struct {
	ended  bool
	err    error
	atTime time.Duration // CurrentTime once it reported
}

// playToTheEnd plays p until it reports OnEnd or OnError.
func playToTheEnd(t *testing.T, p *Pipeline, within time.Duration) endReport {
	t.Helper()
	var once sync.Once
	done := make(chan endReport, 1)
	p.SetOnFrame(func(*image.RGBA) {})
	p.SetOnEnd(func() { once.Do(func() { done <- endReport{ended: true} }) })
	p.SetOnError(func(err error) { once.Do(func() { done <- endReport{err: err} }) })
	p.Play()
	select {
	case r := <-done:
		// Give a wrongly-reported second callback, or a clock still running,
		// time to show.
		time.Sleep(200 * time.Millisecond)
		r.atTime = p.CurrentTime()
		return r
	case <-time.After(within):
		t.Fatal("the pipeline reported neither an end nor an error")
		return endReport{}
	}
}

// TestSourceErrorIsAnErrorNotAnEnd pins #839: a file or stream that stopped on a
// read error (a dropped network connection reads as an I/O error from FFmpeg)
// was reported as OnEnd with the clock at the full duration, so an app could not
// tell it from a track that finished.
func TestSourceErrorIsAnErrorNotAnEnd(t *testing.T) {
	ioErr := errors.New("decoder error: -5 (input/output error)")
	src := &failingSource{clipSource: clipSource{frames: 20}, total: 90, err: ioErr}
	p := newTestPipeline(t, src)
	defer p.Close()

	r := playToTheEnd(t, p, 5*time.Second)
	if r.ended {
		t.Fatal("OnEnd fired for a source that stopped on an error")
	}
	if !errors.Is(r.err, ioErr) {
		t.Fatalf("OnError got %v; want the source's %v", r.err, ioErr)
	}
	// 20 frames is 0.67 s of a 3 s clip. The clock rests where playback
	// stopped, not at the duration and not running on.
	if r.atTime <= 0 || r.atTime >= src.Duration()/2 {
		t.Errorf("time %v after the error; want it near the %v where the frames stopped, well short of %v",
			r.atTime, 20*time.Second/30, src.Duration())
	}
}

// TestCleanEndWithAnErrorSourceIsStillAnEnd is the control for the test above:
// the same source with no error is a normal end, at the full duration.
func TestCleanEndWithAnErrorSourceIsStillAnEnd(t *testing.T) {
	src := &failingSource{clipSource: clipSource{frames: 20}, total: 20}
	p := newTestPipeline(t, src)
	defer p.Close()

	r := playToTheEnd(t, p, 5*time.Second)
	if !r.ended {
		t.Fatalf("OnError(%v) for a source that ended cleanly; want OnEnd", r.err)
	}
	if r.atTime != src.Duration() {
		t.Errorf("time %v after the end; want the %v duration", r.atTime, src.Duration())
	}
}

// TestDroppedHTTPStreamIsAnError is #839 end to end through FFmpeg: a server
// that announces the whole file and closes the connection halfway through it,
// every time. FFmpeg reconnects from where it stopped (see the source's
// httpOptions) and is dropped again at the same byte, so this is a server that
// stays broken, which has to end in OnError. The full download is the control.
// It needs a real audio device, since the detached one never plays out its
// samples.
func TestDroppedHTTPStreamIsAnError(t *testing.T) {
	if _, err := newAudioOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	// Its index is at the front, so it plays from a stream cut short.
	data, err := os.ReadFile("../source/testdata/sample_stream.mp4")
	if err != nil {
		t.Fatal(err)
	}
	half := len(data) / 2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/drop.mp4" {
			http.ServeContent(w, r, "full.mp4", time.Time{}, bytes.NewReader(data))
			return
		}
		var from int // where a reconnect asks to resume
		_, _ = fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-", &from)
		from = min(from, half)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)-from))
		if from > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = w.Write(data[from:half])
		w.(http.Flusher).Flush()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.(*net.TCPConn).Close()
		}
	}))
	defer srv.Close()

	t.Run("dropped", func(t *testing.T) {
		p, err := NewPipelineURL(srv.URL + "/drop.mp4")
		if err != nil {
			t.Fatalf("NewPipelineURL: %v", err)
		}
		defer p.Close()
		r := playToTheEnd(t, p, 10*time.Second)
		t.Logf("dropped: ended=%v err=%v at %v of %v", r.ended, r.err, r.atTime, p.Duration())
		if r.ended || r.err == nil {
			t.Fatalf("a dropped connection reported ended=%v err=%v; want OnError and no OnEnd", r.ended, r.err)
		}
		if r.atTime >= p.Duration()*3/4 {
			t.Errorf("time %v after the drop; want it short of the %v duration, near where the data stopped", r.atTime, p.Duration())
		}
	})
	t.Run("control", func(t *testing.T) {
		p, err := NewPipelineURL(srv.URL + "/full.mp4")
		if err != nil {
			t.Fatalf("NewPipelineURL: %v", err)
		}
		defer p.Close()
		r := playToTheEnd(t, p, 10*time.Second)
		t.Logf("control: ended=%v err=%v at %v of %v", r.ended, r.err, r.atTime, p.Duration())
		if !r.ended {
			t.Fatalf("the full download reported OnError(%v); want OnEnd", r.err)
		}
		if r.atTime != p.Duration() {
			t.Errorf("time %v after the end; want the %v duration", r.atTime, p.Duration())
		}
	})
}

// TestPlayAfterTheEndPlaysAgain pins #842: Play after an audio-only stream
// ended reported the end again at once, and the display loop the first Play
// started was never stopped, so the next Stop waited on it forever.
func TestPlayAfterTheEndPlaysAgain(t *testing.T) {
	if _, err := newAudioOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	p, err := NewPipelineURL("../source/testdata/sample_51.m4a") // 2 s, audio only
	if err != nil {
		t.Fatalf("NewPipelineURL: %v", err)
	}
	defer p.Close()

	if r := playToTheEnd(t, p, 10*time.Second); !r.ended {
		t.Fatalf("first play: OnError(%v); want OnEnd", r.err)
	}
	start := time.Now()
	r := playToTheEnd(t, p, 10*time.Second)
	if !r.ended {
		t.Fatalf("second play: OnError(%v); want OnEnd", r.err)
	}
	// It played from the start rather than reporting the old end: most of the
	// 2 s went by (playToTheEnd adds its 200 ms after the report).
	if took := time.Since(start); took < 1500*time.Millisecond {
		t.Errorf("second play ended after %v; want it to play the %v clip again", took, p.Duration())
	}

	// A seek after the end is where the next Play starts.
	p.Seek(p.Duration() - 500*time.Millisecond)
	start = time.Now()
	if r := playToTheEnd(t, p, 10*time.Second); !r.ended {
		t.Fatalf("play after a seek: OnError(%v); want OnEnd", r.err)
	}
	if took := time.Since(start); took > 1200*time.Millisecond {
		t.Errorf("play from 0.5 s before the end took %v; want it to start at the seek, not at 0", took)
	}

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop hung after playing a second time")
	}
}
