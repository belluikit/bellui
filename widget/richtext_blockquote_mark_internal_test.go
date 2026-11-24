package widget

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// quoteDrawnRows groups the text objects rt draws into rows by their y, top to
// bottom, each row's objects left to right. A code run sits up to a pixel
// lower than the text beside it, so objects within 3 px share a row.
func quoteDrawnRows(t *testing.T, rt *RichText) [][]*canvas.Text {
	t.Helper()
	texts := rowTexts(t, rt)
	sort.SliceStable(texts, func(a, b int) bool { return texts[a].Position().Y < texts[b].Position().Y })
	var rows [][]*canvas.Text
	for _, tx := range texts {
		if n := len(rows); n > 0 && tx.Position().Y-rows[n-1][0].Position().Y < 3 {
			rows[n-1] = append(rows[n-1], tx)
			continue
		}
		rows = append(rows, []*canvas.Text{tx})
	}
	for _, row := range rows {
		sort.SliceStable(row, func(a, b int) bool { return row[a].Position().X < row[b].Position().X })
	}
	return rows
}

// quoteSources are blockquotes whose rows can start with a run that is not
// plain quote text: emphasis, code, or plain text followed by them. Each is
// long enough to wrap in a narrow window.
var quoteSources = []struct {
	name string
	html bool
	src  string
}{
	{"markdown, bold first", false, "> **Bold** start of a quote that runs on long enough to wrap over several rows of a narrow window"},
	{"markdown, italic first", false, "> *Italic* start of a quote that runs on long enough to wrap over several rows of a narrow window"},
	{"markdown, code first", false, "> `code` start of a quote that runs on long enough to wrap over several rows of a narrow window"},
	{"markdown, emphasis inside", false, "> a quote with **bold words** and *italic words* and `code` all through it, long enough to wrap over rows"},
	{"HTML, bold first", true, "<blockquote><b>Bold</b> start of a quote that runs on long enough to wrap over several rows of a narrow window</blockquote>"},
	{"HTML, code first", true, "<blockquote><code>code</code> start of a quote that runs on long enough to wrap over several rows</blockquote>"},
}

func parseQuote(html bool, src string) []RichTextSegment {
	if html {
		return NewRichTextFromHTML(src).Segments
	}
	return NewRichTextFromMarkdown(src).Segments
}

// A quote that starts with a link is drawn, wrapped and given its carets at
// the indent (#1443). Only text segments were checked for the mark, so a row
// starting with a link had none. The oracle is the same row outside a quote:
// the link and the first caret sit exactly one indent further right, and the
// quote's rows still end inside the widget.
func TestRichText_QuoteStartingWithALinkDrawsAtItsIndent(t *testing.T) {
	test.NewTempApp(t)
	const rest = " start of a quote that runs on long enough to wrap over several rows of a narrow window"
	for _, tc := range []struct {
		name         string
		html         bool
		quote, plain string
	}{
		{"markdown", false, "> [a link](https://example.com)" + rest, "[a link](https://example.com)" + rest},
		{"HTML", true, `<blockquote><a href="https://example.com">a link</a>` + rest + "</blockquote>", `<p><a href="https://example.com">a link</a>` + rest + "</p>"},
	} {
		linkX := func(rt *RichText) (float32, bool) {
			for _, o := range test.TempWidgetRenderer(t, rt).(*textRenderer).origObjs {
				// A link is drawn as a container holding its Hyperlink.
				if c, ok := o.(*bellui.Container); ok && len(c.Objects) == 1 {
					if _, ok := c.Objects[0].(*Hyperlink); ok {
						return c.Position().X, true
					}
				}
			}
			return 0, false
		}
		mk := func(src string) (*RichText, bellui.Window) {
			rt := NewRichText(parseQuote(tc.html, src)...)
			rt.Wrapping = bellui.TextWrapWord
			rt.Selectable = true
			return rt, test.NewWindow(rt)
		}
		quote, qw := mk(tc.quote)
		plain, pw := mk(tc.plain)
		_, isLink := quote.Segments[0].(*HyperlinkSegment)
		require.True(t, isLink, "%s: the quote starts with a link", tc.name)
		pad := quote.Theme().Size(theme.SizeNameInnerPadding)
		size := quote.Theme().Size(theme.SizeNameText)
		indent := blockquoteIndent(quote.Theme())
		for width := float32(180); width <= 420; width += 30 {
			where := fmt.Sprintf("%s, width %v", tc.name, width)
			qw.Resize(geo.NewSize(width, 400))
			pw.Resize(geo.NewSize(width, 400))
			qx, ok := linkX(quote)
			require.True(t, ok, "%s: the quote draws its link", where)
			px, ok := linkX(plain)
			require.True(t, ok, "%s: the paragraph draws its link", where)
			assert.InDelta(t, px+indent, qx, 0.01, "%s: the link sits at the indent", where)
			qc := quote.lineSizeToColumn(0, 0, size, pad, true).Width + quote.selection.alignOffsetX(0)
			pc := plain.lineSizeToColumn(0, 0, size, pad, true).Width + plain.selection.alignOffsetX(0)
			assert.InDelta(t, pc+indent, qc, 0.05, "%s: the first caret sits at the indent", where)
			rows := quoteDrawnRows(t, quote)
			assert.Greater(t, len(rows), 1, "%s: the quote must wrap", where)
			for r, row := range rows {
				last := row[len(row)-1]
				end := last.Position().X + bellui.MeasureText(strings.TrimRight(last.Text, " "), last.TextSize, last.TextStyle).Width
				assert.LessOrEqual(t, end, quote.Size().Width-pad+0.5, "%s: row %d ends past the right edge", where, r)
			}
		}
		qw.Close()
		pw.Close()

		// Unwrapped, the minimum width counts the indent: the same segments
		// with the mark cleared are exactly one indent narrower.
		marked := parseQuote(tc.html, tc.quote)
		unmarked := parseQuote(tc.html, tc.quote)
		for _, seg := range unmarked {
			switch s := seg.(type) {
			case *TextSegment:
				s.Style.blockquote = false
			case *HyperlinkSegment:
				s.Style.blockquote = false
			}
		}
		minWidth := func(segs []RichTextSegment) float32 {
			return test.TempWidgetRenderer(t, NewRichText(segs...)).MinSize().Width
		}
		assert.InDelta(t, minWidth(unmarked)+indent, minWidth(marked), 0.01, "%s: the minimum width counts the indent", tc.name)
	}
}

// A blockquote whose rows start with bold, italic or code text draws, wraps
// and places its carets at the blockquote indent, on the dense path and on the
// windowed-block path (#1443). A run was a blockquote only when its style
// equalled the preset, so the emphasised run of "> **Bold** start" was a plain
// paragraph, and a row starting with it was drawn at x = 8 instead of 24.
func TestRichText_EmphasisedQuoteDrawsAtItsIndent(t *testing.T) {
	test.NewTempApp(t)
	paths := []struct {
		name  string
		build func(segs []RichTextSegment) *RichText
		check func(*RichText) bool
	}{
		{"dense", func(segs []RichTextSegment) *RichText {
			return NewRichText(segs...)
		}, func(rt *RichText) bool { return !rt.lazy && !rt.blockLazy && !rt.varHeight }},
		{"windowed blocks", func(segs []RichTextSegment) *RichText {
			// Enough segments for the windowed build, with the quote first so
			// it is in view.
			all := slices.Clone(segs)
			for i := range 2200 {
				all = append(all, &TextSegment{Style: RichTextStyleParagraph, Text: fmt.Sprintf("para %d", i)})
			}
			rt := NewRichText(all...)
			rt.virtualize = true
			return rt
		}, func(rt *RichText) bool { return rt.blockLazy }},
	}
	for _, q := range quoteSources {
		for _, p := range paths {
			rt := p.build(parseQuote(q.html, q.src))
			rt.Wrapping = bellui.TextWrapWord
			rt.Selectable = true
			w := test.NewWindow(rt)
			pad := rt.Theme().Size(theme.SizeNameInnerPadding)
			size := rt.Theme().Size(theme.SizeNameText)
			indent := blockquoteIndent(rt.Theme())
			for width := float32(180); width <= 420; width += 30 {
				where := fmt.Sprintf("%s, %s path, width %v", q.name, p.name, width)
				w.Resize(geo.NewSize(width, 400))
				require.True(t, p.check(rt), "%s: the path must be taken", where)
				rows := quoteDrawnRows(t, rt)
				quoteRows := 0
				for r, row := range rows {
					if strings.HasPrefix(row[0].Text, "para ") {
						break
					}
					quoteRows++
					assert.InDelta(t, pad+indent, row[0].Position().X, 0.01, "%s: row %d %q starts at the indent", where, r, row[0].Text)
					last := row[len(row)-1]
					end := last.Position().X + bellui.MeasureText(strings.TrimRight(last.Text, " "), last.TextSize, last.TextStyle).Width
					assert.LessOrEqual(t, end, rt.Size().Width-pad+0.5, "%s: row %d ends past the right edge", where, r)
					caret := rt.lineSizeToColumn(0, r, size, pad, true).Width + rt.selection.alignOffsetX(r)
					assert.InDelta(t, row[0].Position().X, caret, 0.05, "%s: row %d's first caret", where, r)
				}
				assert.Greater(t, quoteRows, 1, "%s: the quote must wrap", where)
			}
			w.Close()
		}
	}
}

// A quote written in emphasis throughout is one segment, which takes the lazy
// path, and draws and wraps there at the indent too (#1443).
func TestRichText_EmphasisedQuoteDrawsAtItsIndentOnTheLazyPath(t *testing.T) {
	test.NewTempApp(t)
	for _, src := range []string{
		"> *a quote written in italics from its first word to its last, long enough to wrap over several rows*",
		"> **a quote written in bold from its first word to its last, long enough to wrap over several rows**",
	} {
		segs := NewRichTextFromMarkdown(src).Segments
		require.Len(t, segs, 1, src)
		mk := func(virt bool) (*RichText, bellui.Window) {
			rt := NewRichText(&TextSegment{Style: segs[0].(*TextSegment).Style, Text: segs[0].(*TextSegment).Text})
			rt.Wrapping = bellui.TextWrapWord
			rt.Scroll = widget.ScrollVerticalOnly
			rt.Selectable = true
			rt.Virtualize = virt
			return rt, test.NewWindow(rt)
		}
		lazy, lw := mk(true)
		dense, dw := mk(false)
		pad := lazy.Theme().Size(theme.SizeNameInnerPadding)
		size := lazy.Theme().Size(theme.SizeNameText)
		indent := blockquoteIndent(lazy.Theme())
		for width := float32(180); width <= 420; width += 30 {
			where := fmt.Sprintf("%s, width %v", src, width)
			lw.Resize(geo.NewSize(width, 1000))
			dw.Resize(geo.NewSize(width, 1000))
			require.True(t, lazy.lazy, "%s: the lazy path must be taken", where)
			lr, dr := richRowStrings(lazy), richRowStrings(dense)
			require.Greater(t, len(lr), 1, "%s: the quote must wrap", where)
			assert.True(t, slices.Equal(dr, lr), "%s: lazy rows %q, dense rows %q", where, lr, dr)
			for r, tx := range rowTexts(t, lazy) {
				assert.InDelta(t, pad+indent, tx.Position().X, 0.01, "%s: row %q", where, tx.Text)
				caret := lazy.lineSizeToColumn(0, r, size, pad, true).Width + lazy.selection.alignOffsetX(r)
				assert.InDelta(t, tx.Position().X, caret, 0.05, "%s: row %d's first caret", where, r)
			}
		}
		lw.Close()
		dw.Close()
	}
}

// What the parsers make of a quote (#1443): every run in it is marked as a
// blockquote, its text flows on one line as a paragraph's does, and the
// emphasis inside it is kept. Each plain-text run in a quote used to be the
// block preset itself, so a quote broke its line after every run of plain
// text ("> start **bold** end" drew "start" on one row and "bold end",
// unindented, on the next), joined no source lines, dropped HTML's <b> and
// <u>, and showed "&amp;" undecoded.
func TestRichText_QuoteRunsAreMarkedAndFlow(t *testing.T) {
	test.NewTempApp(t)
	rows := func(segs []RichTextSegment) []string {
		rt := NewRichText(segs...)
		rt.Wrapping = bellui.TextWrapWord
		w := test.NewWindow(rt)
		defer w.Close()
		w.Resize(geo.NewSize(800, 400))
		return richRowStrings(rt)
	}
	for _, tc := range []struct {
		name string
		html bool
		src  string
		rows []string
	}{
		{"emphasis inside", false, "> start **bold** end", []string{"start bold end"}},
		{"code inside", false, "> start `code` end", []string{"start code end"}},
		{"source lines", false, "> line one\n> line two", []string{"line one line two"}},
		{"paragraph after", false, "> quote\n\nafter", []string{"quote", "after"}},
		{"two paragraphs", false, "> one\n>\n> two", []string{"one", "two"}},
		{"entity", false, "> a &amp; b", []string{"a & b"}},
		// richRowStrings leaves out a link's text.
		{"link last", false, "> see [docs](https://example.com)\n\nafter", []string{"see ", "after"}},
		{"HTML emphasis inside", true, "<blockquote>start <b>bold</b> end</blockquote>", []string{"start bold end"}},
		{"HTML paragraph after", true, "<blockquote>quote</blockquote><p>after</p>", []string{"quote", "after"}},
		{"HTML paragraphs inside", true, "<blockquote><p>one</p><p>two <i>it</i></p></blockquote>", []string{"one", "two it"}},
	} {
		segs := parseQuote(tc.html, tc.src)
		assert.Equal(t, tc.rows, rows(segs), "%s: rows", tc.name)
		for _, seg := range segs {
			text, ok := seg.(*TextSegment)
			if !ok || strings.TrimSpace(text.Text) == "" {
				continue
			}
			inQuote := text.Text != "after"
			assert.Equal(t, inQuote, text.Style.isBlockquote(), "%s: run %q", tc.name, text.Text)
		}
	}

	for _, tc := range []struct {
		html bool
		src  string
	}{{false, "> start **bold** end"}, {true, "<blockquote>start <b>bold</b> end</blockquote>"}} {
		for _, seg := range parseQuote(tc.html, tc.src) {
			if text, ok := seg.(*TextSegment); ok && text.Text == "bold" {
				assert.True(t, text.Style.TextStyle.Bold, "%s: the bold run stays bold", tc.src)
				assert.True(t, text.Style.TextStyle.Italic, "%s: and keeps the quote's italics", tc.src)
				assert.Equal(t, theme.ColorNameOnSurfaceVariant, text.Style.ColorName, "%s: and its colour", tc.src)
			}
		}
	}
	u := parseQuote(true, "<blockquote>start <u>under</u> <s>struck</s> end</blockquote>")
	require.Len(t, u, 5)
	assert.True(t, u[1].(*TextSegment).Style.TextStyle.Underline, "HTML <u> in a quote stays underlined")
	assert.True(t, u[3].(*TextSegment).Style.TextStyle.Strikethrough, "HTML <s> in a quote stays struck through")

	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseHTML(`<blockquote>start <span style="font-family: MyFont">styled</span> end</blockquote>`)
	require.Len(t, r.Segments, 3)
	styled := r.Segments[1].(*TextSegment)
	assert.Equal(t, src, styled.Style.FontSource, "a named font in a quote is kept")
	assert.True(t, styled.Style.isBlockquote(), "and the run is still a quote run")
}

// A copy of the blockquote preset stays a blockquote whatever fields change;
// a style written out field by field to look like it is not one (#1443).
func TestRichTextStyle_BlockquoteMarkSurvivesCopies(t *testing.T) {
	changes := map[string]func(*RichTextStyle){
		"bold":    func(s *RichTextStyle) { s.TextStyle.Bold = true },
		"inline":  func(s *RichTextStyle) { s.Inline = true },
		"aligned": func(s *RichTextStyle) { s.Alignment = bellui.TextAlignCenter },
		"colour":  func(s *RichTextStyle) { s.ColorName = theme.ColorNamePrimary },
		"size":    func(s *RichTextStyle) { s.SizeName = theme.SizeNameHeadingText },
	}
	for name, change := range changes {
		s := RichTextStyleBlockquote
		change(&s)
		assert.True(t, s.isBlockquote(), "the preset made %s is still a blockquote", name)
	}
	lookAlike := RichTextStyle{
		ColorName: RichTextStyleBlockquote.ColorName,
		SizeName:  RichTextStyleBlockquote.SizeName,
		TextStyle: RichTextStyleBlockquote.TextStyle,
	}
	assert.False(t, lookAlike.isBlockquote(), "a style that only looks like the preset is not a blockquote")
	assert.False(t, RichTextStyleParagraph.isBlockquote())
}
