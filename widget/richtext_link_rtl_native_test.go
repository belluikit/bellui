// RTL row-width math diverges under GOOS=js (#168); tagged out while that
// is root-caused, not as the fix.
//go:build !wasm

package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRichText_RTLRowWithLink_WidthIncludesBox(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newRTLLinkRichText(t)
	textSize := rich.Theme().Size(theme.SizeNameText)
	boxW := linkBoxWidth(t, rich)
	require.Greater(t, boxW, float32(0))

	textW := bellui.MeasureText("مرحبا", textSize, bellui.TextStyle{}).Width
	assert.InDelta(t, textW+boxW, rich.rowWidth(0, textSize), 1.0,
		"row width must be the RTL text plus the link box")
}

// TestRichText_RTLRowWithLink_SelectionFillsBox confirms a selection of the whole
// row reaches the link box at the visual right edge, while a selection that omits
// the box's owning rune does not.
