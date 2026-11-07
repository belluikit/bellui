package pipeline

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/audio"
)

// coverMP3 is 3 s of tone with cover art; see mkcover.sh beside it.
const coverMP3 = "../source/testdata/cover.mp3"

// playCoverMP3 opens coverMP3 with no picture on the real audio device, and
// skips when there is none: the silent stand-in never plays its samples out,
// so an audio-only track never ends on it.
func playCoverMP3(t *testing.T) (*Pipeline, <-chan time.Time) {
	t.Helper()
	if _, err := audio.NewOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	p, err := NewAudioPipelineURL(coverMP3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	ended := make(chan time.Time, 1)
	p.SetOnEnd(func() {
		ended <- time.Now()
	})
	p.Play()
	return p, ended
}

// TestAudioSeekAfterTheDecoderReachedTheEnd pins #882 for a track with no
// picture: the decoder reaches the end of the file a second or two before
// playback does, and waits for the audio to play out. A seek then emptied the
// audio it was waiting on, and the track was reported finished 10 ms later
// instead of playing on from the target. (A video's half of #882 is
// TestSeekAfterTheDecoderReachedTheEnd.)
func TestAudioSeekAfterTheDecoderReachedTheEnd(t *testing.T) {
	p, ended := playCoverMP3(t)
	deadline := time.Now().Add(5 * time.Second)
	for !p.sourceEnded() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !p.sourceEnded() {
		t.Fatal("the decoder never reached the end: the test measures nothing")
	}
	if now := p.CurrentTime(); now > 2700*time.Millisecond {
		t.Fatalf("playback is at %v of 3s with the decoder at the end: the test needs it well behind", now)
	}

	seek := time.Now()
	p.Seek(500 * time.Millisecond)
	select {
	case at := <-ended:
		// 2.5 s of the track are left after the target.
		if took := at.Sub(seek); took < 2300*time.Millisecond {
			t.Errorf("ended %v after a seek to 0.5s of a 3s track; want it to play the 2.5s left", took.Round(10*time.Millisecond))
		}
	case <-time.After(6 * time.Second):
		t.Fatalf("no end 6s after a seek to 0.5s of a 3s track; time %v", p.CurrentTime())
	}
}

// TestAudioEndsWhenItHasPlayed pins the end of #882's measurement: the end of
// a track with no picture was reported when the buffer emptied, with up to two
// of the device player's chunks still to play. The position then read
// 2.70 s of 3 s, and whatever the app did next, a Close or the next track, cut
// that last 0.3 s off.
func TestAudioEndsWhenItHasPlayed(t *testing.T) {
	p, ended := playCoverMP3(t)
	select {
	case <-ended:
	case <-time.After(6 * time.Second):
		t.Fatal("a 3s track did not end")
	}
	if left := p.audioOutput.Unplayed(); left != 0 {
		t.Errorf("%d bytes of the track still to play when its end was reported", left)
	}
	if pos := p.audioOutput.PlaybackPosition(); pos < p.Duration()-50*time.Millisecond {
		t.Errorf("the device had played %v of %v when the end was reported", pos, p.Duration())
	}
}

// openSampleMP4 opens sample.mp4, 4 s of video with sound, on the real audio
// device, and skips when there is none.
func openSampleMP4(t *testing.T) *Pipeline {
	t.Helper()
	if _, err := audio.NewOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	}
	p, err := NewPipelineURL("../source/testdata/sample.mp4")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if !p.HasAudio() || !p.HasVideo() {
		t.Fatal("sample.mp4 is not a video with sound: the test measures nothing")
	}
	p.SetOnFrame(func(*image.RGBA) {})
	return p
}

// TestVideoSeekIntoTheLastSecond pins the rest of #882's freeze: a seek into
// about the last second of a video with sound has all of that audio written
// before the device starts playing it, and the clock, held at the target
// until the audio is seen to move, was told where the audio was only on
// those writes. The picture froze at the target and the end never came, for
// a seek made before Play and, depending on timing, one made just after it.
func TestVideoSeekIntoTheLastSecond(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after time.Duration // after Play; negative is before it
	}{
		{"before Play", -1},
		{"300ms after Play", 300 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := openSampleMP4(t)
			ended := make(chan struct{})
			p.SetOnEnd(func() { close(ended) })
			target := p.Duration() - time.Second
			if tc.after < 0 {
				p.Seek(target)
			}
			p.Play()
			if tc.after >= 0 {
				time.Sleep(tc.after)
				p.Seek(target)
			}
			select {
			case <-ended:
			case <-time.After(4 * time.Second):
				t.Fatalf("no end 4s after a seek to the last second; time %v of %v", p.CurrentTime(), p.Duration())
			}
		})
	}
}

// TestVideoEndsWhenItsSoundHasPlayed: a video's end was reported by the
// display when it took the end marker, after its last frame, with the last
// 54 ms of sample.mp4's sound still to play, cut off by whatever the app did
// on the end.
func TestVideoEndsWhenItsSoundHasPlayed(t *testing.T) {
	p := openSampleMP4(t)
	var left int
	ended := make(chan struct{})
	p.SetOnEnd(func() {
		left = p.audioOutput.Unplayed()
		close(ended)
	})
	p.Play()
	select {
	case <-ended:
	case <-time.After(8 * time.Second):
		t.Fatal("sample.mp4 did not end")
	}
	if left != 0 {
		t.Errorf("%d bytes (%v) of its sound still to play when the end was reported", left, p.audioOutput.PlaybackPosition())
	}
}

// TestAudioEndLetsTheDeviceRest pins #1052's end of a track: the Output went
// on playing silence after the end, so on Android the device was never
// suspended, and a call then reopened its stream in call mode. The end now
// pauses it, and the next Play counts it again.
func TestAudioEndLetsTheDeviceRest(t *testing.T) {
	p, ended := playCoverMP3(t)
	if !p.audioOutput.Playing() {
		t.Fatal("precondition: the Output does not count as playing while the track plays")
	}
	select {
	case <-ended:
	case <-time.After(6 * time.Second):
		t.Fatal("a 3s track did not end")
	}
	if p.audioOutput.Playing() {
		t.Error("the Output still counts as playing after the end")
	}
	p.Play()
	if !p.audioOutput.Playing() {
		t.Error("the Output does not count as playing after Play from the end")
	}
}
