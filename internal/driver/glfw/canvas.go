package glfw

import (
	"image"
	"math"

	"github.com/belluikit/bellui/internal/cache"

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
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with Canvas interface
var _ core.Canvas = (*glCanvas)(nil)

type glCanvas struct {
	common.Canvas

	content        core.CanvasObject
	menu           core.CanvasObject
	padded         bool
	selectableMode bellui.SelectableMode
	size           geo.Size

	onTypedRune func(rune)
	onTypedKey  func(*event.KeyEvent)
	onKeyDown   func(*event.KeyEvent)
	onKeyUp     func(*event.KeyEvent)
	// shortcut    bellui.ShortcutHandler

	scale, detectedScale, texScale float32

	context         driver.WithContext
	webExtraWindows *container.MultipleWindows
}

// Capture paints a fresh frame and returns it. Any goroutine may call it. The
// paint and read-back run on the main thread, which owns the window's GL
// context and paints the window every frame. Run on the caller's thread, the
// GL calls could find no context current, or race the render loop (#562).
func (c *glCanvas) Capture() image.Image {
	if async.IsMainGoroutine() {
		return c.capture()
	}
	var img image.Image
	runOnMain(func() { img = c.capture() })
	return img
}

func (c *glCanvas) capture() image.Image {
	var img image.Image
	w := c.context.(*window)

	// Capture must contain overlays and menus (pre-rebrand tracker #473). Both painters scrape
	// rendered frames rather than re-rendering for capture, and on a static
	// scene no frame may have rendered since the request — the WebGPU
	// fallback then reads the persistent canvas FBO, which excludes the
	// overlay and menu passes. So force a fresh full paint and read that
	// exact frame, per backend.
	if ec, ok := c.Painter().(interface{ EnableCapture() }); ok {
		// WebGPU: arm the per-frame capture copy, then repaint — EndFrame
		// copies the presented surface texture, which Capture reads back.
		ec.EnableCapture()
		c.SetDirty()
		w.driver.repaintWindow(w)
		w.RunWithContext(func() {
			img = c.Painter().Capture(c)
		})
		return img
	}

	// GL: paint into the back buffer and read it before any swap — reading
	// the front buffer races the presentation pipeline and can return a
	// stale frame. Prepare it as repaintWindow does: without that, a capture
	// taken right after a change drew each RepaintBoundary from its old image,
	// as the refreshes that dirty it were still queued (#565).
	prepareFrame(w)
	w.RunWithContext(func() {
		if bc, ok := c.Painter().(interface {
			CaptureBack(core.Canvas) image.Image
		}); ok {
			updateGLContext(w)
			c.paint(c.Size())
			img = bc.CaptureBack(c)
		} else {
			img = c.Painter().Capture(c)
		}
	})
	// The pre-capture paint consumed the dirty state without presenting;
	// mark the canvas dirty again so the regular draw loop presents the
	// (visually identical) frame normally.
	c.SetDirty()
	return img
}

func (c *glCanvas) Content() core.CanvasObject {
	return c.content
}

func (c *glCanvas) DismissMenu() bool {
	if c.menu != nil && c.menu.(*MenuBar).IsActive() {
		c.menu.(*MenuBar).Toggle()
		return true
	}
	return false
}

func (c *glCanvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.Position{}, c.Size()
}

// FindObjectAtPositionMatching is the dispatch hit-walk: overlays, then the
// menu, then the content, preferring the spatial index when it is current.
// It also serves the public driver.ObjectAtPositionMatching through the
// canvas capability assertion (#245), so it must stay equivalent to what tap
// and mouse dispatch use.
func (c *glCanvas) FindObjectAtPositionMatching(mouse geo.Position, matches func(object core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	overlay := c.Overlays().Top()
	// The spatial index is rebuilt during the paint walk, so between a content
	// change and the next frame it does not reflect the current tree — only
	// treat it as authoritative while the canvas is clean (pre-rebrand tracker #590).
	if overlay == nil {
		if idx := c.SpatialIndex(); idx != nil && !c.IsDirty() {
			if obj, pos, ok := idx.FindObjectAtPositionMatching(mouse, matches); ok {
				return obj, pos, 0
			}
			return nil, geo.Position{}, 0
		}
	}
	return driver.FindObjectAtPositionMatching(mouse, matches, c.Overlays().List(), c.menu, c.Content())
}

func (c *glCanvas) MinSize() geo.Size {
	return c.canvasSize(c.content.MinSize())
}

func (c *glCanvas) OnKeyDown() func(*event.KeyEvent) {
	return c.onKeyDown
}

func (c *glCanvas) OnKeyUp() func(*event.KeyEvent) {
	return c.onKeyUp
}

func (c *glCanvas) OnTypedKey() func(*event.KeyEvent) {
	return c.onTypedKey
}

func (c *glCanvas) OnTypedRune() func(rune) {
	return c.onTypedRune
}

func (c *glCanvas) Padded() bool {
	return c.padded
}

func (c *glCanvas) SelectableMode() bellui.SelectableMode {
	return c.selectableMode
}

func (c *glCanvas) SetSelectableMode(mode bellui.SelectableMode) {
	cache.NoteSelectableMode(mode)
	c.selectableMode = mode
}

func (c *glCanvas) PixelCoordinateForPosition(pos geo.Position) (int, int) {
	multiple := c.scale * c.texScale
	scaleInt := func(x float32) int {
		return int(math.Round(float64(x * multiple)))
	}

	return scaleInt(pos.X), scaleInt(pos.Y)
}

func (c *glCanvas) Resize(size geo.Size) {
	// This might not be the ideal solution, but it effectively avoid the first frame to be blurry due to the
	// rounding of the size to the loower integer when scale == 1. It does not affect the other cases as far as we tested.
	// This can easily be seen with a simple hello-world app and a scale == 1 as the text will happear blurry without the following line.
	nearestSize := geo.NewSize(float32(math.Ceil(float64(size.Width))), float32(math.Ceil(float64(size.Height))))

	c.size = nearestSize

	if c.webExtraWindows != nil {
		c.webExtraWindows.Resize(size)
	}
	for _, overlay := range c.Overlays().List() {
		if p, ok := overlay.(*widget.PopUp); ok {
			// TODO: remove this when pre-rebrand tracker #707 is being addressed.
			// “Notifies” the PopUp of the canvas size change.
			p.Refresh()
		} else {
			overlay.Resize(nearestSize)
		}
	}

	content := c.content
	contentSize := c.contentSize(nearestSize)
	contentPos := c.contentPos()
	menu := c.menu
	menuHeight := c.menuHeight()

	content.Resize(contentSize)
	content.Move(contentPos)

	if menu != nil {
		menu.Refresh()
		menu.Resize(geo.NewSize(nearestSize.Width, menuHeight))
	}
}

func (c *glCanvas) Scale() float32 {
	return c.scale
}

func (c *glCanvas) SetContent(content core.CanvasObject) {
	content.Resize(content.MinSize()) // give it the space it wants then calculate the real min

	// the pass above makes some layouts wide enough to wrap, so we ask again what the true min is.
	newSize := c.size.Max(c.canvasSize(content.MinSize()))

	c.setContent(content)

	c.Resize(newSize)
	c.SetDirty()
}

func (c *glCanvas) SetOnKeyDown(typed func(*event.KeyEvent)) {
	c.onKeyDown = typed
}

func (c *glCanvas) SetOnKeyUp(typed func(*event.KeyEvent)) {
	c.onKeyUp = typed
}

func (c *glCanvas) SetOnTypedKey(typed func(*event.KeyEvent)) {
	c.onTypedKey = typed
}

func (c *glCanvas) SetOnTypedRune(typed func(rune)) {
	c.onTypedRune = typed
}

func (c *glCanvas) SetPadded(padded bool) {
	c.padded = padded

	c.content.Move(c.contentPos())
}

func (c *glCanvas) reloadScale() {
	w := c.context.(*window)
	windowVisible := w.visible
	if !windowVisible {
		return
	}

	c.scale = w.calculatedScale()
	c.SetDirty()

	// Invalidate all boundary FBOs since pixel dimensions change with scale.
	if bp, ok := c.Painter().(gpu.BoundaryPainter); ok {
		bp.InvalidateAllBoundaries()
	}

	c.context.RescaleContext()
}

func (c *glCanvas) Size() geo.Size {
	return c.size
}

func (c *glCanvas) ToggleMenu() {
	if c.menu != nil {
		c.menu.(*MenuBar).Toggle()
	}
}

func (c *glCanvas) buildMenu(w *window, m *bellui.MainMenu) {
	c.setMenuOverlay(nil)
	if m == nil {
		return
	}
	if build.HasNativeMenu {
		setupNativeMenu(w, m)
	} else {
		c.setMenuOverlay(buildMenuOverlay(m, w))
	}
}

// canvasSize computes the needed canvas size for the given content size
func (c *glCanvas) canvasSize(contentSize geo.Size) geo.Size {
	canvasSize := contentSize.Add(geo.NewSize(0, c.menuHeight()))
	if c.Padded() {
		return canvasSize.Add(geo.NewSquareSize(theme.Padding() * 2))
	}
	return canvasSize
}

func (c *glCanvas) contentPos() geo.Position {
	contentPos := geo.NewPos(0, c.menuHeight())
	if c.Padded() {
		return contentPos.Add(geo.NewSquareOffsetPos(theme.Padding()))
	}
	return contentPos
}

func (c *glCanvas) contentSize(canvasSize geo.Size) geo.Size {
	contentSize := geo.NewSize(canvasSize.Width, canvasSize.Height-c.menuHeight())
	if c.Padded() {
		return contentSize.Subtract(geo.NewSquareSize(theme.Padding() * 2))
	}
	return contentSize
}

func (c *glCanvas) menuHeight() float32 {
	if c.menu == nil {
		return 0 // no menu or native menu -> does not consume space on the canvas
	}

	return c.menu.MinSize().Height
}

func (c *glCanvas) overlayChanged() {
	c.SetDirty()
}

// boundaryEntry tracks state for a RepaintBoundary during the paint walk.
type boundaryEntry struct {
	obj                    core.CanvasObject
	state                  gpu.BoundaryFBO
	wasDirty               bool
	invWT                  geo.Matrix           // inverse of boundary's worldTransform
	origin                 geo.Position         // boundary's texture origin: its canvas position snapped to a device pixel
	phase                  common.BoundaryPhase // sub-pixel phase the texture is rendered at
	size                   geo.Size             // boundary's logical size
	canvasPixW, canvasPixH int                  // for restoring viewport

	// outerClips is the clip stack in force outside a dirty boundary. Its
	// rects are in the enclosing target's space, so the subtree is rendered
	// with a stack of its own and these apply to the blit instead (#559).
	outerClips *internal.ClipStack
}

// clearCanvasFBO prepares the bound canvas FBO for this frame and returns the
// dirty region to redraw. Partial repaint off (build.PartialRepaint), or the
// first frame: full clear, no region. On: clear just the changed region, or
// full-clear when it covers most of the
// canvas; an empty region (nothing changed) draws nothing and re-presents the
// retained FBO.
func (c *glCanvas) clearCanvasFBO(drp gpu.DirtyRegionPainter, size geo.Size) (partial bool, drPos geo.Position, drSize geo.Size) {
	if build.PartialRepaint && !drp.CanvasFBONeedsFullClear() {
		pos, sz, set := c.PartialDirtyRegion()
		// A refreshed object can sit (partly) outside the canvas — e.g. content
		// scrolled out of a container. Clamp the dirty region to the canvas so the
		// region clip stays in bounds; if nothing on-canvas changed, retain.
		pos, sz = common.ClampRegionToCanvas(pos, sz, size)
		set = set && sz.Width > 0 && sz.Height > 0
		switch {
		case !set:
			return true, geo.Position{}, geo.Size{}
		case common.RegionCoversCanvas(sz, size):
			c.Painter().Clear()
			return false, geo.Position{}, geo.Size{}
		default:
			// Clear just the dirty region; the surrounding canvas FBO is retained.
			// The region clip stays active through the content walk (see paint) so
			// redrawn objects that overlap the region — notably full-canvas
			// backgrounds — don't paint over the retained pixels of culled siblings.
			c.Painter().StartClipping(pos, sz)
			c.Painter().Clear()
			return true, pos, sz
		}
	}
	c.Painter().Clear()
	drp.MarkCanvasFBOCleared()
	c.MarkFullRepaint()
	return false, geo.Position{}, geo.Size{}
}

func (c *glCanvas) paint(size geo.Size) {
	clips := &internal.ClipStack{}
	if c.Content() == nil {
		return
	}

	// --- Canvas FBO setup ---
	drp, hasDRP := c.Painter().(gpu.DirtyRegionPainter)

	canvasPixW := int(float32(scale.ToScreenCoordinate(c, size.Width)) * c.texScale)
	canvasPixH := int(float32(scale.ToScreenCoordinate(c, size.Height)) * c.texScale)

	// partialRepaint and drPos/drSize drive shouldSkipForDirtyRegion below.
	partialRepaint := false
	var drPos geo.Position
	var drSize geo.Size

	if hasDRP {
		drp.EnsureCanvasFBO(canvasPixW, canvasPixH)
		drp.BindCanvasFBO()
		// Partial repaint is on by default on desktop (build.PartialRepaint,
		// #496): only the dirty region is cleared and redrawn. Off, every frame
		// is a full clear and redraw.
		partialRepaint, drPos, drSize = c.clearCanvasFBO(drp, size)
	} else {
		c.Painter().SetOutputSize(canvasPixW, canvasPixH)
		c.Painter().Clear()
	}

	// Check if the painter supports boundary FBO caching.
	bp, hasBoundary := c.Painter().(gpu.BoundaryPainter)

	walk := &glPaintWalk{
		c:              c,
		clips:          clips,
		size:           size,
		bp:             bp,
		hasBoundary:    hasBoundary,
		partialRepaint: partialRepaint,
		drPos:          drPos,
		drSize:         drSize,
		canvasPixW:     canvasPixW,
		canvasPixH:     canvasPixH,
	}

	// Scope the whole paint pass to this canvas' direction so paint-time
	// alignment and text base direction match the layout pass (a no-op for the
	// default DirectionAuto, which defers to the global setting). Restore
	// truncates the stack back to its pre-walk depth even if a paint panic
	// unwinds past the per-node PushDirectionScope/PopDirectionScope Pops below.
	defer uidir.Restore(uidir.Mark())
	uidir.Push(c.Direction())
	defer uidir.Pop(c.Direction())

	// Paint content into the FBO (or default framebuffer if no FBO).
	//
	// For a partial repaint, the dirty-region clip established by clearCanvasFBO
	// stays active through the whole content walk: object culling alone is not
	// enough, because a redrawn object that overlaps the dirty region (e.g. a
	// full-canvas background) would otherwise paint over the retained pixels of
	// culled siblings outside the region. Seed the clip stack with the dirty region
	// so nested clips intersect with it and restore to it (not full) on exit.
	dirtyClip := partialRepaint && drSize.Width > 0 && drSize.Height > 0
	if dirtyClip {
		clips.Push(drPos, drSize)
	}
	// Clip-owners scrolled out of view are visited without their subtrees
	// (#464); every walk that must reach all objects still does.
	c.WalkContentTreesCulled(walk.cull, walk.draw, walk.afterDraw)
	if dirtyClip {
		c.Painter().StopClipping()
		clips.Pop()
	}

	// Blit canvas FBO to the default framebuffer, then paint overlays
	// directly on top so they aren't trapped inside the FBO.
	if hasDRP {
		drp.UnbindAndBlitCanvasFBO(canvasPixW, canvasPixH)
	}
	// The surface keeps nothing from the last frame, so overlays are drawn
	// whole every frame, whatever the dirty region.
	walk.partialRepaint = false
	c.WalkOverlayTrees(walk.draw, walk.afterDraw)

	c.RebuildSpatialIndex()
}

// glPaintWalk bundles the mutable and captured state for a single paint pass so
// the before/after-children callbacks can live as methods instead of closures.
type glPaintWalk struct {
	c     *glCanvas
	clips *internal.ClipStack
	size  geo.Size

	bp          gpu.BoundaryPainter
	hasBoundary bool

	partialRepaint bool
	drPos          geo.Position
	drSize         geo.Size

	canvasPixW, canvasPixH int

	// bStack is the boundary stack for RepaintBoundary FBO management.
	bStack []boundaryEntry
}

// insideCleanBoundary reports whether we are inside a clean (non-dirty) boundary.
func (w *glPaintWalk) insideCleanBoundary() bool {
	return len(w.bStack) > 0 && !w.bStack[len(w.bStack)-1].wasDirty
}

// cull is the paint walk's driver.CullFunc: common.CullOffClip, except while
// a boundary renders into its texture with no clip of its own open. The clip
// then comes from outside the boundary, and its cache must hold the whole
// subtree: content culled because it was off screen when the boundary
// rendered stayed missing once scrolled into view (#564). A clip opened
// inside the boundary still culls, so a scroll in it skips its hidden rows.
func (w *glPaintWalk) cull(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
	if ab := w.activeBoundary(); ab != nil && ab.wasDirty && w.clips.Top() == nil {
		return false
	}
	return common.CullOffClip(obj, pos, clipPos, clipSize)
}

// activeBoundary returns the active boundary entry (topmost dirty boundary), or nil.
func (w *glPaintWalk) activeBoundary() *boundaryEntry {
	if len(w.bStack) == 0 {
		return nil
	}
	return &w.bStack[len(w.bStack)-1]
}

// shouldSkipForDirtyRegion wraps the common.dirtyRect check with the
// partialRepaint guard so callers just ask "should I skip this object?".
func (w *glPaintWalk) shouldSkipForDirtyRegion(pos geo.Position, objSize geo.Size) bool {
	return w.partialRepaint && !common.RectsOverlap(pos, objSize, w.drPos, w.drSize)
}

// enterBoundary handles a RepaintBoundary node on the way down. It reports
// whether the node was a boundary (in which case the boundary widget itself is
// not painted).
func (w *glPaintWalk) enterBoundary(node *common.RenderCacheNode, pos geo.Position) bool {
	obj := node.Obj()
	if !w.hasBoundary || !node.IsBoundary() {
		return false
	}

	g := common.BoundaryGeometryFor(pos, obj.Size(), w.c.Scale()*w.c.texScale)
	pixW, pixH, objSize := g.PixW, g.PixH, g.Size

	if pixW > 0 && pixH > 0 {
		if w.bp.IsBoundaryClean(obj) && node.BoundaryRenderedAt(g.Phase) {
			// Clean boundary: push skip entry, children will be skipped.
			cleanState := w.bp.EnsureBoundaryFBO(obj, pixW, pixH, objSize)

			// If doing partial repaint and the boundary doesn't
			// intersect the dirty region, skip the blit too.
			w.bStack = append(w.bStack, boundaryEntry{
				obj:      obj,
				state:    cleanState,
				wasDirty: false,
				origin:   g.Origin,
				size:     objSize,
			})
			return true
		}

		// Dirty boundary: create/get FBO, bind it, render subtree into it.
		// Outer clips are in the enclosing target's space: render the subtree
		// with a clip stack of its own, and stop the scissor before the bind,
		// whose clear GL would scissor too (#559).
		state := w.bp.EnsureBoundaryFBO(obj, pixW, pixH, objSize)
		outerClips := w.clips
		w.clips = &internal.ClipStack{}
		w.c.Painter().StopClipping()
		w.bp.BindBoundaryFBO(state)

		invWT, ok := node.WorldTransform().Inverse()
		if !ok {
			invWT = geo.Identity()
		}
		// Content is drawn relative to the snapped origin, which keeps its
		// sub-pixel offset from the pixel grid.
		invWT = geo.NewTranslation(pos.X-g.Origin.X, pos.Y-g.Origin.Y).Multiply(invWT)

		w.bStack = append(w.bStack, boundaryEntry{
			obj:        obj,
			state:      state,
			wasDirty:   true,
			invWT:      invWT,
			origin:     g.Origin,
			phase:      g.Phase,
			size:       objSize,
			canvasPixW: w.canvasPixW,
			canvasPixH: w.canvasPixH,
			outerClips: outerClips,
		})
	}
	// Don't paint the boundary widget itself — it's just a container.
	return true
}

func (w *glPaintWalk) draw(node *common.RenderCacheNode, pos geo.Position) {
	obj := node.Obj()

	// Scope the subtree's paint to a container DirectionScope's direction, the
	// paint-time mirror of the EnsureMinSize layout hook. Done before any early
	// return so it always pairs with the pop in afterDraw (walkTree invokes both
	// callbacks for every node regardless of boundary/dirty-region skips).
	driver.PushDirectionScope(obj)

	// --- RepaintBoundary handling ---
	if w.enterBoundary(node, pos) {
		return
	}

	// If inside a clean boundary, skip all draw calls.
	if w.insideCleanBoundary() {
		return
	}

	// Determine effective pos, frame, and transform for rendering.
	paintPos := pos
	paintFrame := w.size
	paintTransform := node.WorldTransform()
	if ab := w.activeBoundary(); ab != nil && ab.wasDirty {
		// Inside a dirty boundary: adjust coordinates to FBO space.
		paintPos = geo.NewPos(pos.X-ab.origin.X, pos.Y-ab.origin.Y)
		paintFrame = ab.size
		paintTransform = ab.invWT.Multiply(node.WorldTransform())
	}

	// Establish this object's clips BEFORE the dirty-region cull check, so a
	// clip-owner (scroll/ClipToBounds/Card) that is itself outside the dirty region
	// still constrains its descendants. Otherwise a culled clip-owner skips its clip
	// push while its children — which may extend into the dirty region — get drawn
	// unclipped, leaking content past the viewport (pre-rebrand tracker #333). The clip is intersected
	// with the seeded dirty-region clip, so a clip-owner outside the region yields an
	// empty clip and its children draw nothing. A clip-owner the paint walk
	// culled has no children to constrain, and gets no clip (#481).
	clips := common.PaintClips(node)
	if clips.Rect {
		inner := w.clips.Push(paintPos, obj.Size())
		w.c.Painter().StartClipping(inner.Rect())
	}
	if clips.Rounded {
		w.c.Painter().StartRoundedClipping(paintPos, obj.Size(), clips.Radius)
	}
	if clips.Path != nil {
		if clipPath, is := clips.Path.(*canvas.Path); is {
			w.c.Painter().StartPathClipping(clipPath, paintPos, obj.Size())
		}
	}

	// Dirty-region culling: skip the object's own draw calls when it is entirely
	// outside the dirty region. Only applies when not inside a boundary FBO
	// (boundary FBOs manage their own dirty state). Clips were already established
	// above so descendants stay correctly clipped; afterDraw tears them down.
	if w.activeBoundary() == nil && w.shouldSkipForDirtyRegion(pos, obj.Size()) {
		return
	}

	// MD3 disabled state: reduce opacity for disabled widget subtrees.
	if d, ok := obj.(core.Disableable); ok && d.Disabled() {
		w.c.Painter().SetGroupOpacity(0.38)
	}
	w.c.Painter().SetTransform(paintTransform)
	if paintFrame.Width <= 0 || paintFrame.Height <= 0 {
		return
	}
	w.c.Painter().Paint(obj, paintPos, paintFrame)
}

// exitBoundary handles a RepaintBoundary node on the way up. It reports whether
// the node closed the topmost boundary entry.
func (w *glPaintWalk) exitBoundary(node *common.RenderCacheNode, pos geo.Position) bool {
	obj := node.Obj()
	if !(w.hasBoundary && node.IsBoundary() && len(w.bStack) > 0 && w.bStack[len(w.bStack)-1].obj == obj) {
		return false
	}

	entry := w.bStack[len(w.bStack)-1]
	w.bStack = w.bStack[:len(w.bStack)-1]

	if entry.wasDirty && entry.state != nil {
		// Finished rendering subtree into FBO. Unbind and mark clean.
		w.bp.UnbindBoundaryFBO(entry.canvasPixW, entry.canvasPixH)
		entry.state.SetDirty(false)
		node.SetBoundaryRenderedAt(entry.phase)
		w.clips = entry.outerClips
		w.applyClip()
	}

	// Blit the boundary's cached FBO texture at its snapped origin, where it
	// lands pixel for pixel, unless doing partial repaint and the boundary is
	// outside the dirty region.
	if !w.shouldSkipForDirtyRegion(entry.origin, entry.size) {
		w.bp.BlitBoundaryTexture(entry.state, entry.origin, w.size)
	}
	return true
}

// applyClip sets the painter's scissor to the top of the clip stack, or
// clears it when the stack is empty.
func (w *glPaintWalk) applyClip() {
	if top := w.clips.Top(); top != nil {
		w.c.Painter().StartClipping(top.Rect())
	} else {
		w.c.Painter().StopClipping()
	}
}

func (w *glPaintWalk) afterDraw(node *common.RenderCacheNode, pos geo.Position) {
	obj := node.Obj()

	// Pop the DirectionScope pushed in draw (before any early return, to pair).
	driver.PopDirectionScope(obj)

	// --- RepaintBoundary exit handling ---
	if w.exitBoundary(node, pos) {
		return
	}

	// If inside a clean boundary, skip afterPaint too.
	if w.insideCleanBoundary() {
		return
	}

	culled := w.activeBoundary() == nil && w.shouldSkipForDirtyRegion(pos, node.Obj().Size())

	// Group opacity is only set for objects that actually drew (not culled), so
	// only clear it then.
	if !culled {
		if d, ok := obj.(core.Disableable); ok && d.Disabled() {
			w.c.Painter().ClearGroupOpacity()
		}
	}

	// Tear down clips even when the object was culled: draw establishes them
	// unconditionally for clip-owners so descendants stay clipped (pre-rebrand tracker #333), so the
	// push/pop must stay paired regardless of culling.
	clips := common.PaintClips(node)
	if clips.Rounded {
		w.c.Painter().StopRoundedClipping()
	}
	if clips.Path != nil {
		w.c.Painter().StopPathClipping()
	}
	if clips.Rect {
		w.clips.Pop()
		w.applyClip()
	}

	if !culled && build.Mode == bellui.BuildDebug {
		w.c.DrawDebugOverlay(obj, pos, w.size)
	}
}

func (c *glCanvas) setContent(content core.CanvasObject) {
	c.content = content
	c.SetContentTreeAndFocusMgr(content)
}

func (c *glCanvas) setMenuOverlay(b core.CanvasObject) {
	c.menu = b
	c.SetMenuTreeAndFocusMgr(b)

	if c.menu != nil && !c.size.IsZero() {
		c.content.Resize(c.contentSize(c.size))
		c.content.Move(c.contentPos())

		c.menu.Refresh()
		c.menu.Resize(geo.NewSize(c.size.Width, c.menu.MinSize().Height))
	}
}

func (c *glCanvas) applyThemeOutOfTreeObjects() {
	if c.menu != nil {
		app.ApplyThemeTo(c.menu, c) // Ensure our menu gets the theme change message as it's out-of-tree
	}

	c.SetPadded(c.padded) // refresh the padding for potential theme differences
}

func newCanvas() *glCanvas {
	c := &glCanvas{scale: 1.0, texScale: 1.0, padded: true}
	connectKeyboard(c)
	c.Initialize(c, c.overlayChanged)
	c.setContent(&canvas.Rectangle{FillColor: theme.Color(theme.ColorNameBackground)})
	return c
}
