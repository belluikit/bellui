package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// wideIndentTheme draws a blockquote further in (line spacing 6, so an indent
// of 24) than twice the inner padding (16), the width wrapping used to narrow
// a blockquote's first row by.
type wideIndentTheme struct{ bellui.Theme }

func (th wideIndentTheme) Size(n bellui.ThemeSizeName) float32 {
	if n == theme.SizeNameLineSpacing {
		return 6
	}
	return th.Theme.Size(n)
}

// The windowed block build (shapeBlock, for long documents) wraps every row of
// a blockquote at the indented width too (#1437).
func TestRichText_ShapeBlockBlockquoteRowsFitTheirIndentedWidth(t *testing.T) {
	const text = "A blockquote holds a long quotation that wraps over several rows, so every row after the first is measured at some width and drawn at another, and the last word of a full row shows where it ends."
	for _, th := range []bellui.Theme{theme.DefaultTheme(), wideIndentTheme{theme.DefaultTheme()}} {
		app := test.NewTempApp(t)
		app.Settings().SetTheme(th)
		rt := NewRichText()
		rt.Wrapping = bellui.TextWrapWord
		seg := &TextSegment{Style: RichTextStyleBlockquote, Text: text}
		indent := blockquoteIndent(rt.Theme())
		innerPad := rt.Theme().Size(theme.SizeNameInnerPadding)
		runes := []rune(text)
		for width := float32(100); width <= 500; width++ {
			bounds := rt.shapeBlock([]RichTextSegment{seg}, width, innerPad)
			require.Greater(t, len(bounds), 1)
			for _, b := range bounds {
				row := strings.TrimRight(string(runes[b.begin:b.end]), " ")
				m := bellui.MeasureText(row, seg.size(), seg.Style.TextStyle).Width
				require.LessOrEqual(t, m, width-indent+0.01, "width %v, indent %v: row %q", width, indent, row)
			}
		}
	}
}

// Every row of a blockquote fits the width it is drawn in: wrapping narrows
// every row, not only the first, by the indent the layout draws it at
// (#1437: later rows were wrapped at the full width and ran up to the indent
// past the right edge).
func TestRichText_BlockquoteRowsFitTheirIndentedWidth(t *testing.T) {
	const text = "A blockquote holds a long quotation that wraps over several rows, so every row after the first is measured at some width and drawn at another, and the last word of a full row shows where it ends."
	for _, th := range []bellui.Theme{theme.DefaultTheme(), wideIndentTheme{theme.DefaultTheme()}} {
		app := test.NewTempApp(t)
		app.Settings().SetTheme(th)
		for _, style := range []RichTextStyle{RichTextStyleBlockquote, RichTextStyleParagraph} {
			rt := NewRichText(&TextSegment{Style: style, Text: text})
			rt.Wrapping = bellui.TextWrapWord
			w := test.NewWindow(rt)
			pad := rt.Theme().Size(theme.SizeNameInnerPadding)
			for width := float32(120); width <= 500; width++ {
				w.Resize(geo.NewSize(width+20, 800))
				rt.Resize(geo.NewSize(width, 700))
				rows := 0
				for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
					tx, ok := o.(*canvas.Text)
					if !ok || !tx.Visible() || strings.TrimSpace(tx.Text) == "" {
						continue
					}
					rows++
					end := tx.Position().X + bellui.MeasureText(strings.TrimRight(tx.Text, " "), tx.TextSize, tx.TextStyle).Width
					require.LessOrEqual(t, end, width-pad+0.5, "blockquote %v, width %v: row %q ends past the right edge", style.isBlockquote(), width, tx.Text)
				}
				require.Greater(t, rows, 1, "the text must wrap")
			}
			w.Close()
		}
	}
}
