package driver

import "github.com/belluikit/bellui/core"

// disabledOpacity is the MD3 opacity of a disabled widget's subtree.
const disabledOpacity = 0.38

// opacityPainter is the part of a painter the opacity stack drives.
type opacityPainter interface {
	SetGroupOpacity(float32)
	ClearGroupOpacity()
}

// OpacityStack is a paint walk's group opacity: the product of the
// core.OpacityGroup objects (and, where the driver dims them, the disabled
// widgets) the walk is inside. Push it for every node on the way down and Pop
// it for every node on the way up, both unconditionally, so the pairs survive
// every early return a walk takes for culling and boundaries.
//
// A RepaintBoundary's subtree is cached in a texture and blitted. EnterBoundary
// starts the cached content at full opacity, and ExitBoundary returns to the
// ancestors' opacity before the blit, so the ancestors' opacity is applied
// once, when the texture is drawn, and changing it does not stale the cache.
type OpacityStack struct {
	DimDisabled bool
	values      []float32
	// set is the opacity last handed to the painter, as 1 - set so that the
	// zero value means full opacity. The painter is told only of changes:
	// most of a tree has no opacity of its own.
	set float32
}

// Current is the opacity draws are multiplied by now.
func (s *OpacityStack) Current() float32 {
	if n := len(s.values); n > 0 {
		return s.values[n-1]
	}
	return 1
}

// Transparent reports whether draws now would show nothing. The painters read
// a group opacity of 0 as unset, so a walk must skip its draws instead.
func (s *OpacityStack) Transparent() bool { return s.Current() <= 0 }

// Push enters obj, multiplying in its own opacity.
func (s *OpacityStack) Push(p opacityPainter, obj core.CanvasObject) {
	f := float32(1)
	if s.DimDisabled {
		if d, ok := obj.(core.Disableable); ok && d.Disabled() {
			f = disabledOpacity
		}
	}
	if g, ok := obj.(core.OpacityGroup); ok {
		f *= max(0, min(1, g.GroupOpacity()))
	}
	s.push(p, s.Current()*f)
}

// EnterBoundary starts a boundary's cached content at full opacity.
func (s *OpacityStack) EnterBoundary(p opacityPainter) { s.push(p, 1) }

// ExitBoundary returns to the opacity the boundary's texture is drawn with.
func (s *OpacityStack) ExitBoundary(p opacityPainter) { s.Pop(p) }

// Pop leaves the last node pushed.
func (s *OpacityStack) Pop(p opacityPainter) {
	if n := len(s.values); n > 0 {
		s.values = s.values[:n-1]
	}
	s.apply(p)
}

func (s *OpacityStack) push(p opacityPainter, v float32) {
	s.values = append(s.values, v)
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
