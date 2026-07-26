// Package cjk bundles a Noto Sans CJK fallback font and registers it as a global
// font fallback, so Chinese/Japanese/Korean text renders without each app having
// to supply its own font — on every platform, including WebAssembly, where
// system fonts cannot be scanned.
//
// Import it for its side effect to enable CJK everywhere:
//
//	import _ "github.com/belluikit/bellui/theme/cjk"
//
// or call [Register] explicitly. The bundled face is a ~6.5 MB subset of Noto
// Sans CJK SC covering common Han (GB2312 / Shift-JIS / EUC-KR), Hiragana,
// Katakana, common Hangul and CJK punctuation; it is only linked into your
// binary when this package is imported. For glyphs outside the subset, register
// your own fuller font with [bellui.RegisterFontFallback].
//
// Noto Sans CJK is licensed under the SIL Open Font License 1.1.
package cjk

import (
	_ "embed"
	"sync"

	"github.com/belluikit/bellui"
)

//go:embed NotoSansCJKsc-bellui.otf
var notoSansCJK []byte

// Font is the bundled Noto Sans CJK (Simplified Chinese) subset.
var Font = &bellui.StaticResource{
	StaticName:    "NotoSansCJKsc-bellui.otf",
	StaticContent: notoSansCJK,
}

var once sync.Once

// Register adds the bundled CJK font to the global font fallback chain. It is
// idempotent and runs automatically when the package is imported.
func Register() {
	once.Do(func() { bellui.RegisterFontFallback(Font) })
}

func init() { Register() }
