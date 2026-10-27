package mmedia

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/mmedia/internal/audio"
)

// forwardOnly hides a reader's Seek, as a file picked on Android does.
type forwardOnly struct{ r io.Reader }

func (f forwardOnly) Read(p []byte) (int, error) { return f.r.Read(p) }

// TestAudioPlayerPlaysCoverArtMP3 pins #850 through the API: an MP3 with
// cover art, which FFmpeg presents as a one-frame video stream, reported its
// end at once from a file, and after a fraction of a second from a
// forward-only reader. It plays its 3 s now. It needs a real audio device,
// since the silent stand-in never plays out its samples, so an audio-only
// track never ends on it.
func TestAudioPlayerPlaysCoverArtMP3(t *testing.T) {
	if _, err := audio.NewOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	test.NewTempApp(t)
	const fixture = "internal/source/testdata/cover.mp3" // see mkcover.sh there
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		open func(...Option) (*AudioPlayer, error)
	}{
		{"path", func(o ...Option) (*AudioPlayer, error) { return NewAudioPlayerURL(fixture, o...) }},
		{"seekable reader", func(o ...Option) (*AudioPlayer, error) { return NewAudioPlayer(bytes.NewReader(data), o...) }},
		{"forward-only reader", func(o ...Option) (*AudioPlayer, error) {
			return NewAudioPlayer(forwardOnly{bytes.NewReader(data)}, o...)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan error, 1)
			start := time.Now()
			ap, err := tc.open(WithAutoPlay(),
				WithOnEnd(func() { done <- nil }),
				WithOnError(func(err error) { done <- err }))
			if err != nil {
				t.Fatal(err)
			}
			defer ap.Close()
			select {
			case err := <-done:
				took := time.Since(start)
				t.Logf("%s: ended after %v, err=%v", tc.name, took.Round(10*time.Millisecond), err)
				if err != nil {
					t.Fatalf("OnError(%v); want OnEnd", err)
				}
				if took < 2500*time.Millisecond {
					t.Errorf("ended after %v; want it to play the 3 s track", took)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("did not end")
			}
		})
	}
}
