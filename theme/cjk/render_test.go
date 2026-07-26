package cjk_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme/cjk"
)

// TestRendersCJK rasterises CJK text with the bundled face through the software
// painter, proving the embedded font actually renders glyphs (not tofu). The
// render is deterministic: the font is bundled and rasterisation is pure Go, so
// the master is stable across platforms.
func TestRendersCJK(t *testing.T) {
	test.NewTempApp(t)

	txt := canvas.NewText("世界 你好 こんにちは 안녕하세요", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	txt.TextSize = 28
	txt.FontSource = cjk.Font

	test.AssertObjectRendersToImage(t, "cjk_render.png", txt)
}
