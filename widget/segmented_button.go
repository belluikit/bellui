package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var _ core.Accessible = (*SegmentedButton)(nil)

// AccessibleRole returns [core.RoleTabList].
func (s *SegmentedButton) AccessibleRole() core.AccessibleRole { return core.RoleTabList }

// AccessibleLabel returns an empty string.
func (s *SegmentedButton) AccessibleLabel() string { return "" }

// SegmentedButton is an MD3 group of 2-5 mutually exclusive toggle buttons.
type SegmentedButton struct {
	BaseWidget
	Items     []string
	Selected  int       // index of selected item (-1 = none)
	OnChanged func(int) `json:"-"`

	focused bool
}

var _ event.Focusable = (*SegmentedButton)(nil)

// FocusGained is called when this SegmentedButton gains keyboard focus.
func (s *SegmentedButton) FocusGained() {
	s.focused = true
	s.Refresh()
}

// FocusLost is called when this SegmentedButton loses keyboard focus.
func (s *SegmentedButton) FocusLost() {
	s.focused = false
	s.Refresh()
}

// TypedRune is called if a text event happens while this SegmentedButton is
// focused. It is intentionally a no-op.
func (s *SegmentedButton) TypedRune(rune) {}

// TypedKey moves the selection between segments with the left/right arrows. Under
// a right-to-left layout the visual order is reversed, so the right arrow steps
// to the previous segment and the left arrow to the next, matching the mirrored
// pixels.
func (s *SegmentedButton) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyRight:
		if lang.IsRTLDirection() {
			s.cycleSelection(-1)
		} else {
			s.cycleSelection(1)
		}
	case event.KeyLeft:
		if lang.IsRTLDirection() {
			s.cycleSelection(1)
		} else {
			s.cycleSelection(-1)
		}
	}
}

// cycleSelection moves the selection by step, clamped to the item range, and
// fires OnChanged when the selection actually changes.
func (s *SegmentedButton) cycleSelection(step int) {
	if len(s.Items) == 0 {
		return
	}
	next := s.Selected + step
	if next < 0 {
		next = 0
	} else if next >= len(s.Items) {
		next = len(s.Items) - 1
	}
	if next == s.Selected {
		return
	}
	s.Selected = next
	if s.OnChanged != nil {
		s.OnChanged(next)
	}
	s.Refresh()
}

// NewSegmentedButton creates a segmented button group.
func NewSegmentedButton(items []string, changed func(int)) *SegmentedButton {
	s := &SegmentedButton{Items: items, Selected: 0, OnChanged: changed}
	s.ExtendBaseWidget(s)
	return s
}

func (s *SegmentedButton) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *SegmentedButton) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)

	var segments []*segmentWidget
	var objects []core.CanvasObject

	for i, item := range s.Items {
		idx := i
		seg := &segmentWidget{
			text:     item,
			selected: i == s.Selected,
			first:    i == 0,
			last:     i == len(s.Items)-1,
			onTap: func() {
				s.Selected = idx
				// Move keyboard focus to the parent group so that
				// click-then-arrow navigation works without first Tabbing to
				// it (#174). Mirrors widget.Select.Tapped.
				if !s.focused {
					focusIfNotMobile(s.super())
				}
				if s.OnChanged != nil {
					s.OnChanged(idx)
				}
				s.Refresh()
			},
		}
		seg.ExtendBaseWidget(seg)
		segments = append(segments, seg)
		objects = append(objects, seg)
	}

	r := &segButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		segments:     segments,
		btn:          s,
	}
	r.Refresh()
	return r
}

type segmentWidget struct {
	BaseWidget
	text     string
	selected bool
	focused  bool
	first    bool
	last     bool
	onTap    func()
}

func (w *segmentWidget) Tapped(*event.PointEvent) {
	if w.onTap != nil {
		w.onTap()
	}
}

// AccessibleRole returns [core.RoleTab]; the parent SegmentedButton is the tab list.
func (w *segmentWidget) AccessibleRole() core.AccessibleRole { return core.RoleTab }

// AccessibleLabel returns the segment's text.
func (w *segmentWidget) AccessibleLabel() string { return w.text }

// AccessibleState reports the selected and focused states.
func (w *segmentWidget) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	if w.selected {
		state |= core.StateSelected
	}
	if w.focused {
		state |= core.StateFocused
	}
	return state
}

// AccessibleActions returns the "select" action.
func (w *segmentWidget) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this segment"}}
}

// PerformAccessibleAction selects the segment like a tap would.
func (w *segmentWidget) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		w.Tapped(nil)
	}
}

func (w *segmentWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	th := w.Theme()
	bg := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(w.text, color.White)
	label.TextSize = th.Size(theme.SizeNameLabelLarge)
	label.Alignment = bellui.TextAlignCenter

	r := &segmentRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{bg, label}),
		bg:           bg,
		label:        label,
		seg:          w,
	}
	r.Refresh()
	return r
}

type segmentRenderer struct {
	internalWidget.BaseRenderer
	bg    *canvas.Rectangle
	label *canvas.Text
	seg   *segmentWidget
}

func (r *segmentRenderer) MinSize() geo.Size {
	th := r.seg.Theme()
	h := th.Size(theme.SizeNameButtonMinHeight)
	textSize := th.Size(theme.SizeNameLabelLarge)
	textW := bellui.MeasureText(r.seg.text, textSize, bellui.TextStyle{}).Width
	return geo.NewSize(textW+h*0.8, h)
}

func (r *segmentRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	labelH := r.label.MinSize().Height
	r.label.Move(geo.NewPos(0, (size.Height-labelH)/2))
	r.label.Resize(geo.NewSize(size.Width, labelH))
}

func (r *segmentRenderer) Refresh() {
	th := r.seg.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// Corner radius: pill on outer ends, flat on inner edges. Under RTL the
	// segment row is reflected (see segButtonRenderer.Layout), so the logical
	// first segment sits on the right and the logical last on the left; swap
	// which side is rounded so the pill ends stay on the visual outer edges.
	radius := th.Size(theme.SizeNameButtonMinHeight) / 2
	roundLeft, roundRight := r.seg.first, r.seg.last
	if lang.IsRTLDirection() {
		roundLeft, roundRight = r.seg.last, r.seg.first
	}
	if roundLeft {
		r.bg.TopLeftCornerRadius = radius
		r.bg.BottomLeftCornerRadius = radius
	} else {
		r.bg.TopLeftCornerRadius = 0
		r.bg.BottomLeftCornerRadius = 0
	}
	if roundRight {
		r.bg.TopRightCornerRadius = radius
		r.bg.BottomRightCornerRadius = radius
	} else {
		r.bg.TopRightCornerRadius = 0
		r.bg.BottomRightCornerRadius = 0
	}

	// A focused segment shows a thicker primary-coloured ring so keyboard
	// navigation is visible.
	if r.seg.focused {
		r.bg.StrokeWidth = 2
		r.bg.StrokeColor = th.Color(theme.ColorNamePrimary, v)
	} else {
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = th.Color(theme.ColorNameOutline, v)
	}

	if r.seg.selected {
		r.bg.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
		r.label.Color = th.Color(theme.ColorNameOnSecondaryContainer, v)
	} else {
		r.bg.FillColor = th.Color(theme.ColorNameSurface, v)
		r.label.Color = th.Color(theme.ColorNameOnSurface, v)
	}

	r.bg.Refresh()
	r.label.Refresh()
	r.Layout(r.seg.Size())
	canvas.Refresh(r.seg.super())
}

type segButtonRenderer struct {
	internalWidget.BaseRenderer
	segments []*segmentWidget
	btn      *SegmentedButton
}

func (r *segButtonRenderer) MinSize() geo.Size {
	w := float32(0)
	h := r.btn.Theme().Size(theme.SizeNameButtonMinHeight)
	for _, seg := range r.segments {
		min := seg.MinSize()
		w += min.Width
		if min.Height > h {
			h = min.Height
		}
	}
	return geo.NewSize(w, h)
}

func (r *segButtonRenderer) Layout(size geo.Size) {
	segW := size.Width / float32(len(r.segments))
	for i, seg := range r.segments {
		seg.Move(geo.NewPos(float32(i)*segW, 0))
		seg.Resize(geo.NewSize(segW, size.Height))
	}

	// Under RTL reflect the segment row so the first item sits on the right. The
	// per-segment corner rounding is swapped in segmentRenderer.Refresh so the
	// pill ends stay on the outer edges.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *segButtonRenderer) Refresh() {
	for i, seg := range r.segments {
		seg.selected = i == r.btn.Selected
		seg.focused = r.btn.focused && i == r.btn.Selected
		seg.Refresh()
	}
	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn.super())
}
