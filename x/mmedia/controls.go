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

	// OnScrub is called with the position under the pointer as it moves across
	// the seek bar, and OnScrubEnd when it leaves. Neither is a seek: they exist
	// so a player can show what is at that position before the user commits to
	// going there.
	//
	// They fire on hover rather than on drag, which is what makes a preview
	// worth having — the point is to look before committing. Dragging reports
	// through OnChanged/OnChangeEnded as before.
	OnScrub    func(time.Duration)
	OnScrubEnd func()

	playBtn   *widget.Button
	stopBtn   *widget.Button
	muteBtn   *widget.Button
	seekBar   *widget.Slider
	volumeBar *widget.Slider
	timeLabel *widget.Label

	duration time.Duration
	playing  bool

	// preview is the object floated above the seek bar while scrubbing, and
	// previewSize the space to give it. previewAt is the hovered fraction, which
	// is where along the bar it sits. Installed by SetScrubPreview; nil until
	// then, and the layout below simply has nothing to place.
	preview      core.CanvasObject
	previewSize  geo.Size
	previewAt    float64
	previewShown bool

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
	tc.seekBar.OnHover = tc.scrubTo
	tc.seekBar.OnHoverEnd = tc.endScrub

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

// scrubTo reports the position under the pointer and moves the preview to it.
//
// Hovering a disabled seek bar reports nothing: a live source has no timeline,
// so there is no position under the pointer to preview, and the bar is disabled
// for exactly that reason.
func (tc *TransportControls) scrubTo(v float64) {
	if tc.duration <= 0 || tc.seekBar.Disabled() {
		return
	}
	tc.previewAt = clampUnit(v)
	if tc.preview != nil && !tc.previewShown {
		tc.previewShown = true
		tc.preview.Show()
	}
	if tc.OnScrub != nil {
		tc.OnScrub(time.Duration(tc.previewAt * float64(tc.duration)))
	}
	if tc.preview != nil {
		tc.Refresh()
	}
}

// endScrub takes the preview down again. It fires even when the pointer leaves
// mid-drag, which is why hiding is driven from here rather than from a drag end.
func (tc *TransportControls) endScrub() {
	if tc.preview != nil && tc.previewShown {
		tc.previewShown = false
		tc.preview.Hide()
		tc.Refresh()
	}
	if tc.OnScrubEnd != nil {
		tc.OnScrubEnd()
	}
}

// SetScrubPreview installs the object to float above the seek bar while the
// pointer is over it, sized to size.
//
// The controls own the placement rather than the player, because the seek bar's
// geometry is theirs: the preview has to sit over the position under the
// pointer, and only the layout that placed the bar knows where that is.
//
// Pass a nil object to remove it.
func (tc *TransportControls) SetScrubPreview(obj core.CanvasObject, size geo.Size) {
	tc.preview = obj
	tc.previewSize = size
	switch {
	case obj == nil:
		tc.previewShown = false
	case !tc.previewShown:
		obj.Hide()
	}
	// Deliberately not hiding an already-shown preview: this is also how a
	// resize arrives, when the first decoded frame replaces the placeholder's
	// guessed aspect ratio, and that happens mid-hover.
	tc.Refresh()
}

func clampUnit(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
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

// scrubPreviewGap is the space between the top of the controls and the bottom
// of the preview.
const scrubPreviewGap = 6

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
	if tc.preview == nil {
		return widget.NewSimpleRenderer(bar)
	}

	// The bar keeps its own Border layout untouched; the preview is placed by a
	// wrapper around it, so nothing about the existing controls moves.
	return widget.NewSimpleRenderer(
		container.New(&scrubPreviewLayout{controls: tc}, bar, tc.preview),
	)
}

// scrubPreviewLayout gives the bar the whole widget and floats the preview
// above it, centred on the position under the pointer.
//
// The preview is placed at a *negative* Y, so it is drawn outside the controls
// and over the video. A plain bellui.Container does not clip its children, so
// that renders; what it does require is that the controls are painted after the
// video, which is why the player orders them that way. It also has to be a
// non-interactive object — an image, not a widget that wants the pointer — or it
// would take the hover that is driving it and the preview would flicker itself
// out of existence.
type scrubPreviewLayout struct {
	controls *TransportControls
}

func (l *scrubPreviewLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	if len(objects) < 2 {
		return
	}
	bar, preview := objects[0], objects[1]
	bar.Resize(size)
	bar.Move(geo.NewPos(0, 0))

	tc := l.controls
	if !tc.previewShown {
		return
	}

	pw, ph := tc.previewSize.Width, tc.previewSize.Height
	// The seek bar sits inside the bar's own Border layout, and the bar is at
	// the origin of this container, so the slider's position is already in these
	// coordinates.
	track := tc.seekBar.Size().Width
	x := tc.seekBar.Position().X + float32(tc.previewAt)*track - pw/2
	// Kept inside the widget: a preview half off the window is worse than one
	// no longer exactly over the thumb.
	if maxX := size.Width - pw; x > maxX {
		x = maxX
	}
	if x < 0 {
		x = 0
	}

	preview.Resize(geo.NewSize(pw, ph))
	preview.Move(geo.NewPos(x, -ph-scrubPreviewGap))
}

// MinSize reports the bar's own minimum. The preview deliberately contributes
// nothing: it is drawn above the controls, not inside them, so reserving space
// for it would leave a permanent gap for something usually not there.
func (l *scrubPreviewLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) == 0 {
		return geo.Size{}
	}
	return objects[0].MinSize()
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
