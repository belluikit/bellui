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

// Mobile selection handles (pre-rebrand tracker #758): while a focused text widget on a mobile
// device has a non-empty selection, an input-pass-through canvas overlay
// shows two draggable grab handles hanging below the selection edges — the
// native touch affordance for adjusting a selection started by long-press,
// double-tap or Select all. Dragging a handle moves that edge of the
// selection (in an editor the caret follows the dragged edge, so it
// auto-scrolls to keep it visible); the other edge stays pinned and the two
// can never cross. The overlay is generic over selectionHandleHost (pre-rebrand tracker #944):
// Entry and the read-only selectable (Label/RichText) implement it — see
// text_sel_handles.go. Everything runs on the main goroutine.

// updateSelectionHandles shows, repositions or hides the selection handles.
// It is the central seam: every selection mutation funnels through the
// entry's Refresh, which calls it; a per-frame sync animation tracks
// movement the entry doesn't know about (a parent container scrolling).
func (e *Entry) updateSelectionHandles() {
	if !bellui.CurrentDevice().IsMobile() {
		return
	}

	if e.sel == nil || !e.sel.selecting {
		e.touchSelection = false // the touch-initiated selection is gone
	}
	show := e.touchSelection && e.focused && e.Visible() && !e.Disabled() && !e.Password
	if show {
		_, show = e.SelectionRange()
	}
	syncSelectionHandles(e, &e.selHandles, show)
}

// entrySelectionHandles is the overlay root. It reports PassesInputThrough,
// so only the two handle children take input; everywhere else taps, drags
// and keys reach the UI beneath as if the overlay were not there.
type entrySelectionHandles struct {
	BaseWidget

	host       selectionHandleHost
	start, end *entrySelectionHandle
	loupe      *entryLoupe           // nil when the host is not a selectionLoupeHost
	dragging   *entrySelectionHandle // the handle a live drag holds, nil otherwise
	cnv        core.Canvas
	anim       *bellui.Animation
}

func newEntrySelectionHandles(host selectionHandleHost) *entrySelectionHandles {
	h := &entrySelectionHandles{host: host}
	h.start = &entrySelectionHandle{owner: h, isStart: true, grabY: -1}
	h.end = &entrySelectionHandle{owner: h, grabY: -1}
	h.start.ExtendBaseWidget(h.start)
	h.end.ExtendBaseWidget(h.end)
	h.ExtendBaseWidget(h)
	h.loupe = newEntryLoupe(h)
	return h
}

// loupeHost implements loupeOwner (pre-rebrand tracker #942): the drag magnifier is available
// when the host implements the optional selectionLoupeHost extension.
func (h *entrySelectionHandles) loupeHost() selectionLoupeHost {
	if lh, ok := h.host.(selectionLoupeHost); ok {
		return lh
	}
	return nil
}

// loupeClearance implements loupeOwner: the handle dot hangs below the line.
func (h *entrySelectionHandles) loupeClearance() float32 {
	return h.start.desiredSize().Height
}

// loupeTarget implements loupeOwner: the dragged selection edge.
func (h *entrySelectionHandles) loupeTarget() (row, col int, ok bool) {
	if h.dragging == nil {
		return 0, 0, false
	}
	rowA, colA, rowB, colB := h.host.selectionHandleEdges()
	if h.dragging.isStart {
		return rowA, colA, true
	}
	return rowB, colB, true
}

// loupeSelection implements loupeOwner: the full selected range.
func (h *entrySelectionHandles) loupeSelection() (rowA, colA, rowB, colB int, ok bool) {
	rowA, colA, rowB, colB = h.host.selectionHandleEdges()
	return rowA, colA, rowB, colB, true
}

// PassesInputThrough marks the overlay input-transparent outside the handles
// (the Toast/Snackbar pattern, pre-rebrand tracker #569); it also keeps the overlay from owning
// the keyboard focus, which stays on the entry.
func (h *entrySelectionHandles) PassesInputThrough() bool {
	return true
}

func (h *entrySelectionHandles) CreateRenderer() core.WidgetRenderer {
	return &entrySelectionHandlesRenderer{h: h}
}

func (h *entrySelectionHandles) showOn(cnv core.Canvas) {
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

func (h *entrySelectionHandles) hide() {
	h.stopSync()
	h.dragging = nil
	h.loupe.dismiss()
	if h.cnv != nil {
		h.cnv.Overlays().Remove(h)
		h.cnv = nil
	}
}

// sync repositions both handles under the current selection edges, hiding a
// handle whose edge is outside the host's visible bounds.
func (h *entrySelectionHandles) sync() {
	// Normalized selection edges in (row, column) space, anchor/caret order
	// independent (the selection() convention).
	rowA, colA, rowB, colB := h.host.selectionHandleEdges()

	// The host's origin in overlay coordinates (canvas-absolute, so the
	// display inset is accounted for — see hostAbsolutePosition).
	hostObj := h.host.selectionHandleObject()
	hostOrigin := hostAbsolutePosition(hostObj).Subtract(h.Position())
	hostSize := hostObj.Size()
	scroll := h.host.selectionHandleScroll()

	place := func(hd *entrySelectionHandle, row, col int) {
		rect := h.host.selectionHandleCaretRect(row, col)
		anchor := rect.Pos.Subtract(scroll) // host-relative caret top
		if anchor.Y+rect.Size.Height <= 0 || anchor.Y >= hostSize.Height ||
			anchor.X < -rect.Size.Width || anchor.X > hostSize.Width {
			hd.Hide()
			return
		}

		size := hd.desiredSize()
		pos := hostOrigin.Add(anchor).
			AddXY(-size.Width/2, rect.Size.Height) // centered below the caret
		if hd.Size() != size {
			hd.Resize(size)
		}
		if hd.Position() != pos {
			hd.Move(pos)
		}
		if !hd.Visible() {
			hd.Show()
		}
	}
	place(h.start, rowA, colA)
	place(h.end, rowB, colB)
	h.loupe.sync()
}

// startSync arms the per-frame reposition driver (the pre-rebrand tracker #417 prefill pattern):
// while the handles are visible it re-evaluates geometry every frame, so
// they track movement the host cannot report — a parent scroll, a window
// resize, the soft keyboard shifting the layout.
func (h *entrySelectionHandles) startSync() {
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
		Tick:        func(float32) { h.host.updateSelectionHandles() },
	}
	h.anim = anim
	anim.Start()
}

func (h *entrySelectionHandles) stopSync() {
	if h.anim == nil {
		return
	}
	h.anim.Stop()
	h.anim = nil
}

// dragHandle moves one selection edge to follow a handle drag. It maps the
// finger to the host's content coordinate space and hands the edge move to
// the host, which pins the opposite edge as the anchor and clamps the
// selection to stay non-empty, so the handles cannot cross or collapse it
// (in an editor the dragged edge carries the caret, so ensureCursorVisible
// auto-scrolls).
func (h *entrySelectionHandles) dragHandle(hd *entrySelectionHandle, d *event.DragEvent) {
	// Finger position in canvas coordinates → host content coordinates.
	// Drivers set AbsolutePosition on drag events; the handle-relative
	// fallback (off by up to the tap slop) covers synthetic events without it.
	finger := d.AbsolutePosition
	hostAbs := hostAbsolutePosition(h.host.selectionHandleObject())
	if finger == (geo.Position{}) {
		finger = h.Position().Add(hd.Position()).Add(d.Position)
	}
	content := finger.Subtract(hostAbs).Add(h.host.selectionHandleScroll())

	// The handle hangs below the line it controls: lift the lookup point from
	// the finger back up into that line, measured from where the finger
	// actually grabbed the handle when known.
	grab := hd.Size().Height / 2
	if hd.grabY >= 0 {
		grab = hd.grabY
	}
	content.Y -= grab + h.host.selectionHandleLineHeight()/2

	h.host.dragSelectionHandle(hd.isStart, content)
}

// showHostMenu re-summons the host's context menu at a handle's selection
// edge — the native "text toolbar reappears" behavior after a handle
// adjustment or a plain tap on a handle (pre-rebrand tracker #945 device feedback: without it an
// adjusted selection offered no way to act on it). The menu is the host's
// own TappedSecondary, so an editable Entry offers Cut/Copy/Paste and the
// read-only selectable its Copy menu.
func (h *entrySelectionHandles) showHostMenu(hd *entrySelectionHandle) {
	if h.cnv == nil {
		return // the selection vanished; nothing to act on
	}
	hostObj := h.host.selectionHandleObject()
	st, ok := hostObj.(event.SecondaryTappable)
	if !ok {
		return
	}

	rowA, colA, rowB, colB := h.host.selectionHandleEdges()
	row, col := rowB, colB
	if hd.isStart {
		row, col = rowA, colA
	}
	rect := h.host.selectionHandleCaretRect(row, col)
	pos := rect.Pos.Subtract(h.host.selectionHandleScroll()).
		AddXY(0, rect.Size.Height) // just below the adjusted edge
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(hostObj).Add(pos)
	st.TappedSecondary(&event.PointEvent{AbsolutePosition: abs, Position: pos})
}

// entrySelectionHandle is one draggable grab handle. Its bounds are a square
// twice the visual dot's diameter, giving the finger a padded touch target.
type entrySelectionHandle struct {
	BaseWidget

	owner   *entrySelectionHandles
	isStart bool

	// grabY is the finger's y offset within the handle at touch-down, so a
	// drag lifts the lookup point back into the controlled line from where
	// the finger actually grabbed (a finger aims at the visible dot in the
	// target's upper half — assuming a center grab overshot the row above,
	// a pre-rebrand tracker #945 device finding). -1 = unknown (synthetic drags without a
	// touch-down), falling back to the target-center assumption.
	grabY float32
}

var (
	_ event.Draggable  = (*entrySelectionHandle)(nil)
	_ mobile.Touchable = (*entrySelectionHandle)(nil)
)

func (s *entrySelectionHandle) desiredSize() geo.Size {
	d := s.owner.host.Theme().Size(theme.SizeNameInlineIcon)
	return geo.NewSize(d*2, d*2)
}

func (s *entrySelectionHandle) CreateRenderer() core.WidgetRenderer {
	dot := canvas.NewCircle(theme.Color(theme.ColorNamePrimary))
	return &entrySelectionHandleRenderer{handle: s, dot: dot}
}

// RetainsFocus keeps a touch on the handle from unfocusing the widget it
// serves (the driver's TouchHandler consults this before unfocusing).
func (s *entrySelectionHandle) RetainsFocus() bool {
	return true
}

// TouchDown claims the touch so it doesn't fall through the pass-through
// overlay onto the widget beneath, where it would move the caret/selection,
// and records where within the handle the finger grabbed.
func (s *entrySelectionHandle) TouchDown(ev *mobile.TouchEvent) {
	s.grabY = ev.Position.Y
}

// TouchUp re-summons the host's context menu: the driver dispatches TouchUp
// only for a tap that never became a drag (a drag ends via DragEnd instead),
// so tapping a handle offers the menu for the current selection.
func (s *entrySelectionHandle) TouchUp(*mobile.TouchEvent) {
	s.grabY = -1
	s.owner.showHostMenu(s)
}

// TouchCancel implements mobile.Touchable.
func (s *entrySelectionHandle) TouchCancel(*mobile.TouchEvent) {
	s.grabY = -1
	s.owner.dragging = nil
	s.owner.loupe.dismiss()
}

// Dragged moves this handle's selection edge and shows the magnifier over
// the drag target (pre-rebrand tracker #942).
func (s *entrySelectionHandle) Dragged(d *event.DragEvent) {
	s.owner.dragging = s
	s.owner.dragHandle(s, d)
	s.owner.loupe.show()
}

// DragEnd dismisses the magnifier, re-syncs the handle under its final edge
// and re-summons the host's context menu so the adjusted selection can be
// acted on (the native toolbar-reappears behavior).
func (s *entrySelectionHandle) DragEnd() {
	s.owner.dragging = nil
	s.owner.loupe.dismiss()
	s.owner.host.updateSelectionHandles()
	s.owner.showHostMenu(s)
}

type entrySelectionHandlesRenderer struct {
	h *entrySelectionHandles
}

func (r *entrySelectionHandlesRenderer) Destroy() {
	r.h.stopSync()
}

func (r *entrySelectionHandlesRenderer) Layout(geo.Size) {
	// handle geometry is driven by sync(), not by the overlay's size
}

func (r *entrySelectionHandlesRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *entrySelectionHandlesRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.h.start, r.h.end}
	if r.h.loupe != nil {
		objs = append(objs, r.h.loupe)
	}
	return objs
}

func (r *entrySelectionHandlesRenderer) Refresh() {
	r.h.start.Refresh()
	r.h.end.Refresh()
	if r.h.loupe != nil && r.h.loupe.Visible() {
		r.h.loupe.Refresh()
	}
}

type entrySelectionHandleRenderer struct {
	handle *entrySelectionHandle
	dot    *canvas.Circle
}

func (r *entrySelectionHandleRenderer) Destroy() {}

func (r *entrySelectionHandleRenderer) Layout(size geo.Size) {
	d := size.Width / 2
	r.dot.Resize(geo.NewSize(d, d))
	// the dot sits at the top of the touch target, its top edge on the
	// selection edge's baseline anchor
	r.dot.Move(geo.NewPos((size.Width-d)/2, 0))
}

func (r *entrySelectionHandleRenderer) MinSize() geo.Size {
	return r.handle.desiredSize()
}

func (r *entrySelectionHandleRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.dot}
}

func (r *entrySelectionHandleRenderer) Refresh() {
	r.dot.FillColor = theme.Color(theme.ColorNamePrimary)
	r.dot.Refresh()
}
