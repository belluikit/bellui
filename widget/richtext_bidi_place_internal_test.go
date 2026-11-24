package widget

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// inkColumns reports, for each column of img, whether rows [y0, y1) hold
// clear ink there: a pixel at least a sixth of full intensity from the
// background, so the faint edge of a glyph is left out.
func inkColumns(img image.Image, y0, y1 int) []bool {
	bgR, _, _, _ := img.At(0, 0).RGBA()
	b := img.Bounds()
	cols := make([]bool, b.Dx())
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := y0; y < y1 && y < b.Max.Y; y++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if d := int(r>>8) - int(bgR>>8); d > 42 || d < -42 {
				cols[x-b.Min.X] = true
				break
			}
		}
	}
	return cols
}

// assertInkAligned checks that got and want hold ink in the same columns, give
// or take a pixel: each segment is placed at a fractional x of its own, which
// the painter rounds apart from the one-segment text's. A word drawn elsewhere,
// or a space lost or added, leaves columns of ink with none beside them in the
// other.
func assertInkAligned(t *testing.T, want, got []bool, msg string) bool {
	t.Helper()
	near := func(cols []bool, x int) bool {
		for d := -1; d <= 1; d++ {
			if x+d >= 0 && x+d < len(cols) && cols[x+d] {
				return true
			}
		}
		return false
	}
	any := false
	var stray []int
	for x := range want {
		any = any || want[x]
		if got[x] && !near(want, x) || want[x] && !near(got, x) {
			stray = append(stray, x)
		}
	}
	return assert.True(t, any, "%s: the oracle draws nothing", msg) &&
		assert.Empty(t, stray, "%s: columns with ink in only one of the two", msg)
}

// hasGlyphs reports whether this build has a font that draws every letter of
// s. A wasm build has no system fonts, so no Hebrew. There a missing letter is
// drawn as U+FFFD, which is wider than the letter's advance: inside one text
// object it spills over its neighbours, and at the edge of a split segment's
// object it is clipped. So no ink comparison between the two can hold there,
// while where each object is placed still can.
func hasGlyphs(s string) bool {
	fonts := paint.CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		f := fonts.ResolveFace(r)
		if f == nil {
			return false
		}
		if _, ok := f.NominalGlyph(r); !ok {
			return false
		}
	}
	return true
}

// skipWithoutGlyphs skips a test whose oracle is drawn ink when this build
// has no font for s (hasGlyphs).
func skipWithoutGlyphs(t *testing.T, s string) {
	t.Helper()
	if !hasGlyphs(s) {
		t.Skipf("no font draws %q on this build, so the one-segment ink is no oracle (hasGlyphs)", s)
	}
}

// A row of several segments holding right-to-left text is drawn in the row's
// bidi visual order, as the same text in one segment is, in either window
// direction and at every alignment (#1441). Drawn side by side, the segments
// read in logical order from the left: "שלום עולם " + "זה טקסט" drew its second
// half on the right, where a reader starts, and the carets, which already
// followed the visual order, sat on the other half's glyphs. Ink extents alone
// hid it: the swapped halves cover nearly the same span.
//
// The texts are right-to-left only. With a Latin word in the row, the
// one-segment text is no oracle: a segment measured on its own is a fraction
// of a pixel wider or narrower than inside the longer run (0.5–0.9 px for
// "Bellui" after "שלום "), with or without this placement, and in a
// right-to-left base both the painter and the caret layout order the runs
// logically (#341).
func TestRichText_RTLSegmentsDrawnInVisualOrder(t *testing.T) {
	test.NewTempApp(t)
	skipWithoutGlyphs(t, "שלום")
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	texts := []struct {
		name  string
		parts []string
	}{
		{"two segments", []string{"שלום עולם ", "זה טקסט"}},
		{"three segments", []string{"שלום ", "עולם זה", " טקסט"}},
		{"split inside a word", []string{"שלום עו", "לם זה טקסט"}},
	}
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		bellui.SetLayoutDirection(dir)
		for _, tx := range texts {
			for _, align := range []bellui.TextAlign{bellui.TextAlignLeading, bellui.TextAlignTrailing, bellui.TextAlignCenter} {
				where := fmt.Sprintf("%s, window direction %v, align %v", tx.name, dir, align)
				st := RichTextStyleInline
				st.Alignment = align
				mk := func(parts ...string) (*RichText, bellui.Window) {
					var segs []RichTextSegment
					for _, p := range parts {
						segs = append(segs, &TextSegment{Style: st, Text: p})
					}
					rt := NewRichText(segs...)
					rt.Selectable = true
					w := test.NewWindow(rt)
					w.Resize(geo.NewSize(400, 60))
					return rt, w
				}
				split, sw := mk(tx.parts...)
				_, ww := mk(strings.Join(tx.parts, ""))
				require.Len(t, split.rowBounds, 1, where)

				assertInkAligned(t, inkColumns(ww.Canvas().Capture(), 0, 40), inkColumns(sw.Canvas().Capture(), 0, 40), where)
				sw.Close()
				ww.Close()
			}
		}
	}
}

// The order holds when the row wraps: each wrapped row of a right-to-left
// paragraph split into segments is drawn as the one-segment paragraph's row.
// The paragraph has no closing full stop: a neutral mark at the end of a line
// becomes a run of its own, which both the painter and the caret layout place
// in logical order (#341), so the one-segment line is no oracle for it.
func TestRichText_RTLSegmentsDrawnInVisualOrderWrapped(t *testing.T) {
	test.NewTempApp(t)
	skipWithoutGlyphs(t, "שלום")
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	parts := []string{"שלום עולם זה ", "טקסט ארוך מאוד", " שעובר לשורה הבאה בסוף"}
	mk := func(parts ...string) (*RichText, bellui.Window) {
		var segs []RichTextSegment
		for _, p := range parts {
			segs = append(segs, &TextSegment{Style: RichTextStyleInline, Text: p})
		}
		rt := NewRichText(segs...)
		rt.Wrapping = bellui.TextWrapWord
		return rt, test.NewWindow(rt)
	}
	split, sw := mk(parts...)
	whole, ww := mk(strings.Join(parts, ""))
	defer sw.Close()
	defer ww.Close()
	compared := 0
	for width := float32(150); width <= 360; width += 3 {
		sw.Resize(geo.NewSize(width, 200))
		ww.Resize(geo.NewSize(width, 200))
		if !slices.Equal(richRowStrings(split), richRowStrings(whole)) {
			continue // rows broken elsewhere: not the same picture
		}
		compared++
		si, wi := sw.Canvas().Capture(), ww.Canvas().Capture()
		for row := range len(split.rowBounds) {
			y0, h := split.RowGeometry(row)
			msg := fmt.Sprintf("width %v, row %d %q", width, row, richRowStrings(split)[row])
			if !assertInkAligned(t, inkColumns(wi, int(y0), int(y0+h)), inkColumns(si, int(y0), int(y0+h)), msg) {
				break
			}
		}
	}
	assert.Greater(t, compared, 20, "the split and whole paragraphs must wrap alike at many widths")
}

// An inline non-text segment in a right-to-left row is drawn where the caret
// layout puts its box, so a tap on the link lands on the link's slot (#1441).
// Which slot that is follows the caret layout's run order, which #341 covers.
func TestRichText_RTLRowPlacesLinkAtItsBox(t *testing.T) {
	test.NewTempApp(t)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		bellui.SetLayoutDirection(dir)
		for _, align := range []bellui.TextAlign{bellui.TextAlignLeading, bellui.TextAlignTrailing, bellui.TextAlignCenter} {
			where := fmt.Sprintf("window direction %v, align %v", dir, align)
			st := RichTextStyleInline
			st.Alignment = align
			rt := NewRichText(
				&TextSegment{Style: st, Text: "שלום "},
				&HyperlinkSegment{Text: "קישור", Alignment: align},
				&TextSegment{Style: st, Text: " עולם"},
			)
			rt.Selectable = true
			w := test.NewWindow(rt)
			w.Resize(geo.NewSize(400, 60))
			require.Len(t, rt.rowBounds, 1, where)

			layout, ok := rt.rowVisualLayout(0)
			require.True(t, ok, where)
			boxLeft, _, ok := layout.BoxExtent(0)
			require.True(t, ok, where)
			pad := rt.Theme().Size(theme.SizeNameInnerPadding)
			want := boxLeft + pad - rt.inset.Width + rt.selection.alignOffsetX(0)

			var link core.CanvasObject
			for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
				if c, ok := o.(*bellui.Container); ok && len(c.Objects) > 0 {
					if _, ok := c.Objects[0].(*Hyperlink); ok {
						link = o
					}
				}
			}
			require.NotNil(t, link, where)
			assert.InDelta(t, want, link.Position().X, 0.05, "%s: the link is drawn at its box", where)
			w.Close()
		}
	}
}

// A segment whose runs a neighbour's text separates in the visual order can't
// be drawn as one object in its place, so its row keeps the side-by-side
// layout. "abc שלום" + " עולם def" in a left-to-right base reads abc, עולם,
// שלום, def from the left: each segment has a piece on both sides of the
// other's.
func TestRichText_BidiPlacementSkipsInterleavedSegments(t *testing.T) {
	test.NewTempApp(t)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	rt := NewRichText(
		&TextSegment{Style: RichTextStyleInline, Text: "abc שלום"},
		&TextSegment{Style: RichTextStyleInline, Text: " עולם def"},
	)
	w := test.NewWindow(rt)
	defer w.Close()
	w.Resize(geo.NewSize(400, 60))
	require.Len(t, rt.rowBounds, 1)
	var objs []core.CanvasObject
	for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
		if _, ok := o.(*canvas.Text); ok {
			objs = append(objs, o)
		}
	}
	require.Len(t, objs, 2)
	assert.Nil(t, rt.bidiRowPlacement(&rt.rowBounds[0], objs, bellui.TextAlignLeading, 380, 0, false))

	// Kept apart, the same words are placed.
	rt.Segments = []RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "abc "},
		&TextSegment{Style: RichTextStyleInline, Text: "שלום עולם"},
	}
	rt.Refresh()
	objs = objs[:0]
	for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
		if _, ok := o.(*canvas.Text); ok {
			objs = append(objs, o)
		}
	}
	require.Len(t, objs, 2)
	assert.NotNil(t, rt.bidiRowPlacement(&rt.rowBounds[0], objs, bellui.TextAlignLeading, 380, 0, false))
}

// rtlSegmentDoc is a right-to-left document of 200 paragraphs, each three
// segments, for the placement benchmarks.
func rtlSegmentDoc() []RichTextSegment {
	var segs []RichTextSegment
	for range 200 {
		segs = append(segs,
			&TextSegment{Style: RichTextStyleInline, Text: "שלום עולם זה טקסט "},
			&TextSegment{Style: RichTextStyleStrong, Text: "מודגש"},
			&TextSegment{Style: RichTextStyleParagraph, Text: " שעובר לשורה הבאה בסוף הפסקה"},
		)
	}
	return segs
}

// Laying out again at the same width reuses each row's placement
// (cachedBidiPlacement): shaping every row again made this 26.9 ms against
// 0.06 ms without placement; cached it is 0.16 ms.
func BenchmarkRichTextRTLSegmentsLayout(b *testing.B) {
	test.NewTempApp(b)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	rt := NewRichText(rtlSegmentDoc()...)
	rt.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(rt)
	defer w.Close()
	w.Resize(geo.NewSize(400, 600))
	r := cache.Renderer(rt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Layout(geo.NewSize(380, 20000))
	}
}

// A resize wraps the rows again, so each is placed afresh: 2.4 ms against
// 1.05 ms without placement.
func BenchmarkRichTextRTLSegmentsResize(b *testing.B) {
	test.NewTempApp(b)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	rt := NewRichText(rtlSegmentDoc()...)
	rt.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(rt)
	defer w.Close()
	w.Resize(geo.NewSize(400, 600))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.Resize(geo.NewSize(float32(300+i%40), 20000))
	}
}

// A row placed in one window direction is placed again when the direction
// changes: the cached placement (cachedBidiPlacement) is only reused in the
// direction it was computed in.
func TestRichText_RTLSegmentsFollowADirectionChange(t *testing.T) {
	test.NewTempApp(t)
	skipWithoutGlyphs(t, "שלום")
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	parts := []string{"שלום עולם ", "זה טקסט"}
	split := NewRichText(&TextSegment{Style: RichTextStyleInline, Text: parts[0]}, &TextSegment{Style: RichTextStyleInline, Text: parts[1]})
	whole := NewRichText(&TextSegment{Style: RichTextStyleInline, Text: strings.Join(parts, "")})
	sw, ww := test.NewWindow(split), test.NewWindow(whole)
	defer sw.Close()
	defer ww.Close()
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL, bellui.DirectionLTR} {
		bellui.SetLayoutDirection(dir)
		sw.Resize(geo.NewSize(400, 60))
		ww.Resize(geo.NewSize(400, 60))
		split.Refresh()
		whole.Refresh()
		assertInkAligned(t, inkColumns(ww.Canvas().Capture(), 0, 40), inkColumns(sw.Canvas().Capture(), 0, 40),
			fmt.Sprintf("window direction %v", dir))
	}
}
