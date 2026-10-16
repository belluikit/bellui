package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var (
	_ event.Tappable          = (*Switch)(nil)
	_ event.Hoverable         = (*Switch)(nil)
	_ event.Focusable         = (*Switch)(nil)
	_ event.Activatable       = (*Switch)(nil)
	_ core.Accessible         = (*Switch)(nil)
	_ core.AccessibleStateful = (*Switch)(nil)
	_ core.AccessibleActioner = (*Switch)(nil)
)

// Switch is an MD3-style toggle switch with a sliding thumb.
type Switch struct {
	DisableableWidget
	On        bool
	OnChanged func(bool) `json:"-"`

	hovered bool
	focused bool
}

// NewSwitch creates a new switch widget.
func NewSwitch(changed func(bool)) *Switch {
	s := &Switch{OnChanged: changed}
	s.ExtendBaseWidget(s)
	return s
}

// AccessibleRole returns [core.RoleSwitch].
func (s *Switch) AccessibleRole() core.AccessibleRole { return core.RoleSwitch }

// AccessibleLabel returns an empty string; switches have no intrinsic text label.
func (s *Switch) AccessibleLabel() string { return "" }

// AccessibleState returns the checked and disabled state.
func (s *Switch) AccessibleState() core.AccessibleState {
	var st core.AccessibleState
	if s.On {
		st |= core.StateChecked
	}
	if s.Disabled() {
		st |= core.StateDisabled
	}
	return st
}

// AccessibleActions returns the "toggle" action.
func (s *Switch) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionToggle, Description: "Toggle the switch"}}
}

// PerformAccessibleAction toggles the switch.
func (s *Switch) PerformAccessibleAction(name string) {
	if name == core.ActionToggle && !s.Disabled() {
		s.Tapped(nil)
	}
}

// Tapped toggles the switch state.
func (s *Switch) Tapped(*event.PointEvent) {
	if s.Disabled() {
		return
	}
	s.On = !s.On
	if s.OnChanged != nil {
		s.OnChanged(s.On)
	}

	// Animate thumb slide.
	r := cache.Renderer(s.super())
	if r == nil {
		s.Refresh()
		return
	}
	sr, ok := r.(*switchRenderer)
	if !ok || !bellui.CurrentApp().Settings().ShowAnimations() {
		s.Refresh()
		return
	}
	sr.animateThumb()
}

func (s *Switch) HoverIn(*event.HoverEvent)    { s.hovered = true; s.Refresh() }
func (s *Switch) HoverMoved(*event.HoverEvent) {}
func (s *Switch) HoverOut()                    { s.hovered = false; s.Refresh() }

// FocusGained is called when the Switch is given keyboard focus.
func (s *Switch) FocusGained() { s.focused = true; s.Refresh() }

// FocusLost is called when the Switch loses keyboard focus.
func (s *Switch) FocusLost() { s.focused = false; s.Refresh() }

// TypedRune is unused; the Switch toggles on Space via TypedKey.
func (s *Switch) TypedRune(rune) {}

// TypedKey toggles the switch on Space, giving it keyboard operability (an a11y
// gap MD3 switches previously had — they toggled on tap only).
func (s *Switch) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeySpace {
		s.Tapped(nil)
	}
}

// ActivatedByKey reports that Space activates the switch, so the driver fires
// the central keyboard-activation signal in parity with a pointer tap
// (pre-rebrand tracker #834, pre-rebrand tracker #700).
func (s *Switch) ActivatedByKey(ev *event.KeyEvent) bool {
	return ev.Name == event.KeySpace
}

func (s *Switch) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *Switch) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	track := canvas.NewRectangle(color.Transparent)
	focus := &canvas.Circle{FillColor: color.Transparent}
	thumb := &canvas.Circle{FillColor: color.Transparent}

	r := &switchRenderer{
		// focus indicator sits behind the thumb so its halo reads as a ring.
		BaseRenderer:   internalWidget.NewBaseRenderer([]core.CanvasObject{track, focus, thumb}),
		track:          track,
		focusIndicator: focus,
		thumb:          thumb,
		sw:             s,
	}
	r.Refresh()
	return r
}

// --- Renderer ---

type switchRenderer struct {
	internalWidget.BaseRenderer
	track          *canvas.Rectangle
	focusIndicator *canvas.Circle
	thumb          *canvas.Circle
	sw             *Switch
}

const (
	switchTrackHeightRatio = 32.0 / 52.0 // MD3 track height as a fraction of track width
	switchThumbRatio       = 0.75        // thumb diameter as a fraction of track height
	switchThumbPadRatio    = 0.125       // thumb inset as a fraction of track height
)

func (r *switchRenderer) MinSize() geo.Size {
	th := r.sw.Theme()
	trackW := th.Size(theme.SizeNameSwitchTrackWidth)
	trackH := trackW * switchTrackHeightRatio
	return geo.NewSize(trackW, trackH)
}

func (r *switchRenderer) Layout(size geo.Size) {
	th := r.sw.Theme()
	trackW := th.Size(theme.SizeNameSwitchTrackWidth)
	trackH := trackW * switchTrackHeightRatio
	thumbSize := trackH * switchThumbRatio
	thumbPad := trackH * switchThumbPadRatio

	r.track.Resize(geo.NewSize(trackW, trackH))
	r.track.Move(geo.NewPos(0, (size.Height-trackH)/2))

	thumbY := (size.Height - thumbSize) / 2
	var thumbX float32
	if r.sw.On {
		thumbX = trackW - thumbSize - thumbPad
	} else {
		thumbX = thumbPad
	}
	r.thumb.Move(geo.NewPos(thumbX, thumbY))
	r.thumb.Resize(geo.NewSize(thumbSize, thumbSize))

	// Focus halo: a ring ~1.6× the thumb, centered on the thumb.
	haloSize := thumbSize * 1.6
	r.focusIndicator.Resize(geo.NewSize(haloSize, haloSize))
	r.focusIndicator.Move(geo.NewPos(thumbX-(haloSize-thumbSize)/2, thumbY-(haloSize-thumbSize)/2))
}

func (r *switchRenderer) Refresh() {
	th := r.sw.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	trackH := th.Size(theme.SizeNameSwitchTrackWidth) * 32.0 / 52.0

	r.track.CornerRadius = trackH / 2

	if r.sw.On {
		r.track.FillColor = th.Color(theme.ColorNamePrimary, v)
		r.thumb.FillColor = th.Color(theme.ColorNameForegroundOnPrimary, v)
		if r.sw.Disabled() {
			r.track.FillColor = th.Color(theme.ColorNameDisabled, v)
			r.thumb.FillColor = th.Color(theme.ColorNameOnSurface, v)
		}
	} else {
		r.track.FillColor = th.Color(theme.ColorNameSurfaceContainerHighest, v)
		r.track.StrokeWidth = 2
		r.track.StrokeColor = th.Color(theme.ColorNameOutline, v)
		r.thumb.FillColor = th.Color(theme.ColorNameOutline, v)
		if r.sw.Disabled() {
			r.track.FillColor = th.Color(theme.ColorNameDisabledButton, v)
			r.track.StrokeColor = th.Color(theme.ColorNameDisabled, v)
			r.thumb.FillColor = th.Color(theme.ColorNameDisabled, v)
		}
	}

	if r.sw.On {
		r.track.StrokeWidth = 0
		r.track.StrokeColor = color.Transparent
	}

	if r.sw.focused && !r.sw.Disabled() {
		r.focusIndicator.FillColor = th.Color(theme.ColorNameFocus, v)
	} else {
		r.focusIndicator.FillColor = color.Transparent
	}

	r.track.Refresh()
	r.focusIndicator.Refresh()
	r.thumb.Refresh()
	r.Layout(r.sw.Size())
	canvas.Refresh(r.sw.super())
}

func (r *switchRenderer) animateThumb() {
	// Update colors immediately.
	r.Refresh()

	th := r.sw.Theme()
	trackW := th.Size(theme.SizeNameSwitchTrackWidth)
	trackH := trackW * 32.0 / 52.0
	thumbSize := trackH * 0.75
	thumbPad := trackH * 0.125

	size := r.sw.Size()
	thumbY := (size.Height - thumbSize) / 2
	var startX, endX float32
	if r.sw.On {
		startX = thumbPad
		endX = trackW - thumbSize - thumbPad
	} else {
		startX = trackW - thumbSize - thumbPad
		endX = thumbPad
	}

	anim := bellui.NewAnimation(canvas.DurationShort4, func(done float32) {
		x := startX + (endX-startX)*done
		r.thumb.Move(geo.NewPos(x, thumbY))
		r.thumb.Refresh()
	})
	anim.Curve = bellui.AnimationEaseStandard
	anim.Start()
}
