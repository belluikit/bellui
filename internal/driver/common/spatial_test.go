package common

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

func matchAll(_ core.CanvasObject) bool { return true }

func TestSpatialIndex_EmptyIndex(t *testing.T) {
	b := newSpatialIndexBuilder(800, 600)
	idx := b.build()
	obj, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(100, 100), matchAll)
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestSpatialIndex_SingleObject(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(100, 50))

	b := newSpatialIndexBuilder(800, 600)
	wt := geo.NewTranslation(200, 100)
	b.add(r, wt, 100, 50, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// Hit inside the object.
	obj, pos, ok := idx.FindObjectAtPositionMatching(geo.NewPos(250, 125), matchAll)
	require.True(t, ok)
	assert.Equal(t, r, obj)
	assert.InDelta(t, 50, pos.X, 0.01)
	assert.InDelta(t, 25, pos.Y, 0.01)

	// Miss — outside the object.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(10, 10), matchAll)
	assert.False(t, ok)

	// Miss — outside canvas.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(-5, -5), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_OverlappingObjects_DeepestWins(t *testing.T) {
	parent := canvas.NewRectangle(color.White)
	parent.Resize(geo.NewSize(200, 200))
	child := canvas.NewRectangle(color.Black)
	child.Resize(geo.NewSize(50, 50))

	b := newSpatialIndexBuilder(800, 600)
	// Parent added first (DFS order: parent before child).
	b.add(parent, geo.NewTranslation(100, 100), 200, 200, geo.NewPos(0, 0), geo.NewSize(800, 600))
	// Child overlaps parent at (150, 150).
	b.add(child, geo.NewTranslation(150, 150), 50, 50, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// Click where both overlap — child (deepest/last) should win.
	obj, pos, ok := idx.FindObjectAtPositionMatching(geo.NewPos(160, 160), matchAll)
	require.True(t, ok)
	assert.Equal(t, child, obj)
	assert.InDelta(t, 10, pos.X, 0.01)
	assert.InDelta(t, 10, pos.Y, 0.01)

	// Click where only parent exists.
	obj, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(110, 110), matchAll)
	require.True(t, ok)
	assert.Equal(t, parent, obj)
}

func TestSpatialIndex_MatchesPredicate(t *testing.T) {
	r1 := canvas.NewRectangle(color.White)
	r1.Resize(geo.NewSize(100, 100))
	r2 := canvas.NewRectangle(color.Black)
	r2.Resize(geo.NewSize(100, 100))

	b := newSpatialIndexBuilder(800, 600)
	b.add(r1, geo.NewTranslation(0, 0), 100, 100, geo.NewPos(0, 0), geo.NewSize(800, 600))
	b.add(r2, geo.NewTranslation(0, 0), 100, 100, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// Only match r1.
	obj, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(50, 50), func(o core.CanvasObject) bool {
		return o == r1
	})
	require.True(t, ok)
	assert.Equal(t, r1, obj)

	// Match none.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(50, 50), func(_ core.CanvasObject) bool {
		return false
	})
	assert.False(t, ok)
}

func TestSpatialIndex_ClipBounds(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(200, 200))

	b := newSpatialIndexBuilder(800, 600)
	// Object at (100, 100) with size 200x200, but clip constrains it to (150, 150, 250, 250).
	b.add(r, geo.NewTranslation(100, 100), 200, 200,
		geo.NewPos(150, 150), geo.NewSize(100, 100))
	idx := b.build()

	// Inside object but outside clip — should miss.
	_, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(120, 120), matchAll)
	assert.False(t, ok)

	// Inside both object and clip — should hit.
	obj, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(200, 200), matchAll)
	require.True(t, ok)
	assert.Equal(t, r, obj)
}

func TestSpatialIndex_FullyClippedObjectSkipped(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(50, 50))

	b := newSpatialIndexBuilder(800, 600)
	// Object at (100, 100) entirely outside clip region (0, 0, 50, 50).
	b.add(r, geo.NewTranslation(100, 100), 50, 50,
		geo.NewPos(0, 0), geo.NewSize(50, 50))
	idx := b.build()

	// The object should not have been inserted at all.
	_, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(125, 125), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_RotatedObject(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(100, 20))

	// Rotate 45° around origin, then translate to (200, 200).
	rot := geo.NewRotation(float32(math.Pi / 4))
	wt := geo.NewTranslation(200, 200).Multiply(rot)

	b := newSpatialIndexBuilder(800, 600)
	b.add(r, wt, 100, 20, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// Point at the center of the rotated rectangle (roughly).
	// Center in local space = (50, 10). In screen space via wt:
	cx, cy := wt.TransformPoint(50, 10)
	obj, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(cx, cy), matchAll)
	require.True(t, ok, "should hit center of rotated object")
	assert.Equal(t, r, obj)

	// Point clearly outside the rotated object.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(200, 200+80), matchAll)
	assert.False(t, ok, "should miss rotated object")
}

func TestSpatialIndex_ScaledObject(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(50, 50))

	// Scale by 2x, position at (100, 100). Screen footprint: (100,100)-(200,200).
	wt := geo.NewTranslation(100, 100).Multiply(geo.NewScale(2, 2))

	b := newSpatialIndexBuilder(800, 600)
	b.add(r, wt, 50, 50, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// Click at (150, 150) — inside the scaled footprint.
	obj, pos, ok := idx.FindObjectAtPositionMatching(geo.NewPos(150, 150), matchAll)
	require.True(t, ok)
	assert.Equal(t, r, obj)
	// Local coords: (150-100)/2 = 25, (150-100)/2 = 25
	assert.InDelta(t, 25, pos.X, 0.01)
	assert.InDelta(t, 25, pos.Y, 0.01)

	// Click at (90, 90) — outside.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(90, 90), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_InvisibleObjectSkipped(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(100, 100))
	r.Hide()

	b := newSpatialIndexBuilder(800, 600)
	b.add(r, geo.NewTranslation(0, 0), 100, 100, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	// The entry is inserted (builder doesn't check visibility), but the query
	// skips invisible objects.
	_, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(50, 50), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_FindAll(t *testing.T) {
	r1 := canvas.NewRectangle(color.White)
	r1.Resize(geo.NewSize(200, 200))
	r2 := canvas.NewRectangle(color.Black)
	r2.Resize(geo.NewSize(100, 100))

	b := newSpatialIndexBuilder(800, 600)
	// DFS order: r1 (parent) then r2 (child).
	b.add(r1, geo.NewTranslation(0, 0), 200, 200, geo.NewPos(0, 0), geo.NewSize(800, 600))
	b.add(r2, geo.NewTranslation(50, 50), 100, 100, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	results := idx.FindAllObjectsAtPositionMatching(geo.NewPos(75, 75), matchAll)
	require.Len(t, results, 2)
	// Deepest (innermost) first after reversal.
	assert.Equal(t, r2, results[0].Object)
	assert.Equal(t, r1, results[1].Object)
}

func TestSpatialIndex_ManyObjects(t *testing.T) {
	const n = 10_000
	objs := make([]*canvas.Rectangle, n)
	b := newSpatialIndexBuilder(1000, 1000)

	for i := range objs {
		r := canvas.NewRectangle(color.Black)
		r.Resize(geo.NewSize(5, 5))
		objs[i] = r
		// Scatter across the canvas.
		x := float32(i%100) * 10
		y := float32(i/100) * 10
		b.add(r, geo.NewTranslation(x, y), 5, 5, geo.NewPos(0, 0), geo.NewSize(1000, 1000))
	}
	idx := b.build()

	// Hit test at a known position.
	obj, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(52, 32), matchAll)
	require.True(t, ok)
	// Object at (50, 30) has index 3*100 + 5 = 305.
	assert.Equal(t, objs[305], obj)

	// Miss — between objects.
	_, _, ok = idx.FindObjectAtPositionMatching(geo.NewPos(56, 36), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_ZeroSizeSkipped(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	// Don't resize — size is 0x0.

	b := newSpatialIndexBuilder(800, 600)
	b.add(r, geo.NewTranslation(100, 100), 0, 0, geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	_, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(100, 100), matchAll)
	assert.False(t, ok)
}

func TestSpatialIndex_EdgeOfCell(t *testing.T) {
	r := canvas.NewRectangle(color.Black)
	r.Resize(geo.NewSize(10, 10))

	// Place at exact cell boundary.
	b := newSpatialIndexBuilder(800, 600)
	b.add(r, geo.NewTranslation(defaultCellSize, defaultCellSize), 10, 10,
		geo.NewPos(0, 0), geo.NewSize(800, 600))
	idx := b.build()

	obj, _, ok := idx.FindObjectAtPositionMatching(
		geo.NewPos(defaultCellSize+5, defaultCellSize+5), matchAll,
	)
	require.True(t, ok)
	assert.Equal(t, r, obj)
}

func TestSpatialIndex_NegativeCoordinates(t *testing.T) {
	b := newSpatialIndexBuilder(800, 600)
	idx := b.build()

	_, _, ok := idx.FindObjectAtPositionMatching(geo.NewPos(-10, -10), matchAll)
	assert.False(t, ok)
}

// BenchmarkSpatialIndex_10K benchmarks hit testing with 10K objects using the
// spatial index. Compare with BenchmarkLinearWalk_10K to see the speedup.
func BenchmarkSpatialIndex_10K(b *testing.B) {
	const n = 10_000
	builder := newSpatialIndexBuilder(1000, 1000)
	for i := range n {
		r := canvas.NewRectangle(color.Black)
		r.Resize(geo.NewSize(5, 5))
		x := float32(i%100) * 10
		y := float32(i/100) * 10
		builder.add(r, geo.NewTranslation(x, y), 5, 5,
			geo.NewPos(0, 0), geo.NewSize(1000, 1000))
	}
	idx := builder.build()
	mouse := geo.NewPos(505, 505)

	b.ResetTimer()
	for range b.N {
		idx.FindObjectAtPositionMatching(mouse, matchAll)
	}
}
