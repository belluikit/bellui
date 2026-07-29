package mmedia

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/theme"
)

// TestMuteButtonTogglesVolume covers the reported bug: the speaker was a
// static widget.NewIcon, so there was no mute control at all.
func TestMuteButtonTogglesVolume(t *testing.T) {
	tc := NewTransportControls()
	var got []float64
	tc.OnVolume = func(v float64) { got = append(got, v) }

	tc.SetVolume(0.8)
	tc.muteBtn.OnTapped() // mute
	if tc.volumeBar.Value != 0 {
		t.Errorf("volume = %v after mute, want 0", tc.volumeBar.Value)
	}
	if !tc.muted {
		t.Error("muted flag not set")
	}

	tc.muteBtn.OnTapped() // unmute
	if tc.volumeBar.Value != 0.8 {
		t.Errorf("volume = %v after unmute, want the previous 0.8", tc.volumeBar.Value)
	}
	if tc.muted {
		t.Error("muted flag still set after unmute")
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 0.8 {
		t.Errorf("OnVolume calls = %v, want [0 0.8]", got)
	}
}

func TestMuteIconReflectsState(t *testing.T) {
	tc := NewTransportControls()
	if tc.muteBtn.Icon != theme.VolumeUpIcon() {
		t.Error("expected the unmuted icon initially")
	}
	tc.muteBtn.OnTapped()
	if tc.muteBtn.Icon != theme.VolumeMuteIcon() {
		t.Error("expected the muted icon after tapping")
	}
	tc.muteBtn.OnTapped()
	if tc.muteBtn.Icon != theme.VolumeUpIcon() {
		t.Error("expected the unmuted icon after tapping again")
	}
}

// TestDraggingVolumeToZeroMutes keeps the button in step with the slider.
func TestDraggingVolumeToZeroMutes(t *testing.T) {
	tc := NewTransportControls()
	tc.volumeBar.OnChanged(0)
	if !tc.muted || tc.muteBtn.Icon != theme.VolumeMuteIcon() {
		t.Error("dragging the slider to zero should present as muted")
	}
	tc.volumeBar.OnChanged(0.5)
	if tc.muted || tc.muteBtn.Icon != theme.VolumeUpIcon() {
		t.Error("dragging back up should clear the muted state")
	}
}

// TestUnmuteFromZeroFallsBackToFull guards against unmuting into silence when
// the volume was already zero at mute time.
func TestUnmuteFromZeroFallsBackToFull(t *testing.T) {
	tc := NewTransportControls()
	tc.SetVolume(0)
	tc.muteBtn.OnTapped() // "unmute" — SetVolume(0) already marked it muted
	if tc.volumeBar.Value == 0 {
		t.Error("unmuting from zero must restore an audible level")
	}
}

// TestResetClearsSeekBar covers the reported bug: Stop left the position bar
// wherever playback had reached.
func TestResetClearsSeekBar(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(30*time.Second, 60*time.Second)
	if tc.seekBar.Value == 0 {
		t.Fatal("precondition: seek bar should be mid-track")
	}

	tc.Reset()
	if tc.seekBar.Value != 0 {
		t.Errorf("seek bar = %v after Reset, want 0", tc.seekBar.Value)
	}
	if got, want := tc.timeLabel.Text, "00:00 / 01:00"; got != want {
		t.Errorf("time label = %q, want %q", got, want)
	}
}

// TestResetKeepsDuration checks the total is preserved, so the label does not
// fall back to 00:00 / 00:00 on stop.
func TestResetKeepsDuration(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(10*time.Second, 2*time.Minute)
	tc.Reset()
	if got, want := tc.timeLabel.Text, "00:00 / 02:00"; got != want {
		t.Errorf("time label = %q, want %q", got, want)
	}
}

func TestFormatDurationHours(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00"},
		{59 * time.Second, "00:59"},
		{90 * time.Second, "01:30"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"},
	} {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
