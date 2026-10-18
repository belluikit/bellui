package glfw

import (
	"context"
	"image/color"
	_ "image/png" // for the icon
	"math"
	"runtime"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/scale"
	internalWidget "github.com/belluikit/bellui/internal/widget"
)

const (
	dragMoveThreshold = 2 // how far can we move before it is a drag
	windowIconSize    = 256
)

func (w *window) Title() string {
	return w.title
}

func (w *window) SetTitle(title string) {
	w.title = title

	w.runOnMainWhenCreated(func() {
		w.view().SetTitle(title)
	})
}

func (w *window) FullScreen() bool {
	return w.fullScreen || w.fullScreenSecondary
}

// minSizeOnScreen gets the padded minimum size of a window content in screen pixels
func (w *window) minSizeOnScreen() (int, int) {
	// get minimum size of content inside the window
	return w.screenSize(w.canvas.MinSize())
}

// screenSize computes the actual output size of the given content size in screen pixels
func (w *window) screenSize(canvasSize geo.Size) (int, int) {
	return scale.ToScreenCoordinate(w.canvas, canvasSize.Width), scale.ToScreenCoordinate(w.canvas, canvasSize.Height)
}

func (w *window) Resize(size geo.Size) {
	w.canvas.size = size
	// we cannot perform this until window is prepared as we don't know its scale!
	bigEnough := size.Max(w.canvas.canvasSize(w.canvas.Content().MinSize()))
	w.runOnMainWhenCreated(func() {
		width, height := scale.ToScreenCoordinate(w.canvas, bigEnough.Width), scale.ToScreenCoordinate(w.canvas, bigEnough.Height)
		if w.fixedSize || !w.visible { // fixed size ignores future `resized` and if not visible we may not get the event
			w.shouldWidth, w.shouldHeight = width, height
			w.width, w.height = width, height
		}

		w.requestedWidth, w.requestedHeight = width, height
		if runtime.GOOS != "js" {
			w.view().SetSize(width, height)
			w.processResized(width, height)
		}
	})
}

func (w *window) FixedSize() bool {
	return w.fixedSize
}

func (w *window) SetFixedSize(fixed bool) {
	w.fixedSize = fixed
	w.runOnMainWhenCreated(func() {
		w.fitContent()
		if !w.centered {
			w.processResized(w.width, w.height)
		}
	})
}

func (w *window) Padded() bool {
	return w.canvas.padded
}

func (w *window) SetPadded(padded bool) {
	w.canvas.SetPadded(padded)

	w.runOnMainWhenCreated(w.fitContent)
}

func (w *window) SelectableMode() bellui.SelectableMode {
	return w.canvas.SelectableMode()
}

func (w *window) SetSelectableMode(mode bellui.SelectableMode) {
	w.canvas.SetSelectableMode(mode)
}

func (w *window) Icon() bellui.Resource {
	if w.icon == nil {
		return bellui.CurrentApp().Icon()
	}

	return w.icon
}

func (w *window) MainMenu() *bellui.MainMenu {
	return w.mainmenu
}

func (w *window) SetMainMenu(menu *bellui.MainMenu) {
	if w.mainmenu == menu {
		// Same menu object — no structural change. On macOS, validateMenuItem:
		// already handles Checked/Enabled state updates dynamically.
		// Rebuilding the native menu here would destroy NSMenuItems that AppKit
		// may still be referencing, causing use-after-free crashes.
		return
	}
	w.mainmenu = menu
	w.runOnMainWhenCreated(func() {
		w.canvas.buildMenu(w, menu)
	})
}

func (w *window) SetOnClosed(closed func()) {
	w.onClosed = closed
}

func (w *window) SetCloseIntercept(callback func()) {
	w.onCloseIntercepted = callback
}

func (w *window) calculatedScale() float32 {
	return calculateScale(userScale(), bellui.CurrentDevice().SystemScaleForWindow(w), w.detectScale())
}

func (w *window) detectTextureScale() float32 {
	view := w.view()
	winWidth, _ := view.GetSize()
	texWidth, _ := view.GetFramebufferSize()
	return float32(texWidth) / float32(winWidth)
}

func (w *window) Show() {
	async.EnsureMain(func() {
		if w.view() != nil {
			w.doShowAgain()
			return
		}

		if !w.created {
			w.created = true
			w.create()
		}

		if w.view() == nil {
			return
		}

		w.visible = true
		view := w.view()
		view.SetTitle(w.title)

		if !build.IsWayland && w.centered {
			w.doCenterOnScreen() // lastly center if that was requested
		}
		view.Show()

		// a native parent/owner relationship (pre-rebrand tracker #598) needs both native windows
		// to exist; the child is shown after its parent, so apply it now.
		w.applyParent()

		// native window-modality (pre-rebrand tracker #498) is expressed against that parent, so
		// apply it once both windows exist too.
		w.applyModal()

		// an attention request (pre-rebrand tracker #517) made before the window mapped is honoured
		// now that a native window exists (no-op on the web target).
		w.applyAttention()

		// a taskbar progress request (pre-rebrand tracker #517) likewise needs the native taskbar
		// button, which exists once the window is shown.
		w.applyProgress()

		// save coordinates
		if !build.IsWayland {
			w.xpos, w.ypos = view.GetPos()
		}

		// this does not work if called before viewport.Show()
		if w.fullScreenSecondary {
			w.doSetFullScreen2(true)
		} else if w.fullScreen {
			w.doSetFullScreen(true)
		}

		// show top canvas element
		if content := w.canvas.Content(); content != nil {
			w.RunWithContext(func() {
				w.driver.repaintWindow(w)
			})
			w.updateAccessibility()
		}
	})
}

func (w *window) Hide() {
	async.EnsureMain(func() {
		if w.closing || w.viewport == nil {
			return
		}

		w.visible = false
		w.viewport.Hide()
	})
}

func (w *window) Close() {
	async.EnsureMain(func() {
		if w.isClosing() {
			return
		}

		// trigger callbacks - early so window still exists
		if fn := w.onClosed; fn != nil {
			w.onClosed = nil // avoid possibility of calling twice
			fn()
		}

		w.closing = true
		// Abandon a pending double-tap before the canvas goes: its timer runs
		// for up to TapDoubleDelay after the gesture and would dispatch into
		// widgets this close is about to tear down (#310).
		w.cancelPendingTap()
		w.viewport.SetShouldClose(true)

		cache.RangeTexturesFor(w.canvas, w.canvas.Painter().Free)
		w.canvas.WalkTrees(nil, func(node *common.RenderCacheNode, _ geo.Position) {
			if wid, ok := node.Obj().(core.Widget); ok {
				cache.DestroyRenderer(wid)
			}
		})
	})
}

func (w *window) ShowAndRun() {
	w.Show()
	bellui.CurrentApp().Run()
}

// Clipboard returns the system clipboard
func (w *window) Clipboard() bellui.Clipboard {
	return NewClipboard()
}

func (w *window) Content() core.CanvasObject {
	return w.canvas.Content()
}

func (w *window) SetContent(content core.CanvasObject) {
	w.canvas.SetContent(content)

	async.EnsureMain(func() {
		w.RunWithContext(w.RescaleContext)
	})
}

func (w *window) Canvas() core.Canvas {
	return w.canvas
}

func (w *window) processClosed() {
	if w.onCloseIntercepted != nil {
		w.onCloseIntercepted()
		return
	}

	w.Close()
}

// destroy this window and, if it's the last window quit the app
func (w *window) destroy(d *gLDriver) {
	cache.CleanCanvas(w.canvas)

	if w.master {
		d.Quit()
	} else if runtime.GOOS == "darwin" {
		d.focusPreviousWindow()
	}
}

func (w *window) drainPendingEvents() {
	for _, fn := range w.pending {
		fn()
	}
	w.pending = nil
}

func (w *window) processMoved(x, y int) {
	if !w.fullScreen { // don't save the move to top left when changing to fullscreen
		// save coordinates
		w.xpos, w.ypos = x, y
	}

	if w.canvas.detectedScale == w.detectScale() {
		return
	}

	w.canvas.detectedScale = w.detectScale()
	w.canvas.reloadScale()
}

func (w *window) processResized(width, height int) {
	canvasSize := w.computeCanvasSize(width, height)
	if !w.fullScreen {
		w.width = scale.ToScreenCoordinate(w.canvas, canvasSize.Width)
		w.height = scale.ToScreenCoordinate(w.canvas, canvasSize.Height)
	}

	if !w.visible { // don't redraw if hidden
		w.canvas.Resize(canvasSize)
		return
	}

	if w.fixedSize {
		w.canvas.Resize(canvasSize)
		w.fitContent()
		return
	}

	w.RunWithContext(func() {
		w.platformResize(canvasSize)
	})
}

func (w *window) processFrameSized(width, height int) {
	if width == 0 || height == 0 || runtime.GOOS != "darwin" {
		return
	}

	winWidth, _ := w.view().GetSize()
	newTexScale := float32(width) / float32(winWidth) // This will be > 1.0 on a HiDPI screen
	if w.canvas.texScale != newTexScale {
		w.canvas.texScale = newTexScale
		w.canvas.Refresh(w.canvas.Content()) // reset graphics to apply texture scale
	}
}

func (w *window) processRefresh() {
	refreshWindow(w)
}

func (w *window) findObjectAtPositionMatching(canvas *glCanvas, mouse geo.Position, matches func(object core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	return canvas.FindObjectAtPositionMatching(mouse, matches)
}

func (w *window) processMouseMoved(xpos float64, ypos float64) {
	previousPos := w.mousePos
	w.mousePos = geo.NewPos(scale.ToBelluiCoordinate(w.canvas, int(xpos)), scale.ToBelluiCoordinate(w.canvas, int(ypos)))
	mousePos := w.mousePos
	mouseDragPos := w.mouseDragPos
	mouseOver := w.mouseOver

	w.cancelLongPressOnMove(mousePos)

	cursor := desktop.Cursor(desktop.DefaultCursor)

	obj, pos, _ := w.findObjectAtPositionMatching(w.canvas, mousePos, func(object core.CanvasObject) bool {
		if cursorable, ok := object.(desktop.Cursorable); ok {
			cursor = cursorable.Cursor()
		}

		_, hover := object.(event.Hoverable)
		return hover
	})

	// While dragging a DnD-opted-in source (pre-rebrand tracker #819), force the grabbing cursor over
	// whatever is under the pointer. Gated on DragImageProvider so ordinary
	// draggables (sliders, scrollbars) keep their normal cursor; a source wanting
	// the cursor without a ghost implements it and returns a nil DragImage.
	if w.mouseDragged != nil {
		if _, ok := w.mouseDragged.(core.DragImageProvider); ok {
			cursor = desktop.Cursor(desktop.GrabbingCursor)
		}
	}

	if w.cursor != cursor {
		// cursor has changed, store new cursor and apply change via glfw
		rawCursor, isCustomCursor := belluiToNativeCursor(cursor)
		w.cursor = cursor

		if view := w.view(); view != nil { // not yet visible? linux weirdness
			if rawCursor == nil {
				view.SetInputMode(CursorMode, CursorHidden)
			} else {
				view.SetInputMode(CursorMode, CursorNormal)
				w.SetCursor(rawCursor)
			}
		}
		w.setCustomCursor(rawCursor, isCustomCursor)
	}

	if w.mouseButton != 0 && w.mouseButton != desktop.MouseButtonSecondary && !w.mouseDragStarted {
		obj, pos, _ := w.findObjectAtPositionMatching(w.canvas, previousPos, func(object core.CanvasObject) bool {
			_, ok := object.(event.Draggable)
			return ok
		})

		deltaX := mousePos.X - mouseDragPos.X
		deltaY := mousePos.Y - mouseDragPos.Y
		overThreshold := math.Abs(float64(deltaX)) >= dragMoveThreshold || math.Abs(float64(deltaY)) >= dragMoveThreshold

		if wid, ok := obj.(event.Draggable); ok && overThreshold {
			w.mouseDragged = wid
			w.mouseDraggedOffset = previousPos.Subtract(pos)
			w.mouseDraggedObjStart = obj.Position()
			w.mouseDragStarted = true
			w.startDragGhost(wid, mousePos)
			w.beginDragTargeting(wid)
		}
	}

	if obj != nil && !w.objIsDragged(obj) {
		ev := &event.HoverEvent{}
		ev.AbsolutePosition = mousePos
		ev.Position = pos

		if hovered, ok := obj.(event.Hoverable); ok {
			if hovered == mouseOver {
				hovered.HoverMoved(ev)
			} else {
				w.mouseOut()
				w.mouseIn(hovered, ev)
			}
		} else if mouseOver != nil {
			isChild := false
			driver.WalkCompleteObjectTree(mouseOver.(core.CanvasObject),
				func(co core.CanvasObject, p1, p2 geo.Position, s geo.Size) bool {
					if co == obj {
						isChild = true
						return true
					}
					return false
				}, nil)
			if !isChild {
				w.mouseOut()
			}
		}
	} else if mouseOver != nil && !w.objIsDragged(mouseOver) {
		w.mouseOut()
	}

	w.updateToolTip(mousePos)

	mouseDragged := w.mouseDragged
	mouseDragPos = w.mouseDragPos
	if mouseDragged != nil && w.mouseButton != desktop.MouseButtonSecondary {
		if w.mouseButton > 0 {
			draggedObjDelta := w.mouseDraggedObjStart.Subtract(mouseDragged.(core.CanvasObject).Position())
			ev := &event.DragEvent{}
			ev.AbsolutePosition = mousePos
			ev.Position = mousePos.Subtract(w.mouseDraggedOffset).Add(draggedObjDelta)
			ev.Dragged = geo.NewDelta(mousePos.X-mouseDragPos.X, mousePos.Y-mouseDragPos.Y)
			wd := mouseDragged
			wd.Dragged(ev)

			// A native drag-out started inside Dragged hands the rest of the
			// gesture to the OS (asynchronously on macOS/Linux): the ending
			// mouse-up is consumed by the native session and never delivered to
			// this window. End the bellui-side gesture now — otherwise the stale
			// pressed state keeps emitting Dragged whenever the pointer passes
			// back over the window, spawning phantom native drags that fight the
			// real one over the platform drag state (pre-rebrand tracker #933). Windows' modal
			// DoDragDrop already cleared the state via its nested release (see
			// below), so only the flag is consumed there.
			if consumeNativeDragOutStarted() && w.mouseDragged != nil {
				w.mouseDragged.DragEnd()
				w.mouseDragStarted = false
				w.endDragGhost()
				w.finishDragTarget(mousePos)
				w.mouseDragged = nil
				w.mouseButton = 0
				return
			}
		}

		// A native drag-out (Windows DoDragDrop) runs a modal message loop inside
		// Dragged and swallows the ending mouse-up, so a nested release already
		// cleared the drag state (mouseDragged, mouseDragStarted). Don't re-arm it
		// in that case, or bellui stays "mid-drag" with a nil target and never
		// starts another drag. Non-blocking backends (macOS NSDraggingSession,
		// Linux XDND) return from Dragged with the drag still active, so they take
		// the normal path.
		if w.mouseDragged != nil {
			if w.dragGhost != nil {
				w.dragGhost.SetPosition(dragGhostPos(mousePos))
			}
			if w.dndDrag {
				w.updateDragTarget(mousePos)
			}

			w.mouseDragStarted = true
			w.mouseDragPos = mousePos
		}
	}
}

func (w *window) objIsDragged(obj any) bool {
	if w.mouseDragged != nil && obj != nil {
		draggedObj, _ := obj.(event.Draggable)
		return draggedObj == w.mouseDragged
	}
	return false
}

func (w *window) mouseIn(obj event.Hoverable, ev *event.HoverEvent) {
	if obj != nil {
		obj.HoverIn(ev)
	}
	w.mouseOver = obj
}

func (w *window) mouseOut() {
	mouseOver := w.mouseOver
	if mouseOver != nil {
		mouseOver.HoverOut()
		w.mouseOver = nil
	}
}

// updateToolTip resolves the deepest object with tooltip text under the pointer
// and, when that target changes, tells the tooltip controller to show it (after
// a delay) or hide the current one. Tooltips are suppressed while a mouse button
// is held or a drag is in progress. Unlike the overlay-based widget.Tooltip,
// this rides the object's own hover — delivered reliably by the driver — so it
// works on the desktop driver (pre-rebrand tracker #800).
func (w *window) updateToolTip(mousePos geo.Position) {
	var target core.CanvasObject
	if w.mouseButton == 0 && !w.mouseDragStarted {
		obj, _, _ := w.findObjectAtPositionMatching(w.canvas, mousePos, func(o core.CanvasObject) bool {
			tt, ok := o.(core.ToolTipable)
			return ok && tt.ToolTip() != ""
		})
		target = obj
	}
	if target == w.tooltipObj {
		return
	}
	w.tooltipObj = target
	core.NotifyToolTip(target, w.canvas)
}

// hideToolTip immediately dismisses any pending or visible tooltip and clears
// the tracked target, so a fresh hover re-triggers the delay.
func (w *window) hideToolTip() {
	if w.tooltipObj == nil {
		return
	}
	w.tooltipObj = nil
	core.NotifyToolTip(nil, w.canvas)
}

// dragGhostOffset keeps the ghost slightly down-right of the pointer so it isn't
// hidden under the cursor.
const dragGhostOffset = float32(10)

func dragGhostPos(pointer geo.Position) geo.Position {
	return pointer.Add(geo.NewPos(dragGhostOffset, dragGhostOffset))
}

// startDragGhost paints a follow-the-pointer ghost for a drag source that opts
// into in-canvas DnD affordances via core.DragImageProvider (pre-rebrand tracker #819). A nil image
// means the source wants the grab cursor but no ghost.
func (w *window) startDragGhost(src event.Draggable, pointer geo.Position) {
	provider, ok := src.(core.DragImageProvider)
	if !ok {
		return
	}
	img := provider.DragImage()
	if img == nil {
		return
	}
	ghost := internalWidget.NewDragGhost(img, dragGhostPos(pointer))
	w.canvas.Overlays().Add(ghost)
	ghost.SetPosition(dragGhostPos(pointer)) // force initial layout of the content
	w.dragGhost = ghost
}

// endDragGhost removes the drag ghost overlay when the drag finishes.
func (w *window) endDragGhost() {
	if w.dragGhost == nil {
		return
	}
	w.canvas.Overlays().Remove(w.dragGhost)
	w.dragGhost = nil
}

// beginDragTargeting marks whether the starting drag is an in-canvas DnD drag
// (the source opted in via DragImageProvider or DragPayloadProvider) and reads
// the source-declared payload once. Only DnD drags route to DragTargets, so a
// plain draggable (slider, scrollbar) never triggers drop-target callbacks (pre-rebrand tracker #819).
func (w *window) beginDragTargeting(src event.Draggable) {
	_, hasImage := src.(core.DragImageProvider)
	pp, hasPayload := src.(core.DragPayloadProvider)
	w.dndDrag = hasImage || hasPayload
	if hasPayload {
		w.dragPayload = pp.DragPayload()
	}
}

// dragTargetAt resolves the deepest core.DragTarget under the pointer, with its
// position relative to that target.
func (w *window) dragTargetAt(pos geo.Position) (core.DragTarget, geo.Position) {
	obj, rel, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(o core.CanvasObject) bool {
		_, ok := o.(core.DragTarget)
		return ok
	})
	target, _ := obj.(core.DragTarget)
	return target, rel
}

// updateDragTarget routes DragEnter/DragOver/DragLeave to the deepest DragTarget
// under the pointer during an in-canvas DnD drag, mirroring how processDropped
// routes an OS file drop to the deepest bellui.Droppable.
func (w *window) updateDragTarget(pos geo.Position) {
	target, rel := w.dragTargetAt(pos)
	if target == w.dragTarget {
		if target != nil {
			target.DragOver(&core.DragTargetEvent{AbsolutePosition: pos, Position: rel, Payload: w.dragPayload})
		}
		return
	}
	if w.dragTarget != nil {
		w.dragTarget.DragLeave()
	}
	w.dragTarget = target
	if target != nil {
		target.DragEnter(&core.DragTargetEvent{AbsolutePosition: pos, Position: rel, Payload: w.dragPayload})
	}
}

// finishDragTarget fires Drop on the target under the release point (or DragLeave
// if the pointer left it) and clears the DnD drag state.
func (w *window) finishDragTarget(pos geo.Position) {
	if w.dndDrag {
		target, rel := w.dragTargetAt(pos)
		if target != nil && target == w.dragTarget {
			target.Drop(&core.DragTargetEvent{AbsolutePosition: pos, Position: rel, Payload: w.dragPayload})
		} else if w.dragTarget != nil {
			w.dragTarget.DragLeave()
		}
	}
	w.dragTarget = nil
	w.dragPayload = nil
	w.dndDrag = false
}

// refreshCursorOnClick makes processMouseClicked re-query the OS cursor position
// instead of trusting the cached w.mousePos. The click callback carries no
// coordinates and the cache is only as fresh as the last move callback — which
// may never have fired (unfocused window, darwin mostly) or may still be in
// flight when a synthesized click arrives (pre-rebrand tracker #482). Driver tests disable this as
// they simulate input by invoking the callbacks directly, where the live OS
// cursor is unrelated to the simulated positions.
var refreshCursorOnClick = true

func (w *window) processMouseClicked(button desktop.MouseButton, action action, modifiers event.KeyModifier) {
	if view := w.view(); refreshCursorOnClick && view != nil {
		xpos, ypos := view.GetCursorPos()
		w.mousePos = geo.NewPos(scale.ToBelluiCoordinate(w.canvas, int(xpos)), scale.ToBelluiCoordinate(w.canvas, int(ypos)))
	}
	w.mouseDragPos = w.mousePos
	mousePos := w.mousePos
	mouseDragStarted := w.mouseDragStarted

	if action == press {
		w.dismissTapThroughOverlay(mousePos)
		w.hideToolTip() // a click dismisses any visible/pending tooltip
	}

	co, pos, _ := w.findObjectAtPositionMatching(w.canvas, mousePos, func(object core.CanvasObject) bool {
		switch object.(type) {
		case event.Tappable, event.SecondaryTappable, event.DoubleTappable, event.Focusable, desktop.Mouseable:
			return true
		case event.Draggable:
			if mouseDragStarted {
				return true
			}
		}

		return false
	})
	ev := &event.PointEvent{
		Position:         pos,
		AbsolutePosition: mousePos,
	}

	coMouse := co
	if wid, ok := co.(desktop.Mouseable); ok {
		mev := &desktop.MouseEvent{
			Button:   button,
			Modifier: modifiers,
		}
		mev.Position = ev.Position
		mev.AbsolutePosition = mousePos
		w.mouseClickedHandleMouseable(mev, action, wid)
	}

	if wid, ok := co.(event.Focusable); !ok || wid != w.canvas.Focused() {
		ignore := false
		_, _, _ = w.findObjectAtPositionMatching(w.canvas, mousePos, func(object core.CanvasObject) bool {
			switch object.(type) {
			case event.Focusable:
				ignore = true
				return true
			}

			return false
		})

		if !ignore { // if a parent item under the mouse has focus then ignore this tap unfocus
			w.canvas.Unfocus()
		}
	}

	switch action {
	case press:
		w.mouseButton |= button
		w.armLongPress(button, mousePos)
	case release:
		w.mouseButton &= ^button
	}

	mouseDragged := w.mouseDragged
	mouseDragStarted = w.mouseDragStarted
	mouseOver := w.mouseOver
	shouldMouseOut := w.objIsDragged(mouseOver) && !w.objIsDragged(coMouse)
	mousePressed := w.mousePressed

	if action == release && mouseDragged != nil {
		if mouseDragStarted {
			mouseDragged.DragEnd()
			w.mouseDragStarted = false
		}
		w.endDragGhost()
		w.finishDragTarget(mousePos)
		if shouldMouseOut {
			w.mouseOut()
		}
		w.mouseDragged = nil
	}

	_, tap := co.(event.Tappable)
	secondary, altTap := co.(event.SecondaryTappable)
	if tap || altTap {
		switch action {
		case press:
			w.mousePressed = co
		case release:
			if co == mousePressed && button == desktop.MouseButtonSecondary && altTap {
				secondary.TappedSecondary(ev)
			}
		}
	}

	// Check for double click/tap on left mouse button
	if action == release && button == desktop.MouseButtonPrimary && !mouseDragStarted {
		w.mouseClickedHandleTapDoubleTap(co, ev)
	}
}

// cancelLongPressOnMove invalidates a pending long-press fire once the
// pointer travels past the drag threshold (#266): that press is a drag or a
// selection sweep, not a press-and-hold. The origin is the armed press
// position, not the last move, so slow sub-threshold creep accumulates and
// cancels. A pending fire needs no release-time cancellation beyond this:
// fireLongPress re-checks that the primary button is still the only one down,
// and arming the next press bumps the generation.
func (w *window) cancelLongPressOnMove(mousePos geo.Position) {
	if w.mouseButton&desktop.MouseButtonPrimary == 0 {
		return
	}
	if math.Abs(float64(mousePos.X-w.longPressPos.X)) >= dragMoveThreshold ||
		math.Abs(float64(mousePos.Y-w.longPressPos.Y)) >= dragMoveThreshold {
		w.longPressSeq++
	}
}

// armLongPress schedules a hold-fire long-press for the primary-button press
// that just went down (#266): a press held stationary for
// common.TapSecondaryDelay dispatches LongPressed to the LongPressable under
// the pointer while the button is still down — the desktop counterpart of the
// touch drivers' hold-fire (pre-rebrand tracker #758), sharing their timing and their
// cancellation (movement past the drag threshold, release, another button).
// Deliberately for every primary press, mouse included, not just synthesized
// touch: press-and-hold under a mouse is how touch-first toolkits behave
// (GTK's GtkGestureLongPress fires for mice by default), and on a desktop
// touchscreen the synthesized events are indistinguishable anyway.
func (w *window) armLongPress(button desktop.MouseButton, pos geo.Position) {
	w.longPressSeq++ // a new press invalidates any pending fire
	if button == desktop.MouseButtonPrimary {
		// Only a primary press begins a fresh gesture, and only a primary
		// release consults the flag. Clearing it for any button would let a
		// right-click chorded into a hold-fired press hand the tap back to
		// the primary release the hold must silence.
		w.longPressFired = false
	}
	if w.mouseButton != desktop.MouseButtonPrimary {
		return // chorded or non-primary press: not a press-and-hold
	}

	co, _, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(object core.CanvasObject) bool {
		_, ok := object.(event.LongPressable)
		return ok
	})
	if _, ok := co.(event.LongPressable); !ok {
		return // nothing here can long-press; spawn no timer for plain clicks
	}

	w.longPressPos = pos
	seq := w.longPressSeq
	time.AfterFunc(common.TapSecondaryDelay, func() {
		runOnMain(func() {
			w.fireLongPress(seq)
		})
	})
}

// fireLongPress runs on the main goroutine when the hold timer elapses. It
// re-resolves the target under the pointer — overlays may have changed since
// the press was armed — and dispatches LongPressed if the press is still a
// stationary, un-chorded primary hold.
func (w *window) fireLongPress(seq int) {
	if w.longPressSeq != seq || w.mouseButton != desktop.MouseButtonPrimary || w.mouseDragStarted {
		return // released, moved, chorded or dragging since armed
	}

	co, pos, _ := w.findObjectAtPositionMatching(w.canvas, w.mousePos, func(object core.CanvasObject) bool {
		_, ok := object.(event.LongPressable)
		return ok
	})
	wid, ok := co.(event.LongPressable)
	if !ok {
		return
	}

	w.longPressFired = true
	ev := &event.PointEvent{
		Position:         pos,
		AbsolutePosition: w.mousePos,
	}
	wid.LongPressed(ev)
}

// dismissTapThroughOverlay implements the outside-tap pass-through for
// tap-through overlays (pre-rebrand tracker #472: OverlayContainer.TapThrough; pre-rebrand tracker #569: transient
// PopUps like Toast/Snackbar): when the top overlay opted in and a press
// lands outside its content, the overlay is dismissed before the press is
// dispatched. Hit testing then resolves the same click against the layer
// beneath, so one click both closes e.g. a completion popup and places the
// caret in the editor under it. The overlay is matched structurally so the
// driver needs no widget-package import.
func (w *window) dismissTapThroughOverlay(pos geo.Position) {
	if td, ok := w.canvas.Overlays().Top().(interface {
		TapThroughDismiss(geo.Position) bool
	}); ok {
		td.TapThroughDismiss(pos)
	}
}

func (w *window) mouseClickedHandleMouseable(mev *desktop.MouseEvent, action action, wid desktop.Mouseable) {
	switch action {
	case press:
		wid.MouseDown(mev)
	case release:
		mouseDragged := w.mouseDragged
		mouseDraggedOffset := w.mouseDraggedOffset
		if mouseDragged == nil {
			wid.MouseUp(mev)
		} else {
			if dragged, ok := mouseDragged.(desktop.Mouseable); ok {
				mev.Position = mev.AbsolutePosition.Subtract(mouseDraggedOffset)
				dragged.MouseUp(mev)
			} else {
				wid.MouseUp(mev)
			}
		}
	}
}

func (w *window) mouseClickedHandleTapDoubleTap(co core.CanvasObject, ev *event.PointEvent) {
	if w.longPressFired {
		// The hold already dispatched LongPressed (#266): the release of a
		// hold-fired press is not a tap, exactly as on the touch drivers.
		// Unlike them it is not a secondary tap either — desktop right-click
		// owns that surface, and mirroring the mobile lift-opens-the-menu
		// pattern here would pop context menus on slow clicks. (A hold that
		// turned into a drag skips this path; armLongPress clears the stale
		// flag at the next primary press.)
		w.longPressFired = false
		w.mousePressed = nil
		return
	}

	_, doubleTap := co.(event.DoubleTappable)
	if doubleTap {
		w.mouseClickCount++
		w.mouseLastClick = co

		mouseCancelFunc := w.mouseCancelFunc
		if mouseCancelFunc != nil {
			mouseCancelFunc()
			return
		}

		go w.waitForDoubleTap(co, ev)
	} else {
		if wid, ok := co.(event.Tappable); ok && co == w.mousePressed {
			wid.Tapped(ev)
			core.NotifyActivation(co, false)
		}
		w.mousePressed = nil
	}
}

func (w *window) waitForDoubleTap(co core.CanvasObject, ev *event.PointEvent) {
	ctx, mouseCancelFunc := context.WithDeadline(context.TODO(), time.Now().Add(w.driver.DoubleTapDelay()))
	defer runOnMain(mouseCancelFunc)
	runOnMain(func() {
		w.mouseCancelFunc = mouseCancelFunc
	})

	<-ctx.Done()

	runOnMain(func() {
		w.waitForDoubleTapEnded(co, ev)
	})
}

func (w *window) waitForDoubleTapEnded(co core.CanvasObject, ev *event.PointEvent) {
	if w.mouseClickCount == 2 && w.mouseLastClick == co {
		if wid, ok := co.(event.DoubleTappable); ok {
			wid.DoubleTapped(ev)
		}
	} else if co == w.mousePressed {
		if wid, ok := co.(event.Tappable); ok {
			wid.Tapped(ev)
			core.NotifyActivation(co, false)
		}
	}

	w.mouseClickCount = 0
	w.mousePressed = nil
	w.mouseCancelFunc = nil
	w.mouseLastClick = nil
}

func (w *window) processMouseScrolled(xoff float64, yoff float64) {
	if math.Abs(xoff) >= scrollAccelerateCutoff {
		xoff *= scrollAccelerateRate
	}
	if math.Abs(yoff) >= scrollAccelerateCutoff {
		yoff *= scrollAccelerateRate
	}

	delta := geo.NewDelta(float32(xoff)*scrollSpeed, float32(yoff)*scrollSpeed)
	w.dispatchScroll(w.mousePos, delta, event.ScrollSourceMouseWheel)
}

// dispatchScroll delivers a scroll of delta to the scrollables under pos,
// deepest to shallowest: a scrollable that implements ScrollConsumer and
// reports the event not consumed (at boundary, or content fits) passes it to
// the next parent scrollable. The wheel path arrives here with its
// acceleration and speed scaling already applied; the touch scroll path (#291)
// passes finger deltas 1:1. Reports whether any scrollable consumed the
// scroll, so touch momentum can stop dead at a boundary instead of decaying
// against it.
func (w *window) dispatchScroll(pos geo.Position, delta geo.Delta, source event.ScrollSource) bool {
	return w.dispatchScrollChain(w.scrollablesAt(pos), pos, delta, source, false)
}

// scrollablesAt returns the scrollables under pos, innermost first: the
// propagation chain a scroll there is offered to.
func (w *window) scrollablesAt(pos geo.Position) []driver.ObjectAtPosition {
	isScrollable := func(object core.CanvasObject) bool {
		_, ok := object.(event.Scrollable)
		return ok
	}

	overlay := w.canvas.Overlays().Top()
	var scrollables []driver.ObjectAtPosition
	if overlay != nil {
		scrollables = driver.FindAllObjectsAtPositionMatching(pos, isScrollable, overlay)
		// If no scrollable found in overlay, fall through to content beneath.
		if len(scrollables) == 0 {
			if idx := w.canvas.SpatialIndex(); idx != nil && !w.canvas.IsDirty() {
				scrollables = idx.FindAllObjectsAtPositionMatching(pos, isScrollable)
			} else {
				scrollables = driver.FindAllObjectsAtPositionMatching(pos, isScrollable,
					nil, w.canvas.menu, w.canvas.Content())
			}
		}
	} else if idx := w.canvas.SpatialIndex(); idx != nil && !w.canvas.IsDirty() {
		scrollables = idx.FindAllObjectsAtPositionMatching(pos, isScrollable)
	} else {
		scrollables = driver.FindAllObjectsAtPositionMatching(pos, isScrollable,
			nil, w.canvas.menu, w.canvas.Content())
	}

	return scrollables
}

// dispatchScrollChain offers a scroll at pos to the scrollables of chain,
// innermost first, until one consumes it. With relocate, each one's relative
// position is worked out again from where it is now: a chain kept for a whole
// touch gesture moves while the gesture scrolls it.
func (w *window) dispatchScrollChain(scrollables []driver.ObjectAtPosition, pos geo.Position, delta geo.Delta, source event.ScrollSource, relocate bool) bool {
	if len(scrollables) == 0 {
		return false
	}

	ev := &event.ScrollEvent{}
	ev.Scrolled = delta
	ev.AbsolutePosition = pos
	ev.Source = source

	for _, oap := range scrollables {
		ev.Position = oap.Position
		if relocate {
			ev.Position = pos.Subtract(driver.AbsolutePositionForObject(oap.Object, w.canvas.ObjectTrees()))
		}
		if consumer, ok := oap.Object.(driver.ScrollConsumer); ok {
			if consumer.TryScrolled(ev) {
				return true
			}
			continue
		}
		// Fallback: plain Scrollable without propagation support, always consumes.
		if wid, ok := oap.Object.(event.Scrollable); ok {
			wid.Scrolled(ev)
			return true
		}
	}
	return false
}

// processKeyReleased is the release half of the key callback: menu toggles
// armed on press fire here, then the raw key-up reaches the focused widget or
// the canvas hook.
func (w *window) processKeyReleased(keyEvent *event.KeyEvent, pendingMenuToggle, pendingMenuDeactivation event.KeyName) {
	if keyEvent.Name != "" {
		switch keyEvent.Name {
		case pendingMenuToggle:
			w.canvas.ToggleMenu()
		case pendingMenuDeactivation:
			if w.canvas.DismissMenu() {
				return
			}
		}
	}

	if w.canvas.Focused() != nil {
		if focused, ok := w.canvas.Focused().(desktop.Keyable); ok {
			focused.KeyUp(keyEvent)
		}
	} else if w.canvas.onKeyUp != nil {
		w.canvas.onKeyUp(keyEvent)
	}
}

func (w *window) processKeyPressed(keyName event.KeyName, keyASCII event.KeyName, scancode int, action action, keyDesktopModifier event.KeyModifier) {
	keyEvent := &event.KeyEvent{Name: keyName, Physical: event.HardwareKey{ScanCode: scancode}, Modifier: keyDesktopModifier}
	// Set when KeyDown moved focus, as the Linux web view's Ctrl+Tab escape
	// does: the Tab handling below then treats the press as handled.
	keyDownMovedFocus := false

	pendingMenuToggle := w.menuTogglePending
	w.menuTogglePending = desktop.KeyNone
	pendingMenuDeactivation := w.menuDeactivationPending
	w.menuDeactivationPending = desktop.KeyNone
	switch action {
	case release:
		w.processKeyReleased(keyEvent, pendingMenuToggle, pendingMenuDeactivation)
		return // ignore key up in other core events
	case press:
		switch keyName {
		case desktop.KeyAltLeft, desktop.KeyAltRight:
			// compensate for GLFW modifiers bug https://github.com/glfw/glfw/issues/1630
			if (runtime.GOOS == "linux" && keyDesktopModifier == 0) || (runtime.GOOS != "linux" && keyDesktopModifier == event.KeyModifierAlt) {
				w.menuTogglePending = keyName
			}
		case event.KeyEscape:
			w.menuDeactivationPending = keyName
		}
		if focused := w.canvas.Focused(); focused != nil {
			if keyable, ok := focused.(desktop.Keyable); ok {
				keyable.KeyDown(keyEvent)
				keyDownMovedFocus = w.canvas.Focused() != focused
			}
		} else if w.canvas.onKeyDown != nil {
			w.canvas.onKeyDown(keyEvent)
			keyDownMovedFocus = w.canvas.Focused() != nil
		}
	default:
		// key repeat will fall through to TypedKey and TypedShortcut
	}

	// A Tab press whose KeyDown already moved focus is handled: neither a
	// second focus move nor a shortcut or TypedKey on the widget that has just
	// gained focus, which never saw the key go down.
	if keyName == event.KeyTab && keyDownMovedFocus {
		w.tabEscape.Disarm()
		return
	}
	// A held Escape repeats, but it is still the one Escape whose press set
	// the way out, so its repeats leave the state alone.
	if !(action == repeat && keyName == event.KeyEscape) &&
		w.tabEscape.MoveTabFocus(w.canvas, keyName, keyDesktopModifier) {
		return
	}
	if w.triggersShortcut(keyName, keyASCII, keyDesktopModifier) {
		return
	}

	// Escape closes the innermost pop-up, menu or dialog unless the focused
	// widget is using it for a state of its own. This runs ahead of TypedKey
	// because a canvas-hosted dialog focuses nothing, so there is no widget for
	// the key to reach — and where one is focused, a text field would otherwise
	// swallow the key and trap the user in the dialog.
	if action == press && keyName == event.KeyEscape {
		if driver.HandleEscape(w.canvas) {
			return
		}
		w.tabEscape.EscapeNotConsumed(w.canvas.Focused())
	}

	// No shortcut detected, pass down to TypedKey
	focused := w.canvas.Focused()
	if focused != nil {
		// Keyboard-activation parity with pointer taps (pre-rebrand tracker #700): if the focused
		// control declares this key an activation (Space/Enter on a button-like
		// widget), fire the central activation signal, matching the pointer path.
		if act, ok := focused.(event.Activatable); ok && act.ActivatedByKey(keyEvent) {
			if co, ok := focused.(core.CanvasObject); ok {
				core.NotifyActivation(co, true)
			}
		}
		focused.TypedKey(keyEvent)
	} else if w.canvas.onTypedKey != nil {
		w.canvas.onTypedKey(keyEvent)
	}
}

// charInput defines the character with modifiers callback which is called when a
// Unicode character is input.
//
// Characters do not map 1:1 to physical keys, as a key may produce zero, one or more characters.
func (w *window) processCharInput(char rune) {
	w.tabEscape.Disarm()
	if focused := w.canvas.Focused(); focused != nil {
		focused.TypedRune(char)
	} else if w.canvas.onTypedRune != nil {
		w.canvas.onTypedRune(char)
	}
}

// processCompose forwards an in-progress IME composition (preedit) update to the
// focused object when it supports composition. A no-op otherwise. Called by the
// desktop preedit callback (window_desktop.go) and the WASM composition events
// (device_wasm.go). The commit and cancel companions live in the platform files
// where each is used (window_wasm.go / window_desktop.go).
func (w *window) processCompose(ev *event.PreeditEvent) {
	w.tabEscape.Disarm()
	if c, ok := w.canvas.Focused().(event.Composable); ok {
		c.Compose(ev)
	}
}

func (w *window) processFocused(focus bool) {
	// Key-window changes alone don't dirty the canvas, so sync the
	// accessibility tree here: the AXFocused property is gated on this
	// window being current (pre-rebrand tracker #554). A no-op without a screen-reader backend.
	defer w.updateAccessibility()

	if focus {
		if curWindow == nil {
			if f := bellui.CurrentApp().Lifecycle().(*app.Lifecycle).OnEnteredForeground(); f != nil {
				f()
			}
		}
		curWindow = w
		w.canvas.FocusGained()
	} else {
		w.canvas.FocusLost()
		w.tabEscape.Disarm()
		w.mousePos = geo.Position{}
		// Avoid triggering menu on Alt+Tab: glfw sends a fake "alt key released" event immediately after
		// unfocusing the window, and it should be ignored.
		w.menuTogglePending = desktop.KeyNone
		w.menuDeactivationPending = desktop.KeyNone

		// check whether another window was focused or not
		if curWindow != w {
			return
		}

		curWindow = nil
		if f := bellui.CurrentApp().Lifecycle().(*app.Lifecycle).OnExitedForeground(); f != nil {
			f()
		}
	}
}

func (w *window) triggersShortcut(localizedKeyName event.KeyName, key event.KeyName, modifier event.KeyModifier) bool {
	ctrlMod := event.KeyModifierControl
	if isMacOSRuntime() {
		ctrlMod = event.KeyModifierSuper
	}
	// User pressing physical keys Ctrl+V while using a Russian (or any non-ASCII) keyboard layout
	// is reported as a event.KeyUnknown key with Control modifier. We should still consider this
	// as a "Paste" shortcut.
	// See https://github.com/fyne-io/fyne/pull/2587 for discussion.
	keyName := localizedKeyName
	resemblesShortcut := (modifier&(event.KeyModifierControl|event.KeyModifierSuper) != 0)
	if (localizedKeyName == event.KeyUnknown) && resemblesShortcut && key != event.KeyUnknown {
		keyName = key
	}

	var shortcut core.Shortcut
	if modifier == ctrlMod {
		shortcut = common.BuiltinKeyboardShortcut(keyName, NewClipboard())
	}

	if modifier == event.KeyModifierShift {
		switch keyName {
		case event.KeyInsert: // detect paste shortcut
			shortcut = &bellui.ShortcutPaste{
				Clipboard: NewClipboard(),
			}
		case event.KeyDelete: // detect cut shortcut
			shortcut = &bellui.ShortcutCut{
				Clipboard: NewClipboard(),
			}
		}
	}

	if shortcut == nil && modifier != 0 && !isKeyModifier(keyName) && modifier != event.KeyModifierShift {
		shortcut = &desktop.CustomShortcut{
			KeyName:  keyName,
			Modifier: modifier,
		}
	}

	if shortcut != nil {
		if w.triggerMainMenuShortcut(shortcut) {
			return true
		}
		if focused, ok := w.canvas.Focused().(bellui.Shortcutable); ok {
			shouldRunShortcut := true
			type selectableText interface {
				core.Disableable
				SelectedText() string
			}
			if selectableTextWid, ok := focused.(selectableText); ok && selectableTextWid.Disabled() {
				shouldRunShortcut = shortcut.ShortcutName() == "Copy"
			}
			if shouldRunShortcut {
				focused.TypedShortcut(shortcut)
			}
			return shouldRunShortcut
		}
		w.canvas.TypedShortcut(shortcut)
		return true
	}

	return false
}

func (w *window) triggerMenuShortcut(sh core.Shortcut, m *bellui.Menu) bool {
	for _, i := range m.Items {
		if i.Shortcut != nil && i.Shortcut.ShortcutName() == sh.ShortcutName() {
			if f := i.Action; f != nil {
				f()
				return true
			}
		}

		if i.ChildMenu != nil && w.triggerMenuShortcut(sh, i.ChildMenu) {
			return true
		}
	}

	return false
}

func (w *window) triggerMainMenuShortcut(sh core.Shortcut) bool {
	if w.mainmenu == nil {
		return false
	}

	for _, m := range w.mainmenu.Items {
		if w.triggerMenuShortcut(sh, m) {
			return true
		}
	}

	return false
}

func (w *window) RunWithContext(f func()) {
	if w.isClosing() {
		return
	}
	// A WebGPU backend window is created with no GL context (glfw.NoAPI), so
	// MakeContextCurrent would fail; needsGLContext gates the GL-only binding.
	if needsGLContext {
		w.view().MakeContextCurrent()
		defer w.DetachCurrentContext()
	}

	f()
}

func (w *window) Context() any {
	return nil
}

func (w *window) runOnMainWhenCreated(fn func()) {
	if w.view() != nil {
		async.EnsureMain(fn)
		return
	}

	w.pending = append(w.pending, fn)
}

func (d *gLDriver) CreateWindow(title string) (win bellui.Window) {
	if runtime.GOOS != "js" {
		async.EnsureMain(func() {
			win = d.createWindow(title, true)
		})
		return win
	}

	// handling multiple windows by overlaying on the root for web
	var root bellui.Window
	hasVisible := false
	for _, w := range d.windows {
		if w.(*window).visible {
			hasVisible = true
			root = w
			break
		}
	}

	if !hasVisible {
		return d.createWindow(title, true)
	}

	c := root.Canvas().(*glCanvas)
	multi := c.webExtraWindows
	if multi == nil {
		multi = container.NewMultipleWindows()
		multi.Resize(c.Size())
		c.webExtraWindows = multi
	}
	inner := container.NewInnerWindow(title, canvas.NewRectangle(color.Transparent))
	multi.Add(inner)

	return wrapInnerWindow(inner, root, d)
}

func (d *gLDriver) createWindow(title string, decorate bool) bellui.Window {
	var ret *window
	if title == "" {
		title = defaultTitle
	}

	d.init()

	ret = &window{title: title, decorate: decorate, driver: d}
	ret.canvas = newCanvas()
	ret.canvas.FixedSize = canvasFixedSize
	ret.canvas.context = ret
	ret.SetIcon(ret.icon)
	d.addWindow(ret)
	return ret
}

func (w *window) doShowAgain() {
	if w.isClosing() {
		return
	}

	view := w.view()
	if !build.IsWayland {
		view.SetPos(w.xpos, w.ypos)
	}
	view.Show()
	w.visible = true

	if w.fullScreenSecondary {
		w.doSetFullScreen2(true)
	} else if w.fullScreen {
		w.doSetFullScreen(true)
	}

	w.RunWithContext(func() {
		w.driver.repaintWindow(w)
	})
}

func (w *window) isClosing() bool {
	return w.closing || w.viewport == nil
}

func (w *window) toggleVisible() {
	if w.visible {
		w.Hide()
	} else {
		w.Show()
	}
}

func (d *gLDriver) CreateSplashWindow() bellui.Window {
	win := d.createWindow("", false)
	win.SetPadded(false)
	win.CenterOnScreen()
	return win
}

func (d *gLDriver) AllWindows() []bellui.Window {
	return d.windows
}

func isKeyModifier(keyName event.KeyName) bool {
	return keyName == desktop.KeyShiftLeft || keyName == desktop.KeyShiftRight ||
		keyName == desktop.KeyControlLeft || keyName == desktop.KeyControlRight ||
		keyName == desktop.KeyAltLeft || keyName == desktop.KeyAltRight ||
		keyName == desktop.KeySuperLeft || keyName == desktop.KeySuperRight
}
