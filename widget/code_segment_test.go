package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func codeBlockRenderer(t *testing.T, code string) core.WidgetRenderer {
	t.Helper()
	seg := &CodeSegment{Text: code, Language: "go"}
	cb := seg.Visual().(*richCodeBlock)
	return test.TempWidgetRenderer(t, cb)
}

func TestCodeSegment_MultiLineLayout(t *testing.T) {
	test.NewTempApp(t)

	one := codeBlockRenderer(t, "x := 1")
	multi := codeBlockRenderer(t, "x := 1\ny := 2\nz := 3")

	assert.NotEmpty(t, one.Objects(), "CreateRenderer builds the token text objects")
	assert.Greater(t, multi.MinSize().Height, one.MinSize().Height,
		"a multi-line block is taller than a single-line one")
}

func TestCodeSegment_SyntaxColorVariants(t *testing.T) {
	test.NewTempApp(t)

	light := syntaxColor(tokenKeyword, theme.VariantLight)
	dark := syntaxColor(tokenKeyword, theme.VariantDark)

	assert.Equal(t, syntaxLightColors[tokenKeyword], light, "light variant uses the light palette")
	assert.Equal(t, syntaxDarkColors[tokenKeyword], dark, "dark variant uses the dark palette")
	assert.NotEqual(t, light, dark, "light and dark keyword colours differ")

	// An unmapped token kind falls back to the theme foreground colour.
	assert.Equal(t, theme.Color(theme.ColorNameForeground), syntaxColor(tokenText, theme.VariantLight))
}

func TestCodeSegment_SegmentBasics(t *testing.T) {
	seg := &CodeSegment{Text: "package main", Language: "go"}
	assert.False(t, seg.Inline(), "code blocks are block-level")
	assert.Equal(t, "package main", seg.Textual())
}
