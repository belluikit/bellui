package component

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/style"
)

// The active drawing tool is drawn in the accent colour, the others in the
// button colour; Clear All is never active.
func TestRenderDrawingToolsHighlightsActive(t *testing.T) {
	test.NewTempApp(t)
	r := style.NewResolver(theme.DefaultTheme(), theme.VariantDark)
	cfg := &ToolboxConfig{DrawingTools: []interact.DrawingToolMode{interact.DrawingToolNone, interact.DrawingToolTextLabel}}
	layout := &ToolboxLayout{}
	objs := RenderDrawingTools(cfg, interact.DrawingToolTextLabel, 0, 0, 400, r, layout)

	if len(layout.Buttons) != 3 || len(objs) != 6 {
		t.Fatalf("%d buttons, %d objects; want Select, Text, Clear All as background + label each", len(layout.Buttons), len(objs))
	}
	for i, wantActive := range []bool{false, true, false} {
		bg := objs[2*i].(*canvas.Rectangle)
		want := r.ButtonBackgroundColor()
		if wantActive {
			want = r.AccentColor()
		}
		if bg.FillColor != want {
			t.Errorf("button %q drawn in %v, want %v", layout.Buttons[i].Action, bg.FillColor, want)
		}
	}
}

// The drawing tools wrap at the width they are given, and their height counts
// every row; a toolbox without drawing tools takes no height.
func TestDrawingToolsWrap(t *testing.T) {
	cfg := &ToolboxConfig{DrawingTools: interact.DefaultDrawingTools()}
	one := DrawingToolsHeight(cfg, 2000)
	if one != ToolboxButtonHeight {
		t.Errorf("one row is %v high, want %v", one, ToolboxButtonHeight)
	}
	narrow := DrawingToolsHeight(cfg, 150)
	if narrow <= one {
		t.Errorf("at 150pt the tools take %v, no more than one row (%v)", narrow, one)
	}
	_, buttons := drawingToolButtons(cfg, 10, 0, 150)
	for _, b := range buttons {
		if b.Rect.Pos.X > 10 && b.Rect.Pos.X+b.Rect.Size.Width > 160 {
			t.Errorf("button %q at %v..%v overruns the 150pt row", b.Action, b.Rect.Pos.X, b.Rect.Pos.X+b.Rect.Size.Width)
		}
	}
	if h := DrawingToolsHeight(&ToolboxConfig{SaveAsImage: true}, 400); h != 0 {
		t.Errorf("a toolbox without drawing tools takes %v", h)
	}
	if h := DrawingToolsHeight(nil, 400); h != 0 {
		t.Errorf("no toolbox takes %v", h)
	}
}
