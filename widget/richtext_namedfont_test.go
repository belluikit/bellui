package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// firstFontSource returns the FontSource of the first TextSegment in a parsed
// RichText, or nil when there is none.
func firstFontSource(t *testing.T, r *RichText) bellui.Resource {
	t.Helper()
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && text.Text != "" {
			return text.Style.FontSource
		}
	}
	return nil
}

// TestRichText_NamedFont_SpanFontFamily registers a named font and confirms a
// <span style="font-family: …"> selects it on the produced TextSegment.
func TestRichText_NamedFont_SpanFontFamily(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseHTML(`<span style="font-family: MyFont">styled</span>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_FontFace confirms the legacy <font face="…"> attribute
// selects a registered font.
func TestRichText_NamedFont_FontFace(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseHTML(`<font face="MyFont">styled</font>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_CommaListFirstMatch verifies the first family in a
// comma-separated list that is registered wins, skipping earlier unregistered
// (or generic) names.
func TestRichText_NamedFont_CommaListFirstMatch(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseHTML(`<span style="font-family: Nope, MyFont, Arial">styled</span>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_Unregistered confirms an unknown family falls through
// to the default behaviour: nil FontSource, unchanged.
func TestRichText_NamedFont_Unregistered(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseHTML(`<span style="font-family: Unknown">plain</span>`)

	assert.Nil(t, firstFontSource(t, r))
}

// TestRichText_NamedFont_NoRegistryIsInert confirms that without any registered
// font, font-family is a no-op (no allowlist → nothing selectable).
func TestRichText_NamedFont_NoRegistryIsInert(t *testing.T) {
	r := NewRichText()
	r.ParseHTML(`<span style="font-family: MyFont">plain</span>`)

	assert.Nil(t, firstFontSource(t, r))
}

// TestRichText_NamedFont_CaseWhitespaceInsensitive confirms the family match is
// case- and whitespace-insensitive at both registration and lookup.
func TestRichText_NamedFont_CaseWhitespaceInsensitive(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("  My Font  ", src)
	r.ParseHTML(`<span style="font-family:   my FONT  ">styled</span>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_QuotedFamily confirms CSS quotes around the family name
// are tolerated.
func TestRichText_NamedFont_QuotedFamily(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("My Font", src)
	r.ParseHTML(`<span style="font-family: 'My Font'">styled</span>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_AppendHTML confirms AppendHTML honours the registry too.
func TestRichText_NamedFont_AppendHTML(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.AppendHTML(`<span style="font-family: MyFont">styled</span>`)

	assert.Equal(t, src, firstFontSource(t, r))
}

// TestRichText_NamedFont_Markdown confirms a block-level HTML fragment in
// markdown gets named fonts "for free" via the shared HTML parser delegation:
// the whole fragment (including the nested <span> and its content) parses
// through parseHTML, so the font resolves.
func TestRichText_NamedFont_Markdown(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`<div><span style="font-family: MyFont">styled</span></div>`)

	assert.Equal(t, src, mdFontOfText(t, r, "styled"))
}

// mdFontOfText returns the FontSource of the parsed TextSegment whose text
// contains want (trimmed), or nil. Markdown emits per-run TextSegments, so a
// wrapped word is its own segment.
func mdFontOfText(t *testing.T, r *RichText, want string) bellui.Resource {
	t.Helper()
	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && strings.Contains(text.Text, want) {
			return text.Style.FontSource
		}
	}
	return nil
}

// TestRichText_NamedFont_MarkdownInlineSpan is the #652 fix: a *bare inline*
// <span style="font-family: …"> interspersed with markdown text — which goldmark
// splits into separate RawHTML open/close nodes around an ordinary markdown Text
// node — now resolves the named font on the wrapped text.
func TestRichText_NamedFont_MarkdownInlineSpan(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`before <span style="font-family: MyFont">styled</span> after`)

	assert.Equal(t, src, mdFontOfText(t, r, "styled"), "wrapped text uses the named font")
	assert.Nil(t, mdFontOfText(t, r, "before"), "text before the span stays default")
	assert.Nil(t, mdFontOfText(t, r, "after"), "text after the span stays default")
}

// TestRichText_NamedFont_MarkdownInlineFontFace covers the legacy inline
// <font face="…"> form in markdown.
func TestRichText_NamedFont_MarkdownInlineFontFace(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`x <font face="MyFont">styled</font> y`)

	assert.Equal(t, src, mdFontOfText(t, r, "styled"))
}

// TestRichText_NamedFont_MarkdownInlineSpanComposesBold confirms the inline span
// font composes with markdown emphasis nested inside it.
func TestRichText_NamedFont_MarkdownInlineSpanComposesBold(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`a <span style="font-family: MyFont">**bolded**</span> b`)

	for _, seg := range r.Segments {
		if text, ok := seg.(*TextSegment); ok && strings.Contains(text.Text, "bolded") {
			assert.Equal(t, src, text.Style.FontSource, "font applies through emphasis")
			assert.True(t, text.Style.TextStyle.Bold, "bold still composes")
			return
		}
	}
	t.Fatal("bolded segment not found")
}

// TestRichText_NamedFont_MarkdownInlineSpanLink confirms the inline span font
// reaches a link inside it — exercising HyperlinkSegment.Style.FontSource (#651).
func TestRichText_NamedFont_MarkdownInlineSpanLink(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`see <span style="font-family: MyFont">[docs](https://example.com)</span> now`)

	var found bellui.Resource
	for _, seg := range r.Segments {
		if link, ok := seg.(*HyperlinkSegment); ok {
			found = link.Style.FontSource
		}
	}
	assert.Equal(t, src, found, "the link inside the span carries the named font")
}

// TestRichText_NamedFont_MarkdownInlineSpanUnregistered confirms a span whose
// family is not registered leaves the wrapped text at the default font.
func TestRichText_NamedFont_MarkdownInlineSpanUnregistered(t *testing.T) {
	r := NewRichText()
	r.RegisterNamedFont("MyFont", distinctFont())
	r.ParseMarkdown(`a <span style="font-family: Unknown">plain</span> b`)

	assert.Nil(t, mdFontOfText(t, r, "plain"))
}

// TestRichText_NamedFont_MarkdownInlineSpanNested confirms nested inline spans
// resolve innermost-wins and an inner span with no family inherits the outer.
func TestRichText_NamedFont_MarkdownInlineSpanNested(t *testing.T) {
	outer := distinctFont()
	inner := bellui.NewStaticResource("inner", []byte("x"))
	r := NewRichText()
	r.RegisterNamedFont("Outer", outer)
	r.RegisterNamedFont("Inner", inner)
	r.ParseMarkdown(`<span style="font-family: Outer">a <span style="font-family: Inner">b</span> c</span>`)

	assert.Equal(t, inner, mdFontOfText(t, r, "b"), "innermost span wins")
	assert.Equal(t, outer, mdFontOfText(t, r, "a"), "outer span text uses outer font")
	assert.Equal(t, outer, mdFontOfText(t, r, "c"), "text after inner close returns to outer")
}

// TestRichText_NamedFont_MarkdownInlineSpanUnbalanced guards that a stray close
// tag (or an unclosed span) neither panics nor leaks the font past the run.
func TestRichText_NamedFont_MarkdownInlineSpanUnbalanced(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)

	assert.NotPanics(t, func() {
		r.ParseMarkdown(`stray </span> close`)
	})
	assert.Nil(t, mdFontOfText(t, r, "stray"))
}

// TestRichText_RegisterNamedFont_NilRemoves confirms registering nil removes an
// entry and that empty names are ignored.
func TestRichText_RegisterNamedFont_NilRemoves(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	require.Equal(t, src, r.lookupNamedFont("myfont"))

	r.RegisterNamedFont("MyFont", nil)
	assert.Nil(t, r.lookupNamedFont("myfont"))

	r.RegisterNamedFont("", src) // no-op
	assert.Nil(t, r.lookupNamedFont(""))
}

// TestRichText_NamedFont_MarkdownHeadingInlineSpan is the #870 fix: a bare inline
// <span style="font-family: …"> inside a markdown heading now resolves the named
// font on the wrapped text, and the heading styling (level, bold) still applies.
func TestRichText_NamedFont_MarkdownHeadingInlineSpan(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`# Title <span style="font-family: MyFont">styled</span>`)

	var found *TextSegment
	for _, seg := range r.Segments {
		if ts, ok := seg.(*TextSegment); ok && strings.Contains(ts.Text, "styled") {
			found = ts
		}
	}
	require.NotNil(t, found, "styled heading run not found")
	assert.Equal(t, src, found.Style.FontSource, "named font resolves inside the heading")
	assert.Equal(t, 1, found.Style.HeadingLevel, "heading level still applied")
	assert.True(t, found.Style.TextStyle.Bold, "heading bold still applied")
}

// TestRichText_NamedFont_MarkdownHeadingPlainTextUnstyled confirms heading text
// outside any span stays at the default font (no leakage), and a heading with no
// span is unchanged.
func TestRichText_NamedFont_MarkdownHeadingPlainTextUnstyled(t *testing.T) {
	src := distinctFont()
	r := NewRichText()
	r.RegisterNamedFont("MyFont", src)
	r.ParseMarkdown(`## Plain <span style="font-family: MyFont">fancy</span> tail`)

	for _, seg := range r.Segments {
		ts, ok := seg.(*TextSegment)
		if !ok {
			continue
		}
		switch {
		case strings.Contains(ts.Text, "fancy"):
			assert.Equal(t, src, ts.Style.FontSource, "spanned run uses the font")
		case strings.TrimSpace(ts.Text) != "":
			assert.Nil(t, ts.Style.FontSource, "unspanned heading text %q stays default", ts.Text)
		}
		assert.Equal(t, 2, ts.Style.HeadingLevel, "all runs keep the heading level")
	}
}
