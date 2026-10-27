//go:build !wasm

package mobile

import (
	"image"

	"github.com/belluikit/bellui/internal/cache"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

var _ core.Canvas = (*canvas)(nil)

type canvas struct {
	common.Canvas
	common.TouchHandler
	content        core.CanvasObject
	device         *device
	initialized    bool
	menu           core.CanvasObject
	padded         bool
	selectableMode bellui.SelectableMode
	scale          float32
	size           geo.Size
	windowHead     core.CanvasObject

	// tabEscape is the Escape-then-Tab way out of a widget that keeps Tab (#357).
	tabEscape intdriver.TabEscape

	// touchScrollGen is bumped by every contact that lands, which stops the
	// momentum of an earlier pan; touchScroll is the current pan's scroll
	// chain (touch_scroll.go, #402). Main-goroutine state.
	touchScrollGen int
	touchScroll    touchScrollLatch
	// touchScrollCoast is the touchScrollGen of the pan whose momentum is
	// running, 0 when none is: a contact landing on it catches it (#792).
	touchScrollCoast int

	onTypedKey  func(ev *event.KeyEvent)
	onTypedRune func(rune)
}

func newCanvas(dev bellui.Device) core.Canvas {
	d, _ := dev.(*device)
	ret := &canvas{
		Canvas: common.Canvas{
			OnFocus:   d.handleKeyboard,
			OnUnfocus: d.hideVirtualKeyboard,
		},
		device: d,
		padded: true,
		scale:  dev.SystemScaleForWindow(nil), // we don't need a window parameter on mobile,
	}
	// A focused object is brought into view: when it opens the keyboard, and
	// when the keyboard is already up and covers it (#654).
	ret.OnFocus = func(obj event.Focusable) {
		d.handleKeyboard(obj)
		ret.revealFocused()
	}
	// A tap on the focused widget brings back the keyboard the system hid
	// while the widget kept the focus (#822, #806).
	ret.OnTapFocused = func(obj event.Focusable) {
		if d != nil {
			d.handleKeyboard(obj)
		}
	}
	ret.InitTouchHandler(&mobileFinder{canvas: ret})
	ret.initTouchScroll()
	ret.LongPressCallback = func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}
	ret.OnTapUp = func(pos geo.Position) bool {
		if ret.menu != nil && ret.Overlays().Top() == nil && pos.X > ret.menu.Size().Width {
			ret.menu.Hide()
			ret.menu.Refresh()
			ret.setMenu(nil)
			return true
		}
		return false
	}
	ret.Initialize(ret, ret.overlayChanged)
	return ret
}

// Capture paints a fresh frame and returns it. Any goroutine may call it. The
// paint and read-back run on the main goroutine, the app's event loop, which
// paints the window every frame. Run on the caller's goroutine, they raced
// that paint, and under -tags gl issued GL calls off the loop that owns the
// context (#573; desktop had the same shape, #562). It returns nil when no
// frame can be painted: the window is not the one on screen, its surface is
// not ready, or the painter began no frame.
func (c *canvas) Capture() image.Image {
	if async.IsMainGoroutine() {
		return c.capture()
	}
	var img image.Image
	bellui.CurrentApp().Driver().DoFromGoroutine(func() { img = c.capture() }, true)
	return img
}

func (c *canvas) capture() image.Image {
	// The frame count tells whether the paint below began a frame; what the
	// painter holds otherwise is an earlier frame or nothing (#631).
	counter, counts := c.Painter().(interface{ FramesBegun() uint64 })
	var before uint64
	if counts {
		before = counter.FramesBegun()
	}
	began := func() bool { return !counts || counter.FramesBegun() != before }

	d, _ := bellui.CurrentApp().Driver().(*driver)
	if d == nil {
		return nil
	}
	w := d.currentWindow()
	if w == nil || w.canvas != c || w.surfaceStale || !c.initialized || !d.renderReady(w) {
		return nil
	}
	// Prepare the frame as handlePaint does, so refreshed objects draw their
	// new state rather than cached textures (#565 on desktop).
	c.FreeDirtyTextures()
	size := d.prepareFrame(w)

	if ec, ok := c.Painter().(interface{ EnableCapture() }); ok {
		// WebGPU: arm the per-frame capture copy, then paint and present, which
		// copies the presented surface, overlays included (#473 on desktop).
		ec.EnableCapture()
		d.claimNativeSurface(c, w)
		d.paintWindow(w, size)
		d.presentPainter(c)
		d.app.Publish()
		// presentPainter abandons the frame when Android destroyed the window
		// during the paint, and nothing was copied then.
		if !began() || !d.nativeSurfacePresentable() {
			return nil
		}
		return c.Painter().Capture(c)
	}

	// GL: paint into the back buffer and read it before the swap. OpenGL ES
	// cannot read the default framebuffer's front buffer.
	d.paintWindow(w, size)
	if !began() {
		return nil
	}
	var img image.Image
	if bc, ok := c.Painter().(interface{ CaptureBack(core.Canvas) image.Image }); ok {
		img = bc.CaptureBack(c)
	} else {
		img = c.Painter().Capture(c)
	}
	// The capture paint was not presented; the next paint tick presents it.
	c.SetDirty()
	return img
}

func (c *canvas) Content() core.CanvasObject {
	return c.content
}

func (c *canvas) InteractiveArea() (geo.Position, geo.Size) {
	var pos geo.Position
	var size geo.Size
	if c.device == nil {
		// running in test mode
		size = c.Size()
	} else {
		safeLeft := float32(c.device.safeLeft) / c.scale
		safeTop := float32(c.device.safeTop) / c.scale
		safeRight := float32(c.device.safeRight) / c.scale
		safeBottom := float32(c.device.safeBottom) / c.scale
		pos = geo.NewPos(safeLeft, safeTop)
		size = c.size.SubtractWidthHeight(safeLeft+safeRight, safeTop+safeBottom)
	}
	if c.windowHeadIsDisplacing() {
		offset := c.windowHead.MinSize().Height
		pos = pos.AddXY(0, offset)
		size = size.SubtractWidthHeight(0, offset)
	}
	return pos, size
}

func (c *canvas) MinSize() geo.Size {
	return c.size // TODO check
}

func (c *canvas) OnTypedKey() func(*event.KeyEvent) {
	return c.onTypedKey
}

func (c *canvas) OnTypedRune() func(rune) {
	return c.onTypedRune
}

func (c *canvas) PixelCoordinateForPosition(pos geo.Position) (int, int) {
	return int(pos.X * c.scale), int(pos.Y * c.scale)
}

func (c *canvas) Resize(size geo.Size) {
	if size == c.size {
		return
	}

	c.sizeContent(size)
}

func (c *canvas) Scale() float32 {
	return c.scale
}

func (c *canvas) SelectableMode() bellui.SelectableMode {
	return c.selectableMode
}

func (c *canvas) SetSelectableMode(mode bellui.SelectableMode) {
	cache.NoteSelectableMode(mode)
	c.selectableMode = mode
}

func (c *canvas) SetContent(content core.CanvasObject) {
	c.setContent(content)
	c.sizeContent(c.Size()) // fixed window size for mobile, cannot stretch to new content
	c.SetDirty()
}

func (c *canvas) SetOnTypedKey(typed func(*event.KeyEvent)) {
	c.onTypedKey = typed
}

func (c *canvas) SetOnTypedRune(typed func(rune)) {
	c.onTypedRune = typed
}

func (c *canvas) Size() geo.Size {
	return c.size
}

func (c *canvas) applyThemeOutOfTreeObjects() {
	if c.menu != nil {
		app.ApplyThemeTo(c.menu, c) // Ensure our menu gets the theme change message as it's out-of-tree
	}
	if c.windowHead != nil {
		app.ApplyThemeTo(c.windowHead, c) // Ensure our child windows get the theme change message as it's out-of-tree
	}
}

// FindObjectAtPositionMatching is the dispatch hit-walk: overlays, then the
// menu when one is open, else the window head and content, preferring the
// spatial index when it is current. It also serves the public
// driver.ObjectAtPositionMatching through the canvas capability assertion
// (#245), so it must stay equivalent to what tap dispatch uses.
func (c *canvas) FindObjectAtPositionMatching(pos geo.Position, test func(object core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	overlay := c.Overlays().Top()
	// Only trust the spatial index while the canvas is clean — it is rebuilt
	// during the paint walk and is stale between a change and the next frame (pre-rebrand tracker #590).
	if overlay == nil {
		if idx := c.SpatialIndex(); idx != nil && !c.IsDirty() {
			if obj, lpos, ok := idx.FindObjectAtPositionMatching(pos, test); ok {
				return obj, lpos, 0
			}
			return nil, geo.Position{}, 0
		}
	}
	if c.menu != nil {
		return intdriver.FindObjectAtPositionMatching(pos, test, c.Overlays().List(), c.menu)
	}
	return intdriver.FindObjectAtPositionMatching(pos, test, c.Overlays().List(), c.windowHead, c.content)
}

func (c *canvas) overlayChanged() {
	// A keyboard the user dismissed stays closed through overlay changes;
	// handleKeyboard, from a focus change or a tap, reopens it (#822).
	if c.device == nil || !c.device.keyboardDismissed {
		c.device.handleKeyboard(c.Focused())
	}
	c.SetDirty()
}

func (c *canvas) setContent(content core.CanvasObject) {
	c.content = content
	c.SetContentTreeAndFocusMgr(content)
}

func (c *canvas) setMenu(menu core.CanvasObject) {
	c.menu = menu
	c.SetMenuTreeAndFocusMgr(menu)
}

func (c *canvas) setWindowHead(head core.CanvasObject) {
	if c.padded {
		head = container.NewPadded(head)
	}
	c.windowHead = head
	c.SetMobileWindowHeadTree(head)
}

func (c *canvas) sizeContent(size geo.Size) {
	if c.content == nil { // window may not be configured yet
		return
	}

	c.size = size
	areaPos, areaSize := c.InteractiveArea()

	if c.windowHead != nil {
		var headSize geo.Size
		headPos := areaPos
		if c.windowHeadIsDisplacing() {
			headSize = geo.NewSize(areaSize.Width, c.windowHead.MinSize().Height)
			headPos = headPos.SubtractXY(0, headSize.Height)
		} else {
			headSize = c.windowHead.MinSize()
		}
		c.windowHead.Resize(headSize)
		c.windowHead.Move(headPos)
	}

	for _, overlay := range c.Overlays().List() {
		if p, ok := overlay.(*widget.PopUp); ok {
			// TODO: remove this when pre-rebrand tracker #707 is being addressed.
			// “Notifies” the PopUp of the canvas size change.
			p.Refresh()
		} else {
			overlay.Resize(areaSize)
			overlay.Move(areaPos)
		}
	}

	if c.padded {
		c.content.Resize(areaSize.Subtract(geo.NewSize(theme.Padding()*2, theme.Padding()*2)))
		c.content.Move(areaPos.Add(geo.NewPos(theme.Padding(), theme.Padding())))
	} else {
		c.content.Resize(areaSize)
		c.content.Move(areaPos)
	}
}

// mobileFinder adapts the mobile canvas to the common.ObjectFinder interface.
type mobileFinder struct {
	canvas *canvas
}

func (f *mobileFinder) FindObjectAtPositionMatching(pos geo.Position, test func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	return f.canvas.FindObjectAtPositionMatching(pos, test)
}

func (f *mobileFinder) CanvasForTouch() core.Canvas {
	return f.canvas
}

func (c *canvas) windowHeadIsDisplacing() bool {
	if c.windowHead == nil {
		return false
	}

	chromeBox := c.windowHead.(*bellui.Container)
	if c.padded {
		chromeBox = chromeBox.Objects[0].(*bellui.Container) // the padded container
	}
	return len(chromeBox.Objects) > 1
}
