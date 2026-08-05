//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/widget"
)

// Every standard cursor must resolve to a real shape: the table is indexed by
// the enum and a nil entry means "hide the pointer" (that is how HiddenCursor
// works), so a shape the platform cannot create loses the cursor rather than
// degrading it. The diagonals (#269) are the case that bites — an X11/Wayland
// theme without them, or a Cocoa build without the private NSCursor selector,
// yields nil from CreateStandardCursor, and initCursors' fallback is what
// keeps them live. This runs after the driver init in TestMain.
func TestCursors_EveryStandardShapeResolves(t *testing.T) {
	for c := desktop.DefaultCursor; c < desktop.StandardCursor(len(cursors)); c++ {
		if c == desktop.HiddenCursor {
			assert.Nil(t, cursors[c], "HiddenCursor hides the pointer by being nil")
			continue
		}
		assert.NotNil(t, cursors[c], "standard cursor %d resolved to no shape", c)
	}

	// Tripwire: growing the table fails this until it is bumped deliberately,
	// because a new StandardCursor also needs a case in window_wasm.go's
	// belluiToNativeCursor — a missing table entry is clamped to the arrow and
	// a missing wasm case falls through to CSS "default", both silently.
	assert.Len(t, cursors, int(desktop.NESWResizeCursor)+1,
		"a new StandardCursor needs a shape here and a case in window_wasm.go")
}

// The InnerWindow resize corner shows the diagonal resize cursor (#269) even
// though it stacks over the window's content: the driver keeps the top-most
// Cursorable under the pointer while it searches on for a Hoverable, and the
// corner is drawn last. The corner is deliberately not Hoverable itself (#263
// keeps it drag-only), so nothing else pins this down.
func TestWindow_CursorInnerWindowCorner(t *testing.T) {
	w := createWindow("Test")
	// The entry fills the inner window's content area, corner included, and
	// carries a cursor of its own — so the corner has to win to be seen.
	inner := container.NewInnerWindow("Title", widget.NewEntry())
	w.SetPadded(false)
	w.SetContent(inner)
	size := geo.NewSize(200, 200)
	w.Resize(size)
	repaintWindow(w)
	ensureCanvasSize(t, w, size)

	at := func(pos geo.Position) (float64, float64) {
		c := w.Canvas()
		return float64(scale.ToScreenCoordinate(c, pos.X)), float64(scale.ToScreenCoordinate(c, pos.Y))
	}

	// Bottom-right, inside the 16dp corner.
	x, y := at(geo.NewPos(size.Width-4, size.Height-4))
	w.mouseMoved(w.viewport, x, y)
	assert.Equal(t, desktop.Cursor(desktop.NWSEResizeCursor), w.cursor,
		"the resize corner must show the diagonal cursor over the content beneath it")

	// Away from the corner the content's own cursor still wins.
	x, y = at(geo.NewPos(size.Width/2, size.Height/2))
	w.mouseMoved(w.viewport, x, y)
	assert.Equal(t, desktop.Cursor(desktop.TextCursor), w.cursor,
		"content keeps its cursor outside the corner")
}
