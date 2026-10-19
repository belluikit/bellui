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
// uidir stack when obj is a direction scope, so paint-time text base direction and
// leading/trailing alignment inside the subtree resolve against the scope rather
// than the process-global direction. It is the paint-time mirror of the layout
// hook in common.Canvas.EnsureMinSize and must be balanced by PopDirectionScope
// in the matching after-children callback. A non-scope obj is a no-op.
func PushDirectionScope(obj core.CanvasObject) {
	if ds, ok := obj.(directionScoped); ok {
		uidir.Push(ds.ScopedDirection())
	}
}

// PopDirectionScope balances PushDirectionScope for obj. A non-scope obj is a
// no-op.
func PopDirectionScope(obj core.CanvasObject) {
	if ds, ok := obj.(directionScoped); ok {
		uidir.Pop(ds.ScopedDirection())
	}
}

// AbsolutePositionForObject returns the absolute position of an object in a set of object trees.
// If the object is not part of any of the trees, the position (0,0) is returned.
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

// NoClipBounds is the open clip rectangle walkObjectTree starts with: an object
// reached with this clip size has crossed no clipping ancestor.
var NoClipBounds = geo.NewSize(math.MaxInt32, math.MaxInt32)

// ClipBoundsForObject returns the bounds of the nearest clipping ancestor of
// object within the given trees, in absolute canvas coordinates. ok is false
// when object is not found, or when no ancestor clips it (it is drawn against
// the full canvas). A clipping ancestor is one [IsClip] reports — a
// [event.Scrollable], a [bellui.Container] with ClipToBounds, or a widget whose
// renderer clips. It reuses the same clip tracking the hit-test walk uses, so a
// caller can size content (e.g. a popover) to the region that will actually be
// visible rather than overflowing into a clipped area.
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

	beforeFunc := func(walked core.CanvasObject, pos geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
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

		if wt.B == 0 && wt.C == 0 {
			// Pure translation (possibly scale): use fast AABB check.
			if mouse.X < pos.X || mouse.Y < pos.Y {
				return false
			}
			if mouse.X >= pos.X+w || mouse.Y >= pos.Y+h {
				return false
			}
			if matchesHitTarget(walked, matches) {
				found = walked
				foundPos = geo.NewPos(mouse.X-pos.X, mouse.Y-pos.Y)
			}
		} else {
			// Non-trivial transform: inverse-transform mouse into local space.
			invWT, ok := wt.Inverse()
			if !ok {
				return false
			}
			localX, localY := invWT.TransformPoint(mouse.X, mouse.Y)
			if localX < 0 || localY < 0 || localX >= w || localY >= h {
				return false
			}
			if matchesHitTarget(walked, matches) {
				found = walked
				foundPos = geo.NewPos(localX, localY)
			}
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

	beforeFunc := func(walked core.CanvasObject, pos geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
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

		if wt.B == 0 && wt.C == 0 {
			if mouse.X < pos.X || mouse.Y < pos.Y {
				return false
			}
			if mouse.X >= pos.X+w || mouse.Y >= pos.Y+h {
				return false
			}
			if matches(walked) {
				results = append(results, ObjectAtPosition{
					Object:   walked,
					Position: geo.NewPos(mouse.X-pos.X, mouse.Y-pos.Y),
				})
			}
		} else {
			invWT, ok := wt.Inverse()
			if !ok {
				return false
			}
			localX, localY := invWT.TransformPoint(mouse.X, mouse.Y)
			if localX < 0 || localY < 0 || localX >= w || localY >= h {
				return false
			}
			if matches(walked) {
				results = append(results, ObjectAtPosition{
					Object:   walked,
					Position: geo.NewPos(localX, localY),
				})
			}
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
	return walkObjectTree(obj, true, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, nil, beforeChildren, afterChildren, true)
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
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, nil, beforeChildren, afterChildren, false)
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
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, nil, beforeChildren, afterChildren, true)
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
	return walkObjectTree(obj, false, nil, geo.NewPos(0, 0), geo.NewPos(0, 0), clipSize, cull, beforeChildren, afterChildren, true)
}

// CullFunc reports whether obj, at the absolute position pos, draws nothing
// inside the clip (clipPos, clipSize) its parent applies.
type CullFunc func(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool

func walkObjectTree(
	obj core.CanvasObject,
	reverse bool,
	parent core.CanvasObject,
	offset, clipPos geo.Position,
	clipSize geo.Size,
	cull CullFunc,
	beforeChildren func(core.CanvasObject, geo.Position, geo.Position, geo.Size) bool,
	afterChildren func(core.CanvasObject, geo.Position, core.CanvasObject),
	requireVisible bool,
) bool {
	if obj == nil {
		return false
	}
	if requireVisible && !obj.Visible() {
		return false
	}
	pos := obj.Position().Add(offset)

	culled := cull != nil && cull(obj, pos, clipPos, clipSize)
	if t, ok := obj.(core.Transformable); ok && !t.Transform().IsIdentity() {
		cull = nil // positions below no longer match the clip
	}

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
		clipPos = pos
		clipSize = obj.Size()
	}

	if beforeChildren != nil {
		if beforeChildren(obj, pos, clipPos, clipSize) {
			return true
		}
	}

	cancelled := false
	followChild := func(child core.CanvasObject) bool {
		if walkObjectTree(child, reverse, obj, pos, clipPos, clipSize, cull, beforeChildren, afterChildren, requireVisible) {
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
	w, isWid := o.(core.Widget)
	if !isWid {
		return false
	}
	r, rendered := cache.CachedRenderer(w)
	if !rendered {
		return false
	}
	_, isBoundary := r.(interface{ IsRepaintBoundary() })
	return isBoundary
}

// IsRoundedClip checks if a canvas object's renderer implements rounded-corner
// stencil clipping. Returns the corner radius if so.
func IsRoundedClip(o core.CanvasObject) (bool, float32) {
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
