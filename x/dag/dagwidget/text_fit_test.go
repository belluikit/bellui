package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/dag/dag"
)

// A note's text must fit its box at every zoom. Text does not scale exactly
// with its size (8.04 pt measures like 9 pt), so a box sized once at the base
// size and scaled by the zoom let it overflow by ~28 px at zoom 0.67 — the
// gallery's default view.
func TestAnnotationTextFitsAtEveryZoom(t *testing.T) {
	const text = "Join waits for all three inputs. A late CRM file holds the whole run."
	c := newTestCanvas(t)
	if err := c.AddAnnotation(&dag.Annotation{ID: "n", Text: text, Size: [2]float32{300, 50}}); err != nil {
		t.Fatal(err)
	}
	aw := c.annotations["n"]
	for _, z := range []float32{0.5, 0.6, 0.67, 0.75, 0.9, 1, 1.1, 1.33, 1.5, 2} {
		c.Zoom = z
		box := aw.effectiveSize()
		area := (box.Width - 2*annotationPadding) * z // screen space, as the renderer lays it out
		drawn := bellui.MeasureText(text, annotationTextSize*z, bellui.TextStyle{}).Width
		if drawn > area+0.5 {
			t.Errorf("zoom %.2f: text is %.1f px wide but the box leaves %.1f px", z, drawn, area)
		}
	}
}

// A palette entry is as tall as its two lines. The port summary's arrow comes
// from a fallback font whose line is ~22 px at 11 pt, so the old fixed 14 px
// box clipped it.
func TestPaletteItemFitsItsText(t *testing.T) {
	c := newTestCanvas(t)
	comp := &dag.Component{ID: "t", Name: "Transform",
		Inputs:  []dag.PortDef{{Name: "in", Direction: dag.PortInput, Stream: dag.StreamType{Name: "rows"}}},
		Outputs: []dag.PortDef{{Name: "out", Direction: dag.PortOutput, Stream: dag.StreamType{Name: "rows"}}},
	}
	item := newPaletteItem(comp, &Palette{DAGCanvas: c})
	r := item.CreateRenderer().(*paletteItemRenderer)
	size := r.MinSize()
	r.Layout(size)
	for _, obj := range []core.CanvasObject{r.name, r.desc} {
		need := obj.MinSize()
		if obj.Size().Height < need.Height {
			t.Errorf("%T laid out %.1f px high, needs %.1f", obj, obj.Size().Height, need.Height)
		}
		if bottom := obj.Position().Y + obj.Size().Height; bottom > size.Height {
			t.Errorf("%T ends at %.1f, below the entry's %.1f", obj, bottom, size.Height)
		}
	}
}
