package mmedia

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	urlTestMedia = "internal/source/testdata/sample.mp4"
	hlsTestDir   = "internal/source/testdata/hls"
)

// TestNewMediaPlayerURLLocalPath covers the path form, where FFmpeg opens the
// file itself rather than reading through a Go callback.
func TestNewMediaPlayerURLLocalPath(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia)
	if err != nil {
		t.Fatalf("NewMediaPlayerURL: %v", err)
	}
	defer p.Close()

	if w, h := p.VideoSize(); w != 320 || h != 240 {
		t.Errorf("VideoSize = %dx%d, want 320x240", w, h)
	}
	if d := p.Duration(); d < 3*time.Second {
		t.Errorf("Duration = %v, want >= 3s", d)
	}
}

// TestNewMediaPlayerURLOverHTTP is the point of the URL entry point: FFmpeg
// owns the transport, so it can issue its own requests rather than being handed
// a byte stream. Serving from httptest keeps it hermetic.
func TestNewMediaPlayerURLOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("internal/source/testdata")))
	defer srv.Close()

	p, err := NewMediaPlayerURL(srv.URL + "/sample.mp4")
	if err != nil {
		t.Fatalf("NewMediaPlayerURL over http: %v", err)
	}
	defer p.Close()

	if w, h := p.VideoSize(); w != 320 || h != 240 {
		t.Errorf("VideoSize = %dx%d, want 320x240", w, h)
	}
	// A duration proves the trailing moov was reached, which needs range
	// requests — the capability the io.Reader path does not have.
	if d := p.Duration(); d < 3*time.Second {
		t.Errorf("Duration = %v, want >= 3s (seeking over http failed)", d)
	}
}

// TestHLSOverURL is the claim the URL entry point exists for. An HLS playlist
// is a text file listing segment URLs; the demuxer has to fetch those itself,
// which it can only do when it owns the protocol handler.
func TestHLSOverURL(t *testing.T) {
	if _, err := os.Stat(hlsTestDir); err != nil {
		t.Skipf("hls fixture unavailable: %v", err)
	}
	// The playlist alone is not enough, and its absence is easy to cause: a
	// stray *.ts rule in .gitignore once committed the playlist without the
	// segments, so this passed locally and failed in CI with nothing but
	// "Failed to open segment 0" from the demuxer.
	segs, _ := filepath.Glob(filepath.Join(hlsTestDir, "*.ts"))
	if len(segs) == 0 {
		t.Fatalf("%s has a playlist but no .ts segments — check they are committed", hlsTestDir)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(hlsTestDir)))
	defer srv.Close()

	p, err := NewMediaPlayerURL(srv.URL + "/stream.m3u8")
	if err != nil {
		t.Fatalf("NewMediaPlayerURL on an HLS playlist: %v", err)
	}
	defer p.Close()

	if w, h := p.VideoSize(); w != 320 || h != 240 {
		t.Errorf("VideoSize = %dx%d, want 320x240", w, h)
	}
	if d := p.Duration(); d < 3*time.Second {
		t.Errorf("Duration = %v, want >= 3s across the segments", d)
	}
}

// TestHLSViaReaderFails documents why the URL form is not merely a
// convenience: handed the same playlist as bytes, the demuxer has no way to
// fetch the segments, so there is nothing to decode. This is precisely the gap
// the URL entry point closes.
func TestHLSViaReaderFails(t *testing.T) {
	data, err := os.ReadFile(hlsTestDir + "/stream.m3u8")
	if err != nil {
		t.Skipf("hls fixture unavailable: %v", err)
	}

	// Wrapping matters. Handed an *os.File, NewMediaPlayer takes the fast path
	// and gives FFmpeg the filename — at which point HLS does work, because the
	// segments resolve next to the playlist. Hiding both io.Seeker and the
	// concrete type forces the genuine byte-stream path, which is the one with
	// no base to resolve against.
	p, err := NewMediaPlayer(struct{ io.Reader }{bytes.NewReader(data)})
	if err != nil {
		return // Refusing to open is the expected outcome.
	}
	defer p.Close()
	if w, h := p.VideoSize(); w > 0 && h > 0 {
		t.Errorf("the reader path reported %dx%d from a bare playlist — the "+
			"segments cannot have been fetched, so this should not resolve", w, h)
	}
}

func TestNewVideoPlayerURLAndAudioPlayerURL(t *testing.T) {
	v, err := NewVideoPlayerURL(urlTestMedia)
	if err != nil {
		t.Fatalf("NewVideoPlayerURL: %v", err)
	}
	defer v.Close()
	if v.Duration() <= 0 {
		t.Error("video player reported no duration")
	}

	a, err := NewAudioPlayerURL(urlTestMedia)
	if err != nil {
		t.Fatalf("NewAudioPlayerURL: %v", err)
	}
	defer a.Close()
	if a.Duration() <= 0 {
		t.Error("audio player reported no duration")
	}
}

func TestNewMediaPlayerURLRejectsMissing(t *testing.T) {
	if _, err := NewMediaPlayerURL("/nonexistent/nope.mp4"); err == nil {
		t.Error("expected an error for a missing path")
	}
	if _, err := NewMediaPlayerURL("http://127.0.0.1:1/none.mp4"); err == nil {
		t.Error("expected an error for an unreachable url")
	}
}

// TestURLConstructorsHonourOptions checks the shared option handling did not
// regress when the constructors were split into reader and URL forms.
func TestURLConstructorsHonourOptions(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia, WithVolume(0.25), WithLoop())
	if err != nil {
		t.Fatalf("NewMediaPlayerURL: %v", err)
	}
	defer p.Close()

	if got := p.Volume(); got < 0.24 || got > 0.26 {
		t.Errorf("Volume = %v, want 0.25", got)
	}
	if !p.loop {
		t.Error("WithLoop not applied")
	}
}

// TestSeekBarDisabledForLiveSource covers the transport half of the issue: a
// source with no duration cannot be sought, so the bar must not invite it.
func TestSeekBarDisabledForLiveSource(t *testing.T) {
	tc := NewTransportControls()
	tc.SetSeekable(false)
	if !tc.seekBar.Disabled() {
		t.Error("seek bar should be disabled for a live source")
	}
	if tc.timeLabel.Text != "live" {
		t.Errorf("time label = %q, want %q", tc.timeLabel.Text, "live")
	}

	// A live source still reports elapsed time; it just has no total.
	tc.SetTime(42*time.Second, 0)
	if got, want := tc.timeLabel.Text, "00:42"; got != want {
		t.Errorf("time label = %q, want %q (elapsed only)", got, want)
	}
	if tc.seekBar.Value != 0 {
		t.Error("a live source must not move the seek bar")
	}

	tc.SetSeekable(true)
	if tc.seekBar.Disabled() {
		t.Error("seek bar should re-enable for a seekable source")
	}
}
