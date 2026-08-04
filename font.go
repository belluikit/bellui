package bellui

// fontFallbackRegister is set by internal/painter.init() so that
// RegisterFontFallback can reach the painter's font system without the root
// package importing it.
var fontFallbackRegister func(Resource)

// SetFontFallbackRegister wires the painter's fallback registration in. It is
// called automatically by the painter package on import and is not intended for
// application use.
func SetFontFallbackRegister(fn func(Resource)) {
	fontFallbackRegister = fn
}

// RegisterFontFallback adds res to the global font fallback chain. When the
// active font (and the theme's own fallbacks) lack a glyph for a rune, the
// registered fallback faces are consulted in registration order before any
// system-font scan.
//
// This is the portable way to add coverage for scripts the bundled fonts do not
// include — most commonly CJK. Unlike system-font scanning it works on every
// platform, including WebAssembly, where there are no scannable system fonts, so
// a compiled-in fallback is the only way to render such scripts in the browser.
//
// Register fallbacks once at startup, before building the UI. Registering after
// text has been measured is supported but invalidates the font caches.
//
// Since: 1.0
func RegisterFontFallback(res Resource) {
	if res == nil || fontFallbackRegister == nil {
		return
	}
	fontFallbackRegister(res)
}
