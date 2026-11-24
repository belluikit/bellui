package widget

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// inkExtentX returns the leftmost and rightmost x of ink in rows [y0, y1) of
// img: pixels that differ from its top-left (background) pixel.
func inkExtentX(img image.Image, y0, y1 int) (minX, maxX int) {
	bgR, bgG, bgB, _ := img.At(0, 0).RGBA()
	minX, maxX = -1, -1
	b := img.Bounds()
	for y := y0; y < y1 && y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r == bgR && g == bgG && bl == bgB {
				continue
			}
			if minX < 0 || x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
	}
	return minX, maxX
}

// secondRowY returns the y of the first text object below rt's first row.
func secondRowY(t *testing.T, rt *RichText) float32 {
	t.Helper()
	y0 := float32(-1)
	for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
		tx, ok := o.(*canvas.Text)
		if !ok || !tx.Visible() {
			continue
		}
		if y0 < 0 {
			y0 = tx.Position().Y
		} else if tx.Position().Y > y0 {
			return tx.Position().Y
		}
	}
	return -1
}

// A row that wraps where one segment ends with a space draws its glyphs, and
// places its carets, where the same text in one segment does, in an LTR or
// an RTL window, for Hebrew and for Latin text, at every alignment (#1435,
// #1437). In an RTL window, Hebrew text is drawn with the break space at the
// row's left end, so the glyphs move left, not right, by it.
func TestRichText_BreakSpaceAlignedInEitherDirection(t *testing.T) {
	test.NewTempApp(t)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	texts := []struct {
		name  string
		parts []string
	}{
		{"Hebrew", []string{"שלום עולם זה טקסט ארוך מאוד ", "שעובר לשורה הבאה בסוף."}},
		{"Latin", []string{"Select a word and make it bold, italic ", "or code."}},
	}
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		bellui.SetLayoutDirection(dir)
		for _, tx := range texts {
			for _, align := range []bellui.TextAlign{bellui.TextAlignLeading, bellui.TextAlignTrailing, bellui.TextAlignCenter} {
				st := RichTextStyleInline
				st.Alignment = align
				mk := func(parts ...string) (*RichText, bellui.Window) {
					var segs []RichTextSegment
					for _, p := range parts {
						segs = append(segs, &TextSegment{Style: st, Text: p})
					}
					rt := NewRichText(segs...)
					rt.Wrapping = bellui.TextWrapWord
					rt.Selectable = true
					return rt, test.NewWindow(rt)
				}
				split, sw := mk(tx.parts...)
				whole, ww := mk(strings.Join(tx.parts, ""))
				where := func(width float32) string {
					return fmt.Sprintf("%s, window direction %v, align %v, width %v", tx.name, dir, align, width)
				}
				compared := 0
				for width := float32(150); width <= 360; width++ {
					sw.Resize(geo.NewSize(width, 200))
					ww.Resize(geo.NewSize(width, 200))
					sRows, wRows := richRowStrings(split), richRowStrings(whole)
					trimmed := strings.TrimRight(tx.parts[0], " ")
					if len(sRows) < 2 || sRows[0] != tx.parts[0] || wRows[0] != trimmed {
						continue // not a break at the segment boundary
					}
					compared++
					y1 := int(secondRowY(t, split))
					if hasGlyphs(tx.parts[0]) { // the ink is an oracle only when drawn with real glyphs
						sMin, sMax := inkExtentX(sw.Canvas().Capture(), 0, y1)
						wMin, wMax := inkExtentX(ww.Canvas().Capture(), 0, y1)
						assert.InDelta(t, wMin, sMin, 1, "%s: ink starts where the whole row's does", where(width))
						assert.InDelta(t, wMax, sMax, 1, "%s: ink ends where the whole row's does", where(width))
					}

					textSize := split.Theme().Size(theme.SizeNameText)
					pad := split.Theme().Size(theme.SizeNameInnerPadding)
					for col := 0; col < len([]rune(trimmed)); col++ {
						sx := split.lineSizeToColumn(col, 0, textSize, pad, true).Width + split.selection.alignOffsetX(0)
						wx := whole.lineSizeToColumn(col, 0, textSize, pad, true).Width + whole.selection.alignOffsetX(0)
						if !assert.InDelta(t, wx, sx, 0.05, "%s: caret x of column %d", where(width), col) {
							break
						}
					}
				}
				assert.Greater(t, compared, 10, "%s, %v: the boundary break must be measured at many widths", tx.name, dir)
				sw.Close()
				ww.Close()
			}
		}
	}
}
