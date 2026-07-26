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

// Mobile caret drag-handle (#943, the #758 follow-up): right after a plain
// touch tap places the caret in a focused Entry on a mobile device, a single
// draggable dot hangs below the insertion point so the caret can be
// repositioned precisely without selecting. It auto-hides after a few seconds
// of inactivity (like native platforms), on typing, on focus loss, and as
// soon as a selection starts (the two selection-edge handles take over).
// Touch-gated: only Entry.TouchDown summons it, so desktop behavior and
// keyboard-driven render masters are unchanged. Everything runs on the main
// goroutine.

// caretHandleTimeout is how long the caret handle stays up without being
// touched or dragged before it auto-hides, matching native mobile timing.
const caretHandleTimeout = 4 * time.Second

// showCaretHandle arms the caret handle after a touch placed the caret.
// A no-op off mobile and for unfocused or disabled entries.
func (e *Entry) showCaretHandle() {
	if !bellui.CurrentDevice().IsMobile() || !e.focused || e.Disabled() {
		return
	}
	if e.caretHandle == nil {
		e.caretHandle = newEntryCaretHandle(e)
	}
	e.caretHandle.active = true
	e.caretHandle.shownAt = time.Now()
	e.updateCaretHandle()
}

// hideCaretHandle dismisses the caret handle immediately (typing, Hide).
func (e *Entry) hideCaretHandle() {
	if e.caretHandle == nil {
		return
	}
	e.caretHandle.active = false
	e.caretHandle.hide()
}

// updateCaretHandle shows, repositions or hides the caret handle. Mirroring
// updateSelectionHandles, it is re-run from the entry's Refresh and from a
// per-frame sync animation while shown — the latter is also what enforces
// the inactivity timeout. Focus loss, invisibility and the timeout disarm
// the handle (only the next touch tap re-arms it); an active selection only
// hides it, staying armed, because the driver clears a collapsed selection
// in Tapped *after* the arming TouchDown — the handle then reappears the
// moment the selection is gone, matching native tap-to-collapse behavior.
func (e *Entry) updateCaretHandle() {
	if !bellui.CurrentDevice().IsMobile() {
		return
	}
	h := e.caretHandle
	if h == nil {
		return
	}

	if !h.active || !e.focused || !e.Visible() || e.Disabled() ||
		time.Since(h.shownAt) >= caretHandleTimeout {
		h.active = false
		h.hide()
		return
	}
	show := true
	if e.sel != nil {
		if _, hasSel := e.SelectionRange(); hasSel {
			show = false // hidden while the selection handles take over
		}
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

// entryCaretHandle is the overlay root. It reports PassesInputThrough, so
// only the grab-dot child takes input; everywhere else taps, drags and keys
// reach the UI beneath as if the overlay were not there (the #569 pattern,
// shared with entrySelectionHandles).
type entryCaretHandle struct {
	BaseWidget

	entry *Entry
	grab  *entryCaretGrab
	loupe *entryLoupe
	cnv   core.Canvas
	anim  *bellui.Animation

	// active marks the handle as armed by a touch tap; shownAt is the last
	// interaction time driving the inactivity timeout. dragging is true while
	// a live grab drag holds the dot (it keeps the #942 magnifier up).
	active   bool
	dragging bool
	shownAt  time.Time
}

func newEntryCaretHandle(e *Entry) *entryCaretHandle {
	h := &entryCaretHandle{entry: e}
	h.grab = &entryCaretGrab{owner: h, grabY: -1}
	h.grab.ExtendBaseWidget(h.grab)
	h.ExtendBaseWidget(h)
	h.loupe = newEntryLoupe(h)
	return h
}

// loupeHost implements loupeOwner (#942): the caret-handle drag shows the
// same magnifier as a selection-handle drag.
func (h *entryCaretHandle) loupeHost() selectionLoupeHost {
	return h.entry
}

// loupeClearance implements loupeOwner: the grab dot hangs below the line.
func (h *entryCaretHandle) loupeClearance() float32 {
	return h.grab.desiredSize().Height
}

// loupeTarget implements loupeOwner: the caret being dragged.
func (h *entryCaretHandle) loupeTarget() (row, col int, ok bool) {
	if !h.dragging {
		return 0, 0, false
	}
	return h.entry.CursorRow, h.entry.CursorColumn, true
}

// loupeSelection implements loupeOwner: a caret drag has no selection.
func (h *entryCaretHandle) loupeSelection() (rowA, colA, rowB, colB int, ok bool) {
	return 0, 0, 0, 0, false
}

// PassesInputThrough marks the overlay input-transparent outside the grab dot;
// it also keeps the overlay from owning the keyboard focus.
func (h *entryCaretHandle) PassesInputThrough() bool {
	return true
}

func (h *entryCaretHandle) CreateRenderer() core.WidgetRenderer {
	return &entryCaretHandleRenderer{h: h}
}

func (h *entryCaretHandle) showOn(cnv core.Canvas) {
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

func (h *entryCaretHandle) hide() {
	h.stopSync()
	h.dragging = false
	h.loupe.dismiss()
	if h.cnv != nil {
		h.cnv.Overlays().Remove(h)
		h.cnv = nil
	}
}

// sync repositions the grab dot under the current caret, hiding it while the
// caret is outside the entry's visible bounds.
func (h *entryCaretHandle) sync() {
	e := h.entry

	rect := e.caretRect(e.CursorRow, e.CursorColumn, true)

	// The entry's origin in overlay coordinates (canvas-absolute, so the
	// display inset is accounted for — see hostAbsolutePosition).
	entryOrigin := hostAbsolutePosition(e.super()).Subtract(h.Position())
	entrySize := e.Size()

	anchor := rect.Pos.Subtract(e.scroll.Offset) // entry-relative caret top
	if anchor.Y+rect.Size.Height <= 0 || anchor.Y >= entrySize.Height ||
		anchor.X < -rect.Size.Width || anchor.X > entrySize.Width {
		h.grab.Hide()
		return
	}

	size := h.grab.desiredSize()
	pos := entryOrigin.Add(anchor).
		AddXY(-size.Width/2, rect.Size.Height) // centered below the caret
	if h.grab.Size() != size {
		h.grab.Resize(size)
	}
	if h.grab.Position() != pos {
		h.grab.Move(pos)
	}
	if !h.grab.Visible() {
		h.grab.Show()
	}
	h.loupe.sync()
}

// startSync arms the per-frame reposition driver (the #417 prefill pattern):
// while the handle is visible it re-evaluates geometry every frame, so it
// tracks movement the entry cannot report — a parent scroll, a window resize,
// the soft keyboard shifting the layout — and enforces the inactivity timeout.
func (h *entryCaretHandle) startSync() {
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

func (h *entryCaretHandle) stopSync() {
	if h.anim == nil {
		return
	}
	h.anim.Stop()
	h.anim = nil
}

// drag moves the caret to follow the grab dot, without touching the
// selection. The finger→line offset math matches entrySelectionHandles'
// dragHandle; the caret move drives ensureCursorVisible via the entry's
// Refresh, so the viewport auto-scrolls to follow.
func (h *entryCaretHandle) drag(d *event.DragEvent) {
	e := h.entry
	e.syncSelectable()

	// Finger position in canvas coordinates → entry content coordinates.
	// Drivers set AbsolutePosition on drag events; the handle-relative
	// fallback (off by up to the tap slop) covers synthetic events without it.
	finger := d.AbsolutePosition
	entryAbs := hostAbsolutePosition(e.super())
	if finger == (geo.Position{}) {
		finger = h.Position().Add(h.grab.Position()).Add(d.Position)
	}
	content := finger.Subtract(entryAbs).Add(e.scroll.Offset)

	// The dot hangs below the line it controls: lift the lookup point from
	// the finger back up into that line, measured from where the finger
	// actually grabbed the dot when known.
	th := e.Theme()
	lineHeight := e.text.charMinSize(false, e.TextStyle, th.Size(theme.SizeNameText)).Height
	grab := h.grab.Size().Height / 2
	if h.grab.grabY >= 0 {
		grab = h.grab.grabY
	}
	content.Y -= grab + lineHeight/2

	prevRow, prevCol := e.CursorRow, e.CursorColumn
	e.updateMousePointer(content, false)
	h.shownAt = time.Now() // interacting: reset the inactivity timeout
	if e.CursorRow != prevRow || e.CursorColumn != prevCol {
		e.Refresh() // moves the caret visual, auto-scrolls, re-syncs the handle
	}
}

// entryCaretGrab is the draggable grab dot. Its bounds are a square twice the
// visual dot's diameter, giving the finger a padded touch target.
type entryCaretGrab struct {
	BaseWidget

	owner *entryCaretHandle

	// grabY mirrors entrySelectionHandle.grabY: the finger's y offset within
	// the dot at touch-down, so the drag lift measures from the real grab
	// point. -1 = unknown (synthetic drags), target-center fallback.
	grabY float32
}

var (
	_ event.Draggable  = (*entryCaretGrab)(nil)
	_ mobile.Touchable = (*entryCaretGrab)(nil)
)

func (g *entryCaretGrab) desiredSize() geo.Size {
	d := g.owner.entry.Theme().Size(theme.SizeNameInlineIcon)
	return geo.NewSize(d*2, d*2)
}

func (g *entryCaretGrab) CreateRenderer() core.WidgetRenderer {
	dot := canvas.NewCircle(theme.Color(theme.ColorNamePrimary))
	return &entryCaretGrabRenderer{grab: g, dot: dot}
}

// RetainsFocus keeps a touch on the grab dot from unfocusing the entry it
// serves (the driver's TouchHandler consults this before unfocusing).
func (g *entryCaretGrab) RetainsFocus() bool {
	return true
}

// TouchDown claims the touch so it doesn't fall through the pass-through
// overlay onto the entry beneath, where it would move the caret. Grabbing
// the dot also resets the inactivity timeout.
func (g *entryCaretGrab) TouchDown(ev *mobile.TouchEvent) {
	g.grabY = ev.Position.Y
	g.owner.shownAt = time.Now()
}

// TouchUp implements mobile.Touchable.
func (g *entryCaretGrab) TouchUp(*mobile.TouchEvent) {
	g.grabY = -1
}

// TouchCancel implements mobile.Touchable.
func (g *entryCaretGrab) TouchCancel(*mobile.TouchEvent) {
	g.grabY = -1
	g.owner.dragging = false
	g.owner.loupe.dismiss()
}

// Dragged moves the caret with the dot and shows the magnifier over the
// drag target (#942).
func (g *entryCaretGrab) Dragged(d *event.DragEvent) {
	g.owner.dragging = true
	g.owner.drag(d)
	g.owner.loupe.show()
}

// DragEnd dismisses the magnifier and re-syncs the dot under the caret's
// final position.
func (g *entryCaretGrab) DragEnd() {
	g.owner.dragging = false
	g.owner.loupe.dismiss()
	g.owner.entry.updateCaretHandle()
}

type entryCaretHandleRenderer struct {
	h *entryCaretHandle
}

func (r *entryCaretHandleRenderer) Destroy() {
	r.h.stopSync()
}

func (r *entryCaretHandleRenderer) Layout(geo.Size) {
	// grab-dot geometry is driven by sync(), not by the overlay's size
}

func (r *entryCaretHandleRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *entryCaretHandleRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.h.grab}
	if r.h.loupe != nil {
		objs = append(objs, r.h.loupe)
	}
	return objs
}

func (r *entryCaretHandleRenderer) Refresh() {
	r.h.grab.Refresh()
	if r.h.loupe != nil && r.h.loupe.Visible() {
		r.h.loupe.Refresh()
	}
}

type entryCaretGrabRenderer struct {
	grab *entryCaretGrab
	dot  *canvas.Circle
}

func (r *entryCaretGrabRenderer) Destroy() {}

func (r *entryCaretGrabRenderer) Layout(size geo.Size) {
	d := size.Width / 2
	r.dot.Resize(geo.NewSize(d, d))
	// the dot sits at the top of the touch target, its top edge on the
	// caret's baseline anchor
	r.dot.Move(geo.NewPos((size.Width-d)/2, 0))
}

func (r *entryCaretGrabRenderer) MinSize() geo.Size {
	return r.grab.desiredSize()
}

func (r *entryCaretGrabRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.dot}
}

func (r *entryCaretGrabRenderer) Refresh() {
	r.dot.FillColor = theme.Color(theme.ColorNamePrimary)
	r.dot.Refresh()
}
