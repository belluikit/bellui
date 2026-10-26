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

// The default angles leave the gap at the bottom, and values fill clockwise
// from the bottom left, as in ECharts. Angles an app sets are read the same
// way: Min at StartAngle, clockwise to Max at EndAngle (#343).
func TestGauge_FillsClockwiseFromTheBottomLeft(t *testing.T) {
	arcs := gaugeArcs(&GaugeSeries{Value: 25})
	track, value := arcs[0], arcs[1]
	if track.StartAngle != -135 || track.EndAngle != 135 {
		t.Errorf("default track runs %v to %v, want -135 to 135 (the gap at the bottom)", track.StartAngle, track.EndAngle)
	}
	// A quarter of 270 degrees from the bottom left: up the left side.
	if value.StartAngle != -135 || value.EndAngle != -67.5 {
		t.Errorf("value 25 of 100 runs %v to %v, want -135 to -67.5 (from the bottom left)", value.StartAngle, value.EndAngle)
	}

	explicit := gaugeArcs(&GaugeSeries{Value: 50, StartAngle: -90, EndAngle: 90})
	if explicit[0].StartAngle != -90 || explicit[0].EndAngle != 90 {
		t.Errorf("explicit -90/90 track runs %v to %v, want the top half, -90 to 90", explicit[0].StartAngle, explicit[0].EndAngle)
	}
	if explicit[1].EndAngle != 0 {
		t.Errorf("value 50 on a -90/90 gauge ends at %v, want 0 (the top)", explicit[1].EndAngle)
	}

	// An EndAngle given below StartAngle is reached clockwise all the same.
	wrapped := gaugeArcs(&GaugeSeries{Value: 10, StartAngle: 225, EndAngle: 135})[0]
	if wrapped.StartAngle != 225 || wrapped.EndAngle != 495 {
		t.Errorf("explicit 225/135 track runs %v to %v, want 225 to 495 (270 degrees clockwise)", wrapped.StartAngle, wrapped.EndAngle)
	}
}

// Coloured ranges sit on the same clockwise sweep as the value.
func TestGauge_RangesFollowTheSweep(t *testing.T) {
	sectors := []GaugeSector{{Min: 0, Max: 50, Color: color.NRGBA{G: 180, A: 255}}}
	low := gaugeArcs(&GaugeSeries{Value: 10, Sectors: sectors})[1]
	if low.StartAngle != -135 || low.EndAngle != 0 {
		t.Errorf("range 0-50 runs %v to %v, want -135 to 0 (bottom left to the top)", low.StartAngle, low.EndAngle)
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
