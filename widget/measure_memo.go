package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// measureMemo keeps one bellui.MeasureText result and everything it depends
// on. A widget that asks for the same measurement on every min-size pass then
// skips the global font metrics lookup, which hashes the string, size and
// style each time (#492, #487).
type measureMemo struct {
	key  measureKey
	size geo.Size
	ok   bool
}

// measureKey is the text and size, the style as the font metrics cache keys
// it (pointer fields resolved to values), and the cache generation, which
// changes whenever fonts or the theme do.
type measureKey struct {
	text                            string
	size                            float32
	bold, italic, monospace, symbol bool
	underline, strikethrough        bool
	tabWidth                        int
	weight                          bellui.TextWeight
	lineHeightScale, letterSpacing  float32
	direction                       bellui.TextDirection
	language                        string
	generation                      uint64
}

// measure returns bellui.MeasureText(text, textSize, style), measuring again
// only when one of its inputs changed.
func (m *measureMemo) measure(text string, textSize float32, style bellui.TextStyle) geo.Size {
	key := measureKey{
		text: text, size: textSize,
		bold: style.Bold, italic: style.Italic, monospace: style.Monospace, symbol: style.Symbol,
		underline: style.Underline, strikethrough: style.Strikethrough,
		tabWidth:        style.TabWidth,
		weight:          style.ResolvedWeight(),
		lineHeightScale: style.ResolvedLineHeightScale(),
		letterSpacing:   style.ResolvedLetterSpacing(),
		direction:       style.Direction,
		language:        style.Language,
		generation:      cache.ThemeCacheGeneration(),
	}
	if m.ok && m.key == key {
		return m.size
	}
	m.size = bellui.MeasureText(text, textSize, style)
	m.key, m.ok = key, true
	return m.size
}
