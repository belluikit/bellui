package common

import (
	"image/color"
	"reflect"
	"sync/atomic"
	"time"

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
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/internal/paintgen"
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
	// visiting is the node a walk's cull is asked about, for CullOffClip;
	// nil outside a cull call. Walks run on the main goroutine.
	visiting *RenderCacheNode

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
	// dirty is read and written on the main goroutine only, so it has no
	// lock (#586). Refresh sets it through async.EnsureMain, which moves a
	// call from another goroutine to main in apps not migrated to bellui.Do.
	dirty bool

	mWindowHeadTree, contentTree, menuTree *renderCacheTree

	// FixedSize is set by drivers whose window cannot grow to fit its content
	// (the browser, where the window is the page's viewport). Content larger
	// than the canvas is then clipped instead of the canvas growing past the
	// window: a canvas laid out wider than the viewport was drawn squeezed
	// into it until the next window resize (#387).
	FixedSize bool

	// nodeMap provides O(1) lookup from CanvasObject to its RenderCacheNode.
	// Used for dirty propagation to find the nearest ancestor RepaintBoundary.
	nodeMap async.Map[core.CanvasObject, *RenderCacheNode]
	// walkGen numbers the full walks (WalkTrees). Each walk stamps the nodes
	// it reaches, then drops nodeMap entries it did not reach: an object that
	// left the canvas would otherwise stay in nodeMap, and so in memory, for
	// as long as the window lives.
	walkGen uint64

	// dirtyRegion tracks the bounding box of all refreshed objects since the
	// last paint. When set, the paint walk can use scissor testing to restrict
	// GL operations to this region.
	dirtyRegion dirtyRect

	// refreshedWas is the union of the bounds the objects refreshed since the
	// last paint had at that paint, gathered by FreeDirtyTextures. It is a part
	// of the region PartialDirtyRegion would find, so once it covers the canvas
	// the frame is a full repaint and the walk can be skipped (#496). Only kept
	// under the partialrepaint build tag.
	refreshedWas dirtyRect
	// paintBoundsStale says the nodes' lastPaint bounds are not those of the
	// last painted frame: it was a full repaint that did not walk to record
	// them. The next PartialDirtyRegion must then repaint fully, and walk to
	// record them again.
	paintBoundsStale bool
	// leftRegion is where the nodes that left the render tree since the last
	// PartialDirtyRegion were painted (forgetNodes), and the rects drivers
	// marked dirty (MarkDirtyRect).
	leftRegion dirtyRect

	// spatialIdx provides O(1) broadphase hit testing. Built during
	// WalkTrees and read during mouse/touch event processing.
	spatialIdx spatialIndexHolder
	// spatialBufs are the two indexes RebuildSpatialIndex alternates between,
	// reusing their slices (#435): each rebuild fills the one not published,
	// so the index a reader just loaded is not touched until the rebuild
	// after next. Every reader runs on the goroutine that paints (the event
	// loop, or bellui.Do), so none holds an index across two rebuilds.
	spatialBufs [2]*SpatialIndex
	spatialNext int

	// layoutDir is the per-canvas user-interface direction. The zero value
	// (DirectionAuto) defers to the process-global setting; an explicit value
	// overrides it for this canvas' whole content, so a single window can flow
	// LTR while the rest of an RTL app does not. It is pushed onto the ambient
	// direction stack around this canvas' layout and paint.
	layoutDir bellui.Direction

	// measuredGen, measuredScale and measuredEpoch are the theme cache
	// generation, scale and cache.MinSizeEpoch EnsureMinSize last measured
	// under; a change re-measures everything.
	measuredGen   uint64
	measuredScale float32
	measuredEpoch uint64

	// dirtyNodes are the nodes refreshed since the last min-size pass, queued
	// from any goroutine (MarkMinSizeDirty); the pass marks them and their
	// ancestors on the main thread (#487).
	dirtyNodes nodeQueue
	// minSizeFull asks the next min-size pass to walk every object: the
	// content changed.
	minSizeFull      atomic.Bool
	lastFullMinSize  time.Time
	measuredOverlays int
	// minSizeVisits counts the nodes the last min-size pass visited.
	minSizeVisits int

	// placed are the nodes whose objects were moved, resized, hidden or
	// refreshed since the last PartialDirtyRegion, queued from any goroutine
	// (MarkPlacementDirty); the region walks from them instead of from the
	// roots (#599).
	placed nodeQueue
	// placeAll asks the next PartialDirtyRegion to walk every tree: one was
	// replaced. paintEpoch is the paintgen epoch the last one walked at.
	placeAll   atomic.Bool
	paintEpoch uint64
	// regionWalk says which trees a PartialDirtyRegion walk is in, if any.
	regionWalk regionWalk
	// regionPass numbers the PartialDirtyRegion walks from placed nodes, and
	// regionRoots holds the nodes one walks from, reused.
	regionPass  uint64
	regionRoots []*RenderCacheNode
	// foreignParents are the nodes with a child that does not report its
	// placement, such as an app's own CanvasObject type: every region walks
	// them. foreignRoot says a content tree's root is such an object, so every
	// region walks everything.
	foreignParents map[*RenderCacheNode]struct{}
	foreignRoot    bool
	// regionVisits counts the nodes the last PartialDirtyRegion visited.
	regionVisits int
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

	// A theme or scale change can change any min size: ask every object.
	// Otherwise only objects that were refreshed, resized, or whose children's
	// min sizes changed are asked again; the rest report what they did (#464).
	gen, scale, epoch := cache.ThemeCacheGeneration(), c.impl.Scale(), layoutgen.Epoch()
	measureAll := gen != c.measuredGen || scale != c.measuredScale || epoch != c.measuredEpoch
	c.measuredGen, c.measuredScale, c.measuredEpoch = gen, scale, epoch

	c.minSizeVisits = 0
	setup := func(node *RenderCacheNode, pos geo.Position) {
		c.minSizeVisits++
		node.descDirty = false
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
		if !node.canvasReg.Current(c.impl) {
			node.canvasReg = cache.RegisterCanvasForObject(obj, c.impl, func() {
				if img, ok := obj.(*canvas.Image); ok {
					img.Refresh() // this may now have a different texScale
				}
			})
		}

		childChanged := node.minSizeChanged
		if node.minSizeChanged {
			c.updateLayout(obj)
			node.minSizeChanged = false
		}

		if !obj.Visible() {
			return
		}
		minSize := node.minSize
		size := obj.Size()
		if needsMeasure(node, obj, size, measureAll, childChanged) {
			minSize = obj.MinSize()
			node.measured, node.measuredSize = true, size
		}

		minSizeChanged := node.minSize != minSize
		if minSizeChanged {
			node.minSize = minSize
			if node.parent != nil {
				// Flag the parent for re-layout when the post-order walk
				// reaches it. Per-node, so changes reported from several
				// subtrees in the same walk all apply (pre-rebrand tracker #1039).
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
	c.walkForMinSize(measureAll, setup, ensureMinSize)
	uidir.Pop(c.layoutDir)

	shouldResize := !c.FixedSize && windowNeedsMinSizeUpdate && (csize.Width < min.Width || csize.Height < min.Height)
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
	// mirror to the new field (so one Gboard session spanned both, pre-rebrand tracker #966), the
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
	objectsToFree := c.refreshQueue.Len()
	for object := c.refreshQueue.Out(); object != nil; object = c.refreshQueue.Out() {
		c.freeObject(object)

		// Track dirty region for scissor culling. The node is looked up once,
		// for this, the partial-repaint region and the boundary below (#543).
		node, mapped := c.noteRefreshed(object)
		if mapped {
			wt := node.worldTransform
			c.dirtyRegion.Union(geo.NewPos(wt.TX, wt.TY), object.Size())
		}

		// Propagate dirty state to the nearest ancestor RepaintBoundary.
		if hasBP && mapped {
			markBoundaryDirtyFrom(bp, node)
		}
	}

	cache.RangeExpiredTexturesFor(c.impl, c.painter.Free)
	return objectsToFree
}

// markBoundaryDirtyFrom walks up the render cache tree from node to find the
// nearest RepaintBoundary ancestor and marks its FBO as needing re-render.
func markBoundaryDirtyFrom(bp gpu.BoundaryPainter, node *RenderCacheNode) {
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
	var head core.CanvasObject
	if c.mWindowHeadTree != nil && c.mWindowHeadTree.root != nil {
		head = c.mWindowHeadTree.root.obj
	}
	trees := make([]core.CanvasObject, 0, len(c.Overlays().List())+3)
	trees = append(trees, content)
	if head != nil {
		trees = append(trees, head)
	}
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

// InDirtyRegion reports whether the rect (pos, size) needs repainting under a
// partial repaint of the region (rPos, rSize). An empty region, a frame with
// nothing dirty, holds nothing. By overlap alone, a rect reaching across the
// empty region at (0, 0) would count as inside it and be painted unclipped,
// over pixels nothing else repainted (#605).
func InDirtyRegion(pos geo.Position, size geo.Size, rPos geo.Position, rSize geo.Size) bool {
	return rSize.Width > 0 && rSize.Height > 0 && RectsOverlap(pos, size, rPos, rSize)
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
// When the refreshed objects' previous bounds alone cover most of the canvas,
// the region would too, so it returns the whole canvas without walking; the
// first later call that cannot skip walks to record the bounds again, and also
// returns the whole canvas (#496). So does the call after a full repaint the
// caller did without asking for a region (MarkFullRepaint).
//
// It walks from the nodes reported moved, resized, hidden or refreshed since
// the last call, not from the roots, unless something no node can report
// changed (region.go, #599).
//
// Only meaningful under the partialrepaint build tag; the per-node bookkeeping
// it relies on is otherwise never populated.
func (c *Canvas) PartialDirtyRegion() (pos geo.Position, size geo.Size, set bool) {
	canvasSize := c.impl.Size()
	was := c.refreshedWas
	c.refreshedWas.Reset()
	placed := c.takePlaced()
	epoch := paintgen.Epoch()
	all := c.placeAll.Swap(false) || epoch != c.paintEpoch || c.foreignRoot
	c.paintEpoch = epoch
	c.regionVisits = 0
	// Stale bounds make the frame full anyway, and when they are stale, was
	// may be too, so it only decides whether the frame walks.
	if was.set && RegionCoversCanvas(clampRect(was, canvasSize), canvasSize) {
		clear(placed)
		c.paintBoundsStale = true
		c.leftRegion.Reset()
		return geo.Position{}, canvasSize, true
	}
	stale := c.paintBoundsStale
	c.paintBoundsStale = false
	var roots []*RenderCacheNode
	if stale || all {
		clear(placed)
	} else {
		roots, all = c.regionRootsFor(placed)
	}

	var region dirtyRect
	// The walk culls what the paint walk culls, so a clip-owner off its clip
	// is visited without the children it would not draw (#464); culledObj is
	// the object cull last skipped the children of. Cull runs just before the
	// object's visit.
	var culledObj core.CanvasObject
	cull := func(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
		if c.CullOffClip(obj, pos, clipPos, clipSize) {
			culledObj = obj
			return true
		}
		return false
	}
	record := func(node *RenderCacheNode, _ geo.Position) {
		c.regionVisits++
		if culledObj == node.obj {
			culledObj = nil
			if !node.childrenCulled {
				// What the children drew until now is no longer drawn: clear
				// it, and forget it, so they count as new when drawn again.
				node.childrenCulled = true
				forgetPaintBounds(node, &region, !stale)
			}
		} else {
			node.childrenCulled = false
		}

		newPos, newSize := paintBounds(node)

		isRefreshed := node.paintRefreshed
		node.paintRefreshed = false
		moved := !node.lastPaintSet || node.lastPaintPos != newPos || node.lastPaintSize != newSize

		if !stale && (isRefreshed || moved) {
			region.Union(padRect(newPos, newSize)) // where it is now
			if node.lastPaintSet {
				region.Union(padRect(node.lastPaintPos, node.lastPaintSize)) // where it was
			}
		}
		if !stale && isRefreshed && !node.childrenCulled {
			// A content refresh can repaint descendants that overflow this node's
			// own bounds (shadows, focus rings, selection); cover the subtree.
			c.unionSubtreeBounds(node, &region)
		}

		node.lastPaintPos = newPos
		node.lastPaintSize = newSize
		node.lastPaintSet = true
	}
	// Not WalkTrees: a full walk starts a node-map generation, which makes
	// every node look itself up again (#487), and sweeps the map and rebuilds
	// the spatial index, which the full walks and the paint walk already do.
	c.regionWalk = regionContent
	if stale || all {
		c.foreignRoot = false
		c.walkContentTrees(cull, nil, record, nil)
	} else {
		for i, root := range roots {
			c.walkTreeFrom(root, cull, nil, record, nil)
			roots[i] = nil
		}
	}
	// Overlays are few and small: walk them whole.
	c.regionWalk = regionOverlay
	for _, tree := range c.overlays.renderCaches {
		if tree != nil {
			c.walkTree(tree, nil, nil, record, nil)
		}
	}
	c.regionWalk = regionNone

	if stale {
		c.leftRegion.Reset()
		return geo.Position{}, canvasSize, true
	}
	if c.leftRegion.set {
		region.Union(c.leftRegion.pos, c.leftRegion.size)
		c.leftRegion.Reset()
	}
	return region.pos, region.size, region.set
}

// notePaintRefresh records for PartialDirtyRegion that node's object was
// refreshed.
func (c *Canvas) notePaintRefresh(node *RenderCacheNode) {
	c.queuePlaced(node)
	node.paintRefreshed = true
	if node.lastPaintSet {
		c.refreshedWas.Union(node.lastPaintPos, node.lastPaintSize)
	}
}

// forgetPaintBounds forgets the bounds every descendant of node was painted
// at, first adding them to region if union is set.
func forgetPaintBounds(node *RenderCacheNode, region *dirtyRect, union bool) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.lastPaintSet && union {
			region.Union(padRect(child.lastPaintPos, child.lastPaintSize))
		}
		child.lastPaintSet = false
		forgetPaintBounds(child, region, union)
	}
}

// MarkDirtyRect adds a rect to the next PartialDirtyRegion. A driver that
// paints something no canvas object stands for, such as the mobile
// screen-reader focus ring, marks where it was and where it will be, so the
// region clears the one and redraws the content under the other (#551).
func (c *Canvas) MarkDirtyRect(pos geo.Position, size geo.Size) {
	c.leftRegion.Union(padRect(pos, size))
}

// MarkFullRepaint tells PartialDirtyRegion that the frame being painted is a
// full repaint that did not ask it for a region, so the bounds it recorded are
// no longer those on screen.
func (c *Canvas) MarkFullRepaint() {
	c.paintBoundsStale = true
}

// clampRect returns the size of the part of r inside a canvas of size canvas.
func clampRect(r dirtyRect, canvas geo.Size) geo.Size {
	minX, minY := geo.Max(r.pos.X, 0), geo.Max(r.pos.Y, 0)
	maxX := geo.Min(r.pos.X+r.size.Width, canvas.Width)
	maxY := geo.Min(r.pos.Y+r.size.Height, canvas.Height)
	if maxX <= minX || maxY <= minY {
		return geo.Size{}
	}
	return geo.NewSize(maxX-minX, maxY-minY)
}

// unionSubtreeBounds expands region to include the current world-space bounds of
// every descendant of node (padded). Used so a refreshed parent also clears the
// overflow drawn by its children.
func (c *Canvas) unionSubtreeBounds(node *RenderCacheNode, region *dirtyRect) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		region.Union(padRect(paintBounds(child)))
		c.unionSubtreeBounds(child, region)
	}
}

// paintBounds returns the world-space box node's object paints in: its
// PaintExtent, through its world transform.
func paintBounds(node *RenderCacheNode) (geo.Position, geo.Size) {
	off, s := PaintExtent(node.obj)
	m := node.worldTransform
	if !off.IsZero() {
		m = m.Multiply(geo.NewTranslation(off.X, off.Y))
	}
	minX, minY, maxX, maxY := m.AABB(s.Width, s.Height)
	return geo.NewPos(minX, minY), geo.NewSize(maxX-minX, maxY-minY)
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
	c.MarkMinSizeDirty(obj)
	c.refreshQueue.In(obj)
	async.EnsureMain(c.SetDirty)
}

// MarkMinSizeDirty records that obj may report a different min size, so the
// next EnsureMinSize asks it again (#464). Refresh calls it; BaseWidget.Refresh
// calls it for the widget itself, whose renderer may refresh only its parts.
// Safe from any goroutine.
func (c *Canvas) MarkMinSizeDirty(obj core.CanvasObject) {
	node, ok := c.nodeMap.Load(obj)
	if !ok {
		// An object no walk has placed in the tree takes no part in layout
		// yet. It joins the tree through a container or widget that was
		// refreshed (marking that path), resized, or shown (counted by
		// layoutgen), or with new content or overlays, which walk everything;
		// so its own refresh needs no walk. A tiles screen refreshes ~200
		// such objects a second (#490), which would otherwise make every pass
		// a full walk.
		return
	}
	node.minSizeDirty.Store(true)
	node.cullGen.Store(0)
	c.dirtyNodes.add(node, &node.queued)
}

// MinSizeVisits reports how many nodes the last EnsureMinSize visited.
func (c *Canvas) MinSizeVisits() int { return c.minSizeVisits }

// fullMinSizeEvery is how often the min-size pass walks every object although
// nothing asks it to: that keeps the caches of untouched subtrees alive and
// sweeps the node map of objects that left the tree (#487).
const fullMinSizeEvery = 10 * time.Second

// markDirtyPaths marks every node refreshed since the last min-size pass, and
// its ancestors, as having a refreshed descendant.
func (c *Canvas) markDirtyPaths() {
	taken := c.dirtyNodes.take()
	for i, n := range taken {
		n.queued.Store(false)
		for p := n; p != nil && !p.descDirty; p = p.parent {
			p.descDirty = true
		}
		taken[i] = nil
	}
}

// needsMeasure reports whether EnsureMinSize must ask obj for its min size
// again (#464): when everything is re-measured, when the node is new, was
// refreshed, changed size or had a child's min size change, and always for a
// primitive, whose min size can change without a refresh (SetMinSize) and
// costs a field read or a cached text measurement to ask; a change then
// reaches the parent as childChanged.
func needsMeasure(node *RenderCacheNode, obj core.CanvasObject, size geo.Size, measureAll, childChanged bool) bool {
	dirty := node.minSizeDirty.Swap(false)
	if measureAll || dirty || childChanged || !node.measured || size != node.measuredSize {
		return true
	}
	_, isWidget := obj.(core.Widget)
	_, isContainer := obj.(*bellui.Container)
	return !isWidget && !isContainer
}

// walkForMinSize walks every object when the shortcut cannot be trusted, and
// otherwise only the paths to what was refreshed (#487).
func (c *Canvas) walkForMinSize(measureAll bool, setup, ensureMinSize func(*RenderCacheNode, geo.Position)) {
	now := time.Now()
	full := measureAll || c.minSizeFull.Swap(false) ||
		len(c.overlays.renderCaches) != c.measuredOverlays || now.Sub(c.lastFullMinSize) >= fullMinSizeEvery
	c.measuredOverlays = len(c.overlays.renderCaches)
	c.markDirtyPaths()
	if full {
		c.WalkTrees(setup, ensureMinSize)
		c.lastFullMinSize = now
		return
	}
	c.walkChangedTrees(setup, ensureMinSize)
}

// walkChangedTrees walks the content and overlay trees as WalkTrees does,
// except that a measured node with no refreshed descendant and an unchanged
// size is visited without its subtree: nothing below it can report a new min
// size (#487). It neither sweeps the node map nor rebuilds the spatial index;
// full walks and the paint walk do those.
func (c *Canvas) walkChangedTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	cull := func(node *RenderCacheNode, obj core.CanvasObject) bool {
		return node.measured && !node.descDirty && obj.Size() == node.measuredSize
	}
	c.walkContentTrees(nil, cull, beforeChildren, afterChildren)
	for _, tree := range c.overlays.renderCaches {
		if tree != nil {
			c.walkTree(tree, nil, cull, beforeChildren, afterChildren)
		}
	}
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
	c.minSizeFull.Store(true)
	c.placeAll.Store(true)

	newFocusMgr := app.NewFocusManager(content)
	if c.contentFocusMgr != nil {
		focused := c.contentFocusMgr.Focused()
		if focused != nil {
			newFocusMgr.Focus(focused) // Focus old object if possible.
		}
	}
	c.contentFocusMgr = newFocusMgr
}

// CheckDirtyAndClear returns true if the canvas is dirty and clears the dirty
// state. Call it on the main goroutine only: the flag has no lock.
func (c *Canvas) CheckDirtyAndClear() bool {
	wasDirty := c.dirty
	c.dirty = false
	return wasDirty
}

// SetDirty marks the canvas as needing a paint. Call it on the main goroutine
// only: the flag has no lock.
func (c *Canvas) SetDirty() {
	c.dirty = true
}

// IsDirty reports whether the canvas has changes that have not been painted
// yet, without clearing the flag. Call it on the main goroutine only. While it
// is true the spatial index has not been rebuilt since the last change, so hit
// testing must not rely on it.
func (c *Canvas) IsDirty() bool {
	return c.dirty
}

// SetMenuTreeAndFocusMgr sets menu tree and focus manager.
//
// This function does not use the canvas lock.
func (c *Canvas) SetMenuTreeAndFocusMgr(menu core.CanvasObject) {
	c.menuTree = &renderCacheTree{root: &RenderCacheNode{obj: menu}}
	c.placeAll.Store(true)
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
	c.placeAll.Store(true)
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
	c.walkGen++
	c.WalkContentTrees(beforeChildren, afterChildren)
	c.WalkOverlayTrees(beforeChildren, afterChildren)
	c.RebuildSpatialIndex()
	gen := c.walkGen
	c.nodeMap.Range(func(obj core.CanvasObject, node *RenderCacheNode) bool {
		if node.walkGen != gen {
			c.nodeMap.Delete(obj)
		}
		return true
	})
}

// WalkContentTrees walks content, window head, and menu trees (not overlays).
func (c *Canvas) WalkContentTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.WalkContentTreesCulled(nil, beforeChildren, afterChildren)
}

// WalkContentTreesCulled walks the content trees as WalkContentTrees does,
// visiting an object cull reports as drawing nothing visible without its
// children (driver.WalkVisibleObjectTreeCulled). The paint walk passes
// CullOffClip, so content scrolled out of view costs one visit per clip-owner
// instead of its whole subtree (#464). Walks that must reach every object
// (layout, hit testing, accessibility) pass nil.
func (c *Canvas) WalkContentTreesCulled(
	cull driver.CullFunc,
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.walkContentTrees(cull, nil, beforeChildren, afterChildren)
}

// walkContentTrees is WalkContentTreesCulled with walkTree's node-level cull.
func (c *Canvas) walkContentTrees(
	cull driver.CullFunc,
	nodeCull func(*RenderCacheNode, core.CanvasObject) bool,
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.walkTree(c.contentTree, cull, nodeCull, beforeChildren, afterChildren)
	if c.mWindowHeadTree != nil && c.mWindowHeadTree.root.obj != nil {
		c.walkTree(c.mWindowHeadTree, cull, nodeCull, beforeChildren, afterChildren)
	}
	if c.menuTree != nil && c.menuTree.root.obj != nil {
		c.walkTree(c.menuTree, cull, nodeCull, beforeChildren, afterChildren)
	}
}

// WalkOverlayTrees walks overlay trees only.
func (c *Canvas) WalkOverlayTrees(
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	for _, tree := range c.overlays.renderCaches {
		if tree != nil {
			c.walkTree(tree, nil, nil, beforeChildren, afterChildren)
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

	b := c.spatialBufs[c.spatialNext]
	if b == nil {
		b = &SpatialIndex{}
		c.spatialBufs[c.spatialNext] = b
	}
	c.spatialNext ^= 1
	b.reset(size.Width, size.Height)

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

	b.finish()
	c.spatialIdx.Store(b)
}

// walkCacheNodeDFS visits a render cache subtree in depth-first order,
// adding each visible, non-zero-sized object to the spatial index builder.
//
// It skips the children the paint walk culled (#481). Their clip-owner misses
// the clip it sits in, so their own clip, the overlap of the two, is empty and
// nothing in them can be hit (#566); and the walk did not visit them, so their
// nodes still say where they were. Visiting them also costs more than the
// culled paint walk saved on a phone: the walk left them out of the CPU cache.
func (c *Canvas) walkCacheNodeDFS(node *RenderCacheNode, b *SpatialIndex) {
	if node == nil {
		return
	}

	obj := node.obj
	if obj != nil && obj.Visible() {
		s := obj.Size()
		if s.Width > 0 && s.Height > 0 {
			b.add(obj, node.worldTransform, s.Width, s.Height, node.clipPos, node.clipSize)
			if node.paintCulled {
				return
			}
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

// walkTree walks one render tree, keeping its nodes in step with the objects.
// cull, if set, decides from the object whether to skip its children; nodeCull
// decides from the object's node. The walk passes nodeCull the node it is about
// to visit for the object, so it costs no node-map lookup unless the tree
// changed there (#487).
func (c *Canvas) walkTree(
	tree *renderCacheTree,
	cull driver.CullFunc,
	nodeCull func(*RenderCacheNode, core.CanvasObject) bool,
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	c.walkTreeFrom(tree.root, cull, nodeCull, beforeChildren, afterChildren)
}

// walkTreeFrom is walkTree from start, a node of a tree, and its subtree:
// it starts where the walk of its parent left it, and changes no other node
// (#599). start's object must be visible.
func (c *Canvas) walkTreeFrom(
	start *RenderCacheNode,
	cull driver.CullFunc,
	nodeCull func(*RenderCacheNode, core.CanvasObject) bool,
	beforeChildren func(*RenderCacheNode, geo.Position),
	afterChildren func(*RenderCacheNode, geo.Position),
) {
	var node, parent, prev *RenderCacheNode
	node, parent = start, start.parent
	parentObj, offset, clipPos, clipSize := start.walkStart()
	if parent != nil && parent.noCullBelow {
		cull, nodeCull = nil, nil // the walk from the root would not cull here
	}
	// culled is the object cull last skipped the children of, and skipped the
	// object cull or nodeCull did. Cull runs just before the object's visit,
	// which records them on the node.
	var culled, skipped core.CanvasObject
	if cull != nil {
		skip := cull
		cull = func(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
			// The node the walk is about to visit, for Canvas.CullOffClip.
			c.visiting = nil
			if node != nil && node.obj == obj {
				c.visiting = node
			}
			cut := skip(obj, pos, clipPos, clipSize)
			c.visiting = nil
			if cut {
				culled, skipped = obj, obj
				return true
			}
			return false
		}
	}
	if nodeCull != nil {
		cull = func(obj core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			n := node // the node the walk is about to visit for obj
			if n == nil || n.obj != obj {
				var ok bool
				if n, ok = c.nodeMap.Load(obj); !ok {
					return false
				}
			}
			if nodeCull(n, obj) {
				skipped = obj
				return true
			}
			return false
		}
	}
	region := c.regionWalk == regionContent
	var hidden func(obj, _ core.CanvasObject)
	if region {
		// A hidden child that does not report placement would not report
		// being shown either: its parent is walked every region (#599).
		hidden = func(obj, _ core.CanvasObject) { noteHiddenChild(parent, obj) }
	}

	bc := func(obj core.CanvasObject, pos geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
		if node != nil && node.obj != obj {
			// The old node and its later siblings leave the tree.
			c.forgetNodes(node)
			if parent.firstChild == node {
				parent.firstChild = nil
			}
			node = nil
		}
		if node == nil {
			node = c.addChild(parent, prev, obj)
		}
		if prev != nil && prev.parent != parent {
			prev = nil
		}

		node.setWorldTransform(obj)
		node.walkPos = pos

		// Cache clip bounds for spatial index construction.
		node.clipPos = clipPos
		node.clipSize = clipSize
		node.paintCulled = culled == obj
		node.childrenSkipped = skipped == obj
		culled, skipped = nil, nil

		c.mapNode(node, obj)
		if region {
			c.notePlacement(node)
		}

		if beforeChildren != nil {
			beforeChildren(node, pos)
		}

		parent = node
		node = parent.firstChild
		return false
	}
	ac := func(obj core.CanvasObject, pos geo.Position, _ core.CanvasObject) {
		if node != nil && node == parent.firstChild && !parent.childrenSkipped {
			// No child was visited, and none skipped: the old ones all left
			// the tree, hidden or removed.
			c.forgetNodes(node)
			parent.firstChild = nil
		}
		node = parent
		parent = node.parent
		if prev != nil && prev.parent != parent {
			if prev.nextSibling != nil {
				c.forgetNodes(prev.nextSibling) // old siblings past the last child
			}
			prev.nextSibling = nil
		}
		if region {
			c.noteForeignChildren(node)
		}

		if afterChildren != nil {
			afterChildren(node, pos)
		}

		prev = node
		node = node.nextSibling
	}
	driver.WalkVisibleSubtreeCulled(start.obj, parentObj, offset, clipPos, clipSize, cull, hidden, bc, ac)
}

// addChild adds a node for obj to parent, after prev or first.
func (c *Canvas) addChild(parent, prev *RenderCacheNode, obj core.CanvasObject) *RenderCacheNode {
	node := &RenderCacheNode{parent: parent, obj: obj}
	if parent.firstChild == nil {
		parent.firstChild = node
	} else {
		prev.nextSibling = node
	}
	if build.PartialRepaint && c.regionWalk == regionNone {
		// The partial-repaint region walks only from reported nodes, so it
		// must hear of one added outside it (#599).
		c.queuePlaced(node)
	}
	return node
}

// walkStart is where a walk from r starts: as the walk of r's parent left it.
func (r *RenderCacheNode) walkStart() (parent core.CanvasObject, offset, clipPos geo.Position, clipSize geo.Size) {
	p := r.parent
	if p == nil {
		return nil, geo.Position{}, geo.Position{}, driver.NoClipBounds
	}
	return p.obj, p.walkPos, p.clipPos, p.clipSize
}

// setWorldTransform computes the node's accumulated world transform,
// parent.worldTransform * Translate(obj.Position()) * obj.Transform(), and
// whether walks cull below it.
func (r *RenderCacheNode) setWorldTransform(obj core.CanvasObject) {
	parentWT := geo.Identity()
	noCull := false
	if r.parent != nil {
		parentWT = r.parent.worldTransform
		noCull = r.parent.noCullBelow
	}
	objPos := obj.Position()
	localT := geo.NewTranslation(objPos.X, objPos.Y)
	if t, ok := obj.(core.Transformable); ok {
		lt := t.Transform()
		if !lt.IsIdentity() {
			localT = localT.Multiply(lt)
			noCull = true
		}
	}
	if parentWT.IsIdentity() {
		r.worldTransform = localT
	} else {
		r.worldTransform = parentWT.Multiply(localT)
	}
	r.noCullBelow = noCull
}

// mapNode keeps the object→node lookup map and the node's boundary status
// current. It stores only a changed mapping: storing over an existing key
// allocates a new entry, which for every node every paint was 23% of a static
// frame's allocation once the spatial index stopped allocating (#435).
//
// A node confirmed in the map during the current full-walk generation is
// still there: the only sweep, at the end of a full walk, deletes nodes that
// walk did not reach, before anything later in the generation is confirmed;
// and forgetNodes removes only nodes leaving the tree, which are never reused.
// So the lookup is needed once per generation, not on every walk (#487).
func (c *Canvas) mapNode(node *RenderCacheNode, obj core.CanvasObject) {
	if node.mappedGen != c.walkGen+1 {
		if cur, ok := c.nodeMap.Load(obj); !ok || cur != node {
			c.nodeMap.Store(obj, node)
		}
		node.mappedGen = c.walkGen + 1 // +1: a new node's zero never matches
		node.boundarySettled = false   // and ask again once per generation
	}
	node.walkGen = c.walkGen
	// Whether a widget is a repaint boundary depends on its renderer's type,
	// so a settled answer is kept rather than looking the renderer up again
	// on every visit (#487).
	if !node.boundarySettled {
		node.isBoundary, node.boundarySettled = driver.RepaintBoundaryStatus(obj)
	}
}

// forgetNodes removes the objects of n, its later siblings and all their
// descendants from the node map, as their nodes leave the render tree. A walk
// that visits only changed paths does not sweep the map, so without this an
// object removed from a container would stay reachable until the next full
// walk (#380, #487). An object that already has a new node keeps it.
//
// Under partial repaint, what the nodes were painted as joins the next dirty
// region (leftRegion): a hidden or removed object's pixels must be cleared.
func (c *Canvas) forgetNodes(n *RenderCacheNode) {
	for ; n != nil; n = n.nextSibling {
		if cur, ok := c.nodeMap.Load(n.obj); ok && cur == n {
			c.nodeMap.Delete(n.obj)
		}
		if build.PartialRepaint && n.lastPaintSet {
			c.leftRegion.Union(padRect(n.lastPaintPos, n.lastPaintSize))
		}
		c.forgetNodes(n.firstChild)
	}
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

// ClampRegionToCanvas intersects a dirty region with the canvas [0,0]-(size)
// box, returning a zero-size region when it falls entirely outside.
func ClampRegionToCanvas(pos geo.Position, sz geo.Size, size geo.Size) (geo.Position, geo.Size) {
	maxX, maxY := pos.X+sz.Width, pos.Y+sz.Height
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}
	if maxX > size.Width {
		maxX = size.Width
	}
	if maxY > size.Height {
		maxY = size.Height
	}
	if maxX <= pos.X || maxY <= pos.Y {
		return pos, geo.Size{}
	}
	return pos, geo.NewSize(maxX-pos.X, maxY-pos.Y)
}

// CullOffClip is the package's CullOffClip for a walk's CullFunc. For the
// object the walk is about to visit it answers from that object's node, which
// remembers its clip and shadow reach (#599); for any other object it asks the
// renderer, as CullOffClip does.
func (c *Canvas) CullOffClip(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
	if n := c.visiting; n != nil && n.obj == obj {
		return n.cullOffClip(pos, clipPos, clipSize)
	}
	return CullOffClip(obj, pos, clipPos, clipSize)
}
