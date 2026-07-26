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

// TestRichText_StyleHeightCache verifies the per-(size,style) ASCII line-height
// memoisation (#313) keeps the variable-height geometry correct: distinct styles
// (headings vs body) keep distinct row heights — the cache must not collapse them
// — and the height prefix is identical whether built cold or warm (transparent).
func TestRichText_StyleHeightCache(t *testing.T) {
	test.NewTempApp(t)
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "## Heading %d\n\nbody paragraph %d\n\n", i, i)
	}
	rich := NewRichTextFromMarkdown(b.String())
	rich.Wrapping = bellui.TextWrapWord
	rich.Scroll = bellui.ScrollVerticalOnly
	rich.Virtualize = true
	rich.Resize(geo.NewSize(700, 800))
	cache.Renderer(rich)
	rich.updateRowBounds()
	rich.ensureHeightPrefix()

	minH, maxH := float32(1e9), float32(0)
	for i := 0; i < len(rich.rowBounds); i++ {
		if rich.rowHidden(i) {
			continue
		}
		h := rich.rowHeightOrEstimate(i)
		if h <= 0 {
			continue
		}
		if h < minH {
			minH = h
		}
		if h > maxH {
			maxH = h
		}
	}
	assert.Greater(t, maxH, minH+0.5,
		"heading rows stay taller than body rows — the (size,style) cache must not collapse heights to one value")

	// Transparency: a cold rebuild yields the identical prefix.
	before := append([]float32(nil), rich.rowYPrefix...)
	rich.styleHeightCache = nil
	rich.rowHeightCache = nil
	rich.prefixHeightDirty = true
	rich.rowYPrefix = rich.rowYPrefix[:0]
	rich.ensureHeightPrefix()
	require.Equal(t, before, rich.rowYPrefix, "height prefix is cache-transparent / deterministic")
}

func TestIsASCII(t *testing.T) {
	for _, s := range []string{"", "hello world 123", "func x() {}", "\t\n ~"} {
		assert.True(t, isASCII(s), "%q is ASCII", s)
	}
	for _, s := range []string{"héllo", "こんにちは", "你好", "café", "emoji 😀"} {
		assert.False(t, isASCII(s), "%q is not ASCII", s)
	}
}
