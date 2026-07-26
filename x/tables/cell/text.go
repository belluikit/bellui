package cell

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
)

// TextRenderer is the default cell renderer that displays values as canvas.Text.
// Using canvas.Text (not widget.Label) because canvas primitives render correctly
// when placed directly into another widget renderer's Objects() slice.
type TextRenderer struct {
	// Alignment controls the text alignment within the cell.
	Alignment bellui.TextAlign
	// TextStyle controls bold, italic, monospace.
	TextStyle bellui.TextStyle
}

// NewTextRenderer creates a TextRenderer with left alignment.
func NewTextRenderer() *TextRenderer {
	return &TextRenderer{
		Alignment: bellui.TextAlignLeading,
	}
}

// CreateCell returns a canvas.Text for displaying text.
func (r *TextRenderer) CreateCell() core.CanvasObject {
	t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
	t.Alignment = r.Alignment
	t.TextStyle = r.TextStyle
	t.TextSize = theme.TextSize()
	return t
}

// UpdateCell sets the text content and refreshes the theme color.
// Alignment is set every time because cells may be recycled from columns
// with different alignment (e.g. a right-aligned Number cell reused for text).
func (r *TextRenderer) UpdateCell(obj core.CanvasObject, params Params) {
	t := obj.(*canvas.Text)
	ts := theme.TextSize()
	text := formatValue(params.Value)
	if params.CellWidth > 0 {
		text = TruncateText(text, params.CellWidth, ts, r.TextStyle)
	}
	t.Text = text
	t.Alignment = r.Alignment
	t.TextStyle = r.TextStyle
	t.Color = theme.Color(theme.ColorNameForeground)
	t.TextSize = ts
	t.Refresh()
}

// TextForValue returns the plain text a screen reader should announce — the same
// value UpdateCell shows, without width truncation (AccessibleValueTexter). For
// text this matches the generic AccessibleText fallback; it is provided so the
// built-in renderers uniformly satisfy the interface.
func (r *TextRenderer) TextForValue(v any) string {
	return formatValue(v)
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// AccessibleText renders a raw cell value to the plain string a screen reader
// should announce, with no width-based truncation. Columns with a
// ValueFormatter use that instead; this is the fallback for the default
// renderers (mirrors formatValue).
func AccessibleText(v any) string {
	return formatValue(v)
}
