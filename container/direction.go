package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/widget"
)

// DirectionScope is a container that establishes an ambient user-interface
// direction for its content, overriding the process-wide
// [github.com/belluikit/bellui.SetLayoutDirection] for everything inside it.
// It lets a left-to-right subtree — for example an LTR settings dialog — flow
// naturally inside an otherwise right-to-left application (and vice versa).
//
// The scope governs layout mirroring, leading/trailing alignment, focus order
// and the base writing direction used while the subtree is measured and laid
// out. As with the process-global setting, mixed-direction text within a single
// run is still controlled per run by [github.com/belluikit/bellui.TextStyle.Direction].
//
// Passing [github.com/belluikit/bellui.DirectionAuto] makes the scope inherit
// the enclosing direction, so it is a no-op wrapper; use [DirectionLTR] or
// [DirectionRTL] to force a direction. Scopes nest, with the innermost winning.
//
// Since: 1.0
type DirectionScope struct {
	widget.BaseWidget

	// Content is the object whose subtree adopts Direction.
	Content core.CanvasObject
	// Direction is the ambient direction applied to Content. Changing it at
	// runtime requires a call to [DirectionScope.Refresh] (use
	// [DirectionScope.SetDirection], which does this for you).
	Direction bellui.Direction

	holder *bellui.Container
}

// NewDirectionScope returns a [DirectionScope] that lays out and paints obj
// under the given direction, regardless of the process-wide setting. Pass
// [github.com/belluikit/bellui.DirectionLTR] or [DirectionRTL] to force a
// direction; [DirectionAuto] inherits the enclosing direction.
//
// Since: 1.0
func NewDirectionScope(obj core.CanvasObject, dir bellui.Direction) *DirectionScope {
	d := &DirectionScope{Content: obj, Direction: dir, holder: NewStack(obj)}
	d.ExtendBaseWidget(d)

	// obj may have been measured/laid-out under the previous (global) direction
	// before being wrapped, so recompute it within the new scope.
	uidir.Push(dir)
	obj.Refresh()
	uidir.Pop(dir)
	return d
}

// CreateRenderer is an internal method.
func (d *DirectionScope) CreateRenderer() core.WidgetRenderer {
	uidir.NoteScope() // from now on a direction lookup must walk for scopes
	return &directionScopeRenderer{scope: d, objs: []core.CanvasObject{d.holder}}
}

// ScopedDirection reports the ambient direction this scope applies to its
// subtree. The driver paint walks read it structurally (via an interface, to
// avoid importing this package) so they can push the scope onto the per-goroutine
// uidir stack while painting the subtree — the paint-time mirror of the layout
// hook in internal/driver/common.Canvas.EnsureMinSize.
func (d *DirectionScope) ScopedDirection() bellui.Direction {
	return d.Direction
}

// SetDirection updates the ambient direction applied to the content and
// re-lays-out the subtree. It is a no-op when the direction is unchanged.
//
// Since: 1.0
func (d *DirectionScope) SetDirection(dir bellui.Direction) {
	if d.Direction == dir {
		return
	}
	d.Direction = dir
	d.Refresh()
}

// Refresh re-applies the scope to its content. Call it after adding items to the
// content container (or any sub-container) so the new items adopt the direction,
// mirroring [ThemeOverride.Refresh].
func (d *DirectionScope) Refresh() {
	if d.holder.Objects[0] != d.Content {
		d.holder.Objects[0] = d.Content
		d.holder.Refresh()
	}

	// Refresh runs outside a layout walk, so push the direction the content
	// takes here, or its re-layout would read the global one (#696).
	dir := d.contentDirection()
	uidir.Push(dir)
	defer uidir.Pop(dir)

	d.Content.Refresh()
	d.BaseWidget.Refresh()
}

// DirectionOf returns the layout direction that applies to obj where it sits,
// always [github.com/belluikit/bellui.DirectionLTR] or
// [github.com/belluikit/bellui.DirectionRTL]: the
// innermost [DirectionScope] enclosing obj, else the direction of obj's canvas
// (an optional SetDirection on the window canvas), else the process-wide
// direction ([github.com/belluikit/bellui.CurrentLayoutDirection], or the
// locale's when that is automatic). An object not on any canvas gets the
// process-wide direction. A scope's own direction applies to its content, so
// for a scope this is the direction it sits in.
//
// [github.com/belluikit/bellui/lang.IsRTLDirection] gives the same answer only
// inside a layout or paint walk, which pushes the scopes it crosses. Widgets
// call DirectionOf instead from code that runs outside one: an event handler
// (a swipe, a tap's hit-test, an arrow key), or a Refresh triggered by a timer
// or a data binding that re-lays-out the widget. Outside a walk it looks obj up
// in its canvas' tree, a cost that grows with the tree; call it once per event
// or Refresh, not per frame.
//
// Since: 1.0
func DirectionOf(obj core.CanvasObject) bellui.Direction {
	if d, ok := driver.DirectionOf(obj); ok {
		return d
	}
	if lang.IsRTLDirection() {
		return bellui.DirectionRTL
	}
	return bellui.DirectionLTR
}

// contentDirection is the direction the scope's content takes: its own, or
// the one it sits in when it is automatic.
func (d *DirectionScope) contentDirection() bellui.Direction {
	if d.Direction != bellui.DirectionAuto {
		return d.Direction
	}
	return DirectionOf(d)
}

type directionScopeRenderer struct {
	scope *DirectionScope

	objs []core.CanvasObject
}

func (r *directionScopeRenderer) Destroy() {
}

func (r *directionScopeRenderer) Layout(s geo.Size) {
	uidir.Push(r.scope.Direction)
	defer uidir.Pop(r.scope.Direction)

	r.scope.holder.Resize(s)
}

func (r *directionScopeRenderer) MinSize() geo.Size {
	uidir.Push(r.scope.Direction)
	defer uidir.Pop(r.scope.Direction)

	return r.scope.Content.MinSize()
}

func (r *directionScopeRenderer) Objects() []core.CanvasObject {
	return r.objs
}

func (r *directionScopeRenderer) Refresh() {
}
