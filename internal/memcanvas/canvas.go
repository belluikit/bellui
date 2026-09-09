// Package memcanvas holds the in-memory, windowless canvas that both the
// software driver ([github.com/belluikit/bellui/driver/software]) and the test
// harness ([github.com/belluikit/bellui/test]) hand out.
//
// It lives here so neither has to import the other. driver/software is linked
// into every desktop and web binary, and bellui/test pulls in testify and
// gopkg.in/yaml.v3 — half a megabyte of code in the wasm build — so a
// production driver importing the test package puts an assertion library in
// every shipped artifact (#304).
package memcanvas

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

// Painter describes a simple type that can render canvases.
// It is aliased as test.SoftwarePainter.
type Painter interface {
	Paint(core.Canvas) image.Image
}

// WindowlessCanvas provides functionality for a canvas to operate without a window.
// It is aliased as both test.WindowlessCanvas and software.WindowlessCanvas.
type WindowlessCanvas interface {
	core.Canvas

	Padded() bool
	Resize(geo.Size)
	SelectableMode() bellui.SelectableMode
	SetPadded(bool)
	SetScale(float32)
	SetSelectableMode(bellui.SelectableMode)
}

// Canvas is the in-memory canvas. Use [WindowlessCanvas] to refer to it
// through an interface; the exported struct methods beyond that interface
// (SetPainter, Hovered, FocusManager, ObjectTrees) exist for the two packages
// that wrap it.
type Canvas struct {
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
	painter      Painter
	propertyLock sync.RWMutex
}

// New returns a single use in-memory canvas.
// This canvas has no painter so calls to Capture() will return a blank image.
func New() *Canvas {
	c := &Canvas{
		focusMgr: intapp.NewFocusManager(nil),
		padded:   true,
		scale:    1.0,
		size:     geo.NewSize(100, 100),
	}
	c.overlays.Canvas = c
	return c
}

// NewWithPainter creates an in-memory canvas with a specific painter.
// The painter will be used to render in the Capture() call.
func NewWithPainter(painter Painter) *Canvas {
	c := New()
	c.painter = painter

	return c
}

// NewTransparentWithPainter creates an in-memory canvas with a specific painter
// and no background color. The painter will be used to render in the Capture() call.
func NewTransparentWithPainter(painter Painter) *Canvas {
	c := NewWithPainter(painter)
	c.transparent = true

	return c
}

// SetPainter swaps in the painter used by Capture. A nil painter leaves the
// canvas rendering only its background.
func (c *Canvas) SetPainter(painter Painter) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.painter = painter
}

// Hovered reports the object the pointer was last moved onto, as recorded by
// SetHovered. Drivers synthesising pointer movement use the pair to decide
// whether to send HoverMoved or a HoverOut/HoverIn transition.
func (c *Canvas) Hovered() event.Hoverable {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.hovered
}

// SetHovered records the object currently under the pointer. See Hovered.
func (c *Canvas) SetHovered(h event.Hoverable) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.hovered = h
}

func (c *Canvas) Capture() image.Image {
	cache.Clean(true)
	size := c.Size()
	bounds := image.Rect(0, 0, scale.ToScreenCoordinate(c, size.Width), scale.ToScreenCoordinate(c, size.Height))
	img := image.NewNRGBA(bounds)

	c.propertyLock.RLock()
	transparent, painter := c.transparent, c.painter
	c.propertyLock.RUnlock()

	if !transparent {
		draw.Draw(img, bounds, image.NewUniform(theme.Color(theme.ColorNameBackground)), image.Point{}, draw.Src)
	}

	if painter != nil {
		draw.Draw(img, bounds, painter.Paint(c), image.Point{}, draw.Over)
	}

	return img
}

func (c *Canvas) Content() core.CanvasObject {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.content
}

func (c *Canvas) Focus(obj event.Focusable) {
	c.FocusManager().Focus(obj)
}

func (c *Canvas) FocusNext() {
	c.FocusManager().FocusNext()
}

func (c *Canvas) FocusPrevious() {
	c.FocusManager().FocusPrevious()
}

func (c *Canvas) Focused() event.Focusable {
	return c.FocusManager().Focused()
}

func (c *Canvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.NewPos(2, 3), c.Size().SubtractWidthHeight(4, 5)
}

func (c *Canvas) OnTypedKey() func(*event.KeyEvent) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.onTypedKey
}

func (c *Canvas) OnTypedRune() func(rune) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.onTypedRune
}

func (c *Canvas) Overlays() core.OverlayStack {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	return &c.overlays
}

func (c *Canvas) Padded() bool {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.padded
}

func (c *Canvas) PixelCoordinateForPosition(pos geo.Position) (int, int) {
	return int(pos.X * c.scale), int(pos.Y * c.scale)
}

func (c *Canvas) Refresh(core.CanvasObject) {
}

func (c *Canvas) SelectableMode() bellui.SelectableMode {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.selectableMode
}

func (c *Canvas) Resize(size geo.Size) {
	c.propertyLock.Lock()
	c.resized = true
	c.propertyLock.Unlock()

	c.doResize(size)
}

func (c *Canvas) doResize(size geo.Size) {
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
func (c *Canvas) Direction() bellui.Direction {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.layoutDir
}

// SetDirection overrides the user-interface direction for this canvas' whole
// content, regardless of the process-global setting, and re-lays-out the content
// so the change applies. Passing [bellui.DirectionAuto] restores the global
// setting. It mirrors the real drivers' Canvas.SetDirection for tests.
func (c *Canvas) SetDirection(d bellui.Direction) {
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

func (c *Canvas) Scale() float32 {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.scale
}

func (c *Canvas) SetContent(content core.CanvasObject) {
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

func (c *Canvas) SetOnTypedKey(handler func(*event.KeyEvent)) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.onTypedKey = handler
}

func (c *Canvas) SetOnTypedRune(handler func(rune)) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.onTypedRune = handler
}

func (c *Canvas) SetPadded(padded bool) {
	c.propertyLock.Lock()
	c.padded = padded
	c.propertyLock.Unlock()

	c.doResize(c.Size())
}

func (c *Canvas) SetSelectableMode(mode bellui.SelectableMode) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.selectableMode = mode
}

func (c *Canvas) SetScale(scale float32) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.scale = scale
}

func (c *Canvas) Size() geo.Size {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	return c.size
}

func (c *Canvas) Unfocus() {
	c.FocusManager().Focus(nil)
}

// FocusManager returns the focus manager governing the canvas: the topmost
// overlay's when one is shown, otherwise the content's.
func (c *Canvas) FocusManager() *intapp.FocusManager {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	if focusMgr := c.overlays.TopFocusManager(); focusMgr != nil {
		return focusMgr
	}
	return c.focusMgr
}

// ObjectTrees returns the canvas content followed by every overlay, the roots
// a driver walks to resolve absolute positions.
func (c *Canvas) ObjectTrees() []core.CanvasObject {
	overlays := c.Overlays().List()
	trees := make([]core.CanvasObject, 0, len(overlays)+1)
	if c.content != nil {
		trees = append(trees, c.content)
	}
	trees = append(trees, overlays...)
	return trees
}
