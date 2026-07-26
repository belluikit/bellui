// Package gpu defines the backend-neutral painter interfaces used by the
// drivers. A concrete GPU backend (e.g. the OpenGL painter in
// internal/painter/gl, or a future WebGPU painter) implements these so the
// drivers never depend on a specific graphics API. The backend is chosen at
// the driver's NewPainter call site.
package gpu

import (
	"image"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Painter defines the functionality of a Bellui GPU renderer.
type Painter interface {
	// Init tell a new painter to initialise, usually called after a context is available
	Init()
	// Capture requests that the specified canvas be drawn to an in-memory image
	Capture(core.Canvas) image.Image
	// Clear tells our painter to prepare a fresh paint
	Clear()
	// Free is used to indicate that a certain canvas object is no longer needed
	Free(core.CanvasObject)
	// Paint a single core.CanvasObject but not its children.
	Paint(core.CanvasObject, geo.Position, geo.Size)
	// SetFrameBufferScale tells us when we have more than 1 framebuffer pixel for each output pixel
	SetFrameBufferScale(float32)
	// SetOutputSize is used to change the resolution of our output viewport
	SetOutputSize(int, int)
	// StartClipping tells us that the following paint actions should be clipped to the specified area.
	StartClipping(geo.Position, geo.Size)
	// StopClipping stops clipping paint actions.
	StopClipping()
	// StartRoundedClipping enables stencil-based clipping to a rounded rectangle.
	StartRoundedClipping(geo.Position, geo.Size, float32)
	// StopRoundedClipping disables stencil-based clipping.
	StopRoundedClipping()
	// StartPathClipping enables stencil-based clipping to an arbitrary path shape.
	StartPathClipping(*canvas.Path, geo.Position, geo.Size)
	// StopPathClipping disables stencil-based path clipping.
	StopPathClipping()
	// SetGroupOpacity sets an opacity multiplier applied to all subsequent draw calls.
	// Used for MD3 disabled state (0.38 opacity).
	SetGroupOpacity(float32)
	// ClearGroupOpacity resets the group opacity to 1.0 (fully opaque).
	ClearGroupOpacity()
	// SetTransform sets the world transform matrix for subsequent draw calls.
	// The matrix is the composed transform from root to the current object.
	SetTransform(geo.Matrix)
}

// BoundaryFBO is an opaque, backend-specific handle to a RepaintBoundary's
// cached off-screen render target. The paint walk treats it as opaque, only
// marking it clean once the subtree has been rendered into it.
type BoundaryFBO interface {
	// SetDirty marks the boundary's cached target as needing (or no longer
	// needing) a re-render of its subtree.
	SetDirty(bool)
}

// BoundaryPainter is an optional interface for painters that support
// RepaintBoundary FBO caching. The paint walk type-asserts to this.
type BoundaryPainter interface {
	EnsureBoundaryFBO(obj core.CanvasObject, pixW, pixH int, logicalSize geo.Size) BoundaryFBO
	BindBoundaryFBO(state BoundaryFBO)
	UnbindBoundaryFBO(canvasPixW, canvasPixH int)
	BlitBoundaryTexture(state BoundaryFBO, pos geo.Position, frame geo.Size)
	FreeBoundaryFBO(obj core.CanvasObject)
	MarkBoundaryDirty(obj core.CanvasObject)
	IsBoundaryClean(obj core.CanvasObject) bool
	InvalidateAllBoundaries()
}

// DirtyRegionPainter is an optional interface for painters that support
// a persistent canvas FBO for dirty-region culling. The paint walk
// type-asserts to this interface.
type DirtyRegionPainter interface {
	// EnsureCanvasFBO creates or resizes the full-canvas FBO.
	EnsureCanvasFBO(pixW, pixH int)
	// BindCanvasFBO binds the canvas FBO as the render target.
	BindCanvasFBO()
	// UnbindAndBlitCanvasFBO unbinds the canvas FBO, restores the default
	// framebuffer, and blits the FBO texture as a full-screen quad.
	UnbindAndBlitCanvasFBO(pixW, pixH int)
	// HasCanvasFBO returns true if the canvas FBO is allocated and ready.
	HasCanvasFBO() bool
	// CanvasFBONeedsFullClear returns true if the FBO was just created or
	// resized and needs a full repaint (no previous content to preserve).
	CanvasFBONeedsFullClear() bool
	// MarkCanvasFBOCleared records that the canvas FBO has been fully cleared
	// at least once, so subsequent frames can use partial clears.
	MarkCanvasFBOCleared()
}
