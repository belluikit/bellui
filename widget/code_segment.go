package widget

import (
	"image/color"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// CodeSegment represents a syntax-highlighted code block within a RichText widget.
// When the language is recognized (go, json, yaml, starlark, protobuf), the code
// is tokenized and rendered with syntax highlighting colors. Otherwise it falls
// back to plain monospace rendering.
type CodeSegment struct {
	Text     string
	Language string
}

// Inline returns false — code blocks are block-level elements.
func (c *CodeSegment) Inline() bool { return false }

// Textual returns the plain-text content of this code block.
func (c *CodeSegment) Textual() string { return c.Text }

// Visual returns a new richCodeBlock widget to render this segment.
func (c *CodeSegment) Visual() core.CanvasObject {
	return newRichCodeBlock(c)
}

// Update applies the current state of this code segment to an existing visual.
func (c *CodeSegment) Update(o core.CanvasObject) {
	cb := o.(*richCodeBlock)
	cb.updateData(c)
}

// Select does nothing for a code block.
func (c *CodeSegment) Select(_, _ geo.Position) {}

// SelectedText returns the empty string for this code block.
func (c *CodeSegment) SelectedText() string { return "" }

// Unselect does nothing for a code block.
func (c *CodeSegment) Unselect() {}

// --- richCodeBlock widget ---

type richCodeBlock struct {
	BaseWidget
	text     string
	language string
	tokens   []codeToken
}

func newRichCodeBlock(seg *CodeSegment) *richCodeBlock {
	cb := &richCodeBlock{}
	cb.ExtendBaseWidget(cb) // before updateData — it calls Refresh
	cb.updateData(seg)
	return cb
}

func (cb *richCodeBlock) updateData(seg *CodeSegment) {
	if cb.text == seg.Text && cb.language == seg.Language {
		return // unchanged — skip expensive re-tokenization
	}
	cb.text = seg.Text
	cb.language = seg.Language
	if lex := codeLexerForLanguage(seg.Language); lex != nil {
		cb.tokens = lex.tokenize(seg.Text)
	} else {
		cb.tokens = []codeToken{{text: seg.Text, kind: tokenText}}
	}
	cb.Refresh()
}

func (cb *richCodeBlock) CreateRenderer() core.WidgetRenderer {
	r := &richCodeBlockRenderer{block: cb}
	r.buildObjects()
	return r
}

// --- renderer ---

type richCodeBlockRenderer struct {
	block   *richCodeBlock
	objects []core.CanvasObject
	minSize geo.Size
}

func (r *richCodeBlockRenderer) buildObjects() {
	textSize := theme.Size(theme.SizeNameText)
	monoStyle := bellui.TextStyle{Monospace: true}
	variant := bellui.CurrentApp().Settings().ThemeVariant()

	r.objects = nil
	lines := splitTokensIntoLines(r.block.tokens)

	lineHeight := bellui.MeasureText("Mg", textSize, monoStyle).Height
	maxWidth := float32(0)
	y := float32(0)

	for _, lineTokens := range lines {
		x := float32(0)
		for _, tok := range lineTokens {
			if tok.text == "" {
				continue
			}
			col := syntaxColor(tok.kind, variant)
			txt := canvas.NewText(tok.text, col)
			txt.TextSize = textSize
			txt.TextStyle = monoStyle
			sz := bellui.MeasureText(tok.text, textSize, monoStyle)
			txt.Move(geo.NewPos(x, y))
			txt.Resize(sz)
			r.objects = append(r.objects, txt)
			x += sz.Width
		}
		if x > maxWidth {
			maxWidth = x
		}
		y += lineHeight
	}
	if y == 0 {
		y = lineHeight // at least one line height for empty blocks
	}
	r.minSize = geo.NewSize(maxWidth, y)
}

func (r *richCodeBlockRenderer) Layout(size geo.Size) {
	// Objects are absolutely positioned in buildObjects; nothing to do.
}

func (r *richCodeBlockRenderer) MinSize() geo.Size {
	return r.minSize
}

func (r *richCodeBlockRenderer) Refresh() {
	r.buildObjects()
	canvas.Refresh(r.block)
}

// Destroy has nothing to release: the renderer holds only canvas.Text objects
// (built from the cached tokens) that the framework's destroy walk tears down.
func (r *richCodeBlockRenderer) Destroy() {}

func (r *richCodeBlockRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *richCodeBlockRenderer) SetObjects(objs []core.CanvasObject) {
	r.objects = objs
}

// splitTokensIntoLines splits a flat list of tokens into per-line groups,
// splitting tokens that span multiple lines.
func splitTokensIntoLines(tokens []codeToken) [][]codeToken {
	var lines [][]codeToken
	var current []codeToken

	for _, tok := range tokens {
		parts := strings.Split(tok.text, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, current)
				current = nil
			}
			if part != "" {
				current = append(current, codeToken{text: part, kind: tok.kind})
			}
		}
	}
	lines = append(lines, current)
	return lines
}

// --- syntax colors ---
// Monokai-inspired palette (matching gameover/cmd/viewer/theme.go).

var syntaxDarkColors = map[tokenKind]color.Color{
	tokenKeyword:     &color.NRGBA{R: 0xF9, G: 0x26, B: 0x72, A: 0xFF}, // pink
	tokenType:        &color.NRGBA{R: 0x66, G: 0xD9, B: 0xEF, A: 0xFF}, // cyan
	tokenBuiltin:     &color.NRGBA{R: 0x66, G: 0xD9, B: 0xEF, A: 0xFF}, // cyan
	tokenString:      &color.NRGBA{R: 0xCE, G: 0x91, B: 0x36, A: 0xFF}, // gold
	tokenNumber:      &color.NRGBA{R: 0xAE, G: 0x81, B: 0xFF, A: 0xFF}, // purple
	tokenComment:     &color.NRGBA{R: 0x75, G: 0x71, B: 0x5E, A: 0xFF}, // grey
	tokenPunctuation: &color.NRGBA{R: 0x66, G: 0xB2, B: 0xFF, A: 0xFF}, // blue
}

var syntaxLightColors = map[tokenKind]color.Color{
	tokenKeyword:     &color.NRGBA{R: 0xAF, G: 0x00, B: 0x5F, A: 0xFF}, // dark magenta
	tokenType:        &color.NRGBA{R: 0x00, G: 0x7A, B: 0xA3, A: 0xFF}, // teal
	tokenBuiltin:     &color.NRGBA{R: 0x00, G: 0x7A, B: 0xA3, A: 0xFF}, // teal
	tokenString:      &color.NRGBA{R: 0x9C, G: 0x57, B: 0x00, A: 0xFF}, // dark orange/brown
	tokenNumber:      &color.NRGBA{R: 0x6C, G: 0x3D, B: 0xC1, A: 0xFF}, // purple
	tokenComment:     &color.NRGBA{R: 0x6A, G: 0x6A, B: 0x6A, A: 0xFF}, // dark grey
	tokenPunctuation: &color.NRGBA{R: 0x1A, G: 0x5F, B: 0xB4, A: 0xFF}, // blue
}

func syntaxColor(kind tokenKind, variant bellui.ThemeVariant) color.Color {
	var palette map[tokenKind]color.Color
	if variant == theme.VariantLight {
		palette = syntaxLightColors
	} else {
		palette = syntaxDarkColors
	}
	if c, ok := palette[kind]; ok {
		return c
	}
	return theme.Color(theme.ColorNameForeground)
}
