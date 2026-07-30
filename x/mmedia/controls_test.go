package mmedia

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// dragSeekBarToEnd drives a real drag gesture rather than calling the callbacks
// directly, so the Dragging() guard is exercised the way the driver drives it.
// The position is mapped through the slider's own ratio calculation, where any x
// beyond the right-hand end offset lands on 1.0 — so this needs no knowledge of
// the theme's padding.
func dragSeekBarToEnd(tc *TransportControls) {
	tc.seekBar.Resize(geo.NewSize(200, tc.seekBar.MinSize().Height))
	tc.seekBar.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(1000, 0)},
	})
}

// TestSeekBarDragIssuesOneSeekOnRelease covers the first half of the reported
// bug: OnChanged fires on every drag increment, and each one used to run a full
// seek — av_seek_frame plus decode-and-discard to the target, hundreds deep
// behind a single gesture.
func TestSeekBarDragIssuesOneSeekOnRelease(t *testing.T) {
	tc := NewTransportControls()
	var seeks []time.Duration
	tc.OnSeek = func(pos time.Duration) { seeks = append(seeks, pos) }
	tc.SetTime(0, time.Minute)

	dragSeekBarToEnd(tc)
	if len(seeks) != 0 {
		t.Fatalf("dragging issued %d seek(s), want none until release: %v", len(seeks), seeks)
	}
	if !tc.seekBar.Dragging() {
		t.Fatal("precondition: slider should report itself as dragging")
	}
	// The label previews the target while the drag is in flight.
	if got, want := tc.timeLabel.Text, "01:00 / 01:00"; got != want {
		t.Errorf("preview label = %q, want %q", got, want)
	}

	tc.seekBar.DragEnd()
	if len(seeks) != 1 {
		t.Fatalf("release issued %d seek(s), want exactly 1: %v", len(seeks), seeks)
	}
	if seeks[0] != time.Minute {
		t.Errorf("seek target = %v, want %v", seeks[0], time.Minute)
	}
}

// TestSeekBarHoldsThumbWhileDragging covers the second half: SetTime runs from
// the pipeline ticker several times a second and used to overwrite whatever the
// user had dragged to, so the thumb jumped out from under the pointer.
func TestSeekBarHoldsThumbWhileDragging(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(0, time.Minute)
	dragSeekBarToEnd(tc)

	dragged := tc.seekBar.Value
	tc.SetTime(15*time.Second, time.Minute) // a tick arriving mid-gesture

	if tc.seekBar.Value != dragged {
		t.Errorf("thumb moved to %v mid-drag, want it held at %v", tc.seekBar.Value, dragged)
	}
	if got, want := tc.timeLabel.Text, "01:00 / 01:00"; got != want {
		t.Errorf("label = %q mid-drag, want the preview %q to survive the tick", got, want)
	}
}

// TestSeekBarResumesAfterRelease checks the guard actually clears — Dragging()
// is cleared in DragEnd before OnChangeEnded runs, so ticks must land again
// immediately afterwards.
func TestSeekBarResumesAfterRelease(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(0, time.Minute)
	dragSeekBarToEnd(tc)
	tc.seekBar.DragEnd()

	tc.SetTime(30*time.Second, time.Minute)
	if got, want := tc.seekBar.Value, 0.5; got != want {
		t.Errorf("thumb = %v after release, want ticks to move it to %v", got, want)
	}
	if got, want := tc.timeLabel.Text, "00:30 / 01:00"; got != want {
		t.Errorf("label = %q after release, want %q", got, want)
	}
}

// TestSeekBarTapSeeksOnce guards the other gesture that commits: a tap fires
// OnChanged and OnChangeEnded together, and must still produce one seek.
func TestSeekBarTapSeeksOnce(t *testing.T) {
	tc := NewTransportControls()
	var seeks []time.Duration
	tc.OnSeek = func(pos time.Duration) { seeks = append(seeks, pos) }
	tc.SetTime(0, time.Minute)

	tc.seekBar.Resize(geo.NewSize(200, tc.seekBar.MinSize().Height))
	tc.seekBar.Tapped(&event.PointEvent{Position: geo.NewPos(1000, 0)})

	if len(seeks) != 1 {
		t.Fatalf("tap issued %d seek(s), want exactly 1: %v", len(seeks), seeks)
	}
	if seeks[0] != time.Minute {
		t.Errorf("seek target = %v, want %v", seeks[0], time.Minute)
	}
}

// TestLiveLabelSurvivesTimeUpdates covers the adjacent defect: SetSeekable put
// "live" in the label, and the next tick replaced it with a bare elapsed time,
// so the marker was never visible in practice.
func TestLiveLabelSurvivesTimeUpdates(t *testing.T) {
	tc := NewTransportControls()
	tc.SetSeekable(false)
	if got, want := tc.timeLabel.Text, "live"; got != want {
		t.Fatalf("label = %q after SetSeekable(false), want %q", got, want)
	}

	tc.SetTime(7*time.Second, 0)
	if got, want := tc.timeLabel.Text, "live · 00:07"; got != want {
		t.Errorf("label = %q after a tick, want %q", got, want)
	}
}

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
