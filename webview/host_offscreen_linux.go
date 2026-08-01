//go:build linux && !android && cgo

package webview

// The composited host: a real widget whose content is the CEF-rendered page.
//
// Frames arrive as BGRA pixel buffers (cefEngine.Paint) and are pushed
// through canvas.Raster's streaming path — the exact pipeline hack/cefprobe
// proved end to end. Because the content is composited like any other widget,
// the hole punch's whole limitation list dissolves: overlays and dialogs draw
// above it, Canvas.Capture sees it, and RepaintBoundary, clipping, group
// opacity and DirectionScope all apply. The price is that bellui forwards
// input and owns focus, which is most of this file.
//
// # Coordinates
//
// CEF's OSR contract is: GetViewRect and input events are in device-
// independent points, paint buffers are in pixels (points × the device scale
// factor reported through GetScreenInfo). bellui's widget coordinates are
// points and its canvas scale is the device scale, so positions cross
// unconverted and only the raster's pixel size involves the scale.
//
// # Two things naive OSR integrations miss
//
//   - Popups. A <select> dropdown renders as a second surface (PET_POPUP)
//     with its own show/rect callbacks, and must be composited over the view
//     or dropdowns are invisible. Done in composite below.
//   - HiDPI. The device scale factor must reach CEF or content rasterises at
//     1x and is scaled up blurrily. screenScale reports the canvas scale,
//     re-captured on every layout — a monitor change re-lays-out the window,
//     so the value cannot go stale.
//
// # Focus and the Tab ring
//
// Unlike widget.NativeViewHost — deliberately not a Tab stop, because the OS
// owns the keyboard once a platform view has it (WCAG 2.1.2) — this host is
// event.Focusable: bellui sees every key, so Tab always moves bellui's focus
// and there is no trap to fall into. The trade-off is the mirror image: the
// page's own Tab navigation is not reachable, because Tab is consumed by the
// canvas before TypedKey; page-internal focus moves with the mouse. IME
// composition is #121 and arrives with gocef's binding for it.

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/gocef"
)

type offscreenHost struct {
	widget.BaseWidget

	view   *WebView
	raster *canvas.Raster
	min    geo.Size

	attached bool

	// The view surface (base) is kept as a stable copy so the popup can be
	// re-composited over it without a fresh view frame. display is a two-slot
	// ring: SetPixels hands a buffer to the raster, whose painter may read it
	// under its own lock until the next SetPixels — so the next frame is
	// always built in the other slot, never in place.
	base         []byte
	baseW, baseH int
	display      [2][]byte
	displayIdx   int

	popup                  []byte
	popupW, popupH         int
	popupX, popupY         int // points, scaled at blit time
	popupShown             bool
	lastViewW, lastViewH   int     // points, what ViewSize answers
	scale                  float32 // canvas scale, captured at layout
	focused                bool
	hovered                bool
	cursor                 desktop.StandardCursor
	buttonMask             uint32 // pressed buttons, as CEF event flags
	lastPos                geo.Position
	lastClickAt            time.Time
	lastClickX, lastClickY int
	lastClickButton        gocef.MouseButton
	clickCount             int
}

func newOffscreenHost(w *WebView) *offscreenHost {
	h := &offscreenHost{view: w, scale: 1}
	h.raster = canvas.NewRaster(nil)
	h.raster.Streaming = true
	h.raster.ScaleMode = canvas.ImageScaleFastest
	w.eng.host = h
	h.ExtendBaseWidget(h)
	return h
}

func (h *offscreenHost) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return &offscreenHostRenderer{host: h}
}

func (h *offscreenHost) MinSize() geo.Size {
	return h.min
}

// Hide stops CEF compositing along with the widget.
func (h *offscreenHost) Hide() {
	h.BaseWidget.Hide()
	h.view.SetVisible(false)
}

// Show resumes compositing and asks for a full repaint, since CEF dropped
// its dirty state while hidden.
func (h *offscreenHost) Show() {
	h.BaseWidget.Show()
	h.view.SetVisible(true)
	if b := h.view.eng.browser; b != nil {
		b.Invalidate()
	}
}

// viewSize is what CEF's GetViewRect reads: the widget size in points.
func (h *offscreenHost) viewSize() (int, int) {
	return h.lastViewW, h.lastViewH
}

func (h *offscreenHost) screenScale() float32 {
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// focusSelf drives the programmatic way into the page: canvas focus on this
// host, which FocusGained then forwards to Blink.
func (h *offscreenHost) focusSelf() bool {
	c := h.canvas()
	if c == nil {
		return false
	}
	c.Focus(h)
	return h.focused
}

func (h *offscreenHost) unfocusSelf() {
	c := h.canvas()
	if c == nil || !h.focused {
		return
	}
	c.Unfocus()
}

func (h *offscreenHost) canvas() core.Canvas {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return nil
	}
	return app.Driver().CanvasForObject(h)
}

// ---------------------------------------------------------------------------
// Renderer. Layout is where the view learns its size and scale, and where the
// engine is first attached — the first moment the host is actually on a
// canvas, mirroring when a NativeViewHost attaches its platform view.

type offscreenHostRenderer struct {
	host *offscreenHost
}

func (r *offscreenHostRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.host.raster}
}

func (r *offscreenHostRenderer) Layout(size geo.Size) {
	h := r.host
	h.raster.Resize(size)

	scaleChanged := false
	if c := h.canvas(); c != nil {
		if s := c.Scale(); s != h.scale {
			// The scale changed after creation — a monitor move, or a
			// settings change. CEF has to be told, or content keeps
			// rasterising at the old density. This is the toolkit's half;
			// Chromium currently ignores the answer anyway (#139), so the
			// blurry-HiDPI symptom outlives this fix until that closes.
			h.scale = s
			scaleChanged = true
		}
	}

	w, hh := int(size.Width), int(size.Height)
	if w != h.lastViewW || hh != h.lastViewH || scaleChanged {
		h.lastViewW, h.lastViewH = w, hh
		if b := h.view.eng.browser; b != nil {
			// CEF re-reads the size through ViewSize — and the screen info
			// through GetScreenInfo; WasResized re-queries both, which is why
			// a pure scale change also routes through here. The fields above
			// must change before this call — that ordering is the contract.
			b.Resize(w, hh)
		}
	}

	h.attachOnce()
}

// attachOnce creates the engine the first time the host is laid out. The
// backend needs no native window handle — the browser is offscreen — so nil
// is passed through the portable Attach, which also loads the initial URL. A
// failure (typically ErrRuntimeMissing) leaves the host inert: it occupies
// its space and draws nothing, the same degradation as every other platform.
func (h *offscreenHost) attachOnce() {
	if h.attached {
		return
	}
	h.attached = true
	if err := h.view.Attach(nil); err != nil {
		bellui.LogError("webview: failed to attach the CEF backend", err)
	}
}

func (r *offscreenHostRenderer) MinSize() geo.Size {
	return r.host.min
}

func (r *offscreenHostRenderer) Refresh() {
	r.host.raster.Refresh()
}

func (r *offscreenHostRenderer) Destroy() {
	r.host.view.Destroy()
	r.host.attached = false
}

// ---------------------------------------------------------------------------
// Frames.

// enginePaint receives one CEF frame — BGRA, pixel-sized, valid only for the
// duration of the call — and publishes the composited result.
func (h *offscreenHost) enginePaint(buf []byte, w, hh int, popup bool) {
	if popup {
		n := w * hh * 4
		if cap(h.popup) < n {
			h.popup = make([]byte, n)
		}
		h.popup = h.popup[:n]
		copy(h.popup, buf)
		swizzleBGRA(h.popup)
		h.popupW, h.popupH = w, hh
	} else {
		n := w * hh * 4
		if cap(h.base) < n {
			h.base = make([]byte, n)
		}
		h.base = h.base[:n]
		copy(h.base, buf)
		swizzleBGRA(h.base)
		h.baseW, h.baseH = w, hh
	}
	h.composite()
}

func (h *offscreenHost) enginePopupShow(show bool) {
	h.popupShown = show
	if !show {
		h.popup = h.popup[:0]
		h.popupW, h.popupH = 0, 0
		h.composite()
	}
}

func (h *offscreenHost) enginePopupRect(x, y, w, hh int) {
	h.popupX, h.popupY = x, y
	_ = w // the popup's own paint carries its pixel size; the rect places it
	_ = hh
}

// composite publishes base with the popup blitted over it, into the display
// slot the raster is not currently holding.
func (h *offscreenHost) composite() {
	if h.baseW == 0 || h.baseH == 0 {
		return
	}
	n := h.baseW * h.baseH * 4
	out := h.display[h.displayIdx]
	if cap(out) < n {
		out = make([]byte, n)
	}
	out = out[:n]
	h.display[h.displayIdx] = out
	h.displayIdx = 1 - h.displayIdx

	copy(out, h.base)

	if h.popupShown && h.popupW > 0 {
		// The popup rect is in points; the buffers are pixels.
		px := int(float32(h.popupX) * h.screenScale())
		py := int(float32(h.popupY) * h.screenScale())
		blit(out, h.baseW, h.baseH, h.popup, h.popupW, h.popupH, px, py)
	}

	h.raster.SetPixels(out, h.baseW, h.baseH)
}

// blit copies src over dst at (x, y), clipped to dst. Both are RGBA rows.
func blit(dst []byte, dw, dh int, src []byte, sw, sh, x, y int) {
	for row := 0; row < sh; row++ {
		dy := y + row
		if dy < 0 || dy >= dh {
			continue
		}
		sx, dx := 0, x
		if dx < 0 {
			sx = -dx
			dx = 0
		}
		width := sw - sx
		if over := dx + width - dw; over > 0 {
			width -= over
		}
		if width <= 0 {
			continue
		}
		copy(dst[(dy*dw+dx)*4:(dy*dw+dx+width)*4], src[(row*sw+sx)*4:(row*sw+sx+width)*4])
	}
}

// swizzleBGRA converts CEF's BGRA to the premultiplied RGBA the raster wants,
// in place. ~1.9 ms at 1080p including the copy (hack/cefprobe); moving it
// into the painter's shader is part of the dirty-rect follow-up.
func swizzleBGRA(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		pix[i], pix[i+2] = pix[i+2], pix[i]
	}
}

// ---------------------------------------------------------------------------
// Cursor.

func (h *offscreenHost) engineCursorChange(cursor int) {
	h.cursor = cefCursorToStandard(cursor)
}

func (h *offscreenHost) Cursor() desktop.Cursor {
	return h.cursor
}

// cefCursorToStandard maps cef_cursor_type_t onto bellui's standard cursors.
// Anything without an equivalent falls back to the default arrow.
func cefCursorToStandard(t int) desktop.StandardCursor {
	switch t {
	case 1: // CT_CROSS
		return desktop.CrosshairCursor
	case 2: // CT_HAND
		return desktop.PointerCursor
	case 3: // CT_IBEAM
		return desktop.TextCursor
	case 6, 13, 15: // CT_EASTRESIZE, CT_WESTRESIZE, CT_EASTWESTRESIZE
		return desktop.HResizeCursor
	case 7, 10, 14: // CT_NORTHRESIZE, CT_SOUTHRESIZE, CT_NORTHSOUTHRESIZE
		return desktop.VResizeCursor
	case 37: // CT_NONE
		return desktop.HiddenCursor
	case 41: // CT_GRAB
		return desktop.GrabCursor
	case 42: // CT_GRABBING
		return desktop.GrabbingCursor
	}
	return desktop.DefaultCursor
}

// ---------------------------------------------------------------------------
// Pointer input. Positions are already in points, which is what CEF wants.

func (h *offscreenHost) MouseDown(ev *desktop.MouseEvent) {
	b := h.view.eng.browser
	if b == nil {
		return
	}
	btn := cefMouseButton(ev.Button)
	x, y := int(ev.Position.X), int(ev.Position.Y)
	h.buttonMask |= cefButtonFlag(btn)
	b.SendMouseClick(x, y, h.eventFlags(ev.Modifier), btn, false, h.countClick(btn, x, y))
}

func (h *offscreenHost) MouseUp(ev *desktop.MouseEvent) {
	b := h.view.eng.browser
	if b == nil {
		return
	}
	btn := cefMouseButton(ev.Button)
	x, y := int(ev.Position.X), int(ev.Position.Y)
	b.SendMouseClick(x, y, h.eventFlags(ev.Modifier), btn, true, h.clickCount)
	h.buttonMask &^= cefButtonFlag(btn)
}

// countClick maintains CEF's click_count, which is how a page distinguishes
// word (2) and line (3) selection. bellui's own double-tap window is the
// authority on what counts as the same click.
func (h *offscreenHost) countClick(btn gocef.MouseButton, x, y int) int {
	const slop = 5 // points, matching typical toolkit double-click slop
	now := time.Now()
	same := btn == h.lastClickButton &&
		now.Sub(h.lastClickAt) <= bellui.DoubleTapDelay() &&
		abs(x-h.lastClickX) <= slop && abs(y-h.lastClickY) <= slop
	if same && h.clickCount < 3 {
		h.clickCount++
	} else {
		h.clickCount = 1
	}
	h.lastClickAt, h.lastClickX, h.lastClickY, h.lastClickButton = now, x, y, btn
	return h.clickCount
}

func (h *offscreenHost) HoverIn(ev *event.HoverEvent) {
	h.hovered = true
	h.sendMove(ev.Position, false)
}

func (h *offscreenHost) HoverMoved(ev *event.HoverEvent) {
	h.sendMove(ev.Position, false)
}

// HoverOut tells the page the pointer left, which is what lets it clear its
// hover state.
func (h *offscreenHost) HoverOut() {
	h.hovered = false
	h.sendMove(h.lastPos, true)
}

// Dragged keeps moves flowing while a button is held — the driver routes
// pressed-pointer movement here rather than to Hoverable, and text selection
// in the page depends on it.
func (h *offscreenHost) Dragged(ev *event.DragEvent) {
	h.sendMove(ev.Position, false)
}

func (h *offscreenHost) DragEnd() {}

func (h *offscreenHost) sendMove(pos geo.Position, leave bool) {
	b := h.view.eng.browser
	if b == nil {
		return
	}
	h.lastPos = pos
	b.SendMouseMove(int(pos.X), int(pos.Y), h.eventFlags(0), leave)
}

func (h *offscreenHost) Scrolled(ev *event.ScrollEvent) {
	b := h.view.eng.browser
	if b == nil {
		return
	}
	b.SendMouseWheel(int(ev.Position.X), int(ev.Position.Y),
		h.eventFlags(0), int(ev.Scrolled.DX), int(ev.Scrolled.DY))
}

func cefMouseButton(b desktop.MouseButton) gocef.MouseButton {
	switch b {
	case desktop.MouseButtonSecondary:
		return gocef.MouseRight
	case desktop.MouseButtonTertiary:
		return gocef.MouseMiddle
	}
	return gocef.MouseLeft
}

// CEF event flags (cef_event_flags_t) — the modifier vocabulary every
// send_*_event call speaks.
const (
	cefFlagShift       = 1 << 1
	cefFlagControl     = 1 << 2
	cefFlagAlt         = 1 << 3
	cefFlagLeftMouse   = 1 << 4
	cefFlagMiddleMouse = 1 << 5
	cefFlagRightMouse  = 1 << 6
	cefFlagCommand     = 1 << 7
)

func cefButtonFlag(b gocef.MouseButton) uint32 {
	switch b {
	case gocef.MouseRight:
		return cefFlagRightMouse
	case gocef.MouseMiddle:
		return cefFlagMiddleMouse
	}
	return cefFlagLeftMouse
}

// eventFlags merges the keyboard modifiers of an event with the pointer
// buttons this host knows to be down.
func (h *offscreenHost) eventFlags(mod event.KeyModifier) uint32 {
	flags := h.buttonMask
	if mod&event.KeyModifierShift != 0 {
		flags |= cefFlagShift
	}
	if mod&event.KeyModifierControl != 0 {
		flags |= cefFlagControl
	}
	if mod&event.KeyModifierAlt != 0 {
		flags |= cefFlagAlt
	}
	if mod&event.KeyModifierSuper != 0 {
		flags |= cefFlagCommand
	}
	return flags
}

// ---------------------------------------------------------------------------
// Keyboard input.

func (h *offscreenHost) FocusGained() {
	h.focused = true
	if b := h.view.eng.browser; b != nil {
		b.SetFocus(true)
	}
}

func (h *offscreenHost) FocusLost() {
	h.focused = false
	if b := h.view.eng.browser; b != nil {
		b.SetFocus(false)
	}
}

// TypedRune carries the text; KeyDown/KeyUp carry the key. Chromium expects
// exactly that split — a raw key down with the key code, then a char event
// with the rune — and sending only one of them is the classic reason a page
// sees modifiers but no text, or text that no keydown handler saw.
func (h *offscreenHost) TypedRune(r rune) {
	b := h.view.eng.browser
	if b == nil || r > 0xFFFF {
		// Astral-plane input arrives through IME composition (#121), not as
		// single UTF-16 units; dropping it beats sending half a surrogate.
		return
	}
	b.SendKey(gocef.KeyChar, h.eventFlags(0), int(r), int(r), uint16(r), uint16(r), false)
}

// TypedKey is intentionally empty: the desktop driver also delivers KeyDown
// and KeyUp to a desktop.Keyable, and those carry the state Chromium wants.
func (h *offscreenHost) TypedKey(*event.KeyEvent) {}

func (h *offscreenHost) KeyDown(ev *event.KeyEvent) {
	h.sendKey(ev, gocef.KeyRawDown)
}

func (h *offscreenHost) KeyUp(ev *event.KeyEvent) {
	h.sendKey(ev, gocef.KeyUp)
}

func (h *offscreenHost) sendKey(ev *event.KeyEvent, t gocef.KeyEventType) {
	b := h.view.eng.browser
	if b == nil {
		return
	}
	vk, ok := windowsKeyCode(ev.Name)
	if !ok {
		return
	}
	b.SendKey(t, h.eventFlags(ev.Modifier), vk, ev.Physical.ScanCode, 0, 0, false)
}

// vkCodes maps bellui's portable key names to the Windows virtual-key codes
// CEF's key events speak natively on every platform. Letters, digits and
// F-keys are derived in windowsKeyCode rather than listed.
var vkCodes = map[event.KeyName]int{
	event.KeyBackspace: 0x08, event.KeyTab: 0x09,
	event.KeyReturn: 0x0D, event.KeyEnter: 0x0D, event.KeyEscape: 0x1B,
	event.KeySpace: 0x20, event.KeyPageUp: 0x21, event.KeyPageDown: 0x22,
	event.KeyEnd: 0x23, event.KeyHome: 0x24,
	event.KeyLeft: 0x25, event.KeyUp: 0x26, event.KeyRight: 0x27, event.KeyDown: 0x28,
	event.KeyInsert: 0x2D, event.KeyDelete: 0x2E,
	desktop.KeyShiftLeft: 0x10, desktop.KeyShiftRight: 0x10,
	desktop.KeyControlLeft: 0x11, desktop.KeyControlRight: 0x11,
	desktop.KeyAltLeft: 0x12, desktop.KeyAltRight: 0x12,
	desktop.KeySuperLeft: 0x5B, desktop.KeySuperRight: 0x5B,
	desktop.KeyCapsLock: 0x14, desktop.KeyMenu: 0x5D, desktop.KeyPrintScreen: 0x2C,
	event.KeySemicolon: 0xBA, event.KeyEqual: 0xBB, event.KeyComma: 0xBC,
	event.KeyMinus: 0xBD, event.KeyPeriod: 0xBE, event.KeySlash: 0xBF,
	event.KeyBackTick: 0xC0, event.KeyLeftBracket: 0xDB, event.KeyBackslash: 0xDC,
	event.KeyRightBracket: 0xDD, event.KeyApostrophe: 0xDE,
}

// windowsKeyCode resolves one key name. Keys with no mapping are not
// forwarded — TypedRune still carries their text.
func windowsKeyCode(name event.KeyName) (int, bool) {
	if vk, ok := vkCodes[name]; ok {
		return vk, true
	}
	s := string(name)
	if len(s) == 1 {
		if c := s[0]; (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') {
			return int(c), true
		}
		return 0, false
	}
	if len(s) >= 2 && len(s) <= 3 && s[0] == 'F' { // F1..F12
		n := 0
		for _, r := range s[1:] {
			if r < '0' || r > '9' {
				return 0, false
			}
			n = n*10 + int(r-'0')
		}
		if n >= 1 && n <= 12 {
			return 0x70 + n - 1, true
		}
	}
	return 0, false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// The interfaces the host answers input through; a missing one here is a
// silently dead input path, so they are asserted.
var (
	_ core.Widget        = (*offscreenHost)(nil)
	_ desktop.Mouseable  = (*offscreenHost)(nil)
	_ desktop.Keyable    = (*offscreenHost)(nil)
	_ desktop.Cursorable = (*offscreenHost)(nil)
	_ event.Focusable    = (*offscreenHost)(nil)
	_ event.Hoverable    = (*offscreenHost)(nil)
	_ event.Scrollable   = (*offscreenHost)(nil)
	_ event.Draggable    = (*offscreenHost)(nil)
)
