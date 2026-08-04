package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	idriver "github.com/belluikit/bellui/internal/driver"
)

// ObjectAtPosition returns the deepest visible object at pos within canvas c,
// together with its absolute bounds. ok is false when no object is hit.
//
// It uses the same hit-test the toolkit uses for tap dispatch: overlays are
// searched top-down first, then the canvas's remaining dispatch roots — on
// canvases that have them, the desktop menu bar and the mobile window head or
// open menu (#245) — then the content; an input-pass-through overlay (a
// tooltip, Toast or Snackbar) does not block objects beneath it. This is the
// "pick" primitive behind the #540 widget inspector, exposed for tooling that
// maps a click back to the object that produced it (e.g. a design tool's
// pick-to-source).
//
// # Coordinate space
//
// pos and the returned bounds are both in the space of
// [bellui.Driver.AbsolutePositionForObject]: relative to the canvas's
// interactive area ([core.Canvas.InteractiveArea]), not to the raw canvas
// tree. To go the other way — highlight a known object — compose
// [bellui.Driver.AbsolutePositionForObject] with the object's size; the
// result matches the bounds returned here (#235). The raw-tree helpers
// [GeometryForObject] and [ClipBoundsForObject] report the other space; the
// two differ by the interactive-area inset, which is non-zero only on mobile
// and the test canvas.
//
// Event positions are in the raw-tree space as well: the tap dispatcher feeds
// this same hit-walk the position it later delivers as
// [event.PointEvent].AbsolutePosition, so a probe taken from an event must
// subtract the [core.Canvas.InteractiveArea] inset before calling here — a
// no-op on desktop, where the inset is zero.
//
// Since: 1.0
func ObjectAtPosition(c core.Canvas, pos geo.Position) (core.CanvasObject, geo.Rectangle, bool) {
	return ObjectAtPositionMatching(c, pos, func(core.CanvasObject) bool { return true })
}

// dispatchHitTester is implemented by canvases whose dispatch hit-walk covers
// more than overlays + content — the glfw menu bar, the mobile window head
// and open menu — or prefers a spatial index. When a canvas exposes it, the
// wrappers here probe through it so a public pick matches tap dispatch
// exactly (#245); otherwise they fall back to walking overlays + content,
// which is the whole tree on canvases without those extra roots (the test
// and embedded canvases).
type dispatchHitTester interface {
	FindObjectAtPositionMatching(pos geo.Position, matches func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int)
	ObjectTrees() []core.CanvasObject
}

// ObjectAtPositionMatching is like [ObjectAtPosition] but returns the deepest
// object for which matches reports true, descending past objects the predicate
// rejects. Use it to skip renderer-internal primitives and resolve, say, the
// deepest object that is one of your own source-mapped widgets.
//
// Since: 1.0
func ObjectAtPositionMatching(c core.Canvas, pos geo.Position, matches func(core.CanvasObject) bool) (core.CanvasObject, geo.Rectangle, bool) {
	if c == nil || matches == nil {
		return nil, geo.Rectangle{}, false
	}

	// The hit-walk itself runs in raw canvas-tree coordinates, but the public
	// contract is the [bellui.Driver.AbsolutePositionForObject] space, so the
	// probe translates in by the interactive-area inset and the bounds
	// translate back out (#235). A zero inset — every desktop driver — makes
	// both a no-op.
	inset, _ := c.InteractiveArea()
	treePos := pos.Add(inset)

	var obj core.CanvasObject
	var trees []core.CanvasObject
	if ht, ok := c.(dispatchHitTester); ok {
		obj, _, _ = ht.FindObjectAtPositionMatching(treePos, matches)
		trees = ht.ObjectTrees()
	} else {
		overlays := c.Overlays().List()
		obj, _, _ = idriver.FindObjectAtPositionMatching(treePos, matches, overlays, c.Content())
		trees = append([]core.CanvasObject{c.Content()}, overlays...)
	}
	if obj == nil {
		return nil, geo.Rectangle{}, false
	}

	origin := idriver.AbsolutePositionForObject(obj, trees).Subtract(inset)
	return obj, geo.Rectangle{Pos: origin, Size: obj.Size()}, true
}
