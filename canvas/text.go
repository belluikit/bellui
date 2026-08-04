package canvas

import (
	"image/color"
	"math"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Text)(nil)

// Text describes a text primitive in a Bellui canvas.
// A text object can have a style set which will apply to the whole string.
// No formatting or text parsing will be performed
type Text struct {
	baseObject
	Alignment bellui.TextAlign // The alignment of the text content

	Color     color.Color      // The main text draw color
	Text      string           // The string content of this Text
	TextSize  float32          // Size of the text - if the Canvas scale is 1.0 this will be equivalent to point size
	TextStyle bellui.TextStyle // The style of the text content

	// SubPixel disables pixel-grid rounding of the text texture size during
	// GL rendering. Enable this for precise positioning in data-visualization
	// layouts where labels must sit at exact fractional coordinates.
	SubPixel bool

	// FontSource defines a resource that can be used instead of the theme for looking up the font.
	// When a font source is set the `TextStyle` may not be effective, as it will be limited to the styles
	// present in the data provided.
	//
	// Since: 2.5
	FontSource bellui.Resource

	rotation float32 // rotation in degrees, applied around text center

	// measureMemo holds the last driver measurement (see MinSizeWithBaseline).
	// Atomic so concurrent MinSize reads stay race-free — MinSize was a pure
	// read before the memo and callers may treat it as one.
	measureMemo atomic.Pointer[textMeasureMemo]
}

// textMeasureMemo is one memoised text measurement: the inputs it depends on,
// the theme/font cache generation it was taken under, and the result.
type textMeasureMemo struct {
	text     string
	textSize float32
	style    bellui.TextStyle
	source   bellui.Resource
	gen      uint64
	size     geo.Size
	baseline float32
}

// Hide will set this text to not be visible
func (t *Text) Hide() {
	t.baseObject.Hide()

	repaint(t)
}

// MinSize returns the minimum size of this text object based on its font size and content.
// This is normally determined by the render implementation.
//
// The measurement is memoised per object: while Text, TextSize, TextStyle and
// FontSource are unchanged (and the theme/font caches have not reset) the last
// result is returned without consulting the driver, so measure-heavy layouts do
// not re-hash unchanged strings through the global metrics cache (#923).
func (t *Text) MinSize() geo.Size {
	s, _ := t.MinSizeWithBaseline()
	return s
}

// MinSizeWithBaseline returns [Text.MinSize] together with the distance from
// the top of the text to its baseline, both served from the same per-object
// measurement memo.
//
// Note that TextStyle is compared as a value: its pointer fields (Weight,
// LineHeightScale, LetterSpacing) are compared by pointer, so a caller mutating
// a pointee in place must assign a fresh pointer (or style) for the change to
// be measured.
//
// Since: 1.0
func (t *Text) MinSizeWithBaseline() (geo.Size, float32) {
	gen := cache.ThemeCacheGeneration()
	if m := t.measureMemo.Load(); m != nil && m.gen == gen &&
		m.text == t.Text && m.textSize == t.TextSize &&
		m.style == t.TextStyle && m.source == t.FontSource {
		return m.size, m.baseline
	}
	size, base := bellui.MeasureTextFull(t.Text, t.TextSize, t.TextStyle, t.FontSource)
	t.measureMemo.Store(&textMeasureMemo{
		text: t.Text, textSize: t.TextSize, style: t.TextStyle, source: t.FontSource,
		gen: gen, size: size, baseline: base,
	})
	return size, base
}

// Move the text to a new position, relative to its parent / canvas
func (t *Text) Move(pos geo.Position) {
	if t.Position() == pos {
		return
	}

	t.baseObject.Move(pos)

	repaint(t)
}

// Resize on a text updates the new size of this object, which may not result in a visual change, depending on alignment.
func (t *Text) Resize(s geo.Size) {
	if s == t.Size() {
		return
	}

	t.baseObject.Resize(s)
	Refresh(t)
}

// SetMinSize has no effect as the smallest size this canvas object can be is based on its font size and content.
func (t *Text) SetMinSize(geo.Size) {
	// no-op
}

// Refresh causes this text to be redrawn with its configured state.
func (t *Text) Refresh() {
	Refresh(t)
}

// SetRotation sets the rotation angle in degrees. The text is rotated around
// its visual center (half of the un-rotated MinSize). The rotation is
// recomputed automatically when the text content or size changes.
//
// SetRotation and [SetTransform] are mutually exclusive — calling one clears
// the effect of the other.
func (t *Text) SetRotation(degrees float32) {
	t.rotation = degrees
	Refresh(t)
}

// Rotation returns the rotation angle in degrees set by [SetRotation].
// If the transform was set directly via [SetTransform], this returns 0.
func (t *Text) Rotation() float32 {
	return t.rotation
}

// Transform returns the local affine transform. When a rotation has been set
// via [SetRotation], the transform is computed lazily from the current text
// size so the rotation center stays correct as text content changes.
func (t *Text) Transform() geo.Matrix {
	if t.rotation == 0 {
		return t.baseObject.Transform()
	}
	s, _ := t.MinSizeWithBaseline()
	rad := float64(t.rotation) * math.Pi / 180
	return geo.NewRotationAround(float32(rad), s.Width/2, s.Height/2)
}

// SetTransform sets an arbitrary affine transform and clears any rotation
// previously set via [SetRotation].
func (t *Text) SetTransform(m geo.Matrix) {
	t.rotation = 0
	t.baseObject.SetTransform(m)
}

// NewText returns a new Text implementation
func NewText(text string, color color.Color) *Text {
	return &Text{
		Color:    color,
		Text:     text,
		TextSize: theme.TextSize(),
	}
}
