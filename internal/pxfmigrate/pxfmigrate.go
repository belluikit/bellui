// Package pxfmigrate holds what Bellui's own formats that moved from JSON to PXF
// share while they still read the JSON (#523): telling the two apart, and
// writing only strings PXF can read back.
package pxfmigrate

import (
	"strings"
	"unicode/utf8"
)

// IsJSON reports whether data is a JSON document that opens with open: '{' for
// an object, '[' for an array. A PXF message starts with a field name, so the
// first byte past JSON whitespace tells the two apart without relying on either
// decoder's errors.
func IsJSON(data []byte, open byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b == open
		}
	}
	return false
}

// ValidUTF8 returns s with each byte that is not part of valid UTF-8 replaced by
// U+FFFD, as encoding/json writes it. PXF writes such a string in a string field
// but refuses to read it back, so a format that moved from JSON sanitises the
// strings it writes the way JSON did, rather than writing data it cannot load.
func ValidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2*utf8.UTFMax)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
