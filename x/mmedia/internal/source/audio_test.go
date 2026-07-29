package source

import (
	"os"
	"testing"
	"time"
)

// multichannelMedia is a 2s 5.1 AAC clip.
const multichannelMedia = "testdata/sample_51.m4a"

func openMultichannel(t *testing.T) *FFmpegSource {
	t.Helper()
	if _, err := os.Stat(multichannelMedia); err != nil {
		t.Skipf("multichannel fixture unavailable: %v", err)
	}
	s, err := NewFFmpegSourceURL(multichannelMedia)
	if err != nil {
		t.Fatalf("NewFFmpegSourceURL: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func collectPCM(s *FFmpegSource, limit int) (chunks, bytes int) {
	s.SetAudioCallback(func(b []byte) {
		chunks++
		bytes += len(b)
	})
	for i := 0; i < limit && !s.HasEnded(); i++ {
		s.Decode(40 * time.Millisecond)
	}
	return chunks, bytes
}

// TestMultichannelPassthrough is the regression test for the downmix bug: the
// bridge hardcoded stereo, so a 5.1 source could not reach the output intact.
func TestMultichannelPassthrough(t *testing.T) {
	s := openMultichannel(t)

	if got := s.Channels(); got != 6 {
		t.Fatalf("Channels() = %d, want 6 (native 5.1)", got)
	}
	chunks, total := collectPCM(s, 40)
	if chunks == 0 || total == 0 {
		t.Fatalf("decoded %d chunks / %d bytes, want non-zero", chunks, total)
	}
	// S16 x 6ch = 12 bytes per frame.
	if total%12 != 0 {
		t.Errorf("total PCM = %d bytes, not a multiple of 12 (S16 x 6ch)", total)
	}
}

// TestSetAudioChannelsDownmix covers the explicit stereo request that
// WithDownmix drives.
func TestSetAudioChannelsDownmix(t *testing.T) {
	s := openMultichannel(t)

	if err := s.SetAudioChannels(2); err != nil {
		t.Fatalf("SetAudioChannels(2): %v", err)
	}
	if got := s.Channels(); got != 2 {
		t.Fatalf("Channels() = %d, want 2 after downmix", got)
	}
	chunks, total := collectPCM(s, 40)
	if chunks == 0 || total == 0 {
		t.Fatalf("decoded %d chunks / %d bytes, want non-zero", chunks, total)
	}
	if total%4 != 0 {
		t.Errorf("total PCM = %d bytes, not a multiple of 4 (S16 x 2ch)", total)
	}
}

// TestSetAudioChannelsOnSilentSource ensures a video-only source treats the
// request as a no-op instead of erroring or panicking.
func TestSetAudioChannelsOnSilentSource(t *testing.T) {
	s := &FFmpegSource{} // hasAudio == false
	if err := s.SetAudioChannels(2); err != nil {
		t.Errorf("SetAudioChannels on a source without audio = %v, want nil", err)
	}
}
