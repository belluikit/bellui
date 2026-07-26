package widget

import (
	"image"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestHyperlinkSegment_Style_Propagates verifies HyperlinkSegment.Visual and
// Update thread the per-segment Style (#651) onto the backing Hyperlink widget,
// so a link can render in a custom font/color/size like a TextSegment.
func TestHyperlinkSegment_Style_Propagates(t *testing.T) {
	src := distinctFont()
	seg := &HyperlinkSegment{
		Text: "link",
		URL:  parseTestURL(t, "https://example.com"),
		Style: RichTextStyle{
			FontSource: src,
			ColorName:  theme.ColorNameError,
			SizeName:   theme.SizeNameHeadingText,
			TextStyle:  bellui.TextStyle{Bold: true},
		},
	}

	link := seg.Visual().(*bellui.Container).Objects[0].(*Hyperlink)
	assert.Equal(t, src, link.FontSource)
	assert.Equal(t, theme.ColorNameError, link.ColorName)
	assert.Equal(t, theme.SizeNameHeadingText, link.SizeName)
	assert.True(t, link.TextStyle.Bold)

	// Update (the reuse path) must apply the same style to an existing visual.
	existing := seg.Visual().(*bellui.Container)
	// mutate the segment, then Update the previously-built visual.
	seg.Style.ColorName = theme.ColorNamePrimary
	seg.Update(existing)
	assert.Equal(t, theme.ColorNamePrimary, existing.Objects[0].(*Hyperlink).ColorName)
}

// TestHyperlinkSegment_Style_NilIsByteIdentical guards the zero-value contract:
// a HyperlinkSegment with an empty Style lays out and renders exactly like one
// that never mentions the field.
func TestHyperlinkSegment_Style_NilIsByteIdentical(t *testing.T) {
	test.NewTempApp(t)

	u := parseTestURL(t, "https://example.com")
	withField := NewRichText(&HyperlinkSegment{Text: "Same", URL: u, Style: RichTextStyle{}})
	without := NewRichText(&HyperlinkSegment{Text: "Same", URL: u})

	assert.Equal(t, without.MinSize(), withField.MinSize())

	wA := test.NewWindow(without)
	wA.Resize(without.MinSize())
	wB := test.NewWindow(withField)
	wB.Resize(withField.MinSize())
	assert.Equal(t, wA.Canvas().Capture().(*image.NRGBA).Pix, wB.Canvas().Capture().(*image.NRGBA).Pix)
}

// TestHyperlinkSegment_FontSource_AffectsMeasurement proves the hyperlink
// measure path (which flows through the Hyperlink widget's MinSize) is
// source-aware: a distinctive font changes the segment's natural width, so
// layout agrees with what the render path draws.
func TestHyperlinkSegment_FontSource_AffectsMeasurement(t *testing.T) {
	test.NewTempApp(t)

	u := parseTestURL(t, "https://example.com")
	plain := NewRichText(&HyperlinkSegment{Text: "Measure me", URL: u})
	custom := NewRichText(&HyperlinkSegment{
		Text: "Measure me", URL: u,
		Style: RichTextStyle{FontSource: distinctFont()},
	})

	assert.NotEqual(t, plain.MinSize().Width, custom.MinSize().Width,
		"custom font must change the measured width of a link segment")
}

// TestHyperlink_ColorName_Override verifies the Hyperlink widget honours an
// explicit ColorName for its text (and leaves the historical primary default
// intact when unset).
func TestHyperlink_ColorName_Override(t *testing.T) {
	test.NewTempApp(t)

	link := NewHyperlink("colored", parseTestURL(t, "https://example.com"))
	link.CreateRenderer()
	link.Refresh()
	th := link.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	got := richTextRenderTexts(&link.provider)[0].Color
	assert.Equal(t, th.Color(theme.ColorNamePrimary, v), got, "default link uses primary")

	link.ColorName = theme.ColorNameError
	link.Refresh()
	got = richTextRenderTexts(&link.provider)[0].Color
	assert.Equal(t, th.Color(theme.ColorNameError, v), got, "override recolors the link text")
}

// TestHyperlink_FontSource_Propagates checks the widget field reaches the
// rendered canvas.Text and its measurement.
func TestHyperlink_FontSource_Propagates(t *testing.T) {
	test.NewTempApp(t)

	src := distinctFont()
	link := NewHyperlink("Rendered", parseTestURL(t, "https://example.com"))
	link.CreateRenderer()
	plainWidth := link.MinSize().Width

	link.FontSource = src
	link.Refresh()
	assert.Equal(t, src, richTextRenderTexts(&link.provider)[0].FontSource)
	assert.NotEqual(t, plainWidth, link.MinSize().Width, "custom font changes the link's min width")
}

func parseTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
