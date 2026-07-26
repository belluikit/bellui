package common

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// CanvasForObject returns the canvas for the specified object.
func CanvasForObject(obj core.CanvasObject) core.Canvas {
	return cache.GetCanvasForObject(obj)
}
