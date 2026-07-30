package mmedia

import (
	"fmt"
	"time"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// volumeBarWidth is the on-screen width of the volume slider.
const volumeBarWidth = 120

// liveLabel marks a source with no duration, where a position fraction is
// meaningless. SetTime keeps it in front of the elapsed time.
const liveLabel = "live"

// TransportControls provides play/pause, seek, mute, volume, and time display
// controls.
type TransportControls struct {
	widget.BaseWidget

	OnPlay   func()
	OnPause  func()
	OnStop   func()
	OnSeek   func(time.Duration)
	OnVolume func(float64)

	playBtn   *widget.Button
	stopBtn   *widget.Button
	muteBtn   *widget.Button
	seekBar   *widget.Slider
	volumeBar *widget.Slider
	timeLabel *widget.Label

	duration time.Duration
	playing  bool

	// muted tracks the toggle state; preMute remembers the level to restore,
	// so unmuting returns to the previous volume rather than to full.
	muted   bool
	preMute float64
}

// NewTransportControls creates a new transport controls widget.
func NewTransportControls() *TransportControls {
	tc := &TransportControls{}

	tc.playBtn = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		if tc.playing {
			if tc.OnPause != nil {
				tc.OnPause()
			}
		} else {
			if tc.OnPlay != nil {
				tc.OnPlay()
			}
		}
	})

	tc.stopBtn = widget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		if tc.OnStop != nil {
			tc.OnStop()
		}
	})

	tc.seekBar = widget.NewSlider(0, 1)
	tc.seekBar.Step = 0.001
	// Dragging only previews. OnChanged fires on every drag increment, and with
	// Step = 0.001 one gesture across a long video produces hundreds of them —
	// each a full seek, which decodes and discards frames up to the target while
	// holding the pipeline's decode lock. The label moves here; the seek waits.
	tc.seekBar.OnChanged = func(v float64) {
		tc.previewTime(v)
	}
	// OnChangeEnded fires once, on DragEnd or Tapped, so a whole gesture costs
	// exactly one seek.
	tc.seekBar.OnChangeEnded = func(v float64) {
		if tc.OnSeek != nil && tc.duration > 0 {
			tc.OnSeek(time.Duration(v * float64(tc.duration)))
		}
	}

	tc.muteBtn = widget.NewButtonWithIcon("", theme.VolumeUpIcon(), func() {
		if tc.muted {
			restore := tc.preMute
			if restore <= 0 {
				restore = 1.0
			}
			tc.applyVolume(restore)
			return
		}
		tc.preMute = tc.volumeBar.Value
		tc.applyVolume(0)
	})

	tc.preMute = 1.0
	tc.volumeBar = widget.NewSlider(0, 1)
	tc.volumeBar.Value = 1.0
	tc.volumeBar.Step = 0.01
	tc.volumeBar.OnChanged = func(v float64) {
		// Dragging to zero is a mute too, so keep the button in step.
		tc.setMuted(v <= 0)
		if v > 0 {
			tc.preMute = v
		}
		if tc.OnVolume != nil {
			tc.OnVolume(v)
		}
	}

	tc.timeLabel = widget.NewLabel("00:00 / 00:00")

	tc.ExtendBaseWidget(tc)
	return tc
}

// SetPlaying updates the play/pause button state.
func (tc *TransportControls) SetPlaying(playing bool) {
	tc.playing = playing
	if playing {
		tc.playBtn.SetIcon(theme.MediaPauseIcon())
	} else {
		tc.playBtn.SetIcon(theme.MediaPlayIcon())
	}
}

// SetTime updates the time display and seek bar position. It runs from the
// pipeline's ticker, several times a second.
//
// While the user is scrubbing it updates neither: moving the thumb would pull it
// out from under the pointer, and rewriting the label would erase the scrub
// preview. The duration is still recorded, since the seek on release converts
// through it.
func (tc *TransportControls) SetTime(current, total time.Duration) {
	if total > 0 {
		tc.duration = total
	}
	if tc.seekBar.Dragging() {
		return
	}

	if total <= 0 {
		// Live source: elapsed time is meaningful, a position fraction is not.
		// The marker SetSeekable put here is kept alongside it — this runs every
		// tick, so writing the elapsed time alone would erase it immediately.
		tc.timeLabel.SetText(liveLabel + " · " + formatDuration(current))
		return
	}
	tc.timeLabel.SetText(fmt.Sprintf("%s / %s", formatDuration(current), formatDuration(total)))
	// Silently: this is the player advancing its own position bar, not the user
	// seeking, and SetValue would be indistinguishable from the latter.
	tc.seekBar.SetValueSilently(float64(current) / float64(total))
}

// previewTime shows where a scrub would land, without seeking there.
func (tc *TransportControls) previewTime(v float64) {
	if tc.duration <= 0 {
		return
	}
	at := time.Duration(v * float64(tc.duration))
	tc.timeLabel.SetText(fmt.Sprintf("%s / %s", formatDuration(at), formatDuration(tc.duration)))
}

// SetVolume updates the volume slider position without firing OnVolume.
func (tc *TransportControls) SetVolume(v float64) {
	tc.volumeBar.SetValueSilently(v)
	tc.setMuted(v <= 0)
	if v > 0 {
		tc.preMute = v
	}
}

// applyVolume moves the slider and notifies, used by the mute toggle. The move
// itself is silent and OnVolume is called directly: the notification belongs to
// the button press, not to the slider having been repositioned.
func (tc *TransportControls) applyVolume(v float64) {
	tc.volumeBar.SetValueSilently(v)
	tc.setMuted(v <= 0)
	if tc.OnVolume != nil {
		tc.OnVolume(v)
	}
}

// setMuted updates the toggle state and its icon.
func (tc *TransportControls) setMuted(muted bool) {
	tc.muted = muted
	if muted {
		tc.muteBtn.SetIcon(theme.VolumeMuteIcon())
	} else {
		tc.muteBtn.SetIcon(theme.VolumeUpIcon())
	}
}

// SetSeekable enables or disables the seek bar. A live source — a camera or an
// endless stream — reports no duration and cannot be sought, so the control is
// disabled rather than left inviting an operation that cannot work.
func (tc *TransportControls) SetSeekable(seekable bool) {
	if seekable {
		tc.seekBar.Enable()
		return
	}
	tc.seekBar.Disable()
	tc.timeLabel.SetText(liveLabel)
}

// Reset returns the seek bar and time display to the start. Stopping halts the
// pipeline's time updates, so without this the bar keeps the position it had
// when playback stopped.
func (tc *TransportControls) Reset() {
	tc.seekBar.SetValueSilently(0)
	tc.timeLabel.SetText(fmt.Sprintf("%s / %s", formatDuration(0), formatDuration(tc.duration)))
}

// CreateRenderer returns the Bellui renderer for this widget.
func (tc *TransportControls) CreateRenderer() core.WidgetRenderer {
	// An HBox sizes children to their MinSize, which leaves a slider barely
	// wide enough to click. GridWrap pins it to a usable width instead —
	// calling Resize here would simply be overridden by the layout.
	volumeSized := container.NewGridWrap(
		geo.NewSize(volumeBarWidth, tc.volumeBar.MinSize().Height),
		tc.volumeBar,
	)
	volumeContainer := container.NewHBox(tc.muteBtn, volumeSized)

	buttons := container.NewHBox(tc.playBtn, tc.stopBtn)
	bar := container.NewBorder(
		nil, nil,
		container.NewHBox(buttons, tc.timeLabel),
		volumeContainer,
		tc.seekBar,
	)

	return widget.NewSimpleRenderer(bar)
}

// MinSize returns the minimum size of the controls.
func (tc *TransportControls) MinSize() geo.Size {
	return geo.NewSize(400, 48)
}

func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if minutes >= 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

var _ core.Widget = (*TransportControls)(nil)
