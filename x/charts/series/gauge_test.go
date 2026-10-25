package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/x/charts/coord"
)

func gaugeArcs(gs *GaugeSeries) []*canvas.Arc {
	ctx := RenderContext{Series: gs, Viewport: coord.Rect{W: 300, H: 300}, Color: color.White}
	var arcs []*canvas.Arc
	for _, o := range NewRenderer(TypeGauge).Render(ctx) {
		if a, ok := o.(*canvas.Arc); ok {
			arcs = append(arcs, a)
		}
	}
	return arcs
}

// The default angles leave the gap at the bottom; 225/-45 left it on the left.
// Angles an app sets keep their meaning (#343).
func TestGauge_DefaultGapAtTheBottom(t *testing.T) {
	track := gaugeArcs(&GaugeSeries{Value: 10})[0]
	if track.StartAngle != -135 || track.EndAngle != 135 {
		t.Errorf("default track runs %v to %v, want -135 to 135 (the gap at the bottom)", track.StartAngle, track.EndAngle)
	}
	explicit := gaugeArcs(&GaugeSeries{Value: 10, StartAngle: 225, EndAngle: -45})[0]
	if explicit.StartAngle != -45 || explicit.EndAngle != 225 {
		t.Errorf("explicit 225/-45 track runs %v to %v, want -45 to 225 as before", explicit.StartAngle, explicit.EndAngle)
	}
}

// With coloured ranges, the value is a ring inside them, so the ranges below
// the value stay visible; the value arc used to paint over them (#343).
func TestGauge_ValueDoesNotHideTheRanges(t *testing.T) {
	sectors := []GaugeSector{
		{Min: 0, Max: 50, Color: color.NRGBA{G: 180, A: 255}},
		{Min: 50, Max: 100, Color: color.NRGBA{R: 220, A: 255}},
	}
	arcs := gaugeArcs(&GaugeSeries{Value: 65, Sectors: sectors})
	if len(arcs) != 4 { // track, two ranges, value
		t.Fatalf("%d arcs, want 4", len(arcs))
	}
	ranges, value := arcs[1], arcs[3]
	rangeInner := ranges.Size().Width / 2 * ranges.CutoutRatio
	valueOuter := value.Size().Width / 2
	if valueOuter >= rangeInner {
		t.Errorf("the value ring reaches radius %.1f, over the ranges from %.1f", valueOuter, rangeInner)
	}

	plain := gaugeArcs(&GaugeSeries{Value: 65})
	if got, want := plain[1].Size(), plain[0].Size(); got != want {
		t.Errorf("without ranges the value arc is %v, want the track's %v as before", got, want)
	}
}
