package mobile

import (
	"image"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/app"
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
	ret.InitTouchHandler(&mobileFinder{canvas: ret})
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

func (c *canvas) Capture() image.Image {
	return c.Painter().Capture(c)
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

func (c *canvas) findObjectAtPositionMatching(pos geo.Position, test func(object core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	overlay := c.Overlays().Top()
	// Only trust the spatial index while the canvas is clean — it is rebuilt
	// during the paint walk and is stale between a change and the next frame (#590).
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
	c.device.handleKeyboard(c.Focused())
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
			// TODO: remove this when #707 is being addressed.
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
	return f.canvas.findObjectAtPositionMatching(pos, test)
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
