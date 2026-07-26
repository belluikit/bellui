package editor_test

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/editor"
	"github.com/belluikit/bellui/x/editor/highlight/starlark"
)

// Golden image tests render the editor with fyne's software painter and compare
// the result against committed master PNGs in testdata/. They catch rendering
// regressions the controller-level tests can't see — gutter marker shapes,
// diagnostic squiggles, decoration placement/clipping, redaction bars.
//
// Software-painter output is platform-specific (font rasterization differs by
// OS/arch), so these run on ONE canonical platform: set FYCODER_GOLDEN=1 there
// (CI sets it on linux/amd64). They skip everywhere else, and skip (rather than
// fail) when a master is missing so the harness can land before masters are
// seeded. Regenerate masters on the canonical platform with:
//
//	FYCODER_GOLDEN=1 UPDATE_TESTDATA=1 go test -run Golden .
//
// NOTE: this exercises the software painter, not the GL/WebGPU backend — it
// validates fycoder's own object placement and clipping, not backend-specific
// glyph/surface rendering, which still needs real-backend verification.
func assertGolden(t *testing.T, master string, c core.Canvas) {
	t.Helper()
	if os.Getenv("FYCODER_GOLDEN") == "" {
		t.Skip("golden tests run only where FYCODER_GOLDEN is set (canonical platform)")
	}
	if os.Getenv("UPDATE_TESTDATA") == "" {
		if _, err := os.Stat(filepath.Join("testdata", master)); err != nil {
			t.Skipf("master testdata/%s missing — regenerate with FYCODER_GOLDEN=1 UPDATE_TESTDATA=1 go test -run Golden .", master)
		}
	}
	test.AssertRendersToImage(t, master, c)
}

const goldenSample = `def evaluate(ctx):
    api_key = "sk-live-7Qd9f2KpZ1xR8vN3"
    score = ctx.score
    if score == None:
        return {"action": "hold"}
    return None
`

// renderGolden lays the editor out in a window at a fixed size so geometry is
// deterministic, and returns its canvas for capture.
func renderGolden(t *testing.T, e *editor.Editor, w, h float32) core.Canvas {
	t.Helper()
	win := test.NewWindow(e)
	t.Cleanup(win.Close)
	win.Resize(geo.NewSize(w, h))
	return win.Canvas()
}

// TestGolden_Editor renders the full editor surface: Starlark highlighting, a
// composite gutter (line numbers + diagnostic markers + breakpoint), diagnostic
// squiggles, and the current-line decorator.
func TestGolden_Editor(t *testing.T) {
	test.NewTempApp(t)
	e := editor.NewEditor()
	e.Highlighter = starlark.New()
	e.Gutter = editor.NewCompositeGutter(
		editor.NewLineNumberGutter(),
		e.DiagnosticGutter(),
		editor.NewBreakpointGutter(nil),
	)
	cur := editor.NewCurrentLineDecorator(1)
	e.Decorators = []editor.RowDecorator{cur}
	e.SetText(goldenSample)
	// One diagnostic per severity so the distinct gutter marker shapes render.
	e.SetDiagnostics([]editor.Diagnostic{
		{Range: editor.TextRange{StartRow: 3, StartCol: 13, EndRow: 3, EndCol: 17}, Severity: editor.SeverityWarning, Message: "use 'is None'"},
		{Range: editor.TextRange{StartRow: 1, StartCol: 4, EndRow: 1, EndCol: 11}, Severity: editor.SeverityInfo, Message: "document this"},
	})
	assertGolden(t, "editor.png", renderGolden(t, e, 460, 170))
}

// TestGolden_Redaction renders a masked secret — the opaque redaction bar over
// the API key value.
func TestGolden_Redaction(t *testing.T) {
	test.NewTempApp(t)
	e := editor.NewEditor()
	e.Highlighter = starlark.New()
	e.Gutter = editor.NewLineNumberGutter()
	e.SetText(goldenSample)
	// The API-key value: 4-space indent + `api_key = "` = 15 chars, value is 24.
	e.SetRedactions([]editor.TextRange{{StartRow: 1, StartCol: 15, EndRow: 1, EndCol: 39}})
	assertGolden(t, "redaction.png", renderGolden(t, e, 460, 170))
}

// fillRowDecorator paints every row, so the bottom partially-visible row always
// produces an overhanging rect — the condition that bleeds past the editor
// without the decoratorOverlay clip (#341 root cause).
type fillRowDecorator struct{}

func (fillRowDecorator) Decorate(_ int, pos geo.Position, size geo.Size) []core.CanvasObject {
	r := canvas.NewRectangle(color.NRGBA{R: 70, G: 150, B: 90, A: 160})
	r.Move(pos)
	r.Resize(size)
	return []core.CanvasObject{r}
}

// TestGolden_DecorationClip guards that row decorations are clipped to the editor
// and don't bleed over a neighbouring widget. The editor is hosted above a
// contrasting status bar and captured at the WHOLE-WINDOW size (not the editor
// subtree), so an unclipped overhang from the bottom edge row would show as green
// over the red bar. With decoratorOverlayRenderer.IsClip() it stays clean.
func TestGolden_DecorationClip(t *testing.T) {
	test.NewTempApp(t)
	e := editor.NewEditor()
	e.SetText(strings.Repeat("line of content here\n", 40))
	e.Decorators = []editor.RowDecorator{fillRowDecorator{}}

	status := canvas.NewRectangle(color.NRGBA{R: 220, G: 70, B: 70, A: 255})
	status.SetMinSize(geo.NewSize(0, 30))
	root := container.NewBorder(nil, status, nil, nil, e)

	win := test.NewWindow(root)
	t.Cleanup(win.Close)
	win.Resize(geo.NewSize(240, 207)) // odd height → bottom row partially cut
	assertGolden(t, "decoration_clip.png", win.Canvas())
}
