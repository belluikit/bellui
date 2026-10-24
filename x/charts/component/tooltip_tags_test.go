package component

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/interact"
)

func tagText(objs []core.CanvasObject, s string) *canvas.Text {
	for _, o := range objs {
		if txt, ok := o.(*canvas.Text); ok && txt.Text == s {
			return txt
		}
	}
	return nil
}

// The axis value tags sit beside the axes as PlotArea says they are drawn:
// the zero side fields keep the old layout (X below, Y left), XAxisTop and
// YAxisRight move them above and to the right. Both the tooltip and the
// crosshair-only paths draw them.
func TestRenderTooltip_AxisTagsFollowAxisSides(t *testing.T) {
	resolver := testResolver()
	state := &TooltipState{Visible: true, Hits: []interact.HitResult{{
		SeriesName: "s", Value: interact.DataValue{X: 3, Y: 42}, PixelPos: geo.NewPos(200, 150),
	}}}
	base := PlotArea{X: 50, Y: 40, W: 300, H: 200}
	top, right := base.Y, base.X+base.W
	bottom, left := base.Y+base.H, base.X

	for _, crosshairOnly := range []bool{false, true} {
		cfg := &TooltipConfig{Crosshair: CrosshairBoth, ShowAxisTags: true, CrosshairOnly: crosshairOnly}
		render := func(pa PlotArea) (x, y *canvas.Text) {
			objs := RenderTooltip(state, cfg, resolver, nil, geo.NewSize(400, 300), pa)
			x, y = tagText(objs, "3.0"), tagText(objs, "42.00")
			if x == nil || y == nil {
				t.Fatalf("crosshairOnly=%v: X tag %v, Y tag %v, want both drawn", crosshairOnly, x, y)
			}
			return x, y
		}

		x, y := render(base)
		if x.Position().Y < bottom {
			t.Errorf("crosshairOnly=%v, default: X tag at y=%v, want below the plot (bottom %v)", crosshairOnly, x.Position().Y, bottom)
		}
		if y.Position().X+y.Size().Width > left {
			t.Errorf("crosshairOnly=%v, default: Y tag ends at x=%v, want left of the plot (left %v)", crosshairOnly, y.Position().X+y.Size().Width, left)
		}

		placed := base
		placed.XAxisTop, placed.YAxisRight = true, true
		x, y = render(placed)
		if x.Position().Y+x.Size().Height > top {
			t.Errorf("crosshairOnly=%v, XAxisTop: X tag ends at y=%v, want above the plot (top %v)", crosshairOnly, x.Position().Y+x.Size().Height, top)
		}
		if y.Position().X < right {
			t.Errorf("crosshairOnly=%v, YAxisRight: Y tag at x=%v, want right of the plot (right %v)", crosshairOnly, y.Position().X, right)
		}
	}
}
