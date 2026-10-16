package widget

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Mobile caret drag-handle for RichEntry (pre-rebrand tracker #999, parity with Entry's pre-rebrand tracker #943): right
// after a plain touch tap places the caret in a focused editor, a single
// draggable dot hangs below the insertion point so the caret can be repositioned
// precisely without selecting. It auto-hides after a few seconds of inactivity
// (native timing), on typing, on focus loss, and as soon as a selection starts
// (the two selection-edge handles take over). Touch-gated: only a touch Tapped
// summons it, so desktop behavior and keyboard-driven render masters are
// unchanged. It reuses the shared magnifier (pre-rebrand tracker #942) via the same loupeOwner seam
// Entry's caret handle uses, so the styled-run loupe path (pre-rebrand tracker #888) magnifies the
// dragged rich row. Everything runs on the main goroutine.

// richEntryCaretHandleTimeout matches Entry's caretHandleTimeout: how long the
// handle stays up without interaction before it auto-hides.
const richEntryCaretHandleTimeout = 4 * time.Second

// showCaretHandle arms the caret handle after a touch placed the caret. A no-op
// off mobile, for a non-touch pointer, and for unfocused or composing editors.
func (e *RichEntry) showCaretHandle() {
	if !bellui.CurrentDevice().IsMobile() || !e.focused || !e.touchPointer || e.composing {
		return
	}
	if e.caretHandle == nil {
		e.caretHandle = newRichEntryCaretHandle(e)
	}
	e.caretHandle.active = true
	e.caretHandle.shownAt = time.Now()
	e.updateCaretHandle()
}

// hideCaretHandle dismisses the caret handle immediately (typing, Hide).
func (e *RichEntry) hideCaretHandle() {
	if e.caretHandle == nil {
		return
	}
	e.caretHandle.active = false
	e.caretHandle.hide()
}

// updateCaretHandle shows, repositions or hides the caret handle. Mirroring
// updateSelectionHandles, it is re-run from the editor's Refresh and from a
// per-frame sync animation while shown — the latter also enforces the inactivity
// timeout. Focus loss, invisibility, composition and the timeout disarm the
// handle (only the next touch tap re-arms it); an active selection only hides it,
// staying armed, so it reappears the moment a collapsed selection is cleared.
func (e *RichEntry) updateCaretHandle() {
	if !bellui.CurrentDevice().IsMobile() {
		return
	}
	h := e.caretHandle
	if h == nil {
		return
	}

	if !h.active || !e.focused || !e.Visible() || e.composing ||
		time.Since(h.shownAt) >= richEntryCaretHandleTimeout {
		h.active = false
		h.hide()
		return
	}
	show := true
	if _, _, hasSel := e.selection(); hasSel {
		show = false // hidden while the selection handles take over
	}
	var cnv core.Canvas
	if show {
		if app := bellui.CurrentApp(); app != nil && app.Driver() != nil {
			cnv = app.Driver().CanvasForObject(e.super())
		}
		show = cnv != nil
	}

	if !show {
		h.hide()
		return
	}
	h.showOn(cnv)
}

// richEntryCaretHandle is the overlay root. It reports PassesInputThrough, so
// only the grab-dot child takes input; everywhere else taps, drags and keys
// reach the UI beneath as if the overlay were not there (the pre-rebrand tracker #569 pattern,
// shared with entrySelectionHandles / entryCaretHandle).
type richEntryCaretHandle struct {
	BaseWidget

	entry *RichEntry
	grab  *richEntryCaretGrab
	loupe *entryLoupe
	cnv   core.Canvas
	anim  *bellui.Animation

	// active marks the handle as armed by a touch tap; shownAt is the last
	// interaction time driving the inactivity timeout. dragging is true while a
	// live grab drag holds the dot (it keeps the pre-rebrand tracker #942 magnifier up).
	active   bool
	dragging bool
	shownAt  time.Time
}

func newRichEntryCaretHandle(e *RichEntry) *richEntryCaretHandle {
	h := &richEntryCaretHandle{entry: e}
	h.grab = &richEntryCaretGrab{owner: h, grabY: -1}
	h.grab.ExtendBaseWidget(h.grab)
	h.ExtendBaseWidget(h)
	h.loupe = newEntryLoupe(h)
	return h
}

// loupeHost implements loupeOwner (pre-rebrand tracker #942): the caret-handle drag shows the same
// magnifier as a selection-handle drag, using RichEntry's styled-run path.
func (h *richEntryCaretHandle) loupeHost() selectionLoupeHost {
	return h.entry
}

// loupeClearance implements loupeOwner: the grab dot hangs below the line.
func (h *richEntryCaretHandle) loupeClearance() float32 {
	return h.grab.desiredSize().Height
}

// loupeTarget implements loupeOwner: the caret being dragged.
func (h *richEntryCaretHandle) loupeTarget() (row, col int, ok bool) {
	if !h.dragging {
		return 0, 0, false
	}
	row, col = h.entry.doc.offsetToRowCol(h.entry.caret)
	return row, col, true
}

// loupeSelection implements loupeOwner: a caret drag has no selection.
func (h *richEntryCaretHandle) loupeSelection() (rowA, colA, rowB, colB int, ok bool) {
	return 0, 0, 0, 0, false
}

// PassesInputThrough marks the overlay input-transparent outside the grab dot;
// it also keeps the overlay from owning the keyboard focus.
func (h *richEntryCaretHandle) PassesInputThrough() bool {
	return true
}

func (h *richEntryCaretHandle) CreateRenderer() core.WidgetRenderer {
	return &richEntryCaretHandleRenderer{h: h}
}

func (h *richEntryCaretHandle) showOn(cnv core.Canvas) {
	if h.cnv != cnv {
		h.hide()
		h.cnv = cnv
		cnv.Overlays().Add(h)
		// The mobile canvas re-seats overlays to its interactive area on the
		// next size change; seed a full-canvas geometry for the first frame.
		h.Resize(cnv.Size())
	}
	h.sync()
	h.startSync()
}

func (h *richEntryCaretHandle) hide() {
	h.stopSync()
	h.dragging = false
	h.loupe.dismiss()
	if h.cnv != nil {
		h.cnv.Overlays().Remove(h)
		h.cnv = nil
	}
}

// sync repositions the grab dot under the current caret, hiding it while the
// caret is outside the editor's visible bounds. RichEntry does not scroll its
// own content, so the caret rect is already editor-relative (no scroll offset).
func (h *richEntryCaretHandle) sync() {
	e := h.entry

	row, col := e.doc.offsetToRowCol(e.caret)
	pos, csize := e.caretRectAt(row, col, e.doc.rowIsImage(row))

	// The editor's origin in overlay coordinates (canvas-absolute, so the
	// display inset is accounted for — see hostAbsolutePosition).
	entryOrigin := hostAbsolutePosition(e.super()).Subtract(h.Position())
	entrySize := e.super().Size()

	if pos.Y+csize.Height <= 0 || pos.Y >= entrySize.Height ||
		pos.X < -csize.Width || pos.X > entrySize.Width {
		h.grab.Hide()
		return
	}

	size := h.grab.desiredSize()
	gpos := entryOrigin.Add(pos).
		AddXY(-size.Width/2, csize.Height) // centered below the caret
	if h.grab.Size() != size {
		h.grab.Resize(size)
	}
	if h.grab.Position() != gpos {
		h.grab.Move(gpos)
	}
	if !h.grab.Visible() {
		h.grab.Show()
	}
	h.loupe.sync()
}

// startSync arms the per-frame reposition driver (the pre-rebrand tracker #417 prefill pattern):
// while visible it re-evaluates geometry every frame, tracking movement the
// editor cannot report — a parent scroll, a window resize, the soft keyboard
// shifting the layout — and enforces the inactivity timeout.
func (h *richEntryCaretHandle) startSync() {
	if h.anim != nil {
		return
	}
	if app := bellui.CurrentApp(); app == nil || app.Driver() == nil {
		return
	}
	anim := &bellui.Animation{
		Duration:    time.Second, // irrelevant: Tick fires every frame while running
		RepeatCount: bellui.AnimationRepeatForever,
		Curve:       bellui.AnimationLinear,
		Tick:        func(float32) { h.entry.updateCaretHandle() },
	}
	h.anim = anim
	anim.Start()
}

func (h *richEntryCaretHandle) stopSync() {
	if h.anim == nil {
		return
	}
	h.anim.Stop()
	h.anim = nil
}

// drag moves the caret to follow the grab dot, without touching the selection.
// The finger→line offset math matches entrySelectionHandles' dragHandle; the
// caret move drives ensureCursorVisible via moveCaret's Refresh, so the viewport
// auto-scrolls to follow.
func (h *richEntryCaretHandle) drag(d *event.DragEvent) {
	e := h.entry
	e.syncSel()

	// Finger position in canvas coordinates → editor content coordinates.
	// Drivers set AbsolutePosition on drag events; the handle-relative fallback
	// (off by up to the tap slop) covers synthetic events without it.
	finger := d.AbsolutePosition
	entryAbs := hostAbsolutePosition(e.super())
	if finger == (geo.Position{}) {
		finger = h.Position().Add(h.grab.Position()).Add(d.Position)
	}
	content := finger.Subtract(entryAbs) // RichEntry does not self-scroll

	// The dot hangs below the line it controls: lift the lookup point from the
	// finger back up into that line, measured from where the finger actually
	// grabbed the dot when known.
	lineHeight := e.selectionHandleLineHeight()
	grab := h.grab.Size().Height / 2
	if h.grab.grabY >= 0 {
		grab = h.grab.grabY
	}
	content.Y -= grab + lineHeight/2

	row, col, _ := e.sel.getRowCol(content)
	off := clampInt(e.doc.rowColToOffset(row, col), 0, e.doc.len())
	h.shownAt = time.Now() // interacting: reset the inactivity timeout
	if off != e.caret || e.selecting {
		e.moveCaret(off, false) // moves the caret visual, auto-scrolls, re-syncs
	}
}

// richEntryCaretGrab is the draggable grab dot. Its bounds are a square twice the
// visual dot's diameter, giving the finger a padded touch target.
type richEntryCaretGrab struct {
	BaseWidget

	owner *richEntryCaretHandle

	// grabY mirrors entryCaretGrab.grabY: the finger's y offset within the dot
	// at touch-down, so the drag lift measures from the real grab point.
	// -1 = unknown (synthetic drags), target-center fallback.
	grabY float32
}

var (
	_ event.Draggable  = (*richEntryCaretGrab)(nil)
	_ mobile.Touchable = (*richEntryCaretGrab)(nil)
)

func (g *richEntryCaretGrab) desiredSize() geo.Size {
	d := g.owner.entry.Theme().Size(theme.SizeNameInlineIcon)
	return geo.NewSize(d*2, d*2)
}

func (g *richEntryCaretGrab) CreateRenderer() core.WidgetRenderer {
	dot := canvas.NewCircle(theme.Color(theme.ColorNamePrimary))
	return &richEntryCaretGrabRenderer{grab: g, dot: dot}
}

// RetainsFocus keeps a touch on the grab dot from unfocusing the editor it
// serves (the driver's TouchHandler consults this before unfocusing).
func (g *richEntryCaretGrab) RetainsFocus() bool {
	return true
}

// TouchDown claims the touch so it doesn't fall through the pass-through overlay
// onto the editor beneath, where it would move the caret. Grabbing the dot also
// resets the inactivity timeout.
func (g *richEntryCaretGrab) TouchDown(ev *mobile.TouchEvent) {
	g.grabY = ev.Position.Y
	g.owner.shownAt = time.Now()
}

// TouchUp implements mobile.Touchable.
func (g *richEntryCaretGrab) TouchUp(*mobile.TouchEvent) {
	g.grabY = -1
}

// TouchCancel implements mobile.Touchable.
func (g *richEntryCaretGrab) TouchCancel(*mobile.TouchEvent) {
	g.grabY = -1
	g.owner.dragging = false
	g.owner.loupe.dismiss()
}

// Dragged moves the caret with the dot and shows the magnifier over the drag
// target (pre-rebrand tracker #942).
func (g *richEntryCaretGrab) Dragged(d *event.DragEvent) {
	g.owner.dragging = true
	g.owner.drag(d)
	g.owner.loupe.show()
}

// DragEnd dismisses the magnifier and re-syncs the dot under the caret's final
// position.
func (g *richEntryCaretGrab) DragEnd() {
	g.owner.dragging = false
	g.owner.loupe.dismiss()
	g.owner.entry.updateCaretHandle()
}

type richEntryCaretHandleRenderer struct {
	h *richEntryCaretHandle
}

func (r *richEntryCaretHandleRenderer) Destroy() {
	r.h.stopSync()
}

func (r *richEntryCaretHandleRenderer) Layout(geo.Size) {
	// grab-dot geometry is driven by sync(), not by the overlay's size
}

func (r *richEntryCaretHandleRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *richEntryCaretHandleRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.h.grab}
	if r.h.loupe != nil {
		objs = append(objs, r.h.loupe)
	}
	return objs
}

func (r *richEntryCaretHandleRenderer) Refresh() {
	r.h.grab.Refresh()
	if r.h.loupe != nil && r.h.loupe.Visible() {
		r.h.loupe.Refresh()
	}
}

type richEntryCaretGrabRenderer struct {
	grab *richEntryCaretGrab
	dot  *canvas.Circle
}

func (r *richEntryCaretGrabRenderer) Destroy() {}

func (r *richEntryCaretGrabRenderer) Layout(size geo.Size) {
	d := size.Width / 2
	r.dot.Resize(geo.NewSize(d, d))
	// the dot sits at the top of the touch target, its top edge on the caret's
	// baseline anchor
	r.dot.Move(geo.NewPos((size.Width-d)/2, 0))
}

func (r *richEntryCaretGrabRenderer) MinSize() geo.Size {
	return r.grab.desiredSize()
}

func (r *richEntryCaretGrabRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.dot}
}

func (r *richEntryCaretGrabRenderer) Refresh() {
	r.dot.FillColor = theme.Color(theme.ColorNamePrimary)
	r.dot.Refresh()
}
