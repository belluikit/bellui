package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
)

// TestFlexHBoxLayout_CrossMaxHeight verifies that MaxHeight clamps a child's
// height on the cross axis (instead of stretching to the container height) and
// leaves it top-aligned, while an unconstrained sibling keeps full height.
func TestFlexHBoxLayout_CrossMaxHeight(t *testing.T) {
	a := NewMinSizeRect(geo.NewSize(10, 10))
	b := NewMinSizeRect(geo.NewSize(10, 10))
	objs := []core.CanvasObject{a, b}

	l := &layout.FlexHBoxLayout{
		Flex:      []float32{1, 1},
		MaxHeight: []float32{30, 0}, // clamp a, leave b full-stretch
		Padding:   4,
	}
	l.Layout(objs, geo.NewSize(100, 80))

	assert.Equal(t, float32(30), a.Size().Height, "a clamped to MaxHeight")
	assert.Equal(t, float32(0), a.Position().Y, "a top-aligned on cross axis")
	assert.Equal(t, float32(80), b.Size().Height, "b stretched to full height")
	assert.Equal(t, float32(0), b.Position().Y)
}

// TestFlexHBoxLayout_CrossMinHeight verifies MinHeight raises a child's height
// above the container extent and that MinSize reserves the cross minimum.
func TestFlexHBoxLayout_CrossMinHeight(t *testing.T) {
	a := NewMinSizeRect(geo.NewSize(10, 10))
	objs := []core.CanvasObject{a}

	l := &layout.FlexHBoxLayout{
		Flex:      []float32{1},
		MinHeight: []float32{50},
		Padding:   4,
	}

	// MinSize must reflect the cross minimum (50), not the child's natural 10.
	assert.Equal(t, float32(50), l.MinSize(objs).Height)

	l.Layout(objs, geo.NewSize(100, 20))
	assert.Equal(t, float32(50), a.Size().Height, "a raised to MinHeight even beyond container")
}

// TestFlexVBoxLayout_CrossMaxWidth verifies MaxWidth clamps a child's width on
// the cross axis and leaves it left-aligned, sibling keeps full width.
func TestFlexVBoxLayout_CrossMaxWidth(t *testing.T) {
	a := NewMinSizeRect(geo.NewSize(10, 10))
	b := NewMinSizeRect(geo.NewSize(10, 10))
	objs := []core.CanvasObject{a, b}

	l := &layout.FlexVBoxLayout{
		Flex:     []float32{1, 1},
		MaxWidth: []float32{40, 0},
		Padding:  4,
	}
	l.Layout(objs, geo.NewSize(100, 80))

	assert.Equal(t, float32(40), a.Size().Width, "a clamped to MaxWidth")
	assert.Equal(t, float32(0), a.Position().X, "a left-aligned on cross axis")
	assert.Equal(t, float32(100), b.Size().Width, "b stretched to full width")
	assert.Equal(t, float32(0), b.Position().X)
}

// TestFlexVBoxLayout_CrossMinWidth verifies MinWidth and its MinSize reservation.
func TestFlexVBoxLayout_CrossMinWidth(t *testing.T) {
	a := NewMinSizeRect(geo.NewSize(10, 10))
	objs := []core.CanvasObject{a}

	l := &layout.FlexVBoxLayout{
		Flex:     []float32{1},
		MinWidth: []float32{60},
		Padding:  4,
	}

	assert.Equal(t, float32(60), l.MinSize(objs).Width)

	l.Layout(objs, geo.NewSize(20, 100))
	assert.Equal(t, float32(60), a.Size().Width, "a raised to MinWidth even beyond container")
}

// TestFlexLayout_NoCrossConstraintsFullStretch locks in the backward-compatible
// full-stretch behavior when no cross-axis arrays are set.
func TestFlexLayout_NoCrossConstraintsFullStretch(t *testing.T) {
	h := NewMinSizeRect(geo.NewSize(10, 10))
	hl := &layout.FlexHBoxLayout{Flex: []float32{1}, Padding: 4}
	hl.Layout([]core.CanvasObject{h}, geo.NewSize(100, 80))
	assert.Equal(t, float32(80), h.Size().Height, "HBox child full height by default")

	v := NewMinSizeRect(geo.NewSize(10, 10))
	vl := &layout.FlexVBoxLayout{Flex: []float32{1}, Padding: 4}
	vl.Layout([]core.CanvasObject{v}, geo.NewSize(100, 80))
	assert.Equal(t, float32(100), v.Size().Width, "VBox child full width by default")
}

// TestFlexHBoxLayout_CrossClampDoesNotAffectMainAxis ensures cross clamping is
// independent of the main-axis flex distribution.
func TestFlexHBoxLayout_CrossClampDoesNotAffectMainAxis(t *testing.T) {
	a := NewMinSizeRect(geo.NewSize(10, 10))
	b := NewMinSizeRect(geo.NewSize(10, 10))
	objs := []core.CanvasObject{a, b}

	l := &layout.FlexHBoxLayout{
		Flex:      []float32{1, 3},
		MaxHeight: []float32{20, 20},
		Padding:   0, // 0 means theme default; spacing folded out of the width check below
	}
	size := geo.NewSize(100, 60)
	l.Layout(objs, size)

	// Both heights clamped regardless of unequal main-axis flex.
	assert.Equal(t, float32(20), a.Size().Height)
	assert.Equal(t, float32(20), b.Size().Height)
	// Main axis: b gets 3x a's width (widths sum to the available main extent).
	assert.InDelta(t, 3.0, float64(b.Size().Width/a.Size().Width), 0.01)
}
