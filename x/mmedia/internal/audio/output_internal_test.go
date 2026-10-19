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
