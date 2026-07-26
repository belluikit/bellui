package test

import (
	"image"
	"image/draw"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	intapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/theme"
)

var dummyCanvas WindowlessCanvas

// WindowlessCanvas provides functionality for a canvas to operate without a window
type WindowlessCanvas interface {
	core.Canvas

	Padded() bool
	Resize(geo.Size)
	SelectableMode() bellui.SelectableMode
	SetPadded(bool)
	SetScale(float32)
	SetSelectableMode(bellui.SelectableMode)
}

type canvas struct {
	size    geo.Size
	resized bool
	scale   float32

	content        core.CanvasObject
	overlays       internal.OverlayStack
	focusMgr       *intapp.FocusManager
	hovered        event.Hoverable
	padded         bool
	selectableMode bellui.SelectableMode
	transparent    bool
	layoutDir      bellui.Direction

	onTypedRune func(rune)
	onTypedKey  func(*event.KeyEvent)

	bellui.ShortcutHandler
	painter      SoftwarePainter
	propertyLock sync.RWMutex
}

// Canvas returns a reusable in-memory canvas used for testing
func Canvas() core.Canvas {
	if dummyCanvas == nil {
		dummyCanvas = NewCanvas()
	}

	return dummyCanvas
}

// NewCanvas returns a single use in-memory canvas used for testing.
// This canvas has no painter so calls to Capture() will return a blank image.
func NewCanvas() WindowlessCanvas {
	c := &canvas{
		focusMgr: intapp.NewFocusManager(nil),
		padded:   true,
		scale:    1.0,
		size:     geo.NewSize(100, 100),
	}
	c.overlays.Canvas = c
	return c
}

// NewCanvasWithPainter allows creation of an in-memory canvas with a specific painter.
// The painter will be used to render in the Capture() call.
func NewCanvasWithPainter(painter SoftwarePainter) WindowlessCanvas {
	c := NewCanvas().(*canvas)
	c.painter = painter

	return c
}

// NewTransparentCanvasWithPainter allows creation of an in-memory canvas with a specific painter without a background color.
// The painter will be used to render in the Capture() call.
//
// Since: 2.2
func NewTransparentCanvasWithPainter(painter SoftwarePainter) WindowlessCanvas {
	c := NewCanvasWithPainter(painter).(*canvas)
	c.transparent = true

	return c
}

func (c *canvas) Capture() image.Image {
	cache.Clean(true)
	size := c.Size()
	bounds := image.Rect(0, 0, scale.ToScreenCoordinate(c, size.Width), scale.ToScreenCoordinate(c, size.Height))
	img := image.NewNRGBA(bounds)
	if !c.transparent {
		draw.Draw(img, bounds, image.NewUniform(theme.Color(theme.ColorNameBackground)), image.Point{}, draw.Src)
	}

	if c.painter != nil {
		draw.Draw(img, bounds, c.painter.Paint(c), image.Point{}, draw.Over)
	}

	return img
}

func (c *canvas) Content() core.CanvasObject {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.content
}

func (c *canvas) Focus(obj event.Focusable) {
	c.focusManager().Focus(obj)
}

func (c *canvas) FocusNext() {
	c.focusManager().FocusNext()
}

func (c *canvas) FocusPrevious() {
	c.focusManager().FocusPrevious()
}

func (c *canvas) Focused() event.Focusable {
	return c.focusManager().Focused()
}

func (c *canvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.NewPos(2, 3), c.Size().SubtractWidthHeight(4, 5)
}

func (c *canvas) OnTypedKey() func(*event.KeyEvent) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.onTypedKey
}

func (c *canvas) OnTypedRune() func(rune) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.onTypedRune
}

func (c *canvas) Overlays() core.OverlayStack {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	return &c.overlays
}

func (c *canvas) Padded() bool {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.padded
}

func (c *canvas) PixelCoordinateForPosition(pos geo.Position) (int, int) {
	return int(pos.X * c.scale), int(pos.Y * c.scale)
}

func (c *canvas) Refresh(core.CanvasObject) {
}

func (c *canvas) SelectableMode() bellui.SelectableMode {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.selectableMode
}

func (c *canvas) Resize(size geo.Size) {
	c.propertyLock.Lock()
	c.resized = true
	c.propertyLock.Unlock()

	c.doResize(size)
}

func (c *canvas) doResize(size geo.Size) {
	c.propertyLock.Lock()
	content := c.content
	padded := c.padded
	dir := c.layoutDir
	c.size = size
	c.propertyLock.Unlock()

	if content == nil {
		return
	}

	// Ensure testcanvas mimics real canvas.Resize behavior.
	// OverlayStack locks internally; copying it would copy its mutex.
	for _, overlay := range c.overlays.List() {
		type popupWidget interface {
			core.CanvasObject
			ShowAtPosition(geo.Position)
		}
		if p, ok := overlay.(popupWidget); ok {
			// TODO: remove this when #707 is being addressed.
			// “Notifies” the PopUp of the canvas size change.
			p.Refresh()
		} else {
			overlay.Resize(size)
		}
	}

	// Scope layout to the per-canvas direction, mirroring the real drivers'
	// common.Canvas.EnsureMinSize push, so a window-wide SetDirection mirrors
	// containers and resolves leading/trailing alignment on the test canvas too.
	uidir.Push(dir)
	defer uidir.Pop(dir)

	if padded {
		padding := theme.Padding()
		content.Resize(size.Subtract(geo.NewSquareSize(padding * 2)))
		content.Move(geo.NewSquareOffsetPos(padding))
	} else {
		content.Resize(size)
		content.Move(geo.NewPos(0, 0))
	}
}

// Direction returns the per-canvas user-interface direction set by
// [canvas.SetDirection]. The default [bellui.DirectionAuto] defers to the
// process-global [bellui.SetLayoutDirection]. It is the test-canvas counterpart of
// the optional direction interface implemented by the real drivers' canvas, so
// the software painter scopes paint to it.
func (c *canvas) Direction() bellui.Direction {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.layoutDir
}

// SetDirection overrides the user-interface direction for this canvas' whole
// content, regardless of the process-global setting, and re-lays-out the content
// so the change applies. Passing [bellui.DirectionAuto] restores the global
// setting. It mirrors the real drivers' Canvas.SetDirection for tests.
func (c *canvas) SetDirection(d bellui.Direction) {
	c.propertyLock.Lock()
	if c.layoutDir == d {
		c.propertyLock.Unlock()
		return
	}
	c.layoutDir = d
	size := c.size
	c.propertyLock.Unlock()

	c.doResize(size)
}

func (c *canvas) Scale() float32 {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.scale
}

func (c *canvas) SetContent(content core.CanvasObject) {
	c.propertyLock.Lock()
	c.content = content
	c.focusMgr = intapp.NewFocusManager(c.content)
	resized := c.resized
	c.propertyLock.Unlock()

	if content == nil {
		return
	}

	minSize := content.MinSize()
	if c.padded {
		minSize = minSize.Add(geo.NewSquareSize(theme.Padding() * 2))
	}

	if resized {
		c.doResize(c.Size().Max(minSize))
	} else {
		c.doResize(minSize)
	}
}

func (c *canvas) SetOnTypedKey(handler func(*event.KeyEvent)) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.onTypedKey = handler
}

func (c *canvas) SetOnTypedRune(handler func(rune)) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.onTypedRune = handler
}

func (c *canvas) SetPadded(padded bool) {
	c.propertyLock.Lock()
	c.padded = padded
	c.propertyLock.Unlock()

	c.doResize(c.Size())
}

func (c *canvas) SetSelectableMode(mode bellui.SelectableMode) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.selectableMode = mode
}

func (c *canvas) SetScale(scale float32) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.scale = scale
}

func (c *canvas) Size() geo.Size {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.size
}

func (c *canvas) Unfocus() {
	c.focusManager().Focus(nil)
}

func (c *canvas) focusManager() *intapp.FocusManager {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	if focusMgr := c.overlays.TopFocusManager(); focusMgr != nil {
		return focusMgr
	}
	return c.focusMgr
}

func (c *canvas) objectTrees() []core.CanvasObject {
	overlays := c.Overlays().List()
	trees := make([]core.CanvasObject, 0, len(overlays)+1)
	if c.content != nil {
		trees = append(trees, c.content)
	}
	trees = append(trees, overlays...)
	return trees
}

func layoutAndCollect(objects []core.CanvasObject, o core.CanvasObject, size geo.Size) []core.CanvasObject {
	objects = append(objects, o)
	switch c := o.(type) {
	case core.Widget:
		r := c.CreateRenderer()
		r.Layout(size)
		for _, child := range r.Objects() {
			objects = layoutAndCollect(objects, child, child.Size())
		}
	case *bellui.Container:
		if c.Layout != nil {
			c.Layout.Layout(c.Objects, size)
		}
		for _, child := range c.Objects {
			objects = layoutAndCollect(objects, child, child.Size())
		}
	}
	return objects
}
