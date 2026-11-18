package painter

import (
	"unicode"

	"github.com/belluikit/typesetting/font"
	"github.com/belluikit/typesetting/shaping"
)

//go:generate go run gen_emoji.go

const (
	textPresentationSelector  = '︎'
	emojiPresentationSelector = '️'
)

// withPresentation wraps faces for segmenting text, so that a variation
// selector after a rune picks its presentation: U+FE0E asks for text, U+FE0F
// for emoji (UTS #51). Fontmap.ResolveFace sees one rune and never the
// selectors, which the segmenter does not resolve a face for, so the wrapper
// reads them from text itself (#1260).
//
// The segmenter resolves runes in logical order, so the next occurrence of r
// at or after the last one resolved is the rune being asked about. A rune it
// cannot place gets its default presentation.
func withPresentation(faces shaping.Fontmap, text []rune) shaping.Fontmap {
	d, ok := faces.(*dynamicFontMap)
	if !ok || d.emoji == nil || !hasPresentationSelector(text) {
		return faces
	}
	return &presentationFontMap{faces: d, text: text}
}

func hasPresentationSelector(text []rune) bool {
	for _, r := range text {
		if r == textPresentationSelector || r == emojiPresentationSelector {
			return true
		}
	}
	return false
}

type presentationFontMap struct {
	faces *dynamicFontMap
	text  []rune
	next  int // index after the last rune resolved
}

func (p *presentationFontMap) ResolveFace(r rune) *font.Face {
	asText := !unicode.Is(emojiPresentation, r)
	for i := p.next; i < len(p.text); i++ {
		if p.text[i] != r {
			continue
		}
		p.next = i + 1
		if i+1 < len(p.text) {
			switch p.text[i+1] {
			case textPresentationSelector:
				asText = true
			case emojiPresentationSelector:
				asText = false
			}
		}
		break
	}
	return p.faces.resolveFace(r, asText)
}
