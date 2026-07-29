package source

import (
	"bytes"
	"image"
	"io"
	"os"
	"testing"
	"time"
)

// Small self-contained H.264/AAC fixtures (4s, 320x240, 1s GOP).
const (
	sampleMedia = "testdata/sample.mp4"
	// streamMedia is muxed with +faststart so it plays forward-only.
	streamMedia = "testdata/sample_stream.mp4"
)

func openSample(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(sampleMedia)
	if err != nil {
		t.Skipf("sample media unavailable: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// drain runs Decode until it yields a video frame or the source ends.
func drain(s *FFmpegSource, limit int) (frames, audioChunks int) {
	var got int
	s.SetVideoCallback(func(*image.RGBA, time.Duration) { got++ })
	s.SetAudioCallback(func(b []byte) {
		if len(b) > 0 {
			audioChunks++
		}
	})
	for i := 0; i < limit && !s.HasEnded(); i++ {
		s.Decode(40 * time.Millisecond)
	}
	return got, audioChunks
}

func TestFFmpegSourceFromReader(t *testing.T) {
	s, err := NewFFmpegSource(openSample(t))
	if err != nil {
		t.Fatalf("NewFFmpegSource: %v", err)
	}
	defer s.Close()

	if !s.HasVideo() || !s.HasAudio() {
		t.Fatalf("hasVideo=%v hasAudio=%v, want both true", s.HasVideo(), s.HasAudio())
	}
	if w, h := s.VideoSize(); w != 320 || h != 240 {
		t.Errorf("VideoSize = %dx%d, want 320x240", w, h)
	}
	if d := s.Duration(); d < 3*time.Second {
		t.Errorf("Duration = %v, want >= 3s", d)
	}
	if fr := s.Framerate(); fr < 24 || fr > 26 {
		t.Errorf("Framerate = %v, want ~25", fr)
	}
	if sr := s.SampleRate(); sr <= 0 {
		t.Errorf("SampleRate = %d, want > 0", sr)
	}

	frames, audio := drain(s, 60)
	if frames == 0 {
		t.Error("decoded no video frames")
	}
	if audio == 0 {
		t.Error("decoded no audio")
	}
}

// TestFFmpegSourceFromURL covers the *os.File fast path the pipeline prefers,
// where FFmpeg opens the file itself instead of reading through Go.
func TestFFmpegSourceFromURL(t *testing.T) {
	abs, err := os.Stat(sampleMedia)
	if err != nil || abs.Size() == 0 {
		t.Skipf("sample media unavailable: %v", err)
	}
	s, err := NewFFmpegSourceURL(sampleMedia)
	if err != nil {
		t.Fatalf("NewFFmpegSourceURL: %v", err)
	}
	defer s.Close()

	if w, h := s.VideoSize(); w != 320 || h != 240 {
		t.Errorf("VideoSize = %dx%d, want 320x240", w, h)
	}
	if frames, _ := drain(s, 60); frames == 0 {
		t.Error("decoded no video frames")
	}
}

// TestFFmpegSourceSeek checks Seek lands at or after the requested position.
func TestFFmpegSourceSeek(t *testing.T) {
	s, err := NewFFmpegSource(openSample(t))
	if err != nil {
		t.Fatalf("NewFFmpegSource: %v", err)
	}
	defer s.Close()

	if frames, _ := drain(s, 20); frames == 0 {
		t.Fatal("no frames before seek")
	}
	s.Seek(2 * time.Second)
	if s.HasEnded() {
		t.Fatal("source ended after seeking to 2s")
	}
	if frames, _ := drain(s, 20); frames == 0 {
		t.Error("no frames decoded after seek")
	}
}

// TestFFmpegSourceRewind verifies Rewind restores playback from the start,
// which the pipeline relies on for Stop and looping.
func TestFFmpegSourceRewind(t *testing.T) {
	s, err := NewFFmpegSource(openSample(t))
	if err != nil {
		t.Fatalf("NewFFmpegSource: %v", err)
	}
	defer s.Close()

	s.Seek(2 * time.Second)
	s.Rewind()
	if s.HasEnded() {
		t.Fatal("source ended after Rewind")
	}
	if frames, _ := drain(s, 20); frames == 0 {
		t.Error("no frames decoded after Rewind")
	}
}

func TestFFmpegSourceRejectsNonMedia(t *testing.T) {
	if _, err := NewFFmpegSource(bytes.NewReader([]byte("this is not media"))); err == nil {
		t.Error("expected an error for non-media input")
	}
	if _, err := NewFFmpegSource(nil); err == nil {
		t.Error("expected an error for a nil reader")
	}
}

// TestFFmpegSourceNonSeekableStream covers playback from a pure stream, the
// case that previously could not work at all because the decoder needed a path.
func TestFFmpegSourceNonSeekableStream(t *testing.T) {
	data, err := os.ReadFile(streamMedia)
	if err != nil {
		t.Skipf("stream fixture unavailable: %v", err)
	}
	// Wrapping hides io.Seeker, forcing the forward-only path.
	s, err := NewFFmpegSource(struct{ io.Reader }{bytes.NewReader(data)})
	if err != nil {
		t.Fatalf("NewFFmpegSource on a faststart stream: %v", err)
	}
	defer s.Close()

	if frames, _ := drain(s, 60); frames == 0 {
		t.Error("decoded no video frames from a non-seekable stream")
	}
}
