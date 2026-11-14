package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
)

// fitRect is a min-sized rectangle that asks for a wider FitWidth, as wrapping text
// does.
type fitRect struct {
	*canvas.Rectangle
	fit float32
}

func (f fitRect) FitWidth() float32 { return f.fit }

func newFitRect(minW, fit float32) fitRect {
	return fitRect{NewMinSizeRect(geo.NewSize(minW, 10)), fit}
}

// A box gives an unstretched FitWidther its FitWidth while the row has room,
// takes width back toward its MinSize as the row overflows, and stops at the
// MinSize (#1195).
func TestBoxes_FitWidth(t *testing.T) {
	horizontal := map[string]func() core.Layout{
		"FlexHBox": func() core.Layout { return &layout.FlexHBoxLayout{Padding: 4, ExactPadding: true} },
		"HBox":     func() core.Layout { return layout.NewCustomPaddedHBoxLayout(4) },
	}
	for name, newLayout := range horizontal {
		t.Run(name, func(t *testing.T) {
			for _, c := range []struct {
				name        string
				width       float32
				textW, btnX float32
			}{
				{"room", 300, 200, 204},
				{"exactly room", 240, 200, 204},
				// 40 too narrow: the text gives back 40 of its 190.
				{"overflow", 200, 160, 164},
				{"no room", 30, 10, 14},
			} {
				text := newFitRect(10, 200)
				btn := NewMinSizeRect(geo.NewSize(36, 10))
				newLayout().Layout([]core.CanvasObject{text, btn}, geo.NewSize(c.width, 20))
				assert.InDelta(t, c.textW, text.Size().Width, 0.01, c.name)
				assert.InDelta(t, c.btnX, btn.Position().X, 0.01, c.name)
			}
		})
	}
}

// Two FitWidthers in an overflowing row give width back in proportion to what
// each asked for beyond its MinSize.
func TestBoxes_FitWidthSharesTheShortfall(t *testing.T) {
	a := newFitRect(10, 110) // asks 100 more
	b := newFitRect(10, 60)  // asks 50 more
	(&layout.FlexHBoxLayout{ExactPadding: true}).Layout([]core.CanvasObject{a, b}, geo.NewSize(140, 20))
	// 170 asked, 30 over: each keeps 4/5 of its ask.
	assert.InDelta(t, 90, a.Size().Width, 0.01)
	assert.InDelta(t, 50, b.Size().Width, 0.01)
}

// A FitWidth does not take a flex child's or a spacer's place before its own:
// the flex child and the spacer get what the text leaves, and MaxWidth caps
// the text's ask.
func TestBoxes_FitWidthBesideFlexAndSpacer(t *testing.T) {
	text := newFitRect(10, 200)
	grow := NewMinSizeRect(geo.NewSize(0, 10))
	(&layout.FlexHBoxLayout{Flex: []float32{0, 1}, ExactPadding: true}).Layout([]core.CanvasObject{text, grow}, geo.NewSize(300, 20))
	assert.InDelta(t, 200, text.Size().Width, 0.01)
	assert.InDelta(t, 100, grow.Size().Width, 0.01)

	text = newFitRect(10, 200)
	grow = NewMinSizeRect(geo.NewSize(0, 10))
	(&layout.FlexHBoxLayout{Flex: []float32{0, 1}, MaxWidth: []float32{120}, ExactPadding: true}).Layout([]core.CanvasObject{text, grow}, geo.NewSize(300, 20))
	assert.InDelta(t, 120, text.Size().Width, 0.01, "MaxWidth caps the FitWidth")
	assert.InDelta(t, 180, grow.Size().Width, 0.01, "the flex child gets what the cap leaves")

	text = newFitRect(10, 200)
	btn := NewMinSizeRect(geo.NewSize(36, 10))
	layout.NewCustomPaddedHBoxLayout(0).Layout([]core.CanvasObject{text, layout.NewSpacer(), btn}, geo.NewSize(300, 20))
	assert.InDelta(t, 200, text.Size().Width, 0.01)
	assert.InDelta(t, 264, btn.Position().X, 0.01, "the spacer takes the rest")
}

// A stretched child ignores its FitWidth, and MinSize still reports the
// narrowest the row can be.
func TestBoxes_FitWidthLeavesStretchAndMinSize(t *testing.T) {
	text := newFitRect(10, 200)
	btn := NewMinSizeRect(geo.NewSize(36, 10))
	l := &layout.FlexHBoxLayout{Flex: []float32{1, 0}, ExactPadding: true}
	l.Layout([]core.CanvasObject{text, btn}, geo.NewSize(400, 20))
	assert.InDelta(t, 364, text.Size().Width, 0.01)
	assert.Equal(t, float32(46), l.MinSize([]core.CanvasObject{text, btn}).Width)
	assert.Equal(t, float32(46), layout.NewCustomPaddedHBoxLayout(0).MinSize([]core.CanvasObject{text, btn}).Width)
}

// Center and a column that does not stretch its children give a FitWidther its
// FitWidth up to their own width, where it used to get its MinSize (#1198).
func TestCenterAndColumns_FitWidth(t *testing.T) {
	type place struct{ x, w float32 }
	layouts := []struct {
		name      string
		newLayout func() core.Layout
		room      place // in 300 wide
		tooNarrow place // in 120 wide
	}{
		{"Center", layout.NewCenterLayout, place{50, 200}, place{0, 120}},
		{"CrossStart", func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossStart} }, place{0, 200}, place{0, 120}},
		{"CrossCenter", func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossCenter} }, place{50, 200}, place{0, 120}},
		{"CrossEnd", func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossEnd} }, place{100, 200}, place{0, 120}},
		{"CrossStretch", func() core.Layout { return &layout.FlexVBoxLayout{} }, place{0, 300}, place{0, 120}},
	}
	for _, l := range layouts {
		for width, want := range map[float32]place{300: l.room, 120: l.tooNarrow} {
			text := newFitRect(10, 200)
			l.newLayout().Layout([]core.CanvasObject{text}, geo.NewSize(width, 100))
			assert.InDelta(t, want.w, text.Size().Width, 0.01, "%s in %v: width", l.name, width)
			assert.InDelta(t, want.x, text.Position().X, 0.01, "%s in %v: x", l.name, width)
		}

		// Before it has a size, and when it is narrower than the MinSize, the
		// container gives the MinSize as before.
		for _, width := range []float32{0, 5} {
			text := newFitRect(10, 200)
			l.newLayout().Layout([]core.CanvasObject{text}, geo.NewSize(width, 100))
			if l.name != "CrossStretch" {
				assert.InDelta(t, 10, text.Size().Width, 0.01, "%s in %v", l.name, width)
			}
		}
	}

	// A column's MaxWidth still caps the child.
	text := newFitRect(10, 200)
	(&layout.FlexVBoxLayout{CrossAlign: layout.CrossStart, MaxWidth: []float32{80}}).Layout([]core.CanvasObject{text}, geo.NewSize(300, 100))
	assert.InDelta(t, 80, text.Size().Width, 0.01)

	// MinSize still reports the narrowest the container can be.
	assert.Equal(t, float32(10), layout.NewCenterLayout().MinSize([]core.CanvasObject{text}).Width)
	assert.Equal(t, float32(10), (&layout.FlexVBoxLayout{CrossAlign: layout.CrossStart}).MinSize([]core.CanvasObject{text}).Width)
}
