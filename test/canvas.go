package test

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/memcanvas"
)

var dummyCanvas WindowlessCanvas

// WindowlessCanvas provides functionality for a canvas to operate without a window
type WindowlessCanvas = memcanvas.WindowlessCanvas

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
	return memcanvas.New()
}

// NewCanvasWithPainter allows creation of an in-memory canvas with a specific painter.
// The painter will be used to render in the Capture() call.
func NewCanvasWithPainter(painter SoftwarePainter) WindowlessCanvas {
	return memcanvas.NewWithPainter(painter)
}

// NewTransparentCanvasWithPainter allows creation of an in-memory canvas with a specific painter without a background color.
// The painter will be used to render in the Capture() call.
//
// Since: 2.2
func NewTransparentCanvasWithPainter(painter SoftwarePainter) WindowlessCanvas {
	return memcanvas.NewTransparentWithPainter(painter)
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
