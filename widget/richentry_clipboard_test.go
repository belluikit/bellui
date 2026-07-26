package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// --- serializer / flatten units ---

func TestSegsToHTML(t *testing.T) {
	segs := []RichTextSegment{
		&TextSegment{Style: boldStyle(), Text: "bold"},
		&TextSegment{Style: normalStyle(), Text: " and "},
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}}, Text: "it"},
		&TextSegment{Style: normalStyle(), Text: "\nnext"},
	}
	assert.Equal(t, "<b>bold</b> and <i>it</i><br>next", segsToHTML(segs))
	// atoms are omitted from HTML
	assert.Equal(t, "", segsToHTML([]RichTextSegment{imgSeg()}))
	// escaping
	assert.Equal(t, "a&lt;b&gt;", segsToHTML([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "a<b>"}}))
}

func TestFlattenHTMLToInline(t *testing.T) {
	segs := flattenHTMLToInline(NewRichTextFromHTML("<p>one</p><p>two</p>").Segments)
	// paragraph break becomes a newline; trailing break trimmed
	out := ""
	for _, s := range segs {
		out += s.(*TextSegment).Text
	}
	assert.Equal(t, "one\ntwo", out)
}

// --- copy → paste round-trips through the flavor-aware clipboard ---

func TestRichEntry_CopyPasteStyledText(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	src := NewRichEntry()
	src.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		&TextSegment{Style: boldStyle(), Text: "cd"},
	})
	w := test.NewWindow(src)
	defer w.Close()
	src.selectAll()
	src.copyToClipboard(cb)
	// copy writes correct HTML (renders in any app)
	html, _ := cb.(bellui.HTMLClipboard).HTML()
	assert.Equal(t, "ab<b>cd</b>", html)

	dst := NewRichEntry()
	w2 := test.NewWindow(dst)
	defer w2.Close()
	dst.caret, dst.anchor = 0, 0
	dst.pasteFromClipboard(cb)

	assert.Equal(t, "abcd", dst.String())
	assert.Equal(t, []string{"ab", "cd"}, segTexts(dst.Segments()))
	assert.True(t, styleIsBold(dst.Segments()[1].(*TextSegment).Style.TextStyle), "bold preserved through copy/paste")
}

// TestRichEntry_HTMLRoundTripPreservesBoundaryWhitespace verifies a single space
// at a *style boundary* survives an HTML copy/paste round-trip now that the HTML
// parser preserves inline boundary whitespace (#668). Previously the parser
// trimmed each inline text node and "x <b>y</b>" collapsed to "xy".
func TestRichEntry_HTMLRoundTripPreservesBoundaryWhitespace(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	src := NewRichEntry()
	src.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "x "},
		&TextSegment{Style: boldStyle(), Text: "y"},
	})
	w := test.NewWindow(src)
	defer w.Close()
	src.selectAll()
	src.copyToClipboard(cb)
	html, _ := cb.(bellui.HTMLClipboard).HTML()
	assert.Equal(t, "x <b>y</b>", html, "written HTML keeps the space")

	dst := NewRichEntry()
	w2 := test.NewWindow(dst)
	defer w2.Close()
	dst.pasteFromClipboard(cb)
	assert.Equal(t, "x y", dst.String(), "boundary space now survives re-parse")
}

func TestRichEntry_CopyPasteImage(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	src := NewRichEntry()
	src.SetSegments([]RichTextSegment{imgSeg()})
	w := test.NewWindow(src)
	defer w.Close()
	src.selectAll()
	src.copyToClipboard(cb)
	if img, ok := cb.(bellui.ImageClipboard).Image(); assert.True(t, ok) {
		assert.NotNil(t, img, "single-image selection goes to the image flavor")
	}

	dst := NewRichEntry()
	dst.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "ab"}})
	w2 := test.NewWindow(dst)
	defer w2.Close()
	dst.caret, dst.anchor = 1, 1
	dst.pasteFromClipboard(cb)
	assert.Equal(t, []string{"T:a", "IMG", "T:b"}, docSegKinds(dst.Segments()))
}

func TestRichEntry_PastePlainFallback(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("just text") // no HTML/image flavor

	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.caret, e.anchor = 0, 0
	e.pasteFromClipboard(cb)
	assert.Equal(t, "just text", e.String())
}

func TestRichEntry_RichPasteIsOneUndoStep(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()
	cb.(bellui.HTMLClipboard).SetHTML("<b>hi</b><i>yo</i>", "hiyo")

	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "X"}})
	w := test.NewWindow(e)
	defer w.Close()
	e.caret, e.anchor = 1, 1
	e.pasteFromClipboard(cb)
	assert.Equal(t, "Xhiyo", e.String())
	assert.Equal(t, []string{"X", "hi", "yo"}, segTexts(e.Segments()))

	e.Undo() // the whole paste reverts in one step
	assert.Equal(t, "X", e.String())
	assert.Equal(t, 1, e.caret)
}

func TestRichEntry_CutWritesRichAndDeletes(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "keep "},
		&TextSegment{Style: boldStyle(), Text: "cut"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	e.selecting, e.anchor, e.caret = true, 5, 8 // "cut"
	e.cutToClipboard(cb)
	assert.Equal(t, "keep ", e.String())
	html, ok := cb.(bellui.HTMLClipboard).HTML()
	assert.True(t, ok)
	assert.Equal(t, "<b>cut</b>", html)
}
