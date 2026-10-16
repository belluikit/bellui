package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// TestRichText_VarHeight_MixedMarkdownWindows verifies the variable-height
// multi-segment virtualisation (pre-rebrand tracker #158) end-to-end on genuinely MIXED content —
// headings, paragraphs, blockquotes, syntax-highlighted code blocks, GFM tables
// and lists, which have differing per-row heights (the existing varheight tests
// use uniform text segments). A large markdown RichText with Virtualize opted in
// must window its rows (bounded object count, not one per row) and re-window to
// deep rows on scroll. Closes the pre-rebrand tracker #168 verification.
func TestRichText_VarHeight_MixedMarkdownWindows(t *testing.T) {
	test.NewTempApp(t)

	var b strings.Builder
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&b, "## Section %d\n\nParagraph %d with **bold**, *italic* and `code` that wraps to vary the row height.\n\n", i, i)
		if i%5 == 0 {
			b.WriteString("> blockquote row\n\n```go\nfunc demo() { fmt.Println(\"code\") }\n```\n\n")
		}
		if i%7 == 0 {
			b.WriteString("| A | B |\n|---|---|\n| 1 | 2 |\n\n")
		}
		fmt.Fprintf(&b, "- item %d-a\n- item %d-b\n\n", i, i)
	}
	rich := NewRichTextFromMarkdown(b.String())
	rich.Wrapping = bellui.TextWrapWord
	rich.Scroll = bellui.ScrollVerticalOnly
	rich.Virtualize = true
	rich.Resize(geo.NewSize(700, 800))

	r := cache.Renderer(rich).(*textRenderer)
	rich.updateRowBounds()
	r.MinSize()
	r.Refresh()
	r.Layout(geo.NewSize(700, 800))

	require.True(t, rich.virtualize, "scrollable RichText opted into virtualisation")
	require.True(t, rich.blockLazy, "mixed multi-segment content uses the windowed block build (#317)")
	totalRows := rich.rows()
	require.Greater(t, totalRows, 2000, "want a large mixed document")
	require.NotEmpty(t, rich.winRows)

	assert.Less(t, rich.winRows[0], 40, "initial window near the top")
	assert.Less(t, len(r.origObjs), 400,
		"virtualised: object count bounded (got %d for %d rows)", len(r.origObjs), totalRows)

	// Scroll ~80% down and re-window via the internal scroll.
	deepRow := totalRows * 8 / 10
	deepY, _ := rich.RowGeometry(deepRow)
	rich.scr.ScrollToOffset(geo.NewPos(0, deepY))
	rich.updateVirtualViewport()
	r.Refresh()
	r.Layout(geo.NewSize(700, 800))

	assert.Greater(t, rich.winRows[0], deepRow-200, "deep scroll materialises deep rows")
	assert.Less(t, len(r.origObjs), 400,
		"object count stays bounded after deep scroll (got %d)", len(r.origObjs))
}
