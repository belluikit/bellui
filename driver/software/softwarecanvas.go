package software

import (
	"github.com/belluikit/bellui/internal/memcanvas"
	"github.com/belluikit/bellui/internal/painter/software"
)

// WindowlessCanvas is a canvas that renders without a window.
// It is the same type as test.WindowlessCanvas.
//
// Since: 1.1
type WindowlessCanvas = memcanvas.WindowlessCanvas

// NewCanvas creates a new canvas in memory that can render without hardware support.
func NewCanvas() WindowlessCanvas {
	return memcanvas.NewWithPainter(software.NewPainter())
}

// NewTransparentCanvas creates a new canvas in memory that can render without hardware support without a background color.
//
// Since: 1.0
func NewTransparentCanvas() WindowlessCanvas {
	return memcanvas.NewTransparentWithPainter(software.NewPainter())
}
