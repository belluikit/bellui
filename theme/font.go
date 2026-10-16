package theme

import (
	"github.com/belluikit/bellui"
)

// DefaultEmojiFont returns the font resource for the built-in emoji font.
// This may return nil if the application was packaged without an emoji font.
//
// Since: 1.0
func DefaultEmojiFont() bellui.Resource {
	return emoji
}

// DefaultSymbolsFallbackFonts returns the bundled symbol fallback faces
// consulted, after the theme and emoji fonts, for glyphs the primary fonts do
// not cover — common UI symbols such as ✓, ☐, arrows, ⌘ and geometric shapes.
// Bundling them keeps those glyphs' metrics identical on every platform
// instead of falling back to a host system font. This returns nil when the
// application was packaged with the no_symbols build tag.
func DefaultSymbolsFallbackFonts() []bellui.Resource {
	return symbolsFallbackFonts
}

// DefaultTextBoldFont returns the font resource for the built-in bold font style.
func DefaultTextBoldFont() bellui.Resource {
	return bold
}

// DefaultTextBoldItalicFont returns the font resource for the built-in bold and italic font style.
func DefaultTextBoldItalicFont() bellui.Resource {
	return bolditalic
}

// DefaultTextFont returns the font resource for the built-in regular font style.
func DefaultTextFont() bellui.Resource {
	return regular
}

// DefaultTextItalicFont returns the font resource for the built-in italic font style.
func DefaultTextItalicFont() bellui.Resource {
	return italic
}

// DefaultTextMonospaceFont returns the font resource for the built-in monospace font face.
func DefaultTextMonospaceFont() bellui.Resource {
	return monospace
}

// DefaultSymbolFont returns the font resource for the built-in symbol font.
//
// Since: 1.0
func DefaultSymbolFont() bellui.Resource {
	return symbol
}

// Font looks up the font for current theme and text style.
//
// Since: 1.0
func Font(style bellui.TextStyle) bellui.Resource {
	return safeFontLookup(style)
}

// TextBoldFont returns the font resource for the bold font style.
func TextBoldFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{Bold: true})
}

// TextBoldItalicFont returns the font resource for the bold and italic font style.
func TextBoldItalicFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{Bold: true, Italic: true})
}

// TextFont returns the font resource for the regular font style.
func TextFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{})
}

// TextItalicFont returns the font resource for the italic font style.
func TextItalicFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{Italic: true})
}

// TextMonospaceFont returns the font resource for the monospace font face.
func TextMonospaceFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{Monospace: true})
}

// SymbolFont returns the font resource for the symbol font style.
//
// Since: 1.0
func SymbolFont() bellui.Resource {
	return safeFontLookup(bellui.TextStyle{Symbol: true})
}

func safeFontLookup(s bellui.TextStyle) bellui.Resource {
	font := Current().Font(s)
	if font != nil {
		return font
	}
	bellui.LogError("Loaded theme returned nil font", nil)

	if s.Monospace {
		return DefaultTextMonospaceFont()
	}
	if s.Bold {
		if s.Italic {
			return DefaultTextBoldItalicFont()
		}
		return DefaultTextBoldFont()
	}
	if s.Italic {
		return DefaultTextItalicFont()
	}
	if s.Symbol {
		return DefaultSymbolFont()
	}

	return DefaultTextFont()
}
