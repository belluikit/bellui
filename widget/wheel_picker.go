package widget

import (
	"math"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/paintgen"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var (
	_ event.Draggable         = (*WheelPicker)(nil)
	_ event.Tappable          = (*WheelPicker)(nil)
	_ core.Accessible         = (*WheelPicker)(nil)
	_ core.AccessibleValuer   = (*WheelPicker)(nil)
	_ core.AccessibleActioner = (*WheelPicker)(nil)
)

// WheelPicker is a short column, or row, of items the user spins through, as
// iOS's picker wheels and Android's NumberPicker do (#977). The item in the
// middle is the selection.
//
// A drag along the axis moves the items with the finger and a fling coasts;
// the wheel always settles with one item centred. Tapping an item off the
// centre spins it there. Items away from the centre are drawn smaller and
// fainter, down to ShrinkTo and FadeTo at the edge of the wheel.
//
// With Wrap the items form a ring: past the last item comes the first. Without
// it the wheel stops at both ends, and a drag past them meets resistance. A
// ring shows each item once, so one with fewer items than VisibleItems leaves
// the rest of the wheel empty.
//
// OnChanged fires once for each change of selection, when the wheel settles,
// not while it spins. Assistive technology reads the wheel as an adjustable
// control: its value is the selected item's accessible label, and increment
// and decrement move the selection by one. Name the wheel itself with
// SetAccessibilityLabel.
type WheelPicker struct {
	DisableableWidget

	// Items are the wheel's items, in order. Each item's accessible label
	// (SetAccessibilityLabel on a widget, or its own label) is what a screen
	// reader announces when it is selected.
	Items []core.CanvasObject
	// Selected is the index of the centred item.
	Selected int
	// OnChanged is called with the new index when the selection changes.
	OnChanged func(int) `json:"-"`

	// Orientation is Vertical (the default for a wheel built with
	// NewWheelPicker) or Horizontal.
	Orientation Orientation
	// Wrap makes the items a ring instead of a list with two ends.
	Wrap bool
	// VisibleItems is how many items the wheel shows at once. An even count
	// is rounded up to the next odd one, so one item is always centred. Zero
	// means 5.
	VisibleItems int
	// ShrinkTo is the scale, between 0 and 1, of an item at the edge of the
	// wheel; items between the centre and the edge scale in proportion. Zero
	// means 0.7. An item keeps its size and is drawn scaled, so its texture
	// is not rebuilt on every frame of a spin (#1183).
	ShrinkTo float32
	// FadeTo is the opacity, between 0 and 1, of an item at the edge of the
	// wheel; items between the centre and the edge fade in proportion. Zero
	// means 0.3; 1 turns the fade off.
	FadeTo float32

	// pos is the wheel's position in items: Selected at rest, fractional
	// while it spins. Without Wrap it may run past either end during a drag.
	pos      float32
	dragging bool
	axisLock wheelAxis
	dragFrom float32 // pos when the gesture's axis locked
	drag     float32 // finger travel along the axis, points
	speed    dragSpeed
	settling *bellui.Animation
}

type wheelAxis uint8

const (
	wheelAxisNone wheelAxis = iota
	wheelAxisAlong
	wheelAxisAcross
)

const (
	wheelDefaultVisible = 5
	wheelDefaultShrink  = 0.7
	wheelDefaultFade    = 0.3

	// wheelEdgeResistance divides a drag past the first or last item of a
	// wheel that does not wrap, as carouselEdgeResistance does.
	wheelEdgeResistance = 3

	// wheelCoast is how far a fling carries the wheel, as a time: the
	// release speed times wheelCoast. It is the distance UIScrollView's
	// normal deceleration (0.998 a millisecond) projects:
	// 0.998 / (1 - 0.998) ms = 0.499 s.
	wheelCoast = 499 * time.Millisecond
)

// wheelNow is the clock fling speeds are measured with; tests replace it.
var wheelNow = time.Now

// NewWheelPicker makes a vertical wheel of the given items, stopping at both
// ends, with the first item selected.
func NewWheelPicker(items []core.CanvasObject, changed func(int)) *WheelPicker {
	w := &WheelPicker{Items: items, OnChanged: changed, Orientation: Vertical}
	w.ExtendBaseWidget(w)
	return w
}

// SetSelected centres item i and fires OnChanged when the selection changed.
// Without Wrap, an index outside the items is clamped to them; with it, the
// index is taken around the ring.
func (w *WheelPicker) SetSelected(i int) {
	if w.setSelected(i) && w.OnChanged != nil {
		w.OnChanged(w.Selected)
	}
}

// SetSelectedSilently is SetSelected without OnChanged.
func (w *WheelPicker) SetSelectedSilently(i int) { w.setSelected(i) }

func (w *WheelPicker) setSelected(i int) bool {
	w.stopSettle()
	i = w.normalise(i)
	changed := i != w.Selected
	w.Selected = i
	w.pos = float32(i)
	w.relayout()
	return changed
}

// normalise maps an index onto the items: around the ring with Wrap,
// clamped without it. A wheel with no items keeps the index, so a selection
// made before the items arrive survives a Refresh in between (#984).
func (w *WheelPicker) normalise(i int) int {
	n := len(w.Items)
	if n == 0 {
		return i
	}
	if w.Wrap {
		return ((i % n) + n) % n
	}
	return max(0, min(n-1, i))
}

func (w *WheelPicker) visible() int {
	v := w.VisibleItems
	if v <= 0 {
		v = wheelDefaultVisible
	}
	if v%2 == 0 {
		v++
	}
	return v
}

func (w *WheelPicker) shrinkTo() float32 {
	if w.ShrinkTo <= 0 || w.ShrinkTo > 1 {
		return wheelDefaultShrink
	}
	return w.ShrinkTo
}

func (w *WheelPicker) fadeTo() float32 {
	if w.FadeTo <= 0 || w.FadeTo > 1 {
		return wheelDefaultFade
	}
	return w.FadeTo
}

func (w *WheelPicker) vertical() bool { return w.Orientation == Vertical }

// along and across are the components of d along and across the wheel's axis.
func (w *WheelPicker) along(d geo.Delta) float32 {
	if w.vertical() {
		return d.DY
	}
	return d.DX
}

func (w *WheelPicker) across(d geo.Delta) float32 {
	if w.vertical() {
		return d.DX
	}
	return d.DY
}

// pitch is the distance between neighbouring items' centres.
func (w *WheelPicker) pitch() float32 {
	s := w.Size()
	if w.vertical() {
		return s.Height / float32(w.visible())
	}
	return s.Width / float32(w.visible())
}

// ClaimsDrag takes a touch drag along the wheel and leaves one across it to
// the scroll around the picker (event.DragClaimer, #662).
func (w *WheelPicker) ClaimsDrag(d geo.Delta) bool {
	return !w.Disabled() && len(w.Items) > 0 && abs32(w.along(d)) > abs32(w.across(d))
}

// Dragged spins the wheel with the finger once the gesture has locked to the
// wheel's axis; a gesture that starts across it is left alone.
func (w *WheelPicker) Dragged(e *event.DragEvent) {
	if w.Disabled() || len(w.Items) == 0 {
		return
	}
	if !w.dragging {
		w.dragging = true
		w.axisLock = wheelAxisNone
		w.drag = 0
		w.speed.reset()
	}
	along, across := abs32(w.along(e.Dragged)), abs32(w.across(e.Dragged))
	if w.axisLock == wheelAxisNone {
		switch {
		case along > across:
			w.axisLock = wheelAxisAlong
			w.stopSettle()
			w.dragFrom = w.pos
		case across > along:
			w.axisLock = wheelAxisAcross
		}
	}
	if w.axisLock != wheelAxisAlong {
		return
	}
	w.drag += w.along(e.Dragged)

	w.speed.add(wheelNow(), w.drag)

	if p := w.pitch(); p > 0 {
		// Moving the items towards the start reveals later items.
		w.pos = w.resisted(w.dragFrom - w.drag/p)
	}
	w.relayout()
}

// resisted is where a drag that would put the wheel at raw leaves it: past
// either end of a wheel that does not wrap, it moves a wheelEdgeResistance-th
// as far.
func (w *WheelPicker) resisted(raw float32) float32 {
	if w.Wrap {
		return raw
	}
	last := float32(len(w.Items) - 1)
	switch {
	case raw < 0:
		return raw / wheelEdgeResistance
	case raw > last:
		return last + (raw-last)/wheelEdgeResistance
	}
	return raw
}

// DragEnd lets the wheel coast with the release speed, then settles it on the
// item nearest to where it comes to rest.
func (w *WheelPicker) DragEnd() {
	if !w.dragging {
		return
	}
	along := w.axisLock == wheelAxisAlong
	v := w.speed.at(wheelNow())
	w.dragging = false
	w.axisLock = wheelAxisNone
	w.speed.reset()
	if !along {
		return
	}
	target := w.pos
	if p := w.pitch(); p > 0 {
		target -= v * float32(wheelCoast.Seconds()) / p
	}
	w.settleTo(int(math.Round(float64(target))))
}

// Tapped spins the tapped item to the centre.
func (w *WheelPicker) Tapped(e *event.PointEvent) {
	if w.Disabled() || len(w.Items) == 0 {
		return
	}
	p := w.pitch()
	if p <= 0 {
		return
	}
	s := w.Size()
	off := e.Position.Y - s.Height/2
	if !w.vertical() {
		off = e.Position.X - s.Width/2
	}
	if steps := int(math.Round(float64(off / p))); steps != 0 {
		w.settleTo(w.Selected + steps)
	}
}

// settleTo spins the wheel to item i, the index taken around the ring with
// Wrap and clamped without it, and selects it when it arrives. Animations off
// in the app settings, it moves there at once.
func (w *WheelPicker) settleTo(i int) {
	w.stopSettle()
	if !w.Wrap {
		i = w.normalise(i)
	}
	from, to := w.pos, float32(i)
	finish := func() {
		w.settling = nil
		w.pos = float32(w.normalise(i))
		w.relayout()
		if sel := w.normalise(i); sel != w.Selected {
			w.Selected = sel
			if w.OnChanged != nil {
				w.OnChanged(sel)
			}
		}
	}
	if from == to || !bellui.CurrentApp().Settings().ShowAnimations() {
		finish()
		return
	}
	d := canvas.DurationMedium2
	if abs32(to-from) > 1.5 {
		d = canvas.DurationLong2
	}
	var a *bellui.Animation
	a = bellui.NewAnimation(d, func(done float32) {
		if w.settling != a {
			return // superseded by a drag, a tap or SetSelected
		}
		if done >= 1 {
			finish()
			return
		}
		w.pos = from + (to-from)*done
		w.relayout()
	})
	a.Curve = bellui.AnimationEaseStandardDecelerate
	w.settling = a
	a.Start()
}

func (w *WheelPicker) stopSettle() {
	if a := w.settling; a != nil {
		w.settling = nil
		a.Stop()
	}
}

// relayout places the items for the wheel's position, without building a
// renderer the wheel does not have yet.
func (w *WheelPicker) relayout() {
	cr, ok := cache.CachedRenderer(w.super())
	if !ok {
		return
	}
	if r, ok := cr.(*wheelPickerRenderer); ok {
		r.layoutItems()
		canvas.Refresh(r.track)
	}
}

// AccessibleRole returns [core.RoleSpinButton]: an adjustable control whose
// value steps through the items.
func (w *WheelPicker) AccessibleRole() core.AccessibleRole { return core.RoleSpinButton }

// AccessibleLabel returns an empty string; name the wheel with
// SetAccessibilityLabel.
func (w *WheelPicker) AccessibleLabel() string { return "" }

// AccessibleValue returns the selected item's accessible label.
func (w *WheelPicker) AccessibleValue() string {
	if w.Selected < 0 || w.Selected >= len(w.Items) {
		return ""
	}
	return accessibleLabelOf(w.Items[w.Selected])
}

// AccessibleRange returns the item indices: 0, the last, and the selected.
func (w *WheelPicker) AccessibleRange() (min, max, current float64) {
	return 0, float64(len(w.Items) - 1), float64(w.Selected)
}

// AccessibleState reports the wheel as disabled when it is.
func (w *WheelPicker) AccessibleState() core.AccessibleState {
	if w.Disabled() {
		return core.StateDisabled
	}
	return 0
}

// AccessibleActions returns the "increment" and "decrement" actions.
func (w *WheelPicker) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{
		{Name: core.ActionIncrement, Description: "Select the next item"},
		{Name: core.ActionDecrement, Description: "Select the previous item"},
	}
}

// PerformAccessibleAction moves the selection by one item.
func (w *WheelPicker) PerformAccessibleAction(name string) {
	if w.Disabled() || len(w.Items) == 0 {
		return
	}
	switch name {
	case core.ActionIncrement:
		w.SetSelected(w.Selected + 1)
	case core.ActionDecrement:
		w.SetSelected(w.Selected - 1)
	}
}

// accessibleLabelOf is what a screen reader names o by: an app-set label
// first, then the object's own.
func accessibleLabelOf(o core.CanvasObject) string {
	if l, ok := o.(core.AccessibleLabelOverrider); ok {
		if s := l.AccessibilityLabel(); s != "" {
			return s
		}
	}
	if a, ok := o.(core.Accessible); ok {
		return a.AccessibleLabel()
	}
	return ""
}

func (w *WheelPicker) MinSize() geo.Size {
	w.ExtendBaseWidget(w)
	return w.BaseWidget.MinSize()
}

func (w *WheelPicker) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	w.Selected = w.normalise(w.Selected)
	w.pos = float32(w.Selected)
	// Items moving in and out at the ends are cut at the wheel's edge.
	track := &bellui.Container{ClipToBounds: true}
	r := &wheelPickerRenderer{BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{track}), track: track, wheel: w}
	r.syncSlots()
	return r
}

type wheelPickerRenderer struct {
	internalWidget.BaseRenderer
	track *bellui.Container
	wheel *WheelPicker
	slots []*wheelSlot
}

// syncSlots gives each item a slot, keeping the slots of items still there.
func (r *wheelPickerRenderer) syncSlots() {
	items := r.wheel.Items
	same := len(r.slots) == len(items)
	for i := 0; same && i < len(items); i++ {
		same = r.slots[i].item == items[i]
	}
	if same {
		return
	}
	r.slots = make([]*wheelSlot, len(items))
	objs := make([]core.CanvasObject, len(items))
	for i, it := range items {
		r.slots[i] = newWheelSlot(it)
		objs[i] = r.slots[i]
	}
	r.track.Objects = objs
}

// wheelSlot holds one item at an opacity and a scale. It covers the whole
// track and leaves the item where layoutItems puts it.
//
// The scale is a transform about the item's centre, not a smaller size: a
// resized image is drawn again from its pixels, and an SVG rasterized again,
// so shrinking the items by size rebuilt every visible item's texture on
// every frame of a spin, about half a phone's CPU (#1183).
type wheelSlot struct {
	BaseWidget
	item    core.CanvasObject
	opacity float32

	mu        sync.RWMutex // the transform is read by the painter
	transform geo.Matrix
}

var (
	_ core.OpacityGroup  = (*wheelSlot)(nil)
	_ core.Transformable = (*wheelSlot)(nil)
)

func newWheelSlot(item core.CanvasObject) *wheelSlot {
	s := &wheelSlot{item: item, opacity: 1}
	s.ExtendBaseWidget(s)
	return s
}

func (s *wheelSlot) GroupOpacity() float32 { return s.opacity }

// Transform returns the scale of the slot's item about its centre.
func (s *wheelSlot) Transform() geo.Matrix {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.transform == (geo.Matrix{}) {
		return geo.Identity()
	}
	return s.transform
}

// SetTransform sets the slot's transform; layoutItems sets it through scaleAbout.
func (s *wheelSlot) SetTransform(m geo.Matrix) {
	s.mu.Lock()
	changed := s.transform != m
	s.transform = m
	s.mu.Unlock()
	if changed {
		paintgen.Changed()
		internalWidget.Repaint(s)
	}
}

// scaleAbout scales the slot's item by scale about the point (cx, cy) of the
// track.
func (s *wheelSlot) scaleAbout(scale, cx, cy float32) {
	if scale == 1 {
		s.SetTransform(geo.Identity())
		return
	}
	s.SetTransform(geo.NewTranslation(cx, cy).
		Multiply(geo.NewScale(scale, scale)).
		Multiply(geo.NewTranslation(-cx, -cy)))
}

func (s *wheelSlot) CreateRenderer() core.WidgetRenderer {
	return &wheelSlotRenderer{objects: []core.CanvasObject{s.item}}
}

type wheelSlotRenderer struct{ objects []core.CanvasObject }

func (*wheelSlotRenderer) Destroy()                       {}
func (*wheelSlotRenderer) Layout(geo.Size)                {}
func (*wheelSlotRenderer) MinSize() geo.Size              { return geo.Size{} }
func (r *wheelSlotRenderer) Objects() []core.CanvasObject { return r.objects }
func (*wheelSlotRenderer) Refresh()                       {}

// itemSize is the largest item's MinSize, the size of the centred slot.
func (r *wheelPickerRenderer) itemSize() geo.Size {
	var s geo.Size
	for _, it := range r.wheel.Items {
		s = s.Max(it.MinSize())
	}
	return s
}

func (r *wheelPickerRenderer) MinSize() geo.Size {
	w := r.wheel
	s := r.itemSize()
	pad := w.Theme().Size(theme.SizeNamePadding)
	n := float32(w.visible())
	if w.vertical() {
		return geo.NewSize(s.Width, n*(s.Height+pad))
	}
	return geo.NewSize(n*(s.Width+pad), s.Height)
}

func (r *wheelPickerRenderer) Layout(size geo.Size) {
	r.track.Resize(size)
	r.layoutItems()
}

// layoutItems centres each item at its distance from the wheel's position,
// scaled down with that distance, and hides the items past the wheel's ends.
// It first gives any items set since the last Refresh their slots: a
// selection change lays the wheel out without one (#984).
func (r *wheelPickerRenderer) layoutItems() {
	w := r.wheel
	r.syncSlots()
	size := r.track.Size()
	n := len(w.Items)
	if n == 0 {
		return
	}
	pitch := w.pitch()
	half := float32(w.visible()) / 2
	shrink, fade := w.shrinkTo(), w.fadeTo()
	slot := r.itemSize()
	if w.vertical() {
		slot.Width = geo.Max(slot.Width, size.Width)
		slot.Height = pitch
	} else {
		slot.Width = pitch
		slot.Height = geo.Max(slot.Height, size.Height)
	}
	for i, it := range w.Items {
		d := float32(i) - w.pos
		if w.Wrap {
			// The nearest copy of the item around the ring.
			d = d - float32(n)*float32(math.Round(float64(d/float32(n))))
		}
		holder := r.slots[i]
		holder.Move(geo.Position{})
		holder.Resize(size)
		if abs32(d) > half+0.5 || size.IsZero() {
			it.Hide()
			continue
		}
		edge := geo.Min(1, abs32(d)/half)
		holder.opacity = 1 - (1-fade)*edge
		scale := 1 - (1-shrink)*edge
		// The item keeps its own size, at most the slot's, and is drawn
		// scaled about its centre.
		ms := it.MinSize()
		is := geo.NewSize(geo.Min(ms.Width, slot.Width), geo.Min(ms.Height, slot.Height))
		cx, cy := size.Width/2, size.Height/2
		if w.vertical() {
			cy += d * pitch
		} else {
			cx += d * pitch
		}
		it.Move(geo.NewPos(cx-is.Width/2, cy-is.Height/2))
		it.Resize(is)
		holder.scaleAbout(scale, cx, cy)
		it.Show()
	}
}

func (r *wheelPickerRenderer) Refresh() {
	w := r.wheel
	r.syncSlots()
	if !w.dragging && w.settling == nil {
		w.Selected = w.normalise(w.Selected)
		w.pos = float32(w.Selected)
	}
	r.Layout(w.Size())
	canvas.Refresh(w.super())
}
