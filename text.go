package bellui

import "github.com/belluikit/bellui/geo"

// TextAlign represents the horizontal alignment of text within a widget or
// canvas object.
type TextAlign int

const (
	// TextAlignLeading specifies a left alignment for left-to-right languages.
	TextAlignLeading TextAlign = iota
	// TextAlignCenter places the text centrally within the available space.
	TextAlignCenter
	// TextAlignTrailing will align the text right for a left-to-right language.
	TextAlignTrailing
)

// TextTruncation controls how a `Label` or `Entry` will truncate its text.
// The default value is `TextTruncateOff` which will not truncate.
//
// Since: 2.4
type TextTruncation int

const (
	// TextTruncateOff means no truncation will be applied, it is the default.
	// This means that the minimum size of a text block will be the space required to display it fully.
	TextTruncateOff TextTruncation = iota
	// TextTruncateClip will truncate text when it reaches the end of the available space.
	TextTruncateClip
	// TextTruncateEllipsis is like regular truncation except that an ellipses (…) will be inserted
	// wherever text has been shortened to fit.
	//
	// Since: 2.4
	TextTruncateEllipsis
)

// Direction represents a left-to-right or right-to-left reading/flow direction.
// It is the single concept behind both the user-interface layout direction (see
// [SetLayoutDirection], which mirrors horizontal containers and resolves
// leading/trailing alignment) and the base writing direction of text (see
// [TextStyle.Direction] via the [TextDirection] alias).
//
// Since: 2.8
type Direction int

const (
	// DirectionAuto derives the direction from the ambient user-interface
	// direction: the [SetLayoutDirection] override when set, otherwise the
	// active locale (right-to-left for Arabic, Hebrew and similar locales). It
	// is the default (zero) value.
	//
	// Since: 2.8
	DirectionAuto Direction = iota
	// DirectionLTR forces a left-to-right direction regardless of locale.
	//
	// Since: 2.8
	DirectionLTR
	// DirectionRTL forces a right-to-left direction regardless of locale.
	//
	// Since: 2.8
	DirectionRTL
)

// TextDirection represents the base writing direction of a block of text. It
// controls the paragraph base direction fed to the Unicode Bidirectional
// Algorithm (UAX #9), which determines how mixed left-to-right and
// right-to-left text — and neutral characters such as spaces and punctuation —
// are visually ordered. It does not affect within-run shaping (Arabic joining,
// Hebrew glyph forms), which is always derived from the text content itself.
//
// It is an alias for [Direction]: text base direction and UI layout direction
// are the same concept, so [TextStyle.Direction] left at the default
// [TextDirectionAuto] follows the same ambient direction that mirrors layouts.
//
// Since: 2.8
type TextDirection = Direction

const (
	// TextDirectionAuto is an alias for [DirectionAuto].
	//
	// Since: 2.8
	TextDirectionAuto = DirectionAuto
	// TextDirectionLTR is an alias for [DirectionLTR].
	//
	// Since: 2.8
	TextDirectionLTR = DirectionLTR
	// TextDirectionRTL is an alias for [DirectionRTL].
	//
	// Since: 2.8
	TextDirectionRTL = DirectionRTL
)

// TextWrap represents how text longer than the widget's width will be wrapped.
type TextWrap int

const (
	// TextWrapOff extends the widget's width to fit the text, no wrapping is applied.
	TextWrapOff TextWrap = iota
	// Retired: the former TextTruncate value. The slot is kept so TextWrapBreak
	// and TextWrapWord keep their values; use the widget Truncation field
	// (TextTruncateClip) for truncation.
	_
	// TextWrapBreak trims the line of characters to the widget's width adding the excess as new line.
	// An Entry with text wrapping will scroll vertically if there is not enough space for all the text.
	TextWrapBreak
	// TextWrapWord trims the line of words to the widget's width adding the excess as new line.
	// An Entry with text wrapping will scroll vertically if there is not enough space for all the text.
	TextWrapWord
)

// Ptr returns a pointer to v. It is a convenience for setting pointer fields
// in struct literals, e.g. bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}.
func Ptr[T any](v T) *T { return &v }

// TextWeight represents the weight (boldness) of a font.
// Values range from 100 (thinnest) to 900 (thickest).
type TextWeight float32

const (
	WeightThin       TextWeight = 100 // Thin weight (100).
	WeightExtraLight TextWeight = 200 // Extra light weight (200).
	WeightLight      TextWeight = 300 // Light weight (300).
	WeightNormal     TextWeight = 400 // Normal weight (400).
	WeightMedium     TextWeight = 500 // Medium weight (500).
	WeightSemiBold   TextWeight = 600 // Semi-bold weight (600).
	WeightBold       TextWeight = 700 // Bold weight (700).
	WeightExtraBold  TextWeight = 800 // Extra-bold weight (800).
	WeightBlack      TextWeight = 900 // Black weight (900).
)

const (
	// DefaultLineHeightScale is the default line-height multiplier (no scaling).
	DefaultLineHeightScale float32 = 1.0

	// DefaultLetterSpacing is the default letter spacing (no extra space).
	DefaultLetterSpacing float32 = 0
)

// TextStyle represents the styles that can be applied to a text canvas object
// or text based widget.
type TextStyle struct {
	Bold      bool // Should text be bold
	Italic    bool // Should text be italic
	Monospace bool // Use the system monospace font instead of regular
	// Since: 2.2
	Symbol bool // Use the system symbol font.
	// Since: 2.1
	TabWidth int // Width of tabs in spaces
	// Since: 2.5
	Underline bool // Should text be underlined.

	// Strikethrough draws a line through the middle of the text. Like
	// [TextStyle.Underline] it renders as a decoration over general text
	// (Label, RichText, canvas.Text) as well as in
	// [github.com/belluikit/bellui/widget.TextGrid].
	//
	// Since: 1.0
	Strikethrough bool // Should text be struck through.

	// Weight specifies the font weight (100–900). When nil, the Bold flag is
	// used: Bold=true maps to WeightBold (700), otherwise WeightNormal (400).
	// Valid values are 100–900. Use [Ptr] for struct literals:
	//
	//	bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}
	Weight *TextWeight

	// LineHeightScale multiplies the default line height derived from font metrics.
	// When nil, the default (1.0×) is used. Values > 1.0 add extra vertical space.
	LineHeightScale *float32

	// LetterSpacing adds extra horizontal space (in points) between glyphs.
	// When nil, no extra spacing is applied. Can point to negative values to
	// tighten text.
	LetterSpacing *float32

	// Direction sets the base writing direction for bidirectional text layout.
	// The zero value [TextDirectionAuto] derives the direction from the active
	// locale. Within-run shaping (Arabic joining, Hebrew forms, mixed-script
	// runs) is always detected from the text content regardless of this field;
	// Direction only controls the base ordering of runs and neutral characters.
	//
	// Since: 2.8
	Direction TextDirection

	// Language overrides the language tag used when shaping this text. It
	// selects script-specific OpenType features and localized glyph forms — for
	// example "sr-Cyrl" renders Serbian Cyrillic forms and "ru" Russian
	// Cyrillic forms — independently of the application locale. The value is a
	// BCP 47 language tag. The zero value (empty string) follows the active
	// locale, which is the default.
	//
	// Like Direction, this only affects shaping of the text content (contextual
	// alternates, localized glyph forms); it does not change which font face is
	// selected or the bidi ordering.
	//
	// Since: 2.8
	Language string
}

// ResolvedWeight returns the effective font weight. If Weight is nil, it falls
// back to [WeightBold] when Bold is true, otherwise [WeightNormal].
func (t TextStyle) ResolvedWeight() TextWeight {
	if t.Weight != nil {
		return *t.Weight
	}
	if t.Bold {
		return WeightBold
	}
	return WeightNormal
}

// ResolvedLineHeightScale returns the effective line-height multiplier.
// If LineHeightScale is nil, returns [DefaultLineHeightScale] (1.0).
func (t TextStyle) ResolvedLineHeightScale() float32 {
	if t.LineHeightScale != nil {
		return *t.LineHeightScale
	}
	return DefaultLineHeightScale
}

// ResolvedLetterSpacing returns the effective letter spacing in points.
// If LetterSpacing is nil, returns [DefaultLetterSpacing] (0).
func (t TextStyle) ResolvedLetterSpacing() float32 {
	if t.LetterSpacing != nil {
		return *t.LetterSpacing
	}
	return DefaultLetterSpacing
}

// textMeasureFunc is the signature for a standalone text measurement function.
type textMeasureFunc = func(string, float32, TextStyle, Resource) (geo.Size, float32)

// registeredMeasure is set by internal/painter.init() so that MeasureText
// and MeasureTextFull can work without a running app or driver.
var registeredMeasure textMeasureFunc

// RegisterTextMeasure registers a fallback text measurement function.
// This is called automatically by the painter package on import.
func RegisterTextMeasure(fn textMeasureFunc) {
	registeredMeasure = fn
}

// measureTextImpl measures text using the driver if available, otherwise the
// registered fallback.
func measureTextImpl(text string, fontSize float32, style TextStyle, source Resource) (geo.Size, float32) {
	if app := CurrentApp(); app != nil {
		if d := app.Driver(); d != nil {
			return d.RenderedTextSize(text, fontSize, style, source)
		}
	}
	if registeredMeasure != nil {
		return registeredMeasure(text, fontSize, style, source)
	}
	panic("bellui: text measurement unavailable — no running app and no painter registered")
}

// MeasureText uses the current driver to calculate the size of text when rendered.
// The font used will be read from the current app's theme.
//
// If no app is running, falls back to the registered text measurement function
// (registered automatically when the painter package is imported).
func MeasureText(text string, size float32, style TextStyle) geo.Size {
	s, _ := measureTextImpl(text, size, style, nil)
	return s
}

// MeasureTextFull measures text and returns both the rendered size and the
// baseline offset (distance from the top to the text baseline). The source
// parameter specifies a custom font resource; pass nil to use the theme font.
//
// Like [MeasureText], this works with or without a running app.
func MeasureTextFull(text string, fontSize float32, style TextStyle, source Resource) (geo.Size, float32) {
	return measureTextImpl(text, fontSize, style, source)
}
