package audio

import (
	"encoding/binary"
	"testing"
	"time"
)

// tone is d of 16-bit stereo at DeviceSampleRate, every sample set to v, so
// what oto holds can be told apart from silence.
func tone(d time.Duration, v int16) []byte {
	b := make([]byte, audioBytes(DeviceSampleRate, 2, d))
	for i := 0; i+1 < len(b); i += 2 {
		binary.LittleEndian.PutUint16(b[i:], uint16(v))
	}
	return b
}

// TestSeekDropsWhatThePlayerPulled pins #855: Rebase cleared the ring buffer
// but not oto's player, which holds up to playerChunk of audio it has already
// pulled, so a seek played 162 ms of the old position first. Once the new
// position's samples arrive, the player holds those. It needs a real audio
// device: on the detached one the player never pulls.
func TestSeekDropsWhatThePlayerPulled(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	defer o.Stop()
	o.WriteSamples(tone(time.Second, 1000)) // the old position
	o.Start()
	time.Sleep(300 * time.Millisecond) // let the player pull a chunk or more
	if o.player.BufferedSize() == 0 {
		t.Skip("the player had pulled nothing yet; this run measures nothing")
	}

	o.Rebase(5 * time.Second)
	o.WriteSamples(tone(time.Second, 2000)) // the new position, as the decoder writes it

	pulled := o.player.BufferedSize()
	beforeRead := o.buffer.TotalRead()
	t.Logf("player holds %d bytes (%v) right after the new samples", pulled, bytesDuration(pulled))
	// What the player holds now came from the ring after the seek: the ring's
	// read count grew by at least that much since Rebase reset its base.
	if fromNew := beforeRead - o.readBase; fromNew < int64(pulled) {
		t.Errorf("the player holds %d bytes, but only %d were read from the new position: %d bytes of the old one are still queued",
			pulled, fromNew, int64(pulled)-fromNew)
	}
	if got := o.PlaybackPosition(); got < 5*time.Second || got > 5*time.Second+50*time.Millisecond {
		t.Errorf("position %v just after the seek; want 5s", got)
	}
}

// TestSeekWhilePausedStaysPaused: emptying the player after a seek must not
// start a paused one.
func TestSeekWhilePausedStaysPaused(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	defer o.Stop()
	o.WriteSamples(tone(time.Second, 1000))
	o.Start()
	time.Sleep(100 * time.Millisecond)
	o.Pause()
	o.Rebase(2 * time.Second)
	o.WriteSamples(tone(time.Second, 2000))
	if o.player.IsPlaying() {
		t.Error("a seek while paused started the player when samples arrived")
	}
	o.Resume()
	if !o.player.IsPlaying() {
		t.Error("Resume did not start the player")
	}
}

func bytesDuration(n int) time.Duration {
	return time.Duration(n) * time.Second / time.Duration(4*DeviceSampleRate)
}
