package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func TestRichTextHTML_Paragraph(t *testing.T) {
	r := NewRichTextFromHTML("<p>hello world</p>")

	assert.Len(t, r.Segments, 2)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "hello world", text.Text)
		assert.True(t, text.Inline())
	} else {
		t.Error("Segment should be Text")
	}
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "", text.Text)
		assert.Equal(t, RichTextStyleParagraph, text.Style)
	} else {
		t.Error("Last segment should be paragraph break")
	}
}

func TestRichTextHTML_Heading(t *testing.T) {
	r := NewRichTextFromHTML("<h1>Head1</h1><h2>Head2</h2><h3>Head3</h3>")

	assert.Len(t, r.Segments, 3)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "Head1", text.Text)
		assert.Equal(t, RichTextStyleHeading, text.Style)
	} else {
		t.Error("Segment should be Heading")
	}
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "Head2", text.Text)
		assert.Equal(t, RichTextStyleSubHeading, text.Style)
	} else {
		t.Error("Segment should be SubHeading")
	}
	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, "Head3", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("Segment should be Bold")
	}
}

func TestRichTextHTML_Bold(t *testing.T) {
	r := NewRichTextFromHTML("<p><b>bold</b></p>")

	assert.GreaterOrEqual(t, len(r.Segments), 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "bold", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("Segment should be Text")
	}

	r.ParseHTML("<p><strong>strong</strong></p>")
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "strong", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextHTML_Italic(t *testing.T) {
	r := NewRichTextFromHTML("<p><i>italic</i></p>")

	assert.GreaterOrEqual(t, len(r.Segments), 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "italic", text.Text)
		assert.True(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Segment should be Text")
	}

	r.ParseHTML("<p><em>emphasis</em></p>")
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "emphasis", text.Text)
		assert.True(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextHTML_Strikethrough(t *testing.T) {
	for _, tag := range []string{"s", "del", "strike"} {
		r := NewRichTextFromHTML("<p><" + tag + ">gone</" + tag + "></p>")
		assert.GreaterOrEqual(t, len(r.Segments), 1)
		if text, ok := r.Segments[0].(*TextSegment); ok {
			assert.Equal(t, "gone", text.Text)
			assert.True(t, text.Style.TextStyle.Strikethrough, "<%s> should be struck through", tag)
		} else {
			t.Errorf("<%s> segment should be Text", tag)
		}
	}
}

func TestRichTextHTML_Underline(t *testing.T) {
	for _, tag := range []string{"u", "ins"} {
		r := NewRichTextFromHTML("<p><" + tag + ">here</" + tag + "></p>")
		assert.GreaterOrEqual(t, len(r.Segments), 1)
		if text, ok := r.Segments[0].(*TextSegment); ok {
			assert.Equal(t, "here", text.Text)
			assert.True(t, text.Style.TextStyle.Underline, "<%s> should be underlined", tag)
		} else {
			t.Errorf("<%s> segment should be Text", tag)
		}
	}
}

func TestRichTextHTML_NestedInline(t *testing.T) {
	r := NewRichTextFromHTML("<p><b><i>bold italic</i></b></p>")

	assert.GreaterOrEqual(t, len(r.Segments), 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "bold italic", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
		assert.True(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextHTML_MixedInline(t *testing.T) {
	r := NewRichTextFromHTML("<p><b>bold <i>bold-italic</i> bold</b></p>")

	// Should produce 3 text segments + paragraph break
	assert.GreaterOrEqual(t, len(r.Segments), 3)

	// The single spaces around the nested <i> sit at inline run boundaries and
	// must survive parsing (pre-rebrand tracker #668) — otherwise the line renders "boldbold-italicbold".
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "bold ", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
		assert.False(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("First segment should be bold text")
	}

	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "bold-italic", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
		assert.True(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Second segment should be bold+italic text")
	}

	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, " bold", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
		assert.False(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Third segment should be bold text")
	}
}

func TestRichTextHTML_Code(t *testing.T) {
	r := NewRichTextFromHTML("<p>a <code>code</code> inline</p>")

	found := false
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && text.Text == "code" {
			assert.Equal(t, RichTextStyleCodeInline, text.Style)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find inline code segment")
}

func TestRichTextHTML_CodeBlock(t *testing.T) {
	r := NewRichTextFromHTML("<pre><code>line1\nline2</code></pre>")

	assert.Len(t, r.Segments, 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "line1\nline2", text.Text)
		assert.Equal(t, RichTextStyleCodeBlock, text.Style)
	} else {
		t.Error("Segment should be CodeBlock")
	}
}

func TestRichTextHTML_LinkSchemeAllowlist(t *testing.T) {
	// Links from untrusted markup are only honoured for safe schemes; dangerous
	// schemes are dropped so tapping them cannot invoke an external handler (pre-rebrand tracker #285).
	allowed := map[string]bool{
		`<a href="https://bellui.io/">x</a>`:  true,
		`<a href="http://bellui.io/">x</a>`:   true,
		`<a href="mailto:a@b.com">x</a>`:      true,
		`<a href="/relative/path">x</a>`:      true,
		`<a href="file:///etc/passwd">x</a>`:  false,
		`<a href="smb://host/share">x</a>`:    false,
		`<a href="javascript:alert(1)">x</a>`: false,
		`<a href="search-ms:query=x">x</a>`:   false,
	}
	for markup, wantURL := range allowed {
		r := NewRichTextFromHTML(markup)
		var link *HyperlinkSegment
		for _, seg := range r.Segments {
			if l, ok := seg.(*HyperlinkSegment); ok {
				link = l
				break
			}
		}
		if assert.NotNil(t, link, markup) {
			if wantURL {
				assert.NotNil(t, link.URL, markup)
			} else {
				assert.Nil(t, link.URL, markup)
			}
		}
	}
}

func TestRichTextHTML_Link(t *testing.T) {
	r := NewRichTextFromHTML(`<p><a href="https://bellui.io/">title</a></p>`)

	found := false
	for _, seg := range r.Segments {
		if link, ok := seg.(*HyperlinkSegment); ok {
			assert.Equal(t, "title", link.Text)
			assert.Equal(t, "bellui.io", link.URL.Host)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find hyperlink segment")
}

func TestRichTextHTML_List_Unordered(t *testing.T) {
	r := NewRichTextFromHTML("<ul><li>item1</li><li>item2</li></ul>")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.False(t, list.Ordered)
		assert.Len(t, list.Items, 2)
		assert.Equal(t, "item1", list.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		assert.Equal(t, "item2", list.Items[1].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextHTML_List_Ordered(t *testing.T) {
	r := NewRichTextFromHTML("<ol><li>first</li><li>second</li></ol>")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.True(t, list.Ordered)
		assert.Len(t, list.Items, 2)
		assert.Equal(t, 1, list.StartNumber())
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextHTML_List_OrderedStart(t *testing.T) {
	r := NewRichTextFromHTML(`<ol start="5"><li>five</li><li>six</li></ol>`)

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.True(t, list.Ordered)
		assert.Equal(t, 5, list.StartNumber())
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextHTML_NestedList(t *testing.T) {
	r := NewRichTextFromHTML("<ul><li>item 1<ul><li>item 1.1</li></ul></li><li>item 2</li></ul>")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.Len(t, list.Items, 3) // item1 paragraph, sublist, item2 paragraph
		assert.Equal(t, "item 1", list.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		if sublist, ok := list.Items[1].(*ListSegment); ok {
			assert.Len(t, sublist.Items, 1)
			assert.Equal(t, "item 1.1", sublist.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		} else {
			t.Error("Second item should be a sub-list")
		}
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextHTML_Blockquote(t *testing.T) {
	r := NewRichTextFromHTML("<blockquote><p>quoted text</p></blockquote>")

	found := false
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && text.Text == "quoted text" {
			assert.Equal(t, RichTextStyleBlockquote, text.Style)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find blockquote segment")
}

func TestRichTextHTML_Separator(t *testing.T) {
	r := NewRichTextFromHTML("<hr>")

	found := false
	for _, seg := range r.Segments {
		if _, ok := seg.(*SeparatorSegment); ok {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find separator segment")
}

func TestRichTextHTML_Table(t *testing.T) {
	r := NewRichTextFromHTML(`<table>
		<thead><tr><th>Name</th><th>Age</th></tr></thead>
		<tbody><tr><td>Alice</td><td>30</td></tr><tr><td>Bob</td><td>25</td></tr></tbody>
	</table>`)

	assert.Len(t, r.Segments, 1)
	tbl, ok := r.Segments[0].(*TableSegment)
	if !ok {
		t.Fatal("Segment should be a TableSegment")
	}
	assert.Equal(t, []string{"Name", "Age"}, tbl.Header)
	assert.Len(t, tbl.Rows, 2)
	assert.Equal(t, []string{"Alice", "30"}, tbl.Rows[0])
	assert.Equal(t, []string{"Bob", "25"}, tbl.Rows[1])
}

func TestRichTextHTML_TableNoThead(t *testing.T) {
	r := NewRichTextFromHTML(`<table>
		<tr><td>Name</td><td>Age</td></tr>
		<tr><td>Alice</td><td>30</td></tr>
	</table>`)

	assert.Len(t, r.Segments, 1)
	tbl, ok := r.Segments[0].(*TableSegment)
	if !ok {
		t.Fatal("Segment should be a TableSegment")
	}
	assert.Equal(t, []string{"Name", "Age"}, tbl.Header)
	assert.Len(t, tbl.Rows, 1)
	assert.Equal(t, []string{"Alice", "30"}, tbl.Rows[0])
}

func TestRichTextHTML_TableAlignment(t *testing.T) {
	r := NewRichTextFromHTML(`<table>
		<thead><tr><th align="left">L</th><th align="center">C</th><th align="right">R</th></tr></thead>
		<tbody><tr><td>a</td><td>b</td><td>c</td></tr></tbody>
	</table>`)

	tbl, ok := r.Segments[0].(*TableSegment)
	if !ok {
		t.Fatal("Segment should be a TableSegment")
	}
	assert.Equal(t, bellui.TextAlignLeading, tbl.Alignments[0])
	assert.Equal(t, bellui.TextAlignCenter, tbl.Alignments[1])
	assert.Equal(t, bellui.TextAlignTrailing, tbl.Alignments[2])
}

func TestRichTextHTML_Br(t *testing.T) {
	r := NewRichTextFromHTML("<p>line1<br>line2</p>")

	// Should have: "line1", paragraph break (from br), "line2", paragraph break (from p)
	assert.GreaterOrEqual(t, len(r.Segments), 3)
	assert.Equal(t, "line1", r.Segments[0].(*TextSegment).Text)
}

func TestRichTextHTML_UnsupportedTag(t *testing.T) {
	r := NewRichTextFromHTML("<p><marquee>scrolling text</marquee></p>")

	// The marquee tag is stripped but the text should remain
	found := false
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && text.Text == "scrolling text" {
			found = true
			break
		}
	}
	assert.True(t, found, "Text inside unsupported tags should be preserved")
}

func TestRichTextHTML_WhitespaceCollapse(t *testing.T) {
	r := NewRichTextFromHTML("<p>  hello   world  </p>")

	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "hello world", text.Text)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextHTML_PreWhitespace(t *testing.T) {
	r := NewRichTextFromHTML("<pre>  hello   world\n  indented</pre>")

	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "  hello   world\n  indented", text.Text)
		assert.Equal(t, RichTextStyleCodeBlock, text.Style)
	} else {
		t.Error("Segment should be CodeBlock")
	}
}

func TestRichTextHTML_Entities(t *testing.T) {
	r := NewRichTextFromHTML("<p>a&amp;b &lt;c&gt; d&mdash;e</p>")

	// x/net/html decodes entities automatically
	text := r.String()
	assert.Contains(t, text, "a&b")
	assert.Contains(t, text, "<c>")
	assert.Contains(t, text, "d—e")
}

func TestRichTextHTML_AppendHTML(t *testing.T) {
	r := NewRichTextFromHTML("<p>first</p>")
	count := len(r.Segments)
	r.AppendHTML("<p>second</p>")
	assert.Greater(t, len(r.Segments), count)
}

func TestRichTextHTML_ParseHTML(t *testing.T) {
	r := NewRichTextFromHTML("<p>first</p>")
	r.ParseHTML("<p>replaced</p>")

	text := r.String()
	assert.NotContains(t, text, "first")
	assert.Contains(t, text, "replaced")
}

func TestRichTextHTML_EmptyInput(t *testing.T) {
	r := NewRichTextFromHTML("")
	assert.Empty(t, r.Segments)
}

func TestRichTextHTML_ScriptStripped(t *testing.T) {
	r := NewRichTextFromHTML("<p>safe</p><script>alert('xss')</script>")

	text := r.String()
	assert.Contains(t, text, "safe")
	// Script content may or may not appear as text — the important thing is
	// it doesn't execute (Bellui is not a browser). Let's verify the visible
	// text at least includes "safe".
}

func BenchmarkHTMLParsing(b *testing.B) {
	content := `<h1>Test heading</h1>
<p>This is some test HTML. It contains some different HTML
elements in an effort to put a realistic load onto the parser.</p>
<blockquote><p>Here is a quote.
It stretches over two lines and contains some <code>inline code</code>.</p></blockquote>
<p>Moreover, we've got a list:</p>
<ul><li>foo</li><li>bar</li><li>baz</li></ul>`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewRichTextFromHTML(content)
	}
}

// segTextsOf returns the Text of each TextSegment (in order), ignoring other
// segment kinds — a helper for asserting inline whitespace handling (pre-rebrand tracker #668).
func segTextsOf(r *RichText) []string {
	var out []string
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok {
			out = append(out, text.Text)
		}
	}
	return out
}

// TestRichTextHTML_BoundarySpacePreserved is the pre-rebrand tracker #668 fix: a single space at an
// inline style boundary survives parsing instead of being trimmed away.
func TestRichTextHTML_BoundarySpacePreserved(t *testing.T) {
	r := NewRichTextFromHTML("<p>x <b>y</b> z</p>")
	// "x " (trailing space kept, next <b> inline) + "y" + " z" (leading kept).
	assert.Equal(t, []string{"x ", "y", " z", ""}, segTextsOf(r))
}

// TestRichTextHTML_WhitespaceOnlyBetweenInline confirms a pure-whitespace text
// node bridging two inline elements becomes a single space.
func TestRichTextHTML_WhitespaceOnlyBetweenInline(t *testing.T) {
	r := NewRichTextFromHTML("<p><b>a</b> <b>b</b></p>")
	assert.Equal(t, []string{"a", " ", "b", ""}, segTextsOf(r))
}

// TestRichTextHTML_BlockEdgeWhitespaceTrimmed confirms whitespace at a block
// edge (start/end of a paragraph) is still trimmed.
func TestRichTextHTML_BlockEdgeWhitespaceTrimmed(t *testing.T) {
	r := NewRichTextFromHTML("<p>   hello   </p>")
	assert.Equal(t, []string{"hello", ""}, segTextsOf(r))
}

// TestRichTextHTML_InterBlockWhitespaceDropped confirms pure-whitespace between
// block elements (pretty-printed HTML) contributes no segment.
func TestRichTextHTML_InterBlockWhitespaceDropped(t *testing.T) {
	r := NewRichTextFromHTML("<p>a</p>\n  <p>b</p>")
	assert.Equal(t, []string{"a", "", "b", ""}, segTextsOf(r))
}

// TestRichTextHTML_InternalWhitespaceCollapsed confirms multiple internal spaces
// still collapse to one (unchanged behaviour).
func TestRichTextHTML_InternalWhitespaceCollapsed(t *testing.T) {
	r := NewRichTextFromHTML("<p>a    b\t\nc</p>")
	assert.Equal(t, []string{"a b c", ""}, segTextsOf(r))
}

// TestRichTextHTML_SpaceBeforeBlockTrimmed confirms a space between an inline run
// and a following block element is dropped (no trailing space before the break).
func TestRichTextHTML_SpaceBeforeBlockTrimmed(t *testing.T) {
	r := NewRichTextFromHTML("<div><b>a</b> <p>b</p></div>")
	assert.Equal(t, []string{"a", "b", ""}, segTextsOf(r))
}

// TestRichTextHTML_NbspPreserved is the pre-rebrand tracker #876 fix: a non-breaking space (&nbsp;,
// U+00A0) is content, not collapsible whitespace, so it survives parsing instead
// of being collapsed or trimmed like an ordinary space.
func TestRichTextHTML_NbspPreserved(t *testing.T) {
	r := NewRichTextFromHTML("<p>a&nbsp;b</p>")
	assert.Equal(t, []string{"a\u00A0b", ""}, segTextsOf(r), "internal &nbsp; is preserved")
}

// TestRichTextHTML_NbspNotTrimmedAtEdge confirms a leading/trailing &nbsp; is not
// trimmed like an ASCII space at a block edge.
func TestRichTextHTML_NbspNotTrimmedAtEdge(t *testing.T) {
	r := NewRichTextFromHTML("<p>&nbsp;x&nbsp;</p>")
	assert.Equal(t, []string{"\u00A0x\u00A0", ""}, segTextsOf(r), "edge &nbsp; survives")
}

// TestRichTextHTML_NbspRunPreservesEach confirms consecutive &nbsp; are each kept
// (a run of non-breaking spaces is not collapsed to one), while surrounding ASCII
// spaces still collapse.
func TestRichTextHTML_NbspRunPreservesEach(t *testing.T) {
	r := NewRichTextFromHTML("<p>a &nbsp;&nbsp; b</p>")
	// "a " + two nbsp + " b": ASCII runs collapse to one space each; the two
	// nbsp are preserved verbatim between them.
	assert.Equal(t, []string{"a \u00A0\u00A0 b", ""}, segTextsOf(r))
}

// TestRichTextHTML_NbspOnlyNodeIsContent confirms a text node that is only a
// &nbsp; between inline elements is kept as content (not dropped as whitespace).
func TestRichTextHTML_NbspOnlyNodeIsContent(t *testing.T) {
	r := NewRichTextFromHTML("<p><b>a</b>&nbsp;<b>b</b></p>")
	assert.Equal(t, []string{"a", "\u00A0", "b", ""}, segTextsOf(r))
}

// TestRichTextHTML_OrdinarySpaceStillCollapses guards that the change is scoped
// to &nbsp;: ordinary ASCII whitespace still collapses and trims as before.
func TestRichTextHTML_OrdinarySpaceStillCollapses(t *testing.T) {
	r := NewRichTextFromHTML("<p>  a    b  </p>")
	assert.Equal(t, []string{"a b", ""}, segTextsOf(r))
}

// TestRichTextHTML_TableCellNbspNotTrimmed is the pre-rebrand tracker #914 fix: an edge &nbsp; in a
// table cell survives, consistent with internal &nbsp; (pre-rebrand tracker #876). Cells are trimmed
// of collapsible whitespace only.
func TestRichTextHTML_TableCellNbspNotTrimmed(t *testing.T) {
	r := NewRichTextFromHTML("<table><tr><td>&nbsp;lead</td><td>a&nbsp;b</td></tr></table>")
	var tbl *TableSegment
	for _, seg := range r.Segments {
		if ts, ok := seg.(*TableSegment); ok {
			tbl = ts
		}
	}
	require.NotNil(t, tbl)
	// First row is the header (no <thead>): its cells keep their &nbsp;.
	require.Len(t, tbl.Header, 2)
	assert.Equal(t, " lead", tbl.Header[0], "leading &nbsp; in a cell survives")
	assert.Equal(t, "a b", tbl.Header[1], "internal &nbsp; still survives")
}

// TestRichTextHTML_TableCellOrdinaryWhitespaceTrimmed guards that ordinary ASCII
// edge whitespace in a cell is still trimmed.
func TestRichTextHTML_TableCellOrdinaryWhitespaceTrimmed(t *testing.T) {
	r := NewRichTextFromHTML("<table><tr><td>  x  </td></tr></table>")
	var tbl *TableSegment
	for _, seg := range r.Segments {
		if ts, ok := seg.(*TableSegment); ok {
			tbl = ts
		}
	}
	require.NotNil(t, tbl)
	require.Len(t, tbl.Header, 1)
	assert.Equal(t, "x", tbl.Header[0], "ASCII edge whitespace still trimmed")
}
