package common

import (
	"image/color"
	"reflect"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/uidir"
)

// SizeableCanvas defines a canvas with size related functions.
type SizeableCanvas interface {
	core.Canvas
	Resize(geo.Size)
	MinSize() geo.Size
}

// Canvas defines common canvas implementation.
type Canvas struct {
	OnFocus   func(obj event.Focusable)
	OnUnfocus func()

	impl SizeableCanvas

	contentFocusMgr *app.FocusManager
	menuFocusMgr    *app.FocusManager
	overlays        overlayStack

	shortcut bellui.ShortcutHandler

	painter gpu.Painter

	// Any object that requests to enter to the refresh queue should
	// not be omitted as it is always a rendering task's decision
	// for skipping frames or drawing calls.
	//
	// If an object failed to ender the refresh queue, the object may
	// disappear or blink from the view at any frames. As of this reason,
	// the refreshQueue is an unbounded queue which is able to cache
	// arbitrary number of core.CanvasObject for the rendering.
	refreshQueue deduplicatedObjectQueue
	dirty        bool

	mWindowHeadTree, contentTree, menuTree *renderCacheTree

	// nodeMap provides O(1) lookup from CanvasObject to its RenderCacheNode.
	// Used for dirty propagation to find the nearest ancestor RepaintBoundary.
	nodeMap async.Map[core.CanvasObject, *RenderCacheNode]

	// dirtyRegion tracks the bounding box of all refreshed objects since the
	// last paint. When set, the paint walk can use scissor testing to restrict
	// GL operations to this region.
	dirtyRegion dirtyRect

	// refreshedObjs is the set of objects pulled from the refresh queue in the
	// most recent FreeDirtyTextures pass, used by PartialDirtyRegion to union
	// the bounds of content-changed (but not necessarily moved) objects. Only
	// populated when the partialrepaint build tag is set.
	refreshedObjs map[core.CanvasObject]struct{}

	// spatialIdx provides O(1) broadphase hit testing. Built during
	// WalkTrees and read during mouse/touch event processing.
	spatialIdx spatialIndexHolder

	// layoutDir is the per-canvas user-interface direction. The zero value
	// (DirectionAuto) defers to the process-global setting; an explicit value
	// overrides it for this canvas' whole content, so a single window can flow
	// LTR while the rest of an RTL app does not. It is pushed onto the ambient
	// direction stack around this canvas' layout and paint.
	layoutDir bellui.Direction
}

// AddShortcut adds a shortcut to the canvas.
func (c *Canvas) AddShortcut(shortcut core.Shortcut, handler func(shortcut core.Shortcut)) {
	c.shortcut.AddShortcut(shortcut, handler)
}

func (c *Canvas) DrawDebugOverlay(obj core.CanvasObject, pos geo.Position, size geo.Size) {
	switch obj.(type) {
	case core.Widget:
		r := canvas.NewRectangle(color.Transparent)
		r.StrokeColor = color.NRGBA{R: 0xcc, G: 0x33, B: 0x33, A: 0xff}
		r.StrokeWidth = 1
		r.Resize(obj.Size())
		c.Painter().Paint(r, pos, size)

		t := canvas.NewText(reflect.ValueOf(obj).Elem().Type().Name(), r.StrokeColor)
		t.TextSize = 10
		c.Painter().Paint(t, pos.AddXY(2, 2), size)
	case *bellui.Container:
		r := canvas.NewRectangle(color.Transparent)
		r.StrokeColor = color.NRGBA{R: 0x33, G: 0x33, B: 0xcc, A: 0xff}
		r.StrokeWidth = 1
		r.Resize(obj.Size())
		c.Painter().Paint(r, pos, size)
	}
}

// EnsureMinSize ensure canvas min size.
//
// This function uses lock.
func (c *Canvas) EnsureMinSize() bool {
	if c.impl.Content() == nil {
		return false
	}
	windowNeedsMinSizeUpdate := false
	csize := c.impl.Size()
	min := c.impl.MinSize()

	setup := func(node *RenderCacheNode, pos geo.Position) {
		if !node.obj.Visible() {
			return
		}
		if th, ok := node.Obj().(*container.ThemeOverride); ok {
			theme.PushRenderingTheme(th.Theme)
		}
		if ds, ok := node.Obj().(*container.DirectionScope); ok {
			uidir.Push(ds.Direction)
		}
	}
	ensureMinSize := func(node *RenderCacheNode, pos geo.Position) {
		obj := node.obj
		cache.SetCanvasForObject(obj, c.impl, func() {
			if img, ok := obj.(*canvas.Image); ok {
				img.Refresh() // this may now have a different texScale
			}
		})

		if node.minSizeChanged {
			c.updateLayout(obj)
			node.minSizeChanged = false
		}

		if !obj.Visible() {
			return
		}
		minSize := obj.MinSize()

		minSizeChanged := node.minSize != minSize
		if minSizeChanged {
			node.minSize = minSize
			if node.parent != nil {
				// Flag the parent for re-layout when the post-order walk
				// reaches it. Per-node, so changes reported from several
				// subtrees in the same walk all apply (#1039).
				node.parent.minSizeChanged = true
			} else {
				windowNeedsMinSizeUpdate = true
				size := obj.Size()
				expectedSize := minSize.Max(size)
				if expectedSize != size && size != csize {
					obj.Resize(expectedSize)
				} else {
					c.updateLayout(obj)
				}
			}
		}

		if _, ok := node.Obj().(*container.ThemeOverride); ok {
			theme.PopRenderingTheme()
		}
		if ds, ok := node.Obj().(*container.DirectionScope); ok {
			uidir.Pop(ds.Direction)
		}
	}
	// Restore the stack to its pre-walk depth even if a layout panic unwinds
	// past the per-node DirectionScope Pops in the visitor below.
	defer uidir.Restore(uidir.Mark())
	uidir.Push(c.layoutDir)
	c.WalkTrees(setup, ensureMinSize)
	uidir.Pop(c.layoutDir)

	shouldResize := windowNeedsMinSizeUpdate && (csize.Width < min.Width || csize.Height < min.Height)
	if shouldResize {
		c.impl.Resize(csize.Max(min))
	}
	return windowNeedsMinSizeUpdate
}

// Focus makes the provided item focused.
func (c *Canvas) Focus(obj event.Focusable) {
	focusMgr := c.focusManager()
	if focusMgr != nil && focusMgr.Focus(obj) { // fast path – probably >99.9% of all cases
		if c.OnFocus != nil {
			c.OnFocus(obj)
		}
		return
	}

	focusMgrs := append([]*app.FocusManager{c.contentFocusMgr, c.menuFocusMgr}, c.overlays.ListFocusManagers()...)

	for _, mgr := range focusMgrs {
		if mgr == nil {
			continue
		}
		if focusMgr != mgr && mgr.Focus(obj) {
			if c.OnFocus != nil {
				c.OnFocus(obj)
			}
			return
		}
	}

	bellui.LogError("Failed to focus object which is not part of the canvas’ content, menu or overlays.", nil)
}

// Focused returns the current focused object.
func (c *Canvas) Focused() event.Focusable {
	mgr := c.focusManager()
	if mgr == nil {
		return nil
	}
	return mgr.Focused()
}

// FocusGained signals to the manager that its content got focus.
// Valid only on Desktop.
func (c *Canvas) FocusGained() {
	mgr := c.focusManager()
	if mgr == nil {
		return
	}
	mgr.FocusGained()
}

// FocusLost signals to the manager that its content lost focus.
// Valid only on Desktop.
func (c *Canvas) FocusLost() {
	mgr := c.focusManager()
	if mgr == nil {
		return
	}
	mgr.FocusLost()
}

// FocusNext focuses the next focusable item.
func (c *Canvas) FocusNext() {
	mgr := c.focusManager()
	if mgr == nil {
		return
	}
	mgr.FocusNext()
	// Fire the driver focus hook like Canvas.Focus does: keyboard traversal is
	// a focus change like any other, and skipping the hook left every OnFocus
	// consumer stale on a Tab switch — the mobile driver never rebound the IME
	// mirror to the new field (so one Gboard session spanned both, #966), the
	// desktop driver never toggled IME mode, wasm never re-aimed its hidden
	// input.
	if c.OnFocus != nil {
		c.OnFocus(mgr.Focused())
	}
}

// FocusPrevious focuses the previous focusable item.
func (c *Canvas) FocusPrevious() {
	mgr := c.focusManager()
	if mgr == nil {
		return
	}
	mgr.FocusPrevious()
	// See FocusNext.
	if c.OnFocus != nil {
		c.OnFocus(mgr.Focused())
	}
}

// FreeDirtyTextures frees dirty textures and returns the number of freed textures.
func (c *Canvas) FreeDirtyTextures() uint64 {
	if c.painter == nil {
		return 0
	}

	bp, hasBP := c.painter.(gpu.BoundaryPainter)

	c.dirtyRegion.Reset()
	if build.PartialRepaint {
		c.refreshedObjs = make(map[core.CanvasObject]struct{}, c.refreshQueue.Len())
	}
	objectsToFree := c.refreshQueue.Len()
	for object := c.refreshQueue.Out(); object != nil; object = c.refreshQueue.Out() {
		c.freeObject(object)

		// Track dirty region for scissor culling.
		if node, ok := c.nodeMap.Load(object); ok {
			wt := node.worldTransform
			c.dirtyRegion.Union(geo.NewPos(wt.TX, wt.TY), object.Size())
		}
		if build.PartialRepaint {
			c.refreshedObjs[object] = struct{}{}
		}

		// Propagate dirty state to the nearest ancestor RepaintBoundary.
		if hasBP {
			c.markBoundaryDirtyFor(bp, object)
		}
	}

	cache.RangeExpiredTexturesFor(c.impl, c.painter.Free)
	return objectsToFree
}

// markBoundaryDirtyFor walks up the render cache tree from obj to find the
// nearest RepaintBoundary ancestor and marks its FBO as needing re-render.
func (c *Canvas) markBoundaryDirtyFor(bp gpu.BoundaryPainter, obj core.CanvasObject) {
	node, ok := c.nodeMap.Load(obj)
	if !ok {
		return
	}
	for n := node; n != nil; n = n.Parent() {
		if n.IsBoundary() {
			bp.MarkBoundaryDirty(n.Obj())
			return
		}
	}
}

// Initialize initializes the canvas.
func (c *Canvas) Initialize(impl SizeableCanvas, onOverlayChanged func()) {
	c.impl = impl
	c.refreshQueue.queue = async.NewCanvasObjectQueue()
	c.overlays.OverlayStack = internal.OverlayStack{
		OnChange: onOverlayChanged,
		Canvas:   impl,
	}
}

// ObjectTrees return canvas object trees.
//
// This function uses lock.
func (c *Canvas) ObjectTrees() []core.CanvasObject {
	var content, menu core.CanvasObject
	if c.contentTree != nil && c.contentTree.root != nil {
		content = c.contentTree.root.obj
	}
	if c.menuTree != nil && c.menuTree.root != nil {
		menu = c.menuTree.root.obj
	}
	trees := make([]core.CanvasObject, 0, len(c.Overlays().List())+2)
	trees = append(trees, content)
	if menu != nil {
		trees = append(trees, menu)
	}
	trees = append(trees, c.Overlays().List()...)
	return trees
}

// Overlays returns the overlay stack.
func (c *Canvas) Overlays() core.OverlayStack {
	return &c.overlays
}

// DirtyRegion returns the accumulated dirty region from the last
// FreeDirtyTextures call. If set is false, the region is undefined
// and a full repaint is needed.
func (c *Canvas) DirtyRegion() (pos geo.Position, size geo.Size, set bool) {
	return c.dirtyRegion.pos, c.dirtyRegion.size, c.dirtyRegion.set
}

// RectsOverlap reports whether two axis-aligned rectangles intersect. Used by
// the drivers to cull objects that fall outside the partial-repaint region.
func RectsOverlap(aPos geo.Position, aSize geo.Size, bPos geo.Position, bSize geo.Size) bool {
	return aPos.X < bPos.X+bSize.Width &&
		aPos.X+aSize.Width > bPos.X &&
		aPos.Y < bPos.Y+bSize.Height &&
		aPos.Y+aSize.Height > bPos.Y
}

// RegionCoversCanvas reports whether a dirty region is large enough that a
// partial repaint saves little, so a plain full clear is preferable.
func RegionCoversCanvas(region, canvas geo.Size) bool {
	return region.Width*region.Height >= 0.8*canvas.Width*canvas.Height
}

// partialRepaintPad is the anti-aliasing safety margin (in canvas/DP units)
// added around each dirty rectangle so sub-pixel coverage at object edges is
// fully cleared and redrawn rather than leaving a stale fringe.
const partialRepaintPad = 2

func padRect(pos geo.Position, size geo.Size) (geo.Position, geo.Size) {
	return geo.NewPos(pos.X-partialRepaintPad, pos.Y-partialRepaintPad),
		geo.NewSize(size.Width+2*partialRepaintPad, size.Height+2*partialRepaintPad)
}

// PartialDirtyRegion runs a draw-free walk that recomputes world transforms and
// returns the region of the canvas that must be cleared and redrawn this frame:
// the union, over every object that was refreshed or has moved/resized since the
// previous paint, of both its previous and its current world-space bounds (so a
// move clears where it left as well as where it lands), padded for anti-aliased
// edges. Refreshed objects additionally contribute their whole subtree so
// overflow children (shadows, ripples, focus rings) are covered. Each node's
// recorded bounds are advanced to the current frame. set is false when nothing
// changed, in which case the caller can skip the redraw entirely.
//
// Only meaningful under the partialrepaint build tag; the per-node bookkeeping
// it relies on is otherwise never populated.
func (c *Canvas) PartialDirtyRegion() (pos geo.Position, size geo.Size, set bool) {
	var region dirtyRect
	refreshed := c.refreshedObjs

	c.WalkTrees(func(node *RenderCacheNode, _ geo.Position) {
		s := node.obj.Size()
		minX, minY, maxX, maxY := node.worldTransform.AABB(s.Width, s.Height)
		newPos := geo.NewPos(minX, minY)
		newSize := geo.NewSize(maxX-minX, maxY-minY)

		_, isRefreshed := refreshed[node.obj]
		moved := !node.lastPaintSet || node.lastPaintPos != newPos || node.lastPaintSize != newSize

		if isRefreshed || moved {
			region.Union(padRect(newPos, newSize)) // where it is now
			if node.lastPaintSet {
				region.Union(padRect(node.lastPaintPos, node.lastPaintSize)) // where it was
			}
		}
		if isRefreshed {
			// A content refresh can repaint descendants that overflow this node's
			// own bounds (shadows, focus rings, selection); cover the subtree.
			c.unionSubtreeBounds(node, &region)
		}

		node.lastPaintPos = newPos
		node.lastPaintSize = newSize
		node.lastPaintSet = true
	}, nil)

	c.refreshedObjs = nil
	return region.pos, region.size, region.set
}

// unionSubtreeBounds expands region to include the current world-space bounds of
// every descendant of node (padded). Used so a refreshed parent also clears the
// overflow drawn by its children.
func (c *Canvas) unionSubtreeBounds(node *RenderCacheNode, region *dirtyRect) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		s := child.obj.Size()
		minX, minY, maxX, maxY := child.worldTransform.AABB(s.Width, s.Height)
		region.Union(padRect(geo.NewPos(minX, minY), geo.NewSize(maxX-minX, maxY-minY)))
		c.unionSubtreeBounds(child, region)
	}
}

// NodeFor returns the RenderCacheNode for the given object, if it exists.
func (c *Canvas) NodeFor(obj core.CanvasObject) (*RenderCacheNode, bool) {
	return c.nodeMap.Load(obj)
}

// Painter returns the canvas painter.
func (c *Canvas) Painter() gpu.Painter {
	return c.painter
}

// Refresh refreshes a canvas object.
func (c *Canvas) Refresh(obj core.CanvasObject) {
	c.refreshQueue.In(obj)
	async.EnsureMain(c.SetDirty)
}

// RemoveShortcut removes a shortcut from the canvas.
func (c *Canvas) RemoveShortcut(shortcut core.Shortcut) {
	c.shortcut.RemoveShortcut(shortcut)
}

// Direction returns the per-canvas user-interface direction set by
// [Canvas.SetDirection]. The default [github.com/belluikit/bellui.DirectionAuto]
// defers to the process-global [github.com/belluikit/bellui.SetLayoutDirection].
func (c *Canvas) Direction() bellui.Direction {
	return c.layoutDir
}

// SetDirection overrides the user-interface direction for this canvas' whole
// content, regardless of the process-global setting, so one window can flow
// left-to-right inside a right-to-left application (or vice versa). Passing
// [github.com/belluikit/bellui.DirectionAuto] restores the global setting.
// The content is re-laid-out and repainted to apply the change.
func (c *Canvas) SetDirection(d bellui.Direction) {
	if c.layoutDir == d {
		return
	}
	c.layoutDir = d
	if c.impl != nil {
		if content := c.impl.Content(); content != nil {
			content.Refresh()
		}
	}
	async.EnsureMain(c.SetDirty)
}

// SetContentTreeAndFocusMgr sets content tree and focus manager.
//
// This function does not use the canvas lock.
func (c *Canvas) SetContentTreeAndFocusMgr(content core.CanvasObject) {
	c.contentTree = &renderCacheTree{root: &RenderCacheNode{obj: content}}

	newFocusMgr := app.NewFocusManager(content)
	if c.contentFocusMgr != nil {
		focused := c.contentFocusMgr.Focused()
		if focused != nil {
			newFocusMgr.Focus(focused) // Focus old object if possible.
		}
	}
	c.contentFocusMgr = newFocusMgr
}

// CheckDirtyAndClear returns true if the canvas is dirty and
// clears the dirty state atomically.
func (c *Canvas) CheckDirtyAndClear() bool {
	wasDirty := c.dirty
	c.dirty = false
	return wasDirty
}

// SetDirty sets canvas dirty flag atomically.
func (c *Canvas) SetDirty() {
	c.dirty = true
}

// IsDirty reports whether the canvas has changes that have not been painted
// yet, without clearing the flag. While it is true the spatial index has not
// been rebuilt since the last change, so hit testing must not rely on it.
func (c *Canvas) IsDirty() bool {
	return c.dirty
}

// SetMenuTreeAndFocusMgr sets menu tree and focus manager.
//
// This function does not use the canvas lock.
func (c *Canvas) SetMenuTreeAndFocusMgr(menu core.CanvasObject) {
	c.menuTree = &renderCacheTree{root: &RenderCacheNode{obj: menu}}
	if menu != nil {
		c.menuFocusMgr = app.NewFocusManager(menu)
	} else {
		c.menuFocusMgr = nil
	}
}

// SetMobileWindowHeadTree sets window head tree.
//
// This function does not use the canvas lock.
func (c *Canvas) SetMobileWindowHeadTree(head core.CanvasObject) {
	c.mWindowHeadTree = &renderCacheTree{root: &RenderCacheNode{obj: head}}
}

// SetPainter sets the canvas painter.
func (c *Canvas) SetPainter(p gpu.Painter) {
	c.painter = p
}

// TypedShortcut handle the registered shortcut.
func (c *Canvas) TypedShortcut(shortcut core.Shortcut) {
	c.shortcut.TypedShortcut(shortcut)
}

// Unfocus unfocuses all the objects in the canvas.
func (c *Canvas) Unfocus() {
	mgr := c.focusManager()
	if mgr == nil {
		return
	}
	if mgr.Focus(nil) && c.OnUnfocus != nil {
		c.OnUnfocus()
	}
}

// WalkTrees walks over all trees (content, menu, overlays) and rebuilds the spatial index.
func (c *Canvas) WalkTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.WalkContentTrees(beforeChildren, afterChildren)
	c.WalkOverlayTrees(beforeChildren, afterChildren)
	c.RebuildSpatialIndex()
}

// WalkContentTrees walks content, window head, and menu trees (not overlays).
func (c *Canvas) WalkContentTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.walkTree(c.contentTree, beforeChildren, afterChildren)
	if c.mWindowHeadTree != nil && c.mWindowHeadTree.root.obj != nil {
		c.walkTree(c.mWindowHeadTree, beforeChildren, afterChildren)
	}
	if c.menuTree != nil && c.menuTree.root.obj != nil {
		c.walkTree(c.menuTree, beforeChildren, afterChildren)
	}
}

// WalkOverlayTrees walks overlay trees only.
func (c *Canvas) WalkOverlayTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	for _, tree := range c.overlays.renderCaches {
		if tree != nil {
			c.walkTree(tree, beforeChildren, afterChildren)
		}
	}
}

// SpatialIndex returns the current spatial index for broadphase hit testing.
// May return nil if the index has not been built yet.
func (c *Canvas) SpatialIndex() *SpatialIndex {
	return c.spatialIdx.Load()
}

// RebuildSpatialIndex walks the render cache trees and populates a new
// SpatialIndex from the cached world transforms and clip bounds.
func (c *Canvas) RebuildSpatialIndex() {
	size := c.impl.Size()
	if size.Width <= 0 || size.Height <= 0 {
		c.spatialIdx.Store(nil)
		return
	}

	b := newSpatialIndexBuilder(size.Width, size.Height)

	addTree := func(tree *renderCacheTree) {
		if tree == nil || tree.root == nil {
			return
		}
		c.walkCacheNodeDFS(tree.root, b)
	}

	addTree(c.contentTree)
	addTree(c.mWindowHeadTree)
	addTree(c.menuTree)
	for _, tree := range c.overlays.renderCaches {
		addTree(tree)
	}

	c.spatialIdx.Store(b.build())
}

// walkCacheNodeDFS visits a render cache subtree in depth-first order,
// adding each visible, non-zero-sized object to the spatial index builder.
func (c *Canvas) walkCacheNodeDFS(node *RenderCacheNode, b *spatialIndexBuilder) {
	if node == nil {
		return
	}

	obj := node.obj
	if obj != nil && obj.Visible() {
		s := obj.Size()
		if s.Width > 0 && s.Height > 0 {
			b.add(obj, node.worldTransform, s.Width, s.Height, node.clipPos, node.clipSize)
		}
	}

	// Children: firstChild, then firstChild.nextSibling, ...
	for child := node.firstChild; child != nil; child = child.nextSibling {
		c.walkCacheNodeDFS(child, b)
	}
}

func (c *Canvas) focusManager() *app.FocusManager {
	if focusMgr := c.overlays.TopFocusManager(); focusMgr != nil {
		return focusMgr
	}
	if c.isMenuActive() {
		return c.menuFocusMgr
	}
	return c.contentFocusMgr
}

func (c *Canvas) freeObject(object core.CanvasObject) {
	// Streaming rasters keep their GPU texture alive for in-place updates; the
	// GL and WebGPU painters both refresh it from drawRaster (#34).
	if rast, ok := object.(*canvas.Raster); ok && rast.Streaming {
		return
	}
	// Image.Refresh will trigger a refresh specific to the object,
	// while recursing on parent widget would just lead to a double texture upload.
	if img, ok := object.(*canvas.Image); ok {
		c.painter.Free(img)
		return
	}

	driver.WalkCompleteObjectTree(object, func(obj core.CanvasObject, _ geo.Position, _ geo.Position, _ geo.Size) bool {
		if _, ok := obj.(*canvas.Image); !ok { // No image refresh while recursing to avoid double texture upload.
			c.painter.Free(obj)
		}
		return false
	}, nil)
}

func (c *Canvas) isMenuActive() bool {
	if c.menuTree == nil || c.menuTree.root == nil || c.menuTree.root.obj == nil {
		return false
	}
	menu := c.menuTree.root.obj
	if am, ok := menu.(activatableMenu); ok {
		return am.IsActive()
	}
	return true
}

func (c *Canvas) walkTree(
	tree *renderCacheTree,
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	var node, parent, prev *RenderCacheNode
	node = tree.root

	bc := func(obj core.CanvasObject, pos geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
		if node != nil && node.obj != obj {
			if parent.firstChild == node {
				parent.firstChild = nil
			}
			node = nil
		}
		if node == nil {
			node = &RenderCacheNode{parent: parent, obj: obj}
			if parent.firstChild == nil {
				parent.firstChild = node
			} else {
				prev.nextSibling = node
			}
		}
		if prev != nil && prev.parent != parent {
			prev = nil
		}

		// Compute the accumulated world transform for this node.
		// worldTransform = parent.worldTransform * Translate(obj.Position()) * obj.Transform()
		parentWT := geo.Identity()
		if parent != nil {
			parentWT = parent.worldTransform
		}
		objPos := obj.Position()
		localT := geo.NewTranslation(objPos.X, objPos.Y)
		if t, ok := obj.(core.Transformable); ok {
			lt := t.Transform()
			if !lt.IsIdentity() {
				localT = localT.Multiply(lt)
			}
		}
		if parentWT.IsIdentity() {
			node.worldTransform = localT
		} else {
			node.worldTransform = parentWT.Multiply(localT)
		}

		// Cache clip bounds for spatial index construction.
		node.clipPos = clipPos
		node.clipSize = clipSize

		// Populate the object→node lookup map and cache boundary status.
		c.nodeMap.Store(obj, node)
		node.isBoundary = driver.IsRepaintBoundary(obj)

		if beforeChildren != nil {
			beforeChildren(node, pos)
		}

		parent = node
		node = parent.firstChild
		return false
	}
	ac := func(obj core.CanvasObject, pos geo.Position, _ core.CanvasObject) {
		node = parent
		parent = node.parent
		if prev != nil && prev.parent != parent {
			prev.nextSibling = nil
		}

		if afterChildren != nil {
			afterChildren(node, pos)
		}

		prev = node
		node = node.nextSibling
	}
	driver.WalkVisibleObjectTree(tree.root.obj, bc, ac)
}

type activatableMenu interface {
	IsActive() bool
}

func (c *Canvas) updateLayout(objToLayout core.CanvasObject) {
	switch cont := objToLayout.(type) {
	case *bellui.Container:
		if cont.Layout != nil {
			layout := cont.Layout
			objects := cont.Objects
			layout.Layout(objects, cont.Size())
		}
	case core.Widget:
		renderer := cache.Renderer(cont)
		renderer.Layout(cont.Size())
	}
}
