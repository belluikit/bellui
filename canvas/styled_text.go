package canvas

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*StyledText)(nil)

// TextRun defines a run of text with its own styling properties.
// Fields left at their zero value inherit from the parent StyledText.
//
// Since: 2.7
type TextRun struct {
	Text       string           // The string content of this run
	Color      color.Color      // Color for this run; nil means use StyledText.Color
	TextSize   float32          // Font size for this run; 0 means use StyledText.TextSize
	TextStyle  bellui.TextStyle // Style for this run
	FontSource bellui.Resource  // Custom font resource; nil means use theme font
}

// StyledText describes multi-styled inline text in a Bellui canvas.
// Each run can have independent color, size, style, and font.
// This is a single-line primitive; use widget.RichText for wrapping.
//
// Since: 2.7
type StyledText struct {
	baseObject
	Alignment bellui.TextAlign // The alignment of the text content

	Color    color.Color // The default text draw color (used when a run's Color is nil)
	TextSize float32     // The default font size (used when a run's TextSize is 0)

	Runs []TextRun // The styled text runs
}

// Hide will set this text to not be visible.
func (st *StyledText) Hide() {
	st.baseObject.Hide()
	repaint(st)
}

// MinSize returns the minimum size of this styled text based on font sizes and content.
func (st *StyledText) MinSize() geo.Size {
	if len(st.Runs) == 0 {
		return geo.Size{}
	}

	totalWidth := float32(0)
	maxHeight := float32(0)
	for _, run := range st.Runs {
		if run.Text == "" {
			continue
		}
		size := run.TextSize
		if size == 0 {
			size = st.TextSize
		}
		s, _ := bellui.MeasureTextFull(run.Text, size, run.TextStyle, run.FontSource)
		totalWidth += s.Width
		if s.Height > maxHeight {
			maxHeight = s.Height
		}
	}
	return geo.NewSize(totalWidth, maxHeight)
}

// Move the styled text to a new position, relative to its parent / canvas.
func (st *StyledText) Move(pos geo.Position) {
	if st.Position() == pos {
		return
	}
	st.baseObject.Move(pos)
	repaint(st)
}

// Resize updates the size of this object.
func (st *StyledText) Resize(s geo.Size) {
	if s == st.Size() {
		return
	}
	st.baseObject.Resize(s)
	Refresh(st)
}

// SetMinSize has no effect as the smallest size is based on font sizes and content.
func (st *StyledText) SetMinSize(geo.Size) {
	// no-op
}

// Refresh causes this styled text to be redrawn with its configured state.
func (st *StyledText) Refresh() {
	Refresh(st)
}

// NewStyledText returns a new StyledText with the given runs and default color.
//
// Since: 2.7
func NewStyledText(runs []TextRun, defaultColor color.Color) *StyledText {
	return &StyledText{
		Color:    defaultColor,
		TextSize: theme.TextSize(),
		Runs:     runs,
	}
}
