package audio

import (
	"testing"
	"time"
)

// TestPlayerBufferIsADuration pins #504: the device's pull size was a fixed
// byte count, which at 44.1 kHz mono came to 370 ms, more than the decoder can
// stay ahead at 1080p60, and the clock ran at half speed. It is a duration now,
// whatever the format.
func TestPlayerBufferIsADuration(t *testing.T) {
	for _, f := range []struct{ rate, channels int }{{44100, 1}, {44100, 2}, {48000, 2}, {22050, 1}, {96000, 6}} {
		bytes := playerBufferBytes(f.rate, f.channels)
		got := time.Duration(bytes/(f.channels*2)) * time.Second / time.Duration(f.rate)
		if diff := got - playerChunk; diff < -time.Millisecond || diff > time.Millisecond {
			t.Errorf("%d Hz × %d: %d bytes is %v of audio; want %v", f.rate, f.channels, bytes, got, playerChunk)
		}
		if bytes%(f.channels*2) != 0 {
			t.Errorf("%d Hz × %d: %d bytes is not whole sample frames", f.rate, f.channels, bytes)
		}
	}
}

// TestAudioCanLeadTheThreadedVideoDecoder pins #504's second half: the
// decoder stops once half the ring is full, and FFmpeg's threaded video decoder
// releases frames several frames after their packets. With 0.34 s of lead
// (128 KB of 48 kHz stereo) video was always decoded late; it needs about half
// a second or more.
func TestAudioCanLeadTheThreadedVideoDecoder(t *testing.T) {
	if lead := ringDuration / 2; lead < 500*time.Millisecond {
		t.Errorf("audio may run %v ahead of playback; want at least 500 ms", lead)
	}
	if got, want := audioBytes(48000, 2, ringDuration), 48000*2*2*int(ringDuration/time.Second); got != want {
		t.Errorf("ring of 48 kHz stereo is %d bytes; want %d", got, want)
	}
}

// TestPositionCountsAtTheDeviceRate pins #506: the playback position counts
// bytes at the rate they are played at. A 48 kHz stream on a device opened at
// 44.1 kHz once had its bytes counted at the stream's rate and played at
// 0.918x. Streams are converted to the device's rate by their decoder now, so
// the Output only ever knows that one rate.
func TestPositionCountsAtTheDeviceRate(t *testing.T) {
	o := newOutput(nil, DeviceSampleRate, 1) // no device: nothing is played

	second := make([]byte, DeviceSampleRate*2) // one second of mono silence
	o.WriteSamples(second)
	drained := make([]byte, o.Buffered())
	o.buffer.Read(drained) // as the device would

	if pos := o.PlaybackPosition(); pos < 990*time.Millisecond || pos > 1010*time.Millisecond {
		t.Errorf("a second of audio played reads as %v; want 1s", pos)
	}
}
