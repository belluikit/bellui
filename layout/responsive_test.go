package layout_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestResponsiveLayout_SingleObjectFullWidth(t *testing.T) {
	obj := NewMinSizeRect(geo.NewSize(50, 50))
	c := layout.NewResponsiveLayout(obj) // no ratios: full row in every class
	c.Resize(geo.NewSize(500, 300))

	wrap := c.Objects[0]
	assert.Equal(t, geo.NewPos(0, 0), wrap.Position())
	assert.Equal(t, float32(500), wrap.Size().Width)
	assert.Equal(t, float32(50), wrap.Size().Height, "row height follows the child's min height")
}

func TestResponsiveLayout_SizeClasses(t *testing.T) {
	obj := NewMinSizeRect(geo.NewSize(10, 10))
	c := layout.NewResponsiveLayout(layout.Responsive(obj, 1, .5, .25, .2, .1))

	for _, tt := range []struct {
		width float32
		ratio float32
		name  string
	}{
		{500, 1, "compact <600"},
		{700, .5, "medium <840"},
		{1000, .25, "expanded <1200"},
		{1400, .2, "large <1600"},
		{1800, .1, "extra-large >=1600"},
	} {
		c.Resize(geo.NewSize(tt.width, 300))
		assert.Equal(t, tt.ratio*tt.width, c.Objects[0].Size().Width, tt.name)
	}
}

func TestResponsiveLayout_RatioInheritance(t *testing.T) {
	obj := NewMinSizeRect(geo.NewSize(10, 10))
	c := layout.NewResponsiveLayout(layout.Responsive(obj, 1, .5)) // expanded+ inherit .5

	c.Resize(geo.NewSize(1000, 300))
	assert.Equal(t, float32(500), c.Objects[0].Size().Width)
	c.Resize(geo.NewSize(1800, 300))
	assert.Equal(t, float32(900), c.Objects[0].Size().Width)
}

func TestResponsiveLayout_RowSharingAndWrap(t *testing.T) {
	pad := theme.Padding()
	obj1 := NewMinSizeRect(geo.NewSize(10, 40))
	obj2 := NewMinSizeRect(geo.NewSize(10, 60))
	obj3 := NewMinSizeRect(geo.NewSize(10, 30))
	c := layout.NewResponsiveLayout(
		layout.Responsive(obj1, .5),
		layout.Responsive(obj2, .5),
		layout.Responsive(obj3, .5),
	)
	W := float32(400)
	c.Resize(geo.NewSize(W, 300))

	// Row 1: two halves share the width minus the padding between them.
	half := (W - pad) / 2
	w1, w2, w3 := c.Objects[0], c.Objects[1], c.Objects[2]
	assert.Equal(t, geo.NewPos(0, 0), w1.Position())
	assert.Equal(t, half, w1.Size().Width)
	assert.Equal(t, geo.NewPos(half+pad, 0), w2.Position())
	assert.Equal(t, half, w2.Size().Width)

	// Both members of a row get the row height (tallest member).
	assert.Equal(t, float32(60), w1.Size().Height)
	assert.Equal(t, float32(60), w2.Size().Height)

	// Row 2: the third half wraps below and keeps its ratio (like a
	// Bootstrap col-6 alone on its row), leaving the rest of the row empty.
	assert.Equal(t, geo.NewPos(0, 60+pad), w3.Position())
	assert.Equal(t, W/2, w3.Size().Width)
}

func TestResponsiveLayout_ThirdsFillRow(t *testing.T) {
	pad := theme.Padding()
	third := float32(1) / 3
	objs := []core.CanvasObject{
		layout.Responsive(NewMinSizeRect(geo.NewSize(10, 10)), third),
		layout.Responsive(NewMinSizeRect(geo.NewSize(10, 10)), third),
		layout.Responsive(NewMinSizeRect(geo.NewSize(10, 10)), third),
	}
	c := layout.NewResponsiveLayout(objs...)
	W := float32(500)
	c.Resize(geo.NewSize(W, 300))

	// Float error in 3*(1/3) must not push the third object to a new row.
	assert.Equal(t, float32(0), c.Objects[0].Position().Y)
	assert.Equal(t, float32(0), c.Objects[1].Position().Y)
	assert.Equal(t, float32(0), c.Objects[2].Position().Y)

	end := c.Objects[2].Position().X + c.Objects[2].Size().Width
	assert.InDelta(t, W, end, 0.01, "thirds fill the row exactly")
	_ = pad
}

func TestResponsiveLayout_HiddenSkipped(t *testing.T) {
	obj1 := NewMinSizeRect(geo.NewSize(10, 10))
	obj2 := NewMinSizeRect(geo.NewSize(10, 10))
	c := layout.NewResponsiveLayout(
		layout.Responsive(obj1, .5),
		layout.Responsive(obj2, .5),
	)
	c.Objects[0].Hide()
	c.Resize(geo.NewSize(400, 300))

	assert.Equal(t, geo.NewPos(0, 0), c.Objects[1].Position(),
		"hidden object leaves no gap")
}

func TestResponsiveLayout_RTLMirrors(t *testing.T) {
	pad := theme.Padding()
	obj1 := NewMinSizeRect(geo.NewSize(10, 10))
	obj2 := NewMinSizeRect(geo.NewSize(10, 10))
	c := layout.NewResponsiveLayout(
		layout.Responsive(obj1, .5),
		layout.Responsive(obj2, .25),
	)

	rtlLayout(t, func() {
		W := float32(400)
		c.Resize(geo.NewSize(W, 300))

		content := W - pad
		w1, w2 := c.Objects[0], c.Objects[1]
		// First object hugs the right edge, second sits to its left;
		// the leftover quarter of the row is at the left edge.
		assert.Equal(t, geo.NewPos(W-content*.5, 0), w1.Position())
		assert.Equal(t, content*.5, w1.Size().Width)
		assert.Equal(t, geo.NewPos(W-content*.5-pad-content*.25, 0), w2.Position())
	})
}

func TestResponsiveLayout_CustomBreakpoints(t *testing.T) {
	obj := NewMinSizeRect(geo.NewSize(10, 10))
	l := &layout.ResponsiveLayout{Breakpoints: []float32{576, 768, 992, 1200}} // Bootstrap
	c := &bellui.Container{Layout: l, Objects: []core.CanvasObject{layout.Responsive(obj, 1, .5, .25)}}

	c.Resize(geo.NewSize(590, 300)) // < 768: second class under Bootstrap, first under MD3
	assert.Equal(t, float32(295), c.Objects[0].Size().Width)
}

func TestResponsiveLayout_MinSize(t *testing.T) {
	pad := theme.Padding()
	obj1 := NewMinSizeRect(geo.NewSize(100, 40))
	obj2 := NewMinSizeRect(geo.NewSize(60, 30))
	c := layout.NewResponsiveLayout(
		layout.Responsive(obj1, .5),
		layout.Responsive(obj2, .5),
	)

	// Before layout: single-column estimate.
	assert.Equal(t, geo.NewSize(100, 70+pad), c.MinSize())

	// After layout: widest child min width, current arrangement height.
	c.Resize(geo.NewSize(400, 300))
	require.Equal(t, geo.NewSize(100, 40), c.MinSize())
}

// TestResponsive_ChildRenders locks in that a [Responsive]-wrapped child is
// actually painted. The original wrapper embedded bellui.Container in a custom
// type, which the exact-type switch in the object-tree walker does not match —
// wrapped children were laid out but never walked, so they did not paint, were
// not hit-testable and panicked the markup renderer (pre-rebrand tracker #767).
func TestResponsive_ChildRenders(t *testing.T) {
	test.NewTempApp(t)
	red := color.NRGBA{R: 0xff, A: 0xff}
	rect := canvas.NewRectangle(red)
	rect.SetMinSize(geo.NewSize(50, 50))

	img := software.Render(layout.NewResponsiveLayout(rect), test.Theme())

	found := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 == 0xff && g>>8 == 0 && bl>>8 == 0 {
				found++
			}
		}
	}
	assert.Positive(t, found, "the wrapped child must be painted")
}

// TestResponsive_MarkupRender locks in that a [Responsive] wrapper renders as a
// plain container in the test markup renderer instead of panicking (pre-rebrand tracker #767).
func TestResponsive_MarkupRender(t *testing.T) {
	test.NewTempApp(t)
	rect := canvas.NewRectangle(color.NRGBA{R: 0xff, A: 0xff})
	rect.SetMinSize(geo.NewSize(50, 50))

	test.AssertObjectRendersToMarkup(t, "responsive_markup.xml", layout.NewResponsiveLayout(rect))
}
