// Package dev provides developer tools for Bellui applications.
//
// Call dev.Enable(window) during development to add:
//   - F12: toggle layout inspector overlay (widget boundaries + type labels)
//   - F11: dump widget tree to console
//   - F10: toggle refresh tracing
//
// These tools are safe to ship — they add no overhead unless toggled on.
//
//	func main() {
//	    w := app.New().NewWindow("My App")
//	    dev.Enable(w) // add dev tools
//	    bellui.ShowAndRun(w)
//	}
package dev

import (
	"fmt"
	"image/color"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// Enable adds developer tools to the given window.
// Press F12 to toggle the layout inspector overlay,
// F11 to dump the widget tree to the console,
// F10 to toggle refresh/layout tracing.
func Enable(w bellui.Window) {
	inspector := &layoutInspector{window: w}

	prev := w.Canvas().OnTypedKey()
	w.Canvas().SetOnTypedKey(func(ev *event.KeyEvent) {
		switch ev.Name {
		case event.KeyF12:
			inspector.toggle()
			return
		case event.KeyF11:
			log.Println("=== Widget Tree ===")
			log.Println(widget.InspectLayout(w.Content()))
			return
		case event.KeyF10:
			widget.DebugRefresh = !widget.DebugRefresh
			widget.DebugLayout = !widget.DebugLayout
			if widget.DebugRefresh {
				log.Println("[dev] Refresh/Layout tracing ON")
			} else {
				log.Println("[dev] Refresh/Layout tracing OFF")
			}
			return
		}

		if prev != nil {
			prev(ev)
		}
	})

	log.Println("[dev] Developer tools enabled (F12=inspector, F11=tree dump, F10=trace)")
}

// layoutInspector draws colored borders around every widget in the tree.
type layoutInspector struct {
	window  bellui.Window
	overlay core.CanvasObject
	active  bool
}

func (li *layoutInspector) toggle() {
	if li.active {
		li.window.Canvas().Overlays().Remove(li.overlay)
		li.overlay = nil
		li.active = false
		log.Println("[dev] Inspector OFF")
		return
	}

	li.overlay = li.buildOverlay()
	li.window.Canvas().Overlays().Add(li.overlay)
	li.active = true
	log.Println("[dev] Inspector ON — widget boundaries visible")
}

func (li *layoutInspector) buildOverlay() core.CanvasObject {
	content := li.window.Content()
	if content == nil {
		return canvas.NewRectangle(color.Transparent)
	}

	var objects []core.CanvasObject
	li.walkObjects(content, geo.NewPos(0, 0), 0, &objects)

	return container.NewWithoutLayout(objects...)
}

// colors for different nesting depths.
var depthColors = []color.Color{
	color.NRGBA{R: 255, G: 0, B: 0, A: 80},   // red
	color.NRGBA{R: 0, G: 150, B: 255, A: 80}, // blue
	color.NRGBA{R: 0, G: 200, B: 0, A: 80},   // green
	color.NRGBA{R: 255, G: 165, B: 0, A: 80}, // orange
	color.NRGBA{R: 180, G: 0, B: 255, A: 80}, // purple
	color.NRGBA{R: 0, G: 200, B: 200, A: 80}, // cyan
}

var borderColors = []color.Color{
	color.NRGBA{R: 255, G: 0, B: 0, A: 200},
	color.NRGBA{R: 0, G: 150, B: 255, A: 200},
	color.NRGBA{R: 0, G: 200, B: 0, A: 200},
	color.NRGBA{R: 255, G: 165, B: 0, A: 200},
	color.NRGBA{R: 180, G: 0, B: 255, A: 200},
	color.NRGBA{R: 0, G: 200, B: 200, A: 200},
}

func (li *layoutInspector) walkObjects(obj core.CanvasObject, absPos geo.Position, depth int, out *[]core.CanvasObject) {
	if obj == nil || !obj.Visible() {
		return
	}

	pos := absPos.Add(obj.Position())
	size := obj.Size()

	if size.Width > 0 && size.Height > 0 {
		ci := depth % len(depthColors)

		// Border rectangle.
		border := canvas.NewRectangle(depthColors[ci])
		border.StrokeColor = borderColors[ci]
		border.StrokeWidth = 1
		border.Move(pos)
		border.Resize(size)
		*out = append(*out, border)

		// Type label.
		typeName := fmt.Sprintf("%T", obj)
		label := canvas.NewText(typeName, borderColors[ci])
		label.TextSize = 8
		label.Move(pos.AddXY(2, 1))
		*out = append(*out, label)
	}

	// Descend into children.
	switch c := obj.(type) {
	case *bellui.Container:
		for _, child := range c.Objects {
			li.walkObjects(child, pos, depth+1, out)
		}
	case core.Widget:
		r := cache.Renderer(c)
		if r != nil {
			for _, child := range r.Objects() {
				li.walkObjects(child, pos, depth+1, out)
			}
		}
	}
}
