package driver

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// disabledOpacity is the MD3 opacity of a disabled widget's subtree.
const disabledOpacity = 0.38

// opacityPainter is the part of a painter the opacity stack drives.
type opacityPainter interface {
	SetGroupOpacity(float32)
	ClearGroupOpacity()
}

// OpacityStack is a paint walk's group opacity: the product of the
// core.OpacityGroup objects and the disabled widgets the walk is inside. Push
// it for every node on the way down and Pop it for every node on the way up,
// both unconditionally, so the pairs survive every early return a walk takes
// for culling and boundaries.
//
// A disabled widget is dimmed unless it draws its own disabled look
// (core.DisabledLook). Every walk, desktop, mobile and software, dims the same
// way, so a widget looks the same on every platform and in test captures
// (#1223).
//
// Under a disabled widget, what a widget with no disabled look of its own
// draws is dimmed too (#1393). A container that draws its own disabled look
// (AppTabs, List, a Form) disables only its Disableable children, so the
// content an app puts in it, a label in a tab or a list row, a link, a widget
// of its own, stayed at full colour while it took no input (#1378). The rule
// dims drawn objects, not subtrees: a text, rectangle or image is dimmed when
// the nearest widget above it does not draw its own disabled look, so a Button
// in the same tab, which does, is not dimmed twice, and containers and
// scrolls pass through. A widget whose owner draws it in the disabled colours
// already says so with core.DisabledLook.
//
// A RepaintBoundary's subtree is cached in a texture and blitted. EnterBoundary
// starts the cached content at full opacity, and ExitBoundary returns to the
// ancestors' opacity before the blit, so the ancestors' opacity is applied
// once, when the texture is drawn, and changing it does not stale the cache.
type OpacityStack struct {
	values []level
	// set is the opacity last handed to the painter, as 1 - set so that the
	// zero value means full opacity. The painter is told only of changes:
	// most of a tree has no opacity of its own.
	set float32
}

// level is one node of the walk: the opacity its draws are multiplied by,
// whether it is inside a disabled widget, whether a disabled widget above it
// is already dimmed, and whether the nearest widget at or above it draws its
// own disabled look.
type level struct {
	opacity                 float32
	inert, dimmed, ownsLook bool
}

func (s *OpacityStack) top() level {
	if n := len(s.values); n > 0 {
		return s.values[n-1]
	}
	return level{opacity: 1}
}

// Current is the opacity draws are multiplied by now.
func (s *OpacityStack) Current() float32 { return s.top().opacity }

// Transparent reports whether draws now would show nothing. The painters read
// a group opacity of 0 as unset, so a walk must skip its draws instead.
func (s *OpacityStack) Transparent() bool { return s.Current() <= 0 }

// Push enters obj, multiplying in its own opacity.
func (s *OpacityStack) Push(p opacityPainter, obj core.CanvasObject) {
	parent := s.top()
	l := level{inert: parent.inert, dimmed: parent.dimmed, ownsLook: parent.ownsLook}
	f := float32(1)
	if _, isWidget := obj.(core.Widget); isWidget {
		own, ok := obj.(core.DisabledLook)
		l.ownsLook = ok && own.DrawsDisabledLook()
	}
	switch {
	case dimsDisabled(obj):
		f = disabledOpacity
		l.dimmed = true
	case parent.inert && !parent.dimmed && !l.ownsLook && drawn(obj):
		f = disabledOpacity
	}
	if sub, ok := obj.(disabledSubtree); ok && sub.DrawsDisabledSubtree() {
		l.dimmed = true // already drawn disabled: nothing below is dimmed again
	}
	if d, ok := obj.(core.Disableable); ok && d.Disabled() {
		l.inert = true
	}
	if g, ok := obj.(core.OpacityGroup); ok {
		f *= max(0, min(1, g.GroupOpacity()))
	}
	l.opacity = parent.opacity * f
	s.push(p, l)
}

// dimsDisabled reports whether obj is a disabled widget the walk dims: one
// that does not draw its own disabled look.
func dimsDisabled(obj core.CanvasObject) bool {
	d, ok := obj.(core.Disableable)
	if !ok || !d.Disabled() {
		return false
	}
	own, ok := obj.(core.DisabledLook)
	return !ok || !own.DrawsDisabledLook()
}

// disabledSubtree is a widget that draws its whole subtree, the content an
// app gave it included, in the disabled colours while it is disabled, as a
// TreeView row recolours its node. Nothing under it is dimmed again (#1393).
type disabledSubtree interface {
	DrawsDisabledSubtree() bool
}

// drawn reports whether obj draws something itself: not a widget or a
// container, which only hold what draws.
func drawn(obj core.CanvasObject) bool {
	if _, ok := obj.(core.Widget); ok {
		return false
	}
	_, isContainer := obj.(*bellui.Container)
	return !isContainer
}

// EnterBoundary starts a boundary's cached content at full opacity. Whether it
// is inert or already dimmed carries in, so the cached content is not dimmed
// twice.
func (s *OpacityStack) EnterBoundary(p opacityPainter) {
	parent := s.top()
	s.push(p, level{opacity: 1, inert: parent.inert, dimmed: parent.dimmed, ownsLook: parent.ownsLook})
}

// ExitBoundary returns to the opacity the boundary's texture is drawn with.
func (s *OpacityStack) ExitBoundary(p opacityPainter) { s.Pop(p) }

// Pop leaves the last node pushed.
func (s *OpacityStack) Pop(p opacityPainter) {
	if n := len(s.values); n > 0 {
		s.values = s.values[:n-1]
	}
	s.apply(p)
}

func (s *OpacityStack) push(p opacityPainter, l level) {
	s.values = append(s.values, l)
	s.apply(p)
}

func (s *OpacityStack) apply(p opacityPainter) {
	v := s.Current()
	if 1-v == s.set {
		return
	}
	s.set = 1 - v
	if v < 1 {
		p.SetGroupOpacity(v)
	} else {
		p.ClearGroupOpacity()
	}
}
