package driver

import (
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/uidir"
)

// directionScoped is implemented by container.DirectionScope. The driver paint
// walks detect it structurally so they can scope the subtree's paint without
// importing the high-level container package (which itself imports this one).
type directionScoped interface {
	ScopedDirection() bellui.Direction
}

// PushDirectionScope pushes obj's ambient layout direction onto the per-goroutine
// uidir stack when obj is a direction scope (or an overlay that inherited its
// opener's direction, see [InheritDirection]), so layout, paint-time text base
// direction and leading/trailing alignment inside the subtree resolve against
// the scope rather than the process-global direction. The layout walk
// (common.Canvas.EnsureMinSize), the paint walks and the theme re-layout call
// it, balanced by PopDirectionScope in the matching after-children callback.
// Any other obj is a no-op.
func PushDirectionScope(obj core.CanvasObject) {
	if ds, ok := obj.(directionScoped); ok {
		uidir.PushOwned(ds.ScopedDirection(), obj, false)
	} else if d, ok := uidir.Inherited(obj); ok {
		uidir.PushOwned(d, obj, true)
	}
}

// PopDirectionScope balances PushDirectionScope for obj: it pops what obj
// pushed there, so a scope's direction or an overlay's inherited one changed
// while the walk was inside obj (a popup re-shown or dismissed from a layout)
// cannot unbalance the stack. A non-scope obj is a no-op.
func PopDirectionScope(obj core.CanvasObject) {
	_, scope := obj.(directionScoped)
	uidir.PopOwned(obj, scope)
}

// AbsolutePositionForObject returns the absolute position of an object in a set of object trees.
// If the object is not part of any of the trees, the position (0,0) is returned.
//
// It is the position from layout, the sum of the positions of the object and
// its ancestors, and ignores transforms (#1003). [AbsolutePositionForPoint]
// and [PositionForAbsolutePoint] follow the world transform.
func AbsolutePositionForObject(object core.CanvasObject, trees []core.CanvasObject) geo.Position {
	var pos geo.Position
	findPos := func(o core.CanvasObject, p geo.Position, _ geo.Position, _ geo.Size) bool {
		if o == object {
			pos = p
			return true
		}
		return false
	}
	for _, tree := range trees {
		if WalkVisibleObjectTree(tree, findPos, nil) {
			break
		}
	}
	return pos
}

// WorldTransformForObject returns the world transform of object within the
// trees, composed the way the painters and the hit-test compose it:
// parent · Translate(Position()) · Transform() (core.Transformable). ok is
// false when object is not in any of the trees.
func WorldTransformForObject(object core.CanvasObject, trees []core.CanvasObject) (wt geo.Matrix, ok bool) {
	stack := []geo.Matrix{geo.Identity()}
	before := func(o core.CanvasObject, _ geo.Position, _ geo.Position, _ geo.Size) bool {
		pos := o.Position()
		local := geo.NewTranslation(pos.X, pos.Y)
		if t, is := o.(core.Transformable); is {
			if lt := t.Transform(); !lt.IsIdentity() {
				local = local.Multiply(lt)
			}
		}
		m := stack[len(stack)-1].Multiply(local)
		if o == object {
			wt, ok = m, true
			return true
		}
		stack = append(stack, m)
		return false
	}
	after := func(core.CanvasObject, geo.Position, core.CanvasObject) {
		if len(stack) > 1 {
			stack = stack[:len(stack)-1]
		}
	}
	for _, tree := range trees {
		stack = stack[:1]
		if WalkVisibleObjectTree(tree, before, after) {
			break
		}
	}
	return wt, ok
}

// AbsolutePositionForPoint returns where pos, in object's own coordinates, is
// drawn within the trees: pos mapped through object's world transform. When
// object is not in the trees it returns pos, as AbsolutePositionForObject's
// (0,0) plus pos would.
func AbsolutePositionForPoint(object core.CanvasObject, pos geo.Position, trees []core.CanvasObject) geo.Position {
	wt, ok := WorldTransformForObject(object, trees)
	if !ok {
		return pos
	}
	x, y := wt.TransformPoint(pos.X, pos.Y)
	return geo.NewPos(x, y)
}

// PositionForAbsolutePoint maps abs, a position within the trees, into
// object's own coordinates through the inverse of its world transform, the
// mapping the hit-test uses (LocalHitPoint). ok is false when object is not in
// the trees or its transform cannot be inverted (a zero scale); abs is then
// returned unchanged.
func PositionForAbsolutePoint(object core.CanvasObject, abs geo.Position, trees []core.CanvasObject) (geo.Position, bool) {
	wt, ok := WorldTransformForObject(object, trees)
	if !ok {
		return abs, false
	}
	inv, ok := wt.Inverse()
	if !ok {
		return abs, false
	}
	x, y := inv.TransformPoint(abs.X, abs.Y)
	return geo.NewPos(x, y), true
}

// NoClipBounds is the open clip rectangle walkObjectTree starts with: an object
// reached with this clip size has crossed no clipping ancestor.
var NoClipBounds = geo.NewSize(math.MaxInt32, math.MaxInt32)

// ClipBoundsForObject returns the region object's clipping ancestors leave
// visible, within the given trees, in absolute canvas coordinates: the bounds
// of the nearest one, intersected with those of the ones above it (#566), and
// empty when they do not overlap. ok is false when object is not found, or
// when no ancestor clips it (it is drawn against the full canvas). A clipping
// ancestor is one [IsClip] reports — a [event.Scrollable], a
// [bellui.Container] with ClipToBounds, or a widget whose renderer clips. It
// reuses the same clip tracking the hit-test walk uses, so a caller can size
// content (e.g. a popover) to the region that will actually be visible rather
// than overflowing into a clipped area.
func ClipBoundsForObject(object core.CanvasObject, trees []core.CanvasObject) (geo.Rectangle, bool) {
	var bounds geo.Rectangle
	var clipped bool
	find := func(o core.CanvasObject, _ geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
		if o != object {
			return false
		}
		if clipSize != NoClipBounds {
			bounds = geo.Rectangle{Pos: clipPos, Size: clipSize}
			clipped = true
		}
		return true // found the object — stop walking
	}
	for _, tree := range trees {
		if WalkVisibleObjectTree(tree, find, nil) {
			break
		}
	}
	return bounds, clipped
}

// FindObjectAtPositionMatching is used to find an object in a canvas at the specified position.
// The matches function determines of the type of object that is found at this position is of a suitable type.
// The various canvas roots and overlays that can be searched are also passed in.
func FindObjectAtPositionMatching(mouse geo.Position, matches func(object core.CanvasObject) bool, overlays []core.CanvasObject, roots ...core.CanvasObject) (core.CanvasObject, geo.Position, int) {
	var found core.CanvasObject
	var foundPos geo.Position

	// Transform stack for composing worldTransform through the tree.
	wtStack := []geo.Matrix{geo.Identity()}

	beforeFunc := func(walked core.CanvasObject, _ geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
		if !walked.Visible() {
			return false
		}

		// Compose worldTransform: parent_WT * Translate(obj.Position()) * obj.Transform()
		parentWT := wtStack[len(wtStack)-1]
		objPos := walked.Position()
		localT := geo.NewTranslation(objPos.X, objPos.Y)
		if t, ok := walked.(core.Transformable); ok {
			lt := t.Transform()
			if !lt.IsIdentity() {
				localT = localT.Multiply(lt)
			}
		}
		wt := parentWT.Multiply(localT)
		wtStack = append(wtStack, wt)

		// Clip check (coarse, unchanged).
		if mouse.X < clipPos.X || mouse.Y < clipPos.Y {
			return false
		}
		if mouse.X >= clipPos.X+clipSize.Width || mouse.Y >= clipPos.Y+clipSize.Height {
			return false
		}

		w := walked.Size().Width
		h := walked.Size().Height

		if local, in := LocalHitPoint(wt, mouse, w, h); in && matchesHitTarget(walked, matches) {
			found = walked
			foundPos = local
		}
		return false
	}

	afterFunc := func(_ core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		if len(wtStack) > 1 {
			wtStack = wtStack[:len(wtStack)-1]
		}
	}

	layer := 0
	// Walk the overlay stack top-down. An input-pass-through overlay (a
	// tooltip, a Toast or Snackbar) must not block interaction with the UI
	// beneath it: when nothing inside it matched at this position, the
	// search continues with the next overlay down. The first overlay that
	// does not pass input through is a barrier (a modal dialog stays
	// modal even with a toast stacked above it).
	for i := len(overlays) - 1; i >= 0; i-- {
		overlay := overlays[i]
		if overlay == nil {
			continue
		}
		wtStack = wtStack[:1] // reset stack between walks
		WalkVisibleObjectTree(overlay, beforeFunc, afterFunc)
		if found != nil || !overlayPassesInput(overlay) {
			return found, foundPos, layer
		}
	}
	for _, root := range roots {
		layer++
		if root == nil {
			continue
		}
		wtStack = wtStack[:1] // reset stack between roots
		WalkVisibleObjectTree(root, beforeFunc, afterFunc)
		if found != nil {
			break
		}
	}

	return found, foundPos, layer
}

// LocalHitPoint maps mouse, in canvas coordinates, into the coordinates of an
// object of size w×h whose world transform is wt, and reports whether it falls
// inside the object, [0, w) × [0, h). The world transform carries the object's
// position, so a translation, a pivot or a transformed ancestor moves where it
// is hit as it moves where it is drawn (#997).
func LocalHitPoint(wt geo.Matrix, mouse geo.Position, w, h float32) (geo.Position, bool) {
	var localX, localY float32
	if wt.B == 0 && wt.C == 0 {
		// Axis-aligned (translation + optional scale). Fast path.
		if wt.A == 0 || wt.D == 0 {
			return geo.Position{}, false // degenerate scale
		}
		localX = (mouse.X - wt.TX) / wt.A
		localY = (mouse.Y - wt.TY) / wt.D
	} else {
		inv, ok := wt.Inverse()
		if !ok {
			return geo.Position{}, false
		}
		localX, localY = inv.TransformPoint(mouse.X, mouse.Y)
	}
	if localX < 0 || localY < 0 || localX >= w || localY >= h {
		return geo.Position{}, false
	}
	return geo.NewPos(localX, localY), true
}

// InputPassThrough is implemented by overlay roots that must not block
// input to the content beneath them — a tooltip, not a dialog. When the
// top overlay reports true, hit-testing falls through to the lower
// roots wherever the overlay itself has no match at the position.
type InputPassThrough interface {
	PassesInputThrough() bool
}

func overlayPassesInput(overlay core.CanvasObject) bool {
	pt, ok := overlay.(InputPassThrough)
	return ok && pt.PassesInputThrough()
}

// matchesHitTarget applies the caller's match predicate, excluding
// input-pass-through surfaces themselves: a transient overlay root (Toast or
// Snackbar's PopUp, a tooltip) may satisfy predicates like Tappable while
// covering the whole canvas, which would defeat its own transparency. Its
// content children remain ordinary hit targets (a Snackbar's action button
// still works).
func matchesHitTarget(walked core.CanvasObject, matches func(object core.CanvasObject) bool) bool {
	if pt, ok := walked.(InputPassThrough); ok && pt.PassesInputThrough() {
		return false
	}
	return matches(walked)
}

// ObjectAtPosition holds an object found at a position along with the mouse position relative to it.
type ObjectAtPosition struct {
	Object   core.CanvasObject
	Position geo.Position
}

// FindAllObjectsAtPositionMatching is like FindObjectAtPositionMatching but returns ALL matching
// objects at the position, ordered from deepest (innermost) to shallowest (outermost).
// This is used for scroll event propagation where the deepest scrollable gets first chance
// to handle the event, and if it doesn't consume it, the next parent scrollable tries.
func FindAllObjectsAtPositionMatching(mouse geo.Position, matches func(object core.CanvasObject) bool, overlay core.CanvasObject, roots ...core.CanvasObject) []ObjectAtPosition {
	var results []ObjectAtPosition

	wtStack := []geo.Matrix{geo.Identity()}

	beforeFunc := func(walked core.CanvasObject, _ geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
		if !walked.Visible() {
			return false
		}

		parentWT := wtStack[len(wtStack)-1]
		objPos := walked.Position()
		localT := geo.NewTranslation(objPos.X, objPos.Y)
		if t, ok := walked.(core.Transformable); ok {
			lt := t.Transform()
			if !lt.IsIdentity() {
				localT = localT.Multiply(lt)
			}
		}
		wt := parentWT.Multiply(localT)
		wtStack = append(wtStack, wt)

		if mouse.X < clipPos.X || mouse.Y < clipPos.Y {
			return false
		}
		if mouse.X >= clipPos.X+clipSize.Width || mouse.Y >= clipPos.Y+clipSize.Height {
			return false
		}

		w := walked.Size().Width
		h := walked.Size().Height

		if local, in := LocalHitPoint(wt, mouse, w, h); in && matches(walked) {
			results = append(results, ObjectAtPosition{Object: walked, Position: local})
		}
		return false
	}

	afterFunc := func(_ core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		if len(wtStack) > 1 {
			wtStack = wtStack[:len(wtStack)-1]
		}
	}

	if overlay != nil {
		WalkVisibleObjectTree(overlay, beforeFunc, afterFunc)
	}
	if overlay == nil || (len(results) == 0 && overlayPassesInput(overlay)) {
		// No overlay, or an input-pass-through overlay (tooltip, toast)
		// with no match at this position: scrollables beneath it must
		// still receive the event.
		for _, root := range roots {
			if root == nil {
				continue
			}
			wtStack = wtStack[:1]
			WalkVisibleObjectTree(root, beforeFunc, afterFunc)
			if len(results) > 0 {
				break
			}
		}
	}

	// Reverse so deepest (innermost) is first — the tree walk visits parents before children.
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results
}

// ScrollConsumer is an optional interface for Scrollable objects that can report
// whether a scroll event was consumed. If TryScrolled returns false, the scroll
// event may propagate to a parent scrollable.
type ScrollConsumer interface {
	event.Scrollable
	TryScrolled(ev *event.ScrollEvent) bool
}

// ReverseWalkVisibleObjectTree will walk an object tree in reverse order for all visible objects
// executing the passed functions following the following rules:
// - beforeChildren is called for the start obj before traversing its children
// - the obj's children are traversed by calling walkObjects on each of the visible items
// - afterChildren is called for the obj after traversing the obj's children
// The walk can be aborted by returning true in one of the functions:
//   - if beforeChildren returns true, further traversing is stopped immediately, the after function
//     will not be called for the obj where the walk stopped, however, it will be called for all its
//     parents
func ReverseWalkVisibleObjectTree(
	obj core.CanvasObject,
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
) bool {
	clipSize := geo.NewSize(math.MaxInt32, math.MaxInt32)
	return walkObjectTree(obj, true, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, geo.Identity(), nil, nil, beforeChildren, afterChildren, true)
}

// WalkCompleteObjectTree will walk an object tree for all objects (ignoring visible state) executing the passed
// functions following the following rules:
// - beforeChildren is called for the start obj before traversing its children
// - the obj's children are traversed by calling walkObjects on each of the items
// - afterChildren is called for the obj after traversing the obj's children
// The walk can be aborted by returning true in one of the functions:
//   - if beforeChildren returns true, further traversing is stopped immediately, the after function
//     will not be called for the obj where the walk stopped, however, it will be called for all its
//     parents
func WalkCompleteObjectTree(
	obj core.CanvasObject,
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
) bool {
	clipSize := geo.NewSize(math.MaxInt32, math.MaxInt32)
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, geo.Identity(), nil, nil, beforeChildren, afterChildren, false)
}

// WalkVisibleObjectTree will walk an object tree for all visible objects executing the passed functions following
// the following rules:
// - beforeChildren is called for the start obj before traversing its children
// - the obj's children are traversed by calling walkObjects on each of the visible items
// - afterChildren is called for the obj after traversing the obj's children
// The walk can be aborted by returning true in one of the functions:
//   - if beforeChildren returns true, further traversing is stopped immediately, the after function
//     will not be called for the obj where the walk stopped, however, it will be called for all its
//     parents
func WalkVisibleObjectTree(
	obj core.CanvasObject,
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
) bool {
	clipSize := geo.NewSize(math.MaxInt32, math.MaxInt32)
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, geo.Identity(), nil, nil, beforeChildren, afterChildren, true)
}

// WalkVisibleObjectTreeCulled walks as WalkVisibleObjectTree does, except that
// an object cull reports as drawing nothing visible is visited without its
// children: beforeChildren and afterChildren run for it, so a caller that
// mirrors the tree stays in step, but its children are neither built nor
// walked (#464). Culling stops below an object with a transform, whose
// children's positions no longer match the clip's.
func WalkVisibleObjectTreeCulled(
	obj core.CanvasObject,
	cull CullFunc,
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
) bool {
	clipSize := geo.NewSize(math.MaxInt32, math.MaxInt32)
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, geo.Identity(), cull, nil, beforeChildren, afterChildren, true)
}

// WalkVisibleSubtreeCulled walks as WalkVisibleObjectTreeCulled does, from
// obj inside a tree rather than from the tree's root: parent is obj's parent,
// offset its position on the canvas and (clipPos, clipSize) the clip obj sits
// in, as a walk from the root would have passed them. cull is nil below an
// object with a transform, as the walk from the root would have it; the
// caller passes nil when an ancestor of obj has one. xf is what the transforms
// above obj do to walk positions, the identity when there are none (see
// walkObjectTree). hidden, if set, is called for each hidden object the walk
// skips, with the object it belongs to.
func WalkVisibleSubtreeCulled(
	obj, parent core.CanvasObject,
	offset, clipPos geo.Position,
	clipSize geo.Size,
	xf geo.Matrix,
	cull CullFunc,
	hidden func(obj, parent core.CanvasObject),
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
) bool {
	return walkObjectTree(obj, false, parent, offset, clipPos, clipSize, xf, cull, hidden, beforeChildren, afterChildren, true)
}

// CullFunc reports whether obj, at the absolute position pos, draws nothing
// inside the clip (clipPos, clipSize) its parent applies.
type CullFunc func(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool

// walkObjectTree walks obj and its subtree. Positions are walk positions, the
// sum of the objects' positions. xf maps them to the canvas: the identity
// until the walk crosses an object with a transform, whose world transform is
// xf·Translate(pos)·Transform(), so below it xf is that times Translate(-pos).
// Clips are on the canvas: a clipping object's is the bounding box of where xf
// puts it (#997).
func walkObjectTree(
	obj core.CanvasObject,
	reverse bool,
	parent core.CanvasObject,
	offset, clipPos geo.Position,
	clipSize geo.Size,
	xf geo.Matrix,
	cull CullFunc,
	hidden func(obj, parent core.CanvasObject),
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
	requireVisible bool,
) bool {
	if obj == nil {
		return false
	}
	if requireVisible && !obj.Visible() {
		if hidden != nil {
			hidden(obj, parent)
		}
		return false
	}
	pos := obj.Position().Add(offset)

	if t, ok := obj.(core.Transformable); ok {
		if m := t.Transform(); !m.IsIdentity() {
			// Positions here and below no longer match the clip: the object is
			// drawn where its transform puts it, not at pos (#997).
			cull = nil
			xf = xf.Multiply(geo.NewTranslation(pos.X, pos.Y)).Multiply(m).Multiply(geo.NewTranslation(-pos.X, -pos.Y))
		}
	}
	culled := cull != nil && cull(obj, pos, clipPos, clipSize)

	// One renderer lookup serves both the children and the clip check (#492).
	var children []core.CanvasObject
	var r core.WidgetRenderer
	switch co := obj.(type) {
	case *bellui.Container:
		children = co.Objects
	case core.Widget:
		r, _ = cache.CachedRenderer(co)
		if !culled {
			if r == nil && requireVisible {
				r = cache.Renderer(co)
			}
			if r != nil {
				children = r.Objects()
			}
		}
	}
	if culled {
		children = nil
	}

	if isClip(obj, r) {
		boxPos, boxSize := pos, obj.Size()
		if !xf.IsIdentity() {
			minX, minY, maxX, maxY := xf.Multiply(geo.NewTranslation(pos.X, pos.Y)).AABB(boxSize.Width, boxSize.Height)
			boxPos, boxSize = geo.NewPos(minX, minY), geo.NewSize(maxX-minX, maxY-minY)
		}
		clipPos, clipSize = intersectClip(clipPos, clipSize, boxPos, boxSize)
	}

	if beforeChildren != nil {
		if beforeChildren(obj, pos, clipPos, clipSize) {
			return true
		}
	}

	cancelled := false
	followChild := func(child core.CanvasObject) bool {
		if walkObjectTree(child, reverse, obj, pos, clipPos, clipSize, xf, cull, hidden, beforeChildren, afterChildren, requireVisible) {
			cancelled = true
			return true
		}
		return false
	}
	if reverse {
		for i := len(children) - 1; i >= 0; i-- {
			if followChild(children[i]) {
				break
			}
		}
	} else {
		for _, child := range children {
			if followChild(child) {
				break
			}
		}
	}

	if afterChildren != nil {
		afterChildren(obj, pos, parent)
	}
	return cancelled
}

// intersectClip is the clip a clip-owner at (pos, size) gives its children
// inside the clip (clipPos, clipSize) it sits in: the overlap of the two, as
// the painters' clip stack computes it, so that what can be hit is what is
// drawn (#566). Under no clip yet, the owner's bounds stand alone; NoClipBounds
// starts at the origin and would cut off an owner above or left of it.
func intersectClip(clipPos geo.Position, clipSize geo.Size, pos geo.Position, size geo.Size) (geo.Position, geo.Size) {
	if clipSize == NoClipBounds {
		return pos, size
	}
	minX, minY := max(clipPos.X, pos.X), max(clipPos.Y, pos.Y)
	maxX := max(minX, min(clipPos.X+clipSize.Width, pos.X+size.Width))
	maxY := max(minY, min(clipPos.Y+clipSize.Height, pos.Y+size.Height))
	return geo.NewPos(minX, minY), geo.NewSize(maxX-minX, maxY-minY)
}

func IsClip(o core.CanvasObject) bool {
	return isClip(o, cachedRenderer(o))
}

// isClip is IsClip given the object's renderer, nil when it has none.
func isClip(o core.CanvasObject, r core.WidgetRenderer) bool {
	if _, scroll := o.(event.Scrollable); scroll {
		return true
	}
	if cont, ok := o.(*bellui.Container); ok && cont.ClipToBounds {
		return true
	}
	_, clip := r.(interface{ IsClip() })
	return clip
}

// cachedRenderer returns a widget's cached renderer, or nil.
func cachedRenderer(o core.CanvasObject) core.WidgetRenderer {
	w, isWid := o.(core.Widget)
	if !isWid {
		return nil
	}
	r, _ := cache.CachedRenderer(w)
	return r
}

// Clips is what an object establishes to clip its children. IsClip,
// IsRoundedClip and IsPathClip each look the renderer up; a paint walk asks
// all three for every object, twice, so ClipsOf answers them from one lookup
// (#492).
type Clips struct {
	Rect    bool    // clips to its bounds, as IsClip
	Rounded bool    // stencil-clips to rounded corners of Radius, as IsRoundedClip
	Radius  float32 // the rounded clip's corner radius
	Path    any     // the clip path, as IsPathClip, when non-nil
}

// ClipsOf reports the clips an object establishes for its children.
func ClipsOf(o core.CanvasObject) Clips {
	r := cachedRenderer(o)
	c := Clips{Rect: isClip(o, r)}
	if cont, ok := o.(*bellui.Container); ok {
		c.Rounded, c.Radius = containerRoundedClip(cont)
		return c
	}
	if r == nil {
		return c
	}
	if rc, ok := r.(interface{ RoundedClipCornerRadius() float32 }); ok {
		c.Radius = rc.RoundedClipCornerRadius()
		c.Rounded = c.Radius > 0
	}
	if pc, ok := r.(interface{ ClipPath() any }); ok {
		c.Path = pc.ClipPath()
	}
	return c
}

// IsRepaintBoundary checks if a canvas object's renderer implements the
// RepaintBoundary marker interface, indicating its subtree should be
// rendered into a cached FBO for dirty-region isolation.
func IsRepaintBoundary(o core.CanvasObject) bool {
	boundary, _ := RepaintBoundaryStatus(o)
	return boundary
}

// RepaintBoundaryStatus is IsRepaintBoundary, and also reports whether the
// answer is settled: false for a widget whose renderer does not exist yet,
// which may turn out to be a boundary once it does. A walk can keep a settled
// answer rather than look the renderer up again on every visit (#487).
func RepaintBoundaryStatus(o core.CanvasObject) (boundary, settled bool) {
	w, isWid := o.(core.Widget)
	if !isWid {
		return false, true
	}
	r, rendered := cache.CachedRenderer(w)
	if !rendered {
		return false, false
	}
	_, isBoundary := r.(interface{ IsRepaintBoundary() })
	return isBoundary, true
}

// containerRoundedClip reports a container's rounded clip: its
// ClipCornerRadius, when it clips to its bounds.
func containerRoundedClip(c *bellui.Container) (bool, float32) {
	if c.ClipToBounds && c.ClipCornerRadius > 0 {
		return true, c.ClipCornerRadius
	}
	return false, 0
}

// IsRoundedClip checks if a canvas object's renderer implements rounded-corner
// stencil clipping. Returns the corner radius if so.
func IsRoundedClip(o core.CanvasObject) (bool, float32) {
	if cont, ok := o.(*bellui.Container); ok {
		return containerRoundedClip(cont)
	}
	w, isWid := o.(core.Widget)
	if !isWid {
		return false, 0
	}
	r, rendered := cache.CachedRenderer(w)
	if !rendered {
		return false, 0
	}
	if rc, ok := r.(interface{ RoundedClipCornerRadius() float32 }); ok {
		radius := rc.RoundedClipCornerRadius()
		return radius > 0, radius
	}
	return false, 0
}

// IsPathClip checks if a canvas object's renderer implements path-based
// stencil clipping. Returns the clip path (as any to avoid import cycles)
// and true if the renderer supports path clipping.
func IsPathClip(o core.CanvasObject) (any, bool) {
	w, isWid := o.(core.Widget)
	if !isWid {
		return nil, false
	}
	r, rendered := cache.CachedRenderer(w)
	if !rendered {
		return nil, false
	}
	type pathClipper interface{ ClipPath() any }
	if pc, ok := r.(pathClipper); ok {
		path := pc.ClipPath()
		if path != nil {
			return path, true
		}
	}
	return nil, false
}

// DestroyRenderers destroys the cached renderers of o and of every widget
// under it, hidden ones included. It never creates a renderer: the walk reads
// only cached ones, so a widget without a renderer is skipped, and so is
// everything only its renderer would have held.
//
// A widget calls it on objects it built and drops without showing, such as an
// item template it only measures: no window holds them, so nothing else would
// release their renderers before the cache expiry (#722).
func DestroyRenderers(o core.CanvasObject) {
	if o == nil {
		return
	}
	// A measured template holds a few widgets: the buffer keeps the walk from
	// allocating on the Refresh path that measures one.
	var buf [8]core.Widget
	widgets := buf[:0]
	WalkCompleteObjectTree(o, nil, func(obj core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		if w, ok := obj.(core.Widget); ok {
			widgets = append(widgets, w)
		}
	})
	// Destroy only after the walk, so that no Destroy can change what the walk
	// reads.
	for _, w := range widgets {
		cache.DestroyRenderer(w)
	}
}
