package source

import (
	"bytes"
	"image"
	"io"
	"os"
	"testing"
	"time"
)

const coverMedia = "testdata/cover.mp3" // 3 s of tone with cover art; see mkcover.sh

// forwardOnly hides a reader's Seek, as a file picked on Android does.
type forwardOnly struct{ r io.Reader }

func (f forwardOnly) Read(p []byte) (int, error) { return f.r.Read(p) }

// decodedSeconds decodes s to its end and returns how much audio it delivered
// and how many video frames.
func decodedSeconds(t *testing.T, s *FFmpegSource) (audio time.Duration, frames int) {
	t.Helper()
	var bytes int
	s.SetVideoCallback(func(*image.RGBA, time.Duration) { frames++ })
	s.SetAudioCallback(func(b []byte) { bytes += len(b) })
	for i := 0; !s.HasEnded(); i++ {
		if i > 100000 {
			t.Fatal("the source did not end")
		}
		s.Decode(0)
	}
	perSecond := 2 * s.Channels() * s.SampleRate()
	return time.Duration(bytes) * time.Second / time.Duration(perSecond), frames
}

// openings are the two ways a player gets a file: by path, and from a reader
// that cannot seek, as a file picked on Android is.
func openings(path string, data []byte) []struct {
	name string
	open func() (*FFmpegSource, error)
} {
	return []struct {
		name string
		open func() (*FFmpegSource, error)
	}{
		{"path", func() (*FFmpegSource, error) { return NewFFmpegSourceURL(path) }},
		{"forward-only reader", func() (*FFmpegSource, error) {
			return NewFFmpegSource(forwardOnly{bytes.NewReader(data)})
		}},
	}
}

// TestCoverArtIsNotVideo pins #850 at its root, goffmpeg#21: FFmpeg presents a
// song's cover art as a one-frame video stream, and a source taking it for
// video handed out audio only between video frames, of which there is one.
// goffmpeg v0.6.1 skips the cover when it picks the video stream, so the file
// opens as audio-only with no IgnoreVideo.
func TestCoverArtIsNotVideo(t *testing.T) {
	data, err := os.ReadFile(coverMedia)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range openings(coverMedia, data) {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.open()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if s.HasVideo() {
				w, h := s.VideoSize()
				t.Fatalf("the cover art opened as a %dx%d video", w, h)
			}
			audio, frames := decodedSeconds(t, s)
			t.Logf("%s: %v of audio, %d video frames", tc.name, audio, frames)
			if frames != 0 {
				t.Errorf("%d video frames delivered from a song with cover art", frames)
			}
			if audio < 2900*time.Millisecond || audio > 3100*time.Millisecond {
				t.Errorf("decoded %v of audio; want the fixture's 3 s", audio)
			}
			if err := s.Err(); err != nil {
				t.Errorf("ended on %v", err)
			}
		})
	}
}

// TestIgnoreVideoPlaysAVideoAsAudio pins what AudioPlayer relies on (#850):
// IgnoreVideo makes a file with a real video stream audio-only, so no video
// frame paces its audio, from a path and from a forward-only reader.
func TestIgnoreVideoPlaysAVideoAsAudio(t *testing.T) {
	const media = "testdata/sample_stream.mp4" // 4 s of video and audio, index at the front
	data, err := os.ReadFile(media)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := NewFFmpegSourceURL(media)
	if err != nil {
		t.Fatal(err)
	}
	if !plain.HasVideo() {
		t.Fatal("the fixture has no video; the test would prove nothing")
	}
	plain.Close()

	for _, tc := range openings(media, data) {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.open()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.IgnoreVideo()
			if s.HasVideo() {
				t.Fatal("HasVideo after IgnoreVideo")
			}
			if w, h := s.VideoSize(); w != 0 || h != 0 {
				t.Errorf("VideoSize %dx%d after IgnoreVideo; want 0x0", w, h)
			}
			audio, frames := decodedSeconds(t, s)
			t.Logf("%s: %v of audio, %d video frames", tc.name, audio, frames)
			if frames != 0 {
				t.Errorf("%d video frames delivered after IgnoreVideo", frames)
			}
			if audio < 3900*time.Millisecond || audio > 4100*time.Millisecond {
				t.Errorf("decoded %v of audio; want the fixture's 4 s", audio)
			}
			if err := s.Err(); err != nil {
				t.Errorf("ended on %v", err)
			}
		})
	}
}
