package driver_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestGeometryForObject_AgreesWithTheHelpersItReplaces is the contract test:
// one walk must answer exactly what the separate calls answered, since
// widget.NativeViewHost swapped them for it on a per-frame path.
//
// Position is checked against Driver.AbsolutePositionForObject *plus* the
// interactive-area inset, because the driver subtracts that inset and this
// helper deliberately does not — it reports position and clip in one space so
// that intersecting them is meaningful. The test canvas has a non-zero inset,
// so this is a live assertion rather than a tautology.
func TestGeometryForObject_AgreesWithTheHelpersItReplaces(t *testing.T) {
	test.NewTempApp(t)

	unclipped := widget.NewLabel("drawn against the canvas")
	clipped := widget.NewLabel("content wider than its viewport, so it is clipped")
	scroll := container.NewScroll(clipped)
	content := container.NewVBox(unclipped, scroll)

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(geo.NewSize(160, 120))

	c := w.Canvas()
	d := bellui.CurrentApp().Driver()

	for _, tt := range []struct {
		name string
		obj  core.CanvasObject
	}{
		{"unclipped", unclipped},
		{"inside a scroll", clipped},
		{"the scroll itself", scroll},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos, clip, isClipped, ok := driver.GeometryForObject(c, tt.obj)

			_, wantOK := driver.AncestorsOfObject(c, tt.obj)
			wantClip, wantClipped := driver.ClipBoundsForObject(c, tt.obj)
			inset, _ := c.InteractiveArea()
			wantPos := d.AbsolutePositionForObject(tt.obj).Add(inset)

			assert.Equal(t, wantOK, ok, "found-ness must match AncestorsOfObject")
			assert.Equal(t, wantPos, pos, "position must be the driver's, un-inset")
			assert.Equal(t, wantClipped, isClipped, "clipped-ness must match ClipBoundsForObject")
			assert.Equal(t, wantClip, clip, "clip bounds must match ClipBoundsForObject")
		})
	}
}

func TestGeometryForObject_NotInTree(t *testing.T) {
	test.NewTempApp(t)

	w := test.NewWindow(container.NewWithoutLayout())
	defer w.Close()

	orphan := canvas.NewRectangle(color.White)
	_, _, _, ok := driver.GeometryForObject(w.Canvas(), orphan)
	assert.False(t, ok, "an object outside the tree is not found")
}

// TestGeometryForObject_HiddenIsNotFound locks the property NativeViewHost
// relies on to hide its native view: the walk is over the *visible* tree, so
// hiding the object or any ancestor reports the same not-found as removing it.
func TestGeometryForObject_HiddenIsNotFound(t *testing.T) {
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	inner := container.NewWithoutLayout(leaf)
	w := test.NewWindow(container.NewWithoutLayout(inner))
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	_, _, _, ok := driver.GeometryForObject(w.Canvas(), leaf)
	require.True(t, ok, "a visible object is found")

	leaf.Hide()
	_, _, _, ok = driver.GeometryForObject(w.Canvas(), leaf)
	assert.False(t, ok, "a hidden object is not found")

	leaf.Show()
	inner.Hide()
	_, _, _, ok = driver.GeometryForObject(w.Canvas(), leaf)
	assert.False(t, ok, "an object with a hidden ancestor is not found")
}

func TestGeometryForObject_FindsOverlayContent(t *testing.T) {
	test.NewTempApp(t)

	w := test.NewWindow(container.NewWithoutLayout())
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	overlay := canvas.NewRectangle(color.Black)
	overlay.Move(geo.NewPos(20, 30))
	c := w.Canvas()
	c.Overlays().Add(overlay)

	pos, _, _, ok := driver.GeometryForObject(c, overlay)
	require.True(t, ok, "content in an overlay is part of the visible tree")
	assert.Equal(t, geo.NewPos(20, 30), pos)
}

func TestGeometryForObject_NilSafe(t *testing.T) {
	_, _, _, ok := driver.GeometryForObject(nil, nil)
	assert.False(t, ok, "nil canvas/object reports not found")
}

// TestGeometryForObject_DoesNotAllocate is the reason the helper exists: it
// runs every frame for every hosted native view (#61), where the three calls
// it replaces cost one path slice plus a roots slice each.
func TestGeometryForObject_DoesNotAllocate(t *testing.T) {
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	tree := core.CanvasObject(leaf)
	for range 8 {
		tree = container.NewWithoutLayout(tree)
	}
	w := test.NewWindow(tree)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	c := w.Canvas()
	allocs := testing.AllocsPerRun(100, func() {
		driver.GeometryForObject(c, leaf)
	})
	assert.Zero(t, allocs, "the per-frame geometry query must not allocate")
}
