package widget

import "unicode"

// The punctuation a CJK line may not start or end with, for word wrap's
// fallback break (#1300). Chinese and Japanese have no spaces, so nearly every
// break in them is that fallback, and it fell wherever the line overflowed:
// "、Choro、昆比亚" started a line with an ideographic comma.
//
// The tables are the UAX #14 classes of the CJK Symbols and Punctuation,
// Hiragana, Katakana, Vertical Forms, CJK Compatibility Forms, Small Form
// Variants and Halfwidth and Fullwidth Forms blocks, from LineBreak-18.0.0.txt:
// CL, CP, NS and EX may not start a line (UAX #14 LB13, LB16, LB21) and OP may
// not end one (LB14). The curly quotes are QU there, which UAX #14 leaves to
// context; Chinese uses ‘“ as opening and ’” as closing marks, so they are
// listed by hand. Latin punctuation is left out: outside CJK text the fallback
// breaks only a word wider than the line, and that stays as it was.

// cjkNoLineStart holds the closing, non-starter and exclamation marks a line
// may not start with: 、。，．）」』】〕！？：； and their kin.
var cjkNoLineStart = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x2019, Hi: 0x2019, Stride: 1},
		{Lo: 0x201D, Hi: 0x201D, Stride: 1},
		{Lo: 0x3001, Hi: 0x3002, Stride: 1},
		{Lo: 0x3005, Hi: 0x3005, Stride: 1},
		{Lo: 0x3009, Hi: 0x3009, Stride: 1},
		{Lo: 0x300B, Hi: 0x300B, Stride: 1},
		{Lo: 0x300D, Hi: 0x300D, Stride: 1},
		{Lo: 0x300F, Hi: 0x300F, Stride: 1},
		{Lo: 0x3011, Hi: 0x3011, Stride: 1},
		{Lo: 0x3015, Hi: 0x3015, Stride: 1},
		{Lo: 0x3017, Hi: 0x3017, Stride: 1},
		{Lo: 0x3019, Hi: 0x3019, Stride: 1},
		{Lo: 0x301B, Hi: 0x301C, Stride: 1},
		{Lo: 0x301E, Hi: 0x301F, Stride: 1},
		{Lo: 0x303B, Hi: 0x303C, Stride: 1},
		{Lo: 0x309B, Hi: 0x309E, Stride: 1},
		{Lo: 0x30A0, Hi: 0x30A0, Stride: 1},
		{Lo: 0x30FB, Hi: 0x30FB, Stride: 1},
		{Lo: 0x30FD, Hi: 0x30FE, Stride: 1},
		{Lo: 0xFE10, Hi: 0xFE16, Stride: 1},
		{Lo: 0xFE18, Hi: 0xFE18, Stride: 1},
		{Lo: 0xFE36, Hi: 0xFE36, Stride: 1},
		{Lo: 0xFE38, Hi: 0xFE38, Stride: 1},
		{Lo: 0xFE3A, Hi: 0xFE3A, Stride: 1},
		{Lo: 0xFE3C, Hi: 0xFE3C, Stride: 1},
		{Lo: 0xFE3E, Hi: 0xFE3E, Stride: 1},
		{Lo: 0xFE40, Hi: 0xFE40, Stride: 1},
		{Lo: 0xFE42, Hi: 0xFE42, Stride: 1},
		{Lo: 0xFE44, Hi: 0xFE44, Stride: 1},
		{Lo: 0xFE48, Hi: 0xFE48, Stride: 1},
		{Lo: 0xFE50, Hi: 0xFE52, Stride: 1},
		{Lo: 0xFE54, Hi: 0xFE57, Stride: 1},
		{Lo: 0xFE5A, Hi: 0xFE5A, Stride: 1},
		{Lo: 0xFE5C, Hi: 0xFE5C, Stride: 1},
		{Lo: 0xFE5E, Hi: 0xFE5E, Stride: 1},
		{Lo: 0xFF01, Hi: 0xFF01, Stride: 1},
		{Lo: 0xFF09, Hi: 0xFF09, Stride: 1},
		{Lo: 0xFF0C, Hi: 0xFF0C, Stride: 1},
		{Lo: 0xFF0E, Hi: 0xFF0E, Stride: 1},
		{Lo: 0xFF1A, Hi: 0xFF1B, Stride: 1},
		{Lo: 0xFF1F, Hi: 0xFF1F, Stride: 1},
		{Lo: 0xFF3D, Hi: 0xFF3D, Stride: 1},
		{Lo: 0xFF5D, Hi: 0xFF5D, Stride: 1},
		{Lo: 0xFF60, Hi: 0xFF61, Stride: 1},
		{Lo: 0xFF63, Hi: 0xFF65, Stride: 1},
		{Lo: 0xFF9E, Hi: 0xFF9F, Stride: 1},
	},
}

// cjkNoLineEnd holds the opening marks a line may not end with: （「『【〔“‘.
var cjkNoLineEnd = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x2018, Hi: 0x2018, Stride: 1},
		{Lo: 0x201C, Hi: 0x201C, Stride: 1},
		{Lo: 0x3008, Hi: 0x3008, Stride: 1},
		{Lo: 0x300A, Hi: 0x300A, Stride: 1},
		{Lo: 0x300C, Hi: 0x300C, Stride: 1},
		{Lo: 0x300E, Hi: 0x300E, Stride: 1},
		{Lo: 0x3010, Hi: 0x3010, Stride: 1},
		{Lo: 0x3014, Hi: 0x3014, Stride: 1},
		{Lo: 0x3016, Hi: 0x3016, Stride: 1},
		{Lo: 0x3018, Hi: 0x3018, Stride: 1},
		{Lo: 0x301A, Hi: 0x301A, Stride: 1},
		{Lo: 0x301D, Hi: 0x301D, Stride: 1},
		{Lo: 0xFE17, Hi: 0xFE17, Stride: 1},
		{Lo: 0xFE35, Hi: 0xFE35, Stride: 1},
		{Lo: 0xFE37, Hi: 0xFE37, Stride: 1},
		{Lo: 0xFE39, Hi: 0xFE39, Stride: 1},
		{Lo: 0xFE3B, Hi: 0xFE3B, Stride: 1},
		{Lo: 0xFE3D, Hi: 0xFE3D, Stride: 1},
		{Lo: 0xFE3F, Hi: 0xFE3F, Stride: 1},
		{Lo: 0xFE41, Hi: 0xFE41, Stride: 1},
		{Lo: 0xFE43, Hi: 0xFE43, Stride: 1},
		{Lo: 0xFE47, Hi: 0xFE47, Stride: 1},
		{Lo: 0xFE59, Hi: 0xFE59, Stride: 1},
		{Lo: 0xFE5B, Hi: 0xFE5B, Stride: 1},
		{Lo: 0xFE5D, Hi: 0xFE5D, Stride: 1},
		{Lo: 0xFF08, Hi: 0xFF08, Stride: 1},
		{Lo: 0xFF3B, Hi: 0xFF3B, Stride: 1},
		{Lo: 0xFF5B, Hi: 0xFF5B, Stride: 1},
		{Lo: 0xFF5F, Hi: 0xFF5F, Stride: 1},
		{Lo: 0xFF62, Hi: 0xFF62, Stride: 1},
	},
}

// cjkLineBreak moves a fallback break at index i of text, so the row before
// it does not end with an opening mark and the row from it does not start with
// a closing one. It moves back first, carrying the character before the
// punctuation down with it, as CSS line-break: normal does. When no point in
// (0, i] will do, as when a row holds a single ideograph, it moves forward to
// the first point that will, or to len(text): the row then runs past the
// width, as a browser lets it, rather than start with "、".
func cjkLineBreak(text []rune, i int) int {
	if i >= len(text) {
		return len(text) // the row already ends the text
	}
	for j := i; j > 0; j-- {
		if cjkBreakAllowed(text, j) {
			return j
		}
	}
	for j := i + 1; j < len(text); j++ {
		if cjkBreakAllowed(text, j) {
			return j
		}
	}
	return len(text)
}

// cjkBreakAllowed reports whether text may break before index j, 0 < j < len(text).
func cjkBreakAllowed(text []rune, j int) bool {
	return !unicode.Is(cjkNoLineStart, text[j]) && !unicode.Is(cjkNoLineEnd, text[j-1])
}
