package editor

import (
	"image/color"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// The app's own content in the gutter and the overlay, in colours nothing of
// the editor's uses, which a disabled editor must leave as they are.
var (
	appGutterColor = color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	appRowColor    = color.NRGBA{R: 0x21, G: 0x43, B: 0x65, A: 0x80}
	appSpanColor   = color.NRGBA{R: 0x31, G: 0x53, B: 0x75, A: 0xff}
)

// appGutter is a GutterProvider the app supplies: it writes "app" on line 1.
type appGutter struct{}

func (appGutter) Width(int, float32) float32 { return 30 }

func (appGutter) Render(line int, size geo.Size, textSize float32) []core.CanvasObject {
	if line != 1 {
		return nil
	}
	return []core.CanvasObject{canvas.NewText("app", appGutterColor)}
}

// drawnObjects walks w's cached renderers and returns the visible canvas
// objects they draw, widgets and containers left out.
func drawnObjects(w core.Widget) []core.CanvasObject {
	var out []core.CanvasObject
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if !o.Visible() {
			return
		}
		switch x := o.(type) {
		case *bellui.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case core.Widget:
			for _, c := range test.WidgetRenderer(x).Objects() {
				walk(c)
			}
		default:
			out = append(out, o)
		}
	}
	walk(w)
	return out
}

func nrgba(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// A disabled Editor draws what it draws itself besides its text in the
// disabled look: its own gutter providers' line numbers, breakpoint dots,
// diagnostic markers and fold chevrons, and its own diagnostics squiggles,
// find-match washes, redaction bars and fold placeholders. The app's gutter
// provider, row decorator and span decorator keep their colours. Enable
// restores everything (#1232).
func TestEditor_DisableDimsGutterAndDecorations(t *testing.T) {
	test.NewTempApp(t)

	ed := NewEditor()
	ed.MinRows = 10
	ed.SetText(strings.Repeat("hello world\n", 9) + "last")
	bp := NewBreakpointGutter(nil)
	bp.Lines = map[int]bool{1: true}
	ed.Gutter = NewCompositeGutter(NewLineNumberGutter(), bp, ed.DiagnosticGutter(), ed.FoldGutter(), appGutter{})
	ed.Decorators = []RowDecorator{&CurrentLineDecorator{Row: 1, Color: appRowColor}}
	ed.SpanDecorators = []SpanDecorator{&StaticSpanDecorator{SpansList: []DecoratedSpan{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}, Kind: SpanBox, Color: appSpanColor},
	}}}
	w := test.NewTempWindow(t, ed)
	w.Resize(geo.NewSize(500, 400))

	ed.SetDiagnostics([]Diagnostic{{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}, Severity: SeverityError}})
	ed.searchSpans.Matches = []TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 5}}
	ed.SetRedactions([]TextRange{{StartRow: 2, StartCol: 6, EndRow: 2, EndCol: 11}})
	ed.SetRedactionMode(Masked)
	ed.SetFoldRegions([]FoldRegion{{Range: TextRange{StartRow: 4, EndRow: 6}, Collapsed: true}})
	ed.RefreshGutter()
	ed.RefreshDecorators()

	disabledText := theme.Color(theme.ColorNameDisabled)
	lineNumbers := themeColor(ColorNameGutterText)
	errorColor := nrgba(themeColor(ColorNameDiagnosticError))
	chevronColor := nrgba(themeColor(ColorNameFoldMarker))
	placeholder := themeColor(ColorNameFoldPlaceholder)
	redaction := nrgba(toOpaque(themeColor(ColorNameRedaction)))
	// The editor's find-match decorator leaves Active at -1 until the find bar
	// navigates, so the match is drawn in the default non-active wash.
	ed.searchSpans.Active = -1
	wash := color.NRGBA{R: 255, G: 180, B: 50, A: 70}

	assertLook := func(t *testing.T, disabled bool) {
		t.Helper()
		// look is what the editor's own mark of colour c is drawn in.
		look := func(c color.NRGBA) color.NRGBA {
			if disabled {
				return nrgba(dimColor(c))
			}
			return c
		}
		wantText := lineNumbers
		if disabled {
			wantText = disabledText
		}

		// The gutter.
		var numbers, app, dots, chevrons int
		for _, o := range drawnObjects(ed.gutterW) {
			switch o := o.(type) {
			case *canvas.Text:
				if o.Text == "app" {
					app++
					assert.Equal(t, appGutterColor, o.Color, "the app's gutter text")
					continue
				}
				_, err := strconv.Atoi(o.Text)
				require.NoError(t, err, "gutter text %q", o.Text)
				numbers++
				assert.Equal(t, wantText, o.Color, "line number %s", o.Text)
			case *canvas.Circle:
				dots++
				want := look(color.NRGBA{R: 220, G: 50, B: 50, A: 255}) // the breakpoint
				if c := nrgba(o.FillColor); c.R == errorColor.R && c.G == errorColor.G && c.B == errorColor.B {
					want = look(errorColor) // the diagnostic marker
				}
				assert.Equal(t, want, nrgba(o.FillColor), "gutter dot")
			case *canvas.Line:
				chevrons++
				assert.Equal(t, look(chevronColor), nrgba(o.StrokeColor), "fold chevron")
			}
		}
		require.Equal(t, 1, app, "the app's gutter provider drew nothing, so the check would prove nothing")
		require.NotZero(t, numbers, "the gutter drew no line numbers")
		require.Equal(t, 2, dots, "the gutter did not draw the breakpoint and the diagnostic marker")
		require.NotZero(t, chevrons, "the gutter drew no fold chevron")

		// The overlay.
		var appRow, appBox, squiggles, washes, bars, placeholders int
		for _, o := range drawnObjects(ed.decOverlay) {
			switch o := o.(type) {
			case *canvas.Text:
				placeholders++
				want := placeholder
				if disabled {
					want = disabledText
				}
				assert.Equal(t, want, o.Color, "fold placeholder %q", o.Text)
			case *canvas.Line:
				squiggles++
				assert.Equal(t, look(errorColor), nrgba(o.StrokeColor), "diagnostic squiggle")
			case *canvas.Rectangle:
				fill := nrgba(o.FillColor)
				switch {
				case o.StrokeColor != nil && nrgba(o.StrokeColor) == appSpanColor:
					appBox++
				case fill == appRowColor:
					appRow++
				case fill.A == 255:
					bars++
					want := redaction
					if disabled {
						want = nrgba(dimOpaque(redaction))
						assert.NotEqual(t, redaction, want, "the dimmed bar is the bar's own colour")
					}
					assert.Equal(t, want, fill, "redaction bar")
				default:
					washes++
					assert.Equal(t, look(wash), fill, "find-match wash")
				}
			}
		}
		require.Equal(t, 1, appRow, "the app's row decoration is missing or recoloured")
		require.Equal(t, 1, appBox, "the app's span decoration is missing or recoloured")
		require.NotZero(t, squiggles, "the overlay drew no diagnostic squiggle")
		require.Equal(t, 1, washes, "the overlay did not draw the find match")
		require.Equal(t, 1, bars, "the overlay did not draw the redaction bar")
		require.Equal(t, 1, placeholders, "the overlay did not draw the fold placeholder")
	}

	assertLook(t, false)
	ed.Disable()
	assertLook(t, true)
	ed.Enable()
	assertLook(t, false)

	// A gutter and an overlay rebuilt while disabled come up dimmed.
	ed.Disable()
	ed.ApplyGutter()
	ed.ApplyDecorators()
	ed.RefreshGutter()
	ed.RefreshDecorators()
	assertLook(t, true)
	ed.Enable()
	assertLook(t, false)
}
