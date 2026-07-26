package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	idriver "github.com/belluikit/bellui/internal/driver"
)

// ObjectAtPosition returns the deepest visible object at pos within canvas c,
// together with its absolute bounds in canvas coordinates. ok is false when no
// object is hit.
//
// It uses the same hit-test the toolkit uses for tap dispatch: overlays are
// searched top-down before the content, and an input-pass-through overlay (a
// tooltip, Toast or Snackbar) does not block objects beneath it. This is the
// "pick" primitive behind the #540 widget inspector, exposed for tooling that
// maps a click back to the object that produced it (e.g. a design tool's
// pick-to-source).
//
// To go the other way — highlight a known object — take its bounds from
// [ObjectAtPosition] here or compose [bellui.Driver.AbsolutePositionForObject]
// with the object's size.
//
// Since: 2.10
func ObjectAtPosition(c core.Canvas, pos geo.Position) (core.CanvasObject, geo.Rectangle, bool) {
	return ObjectAtPositionMatching(c, pos, func(core.CanvasObject) bool { return true })
}

// ObjectAtPositionMatching is like [ObjectAtPosition] but returns the deepest
// object for which matches reports true, descending past objects the predicate
// rejects. Use it to skip renderer-internal primitives and resolve, say, the
// deepest object that is one of your own source-mapped widgets.
//
// Since: 2.10
func ObjectAtPositionMatching(c core.Canvas, pos geo.Position, matches func(core.CanvasObject) bool) (core.CanvasObject, geo.Rectangle, bool) {
	if c == nil || matches == nil {
		return nil, geo.Rectangle{}, false
	}
	overlays := c.Overlays().List()
	obj, _, _ := idriver.FindObjectAtPositionMatching(pos, matches, overlays, c.Content())
	if obj == nil {
		return nil, geo.Rectangle{}, false
	}

	roots := append([]core.CanvasObject{c.Content()}, overlays...)
	origin := idriver.AbsolutePositionForObject(obj, roots)
	return obj, geo.Rectangle{Pos: origin, Size: obj.Size()}, true
}
