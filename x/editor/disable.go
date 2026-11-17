package editor

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// A disabled Editor is visible but inert (#1216, the owner's definition on
// belluikit/player#641): its text is drawn dimmed, and it takes no key, no
// composition, no pointer input and no focus. Its text can be neither selected
// nor copied. That is stricter than a disabled widget.Entry, which keeps its
// text selectable and copyable, so the Editor does not leave it to the Entry:
//
//   - a shield over the whole editor takes every pointer gesture while it is
//     disabled, so no tap, drag, scroll, hover or mouse press reaches the
//     Entry, its selection layer, the gutter or the find bar;
//   - the Entry and the find bar's own widgets are disabled, so Tab skips them
//     and the Entry takes no keys, composition or soft keyboard;
//   - the editor's own key hooks refuse everything, Copy included.
//
// Read-only ([Editor.SetReadOnly]) is a separate state: select and copy, no
// edits. The two are independent flags. While disabled, read-only changes
// nothing, since nothing is allowed; Enable returns to whichever read-only
// state was set, including one set while disabled.
var (
	_ core.Disableable = (*Editor)(nil)

	_ event.Tappable          = (*disabledShield)(nil)
	_ event.DoubleTappable    = (*disabledShield)(nil)
	_ event.SecondaryTappable = (*disabledShield)(nil)
	_ event.LongPressable     = (*disabledShield)(nil)
	_ event.Draggable         = (*disabledShield)(nil)
	_ event.Scrollable        = (*disabledShield)(nil)
	_ event.Hoverable         = (*disabledShield)(nil)
	_ desktop.Mouseable       = (*disabledShield)(nil)
	_ desktop.Cursorable      = (*disabledShield)(nil)
	_ mobile.Touchable        = (*disabledShield)(nil)
)

// DrawsDisabledLook reports that the paint walk must not dim a disabled
// Editor (bellui's core.DisabledLook, belluikit/bellui#1223): its Entry and
// find bar are disabled and draw their own disabled look, and the editor dims
// its own gutter and decorations (#1232). The app's gutter providers and
// decorators are left as the app draws them.
func (*Editor) DrawsDisabledLook() bool { return true }

// Disable makes the editor visible but inert (see the note above). Open
// popovers close, a focus inside the editor is dropped, and the find bar, if
// shown, stays shown and disabled. It is safe before the editor is drawn.
func (e *Editor) Disable() {
	if e.disabled {
		return
	}
	e.disabled = true

	e.hover.cancelTimer()
	e.hover.dismiss()
	e.signature.dismiss()
	e.dismissPopover()
	e.chordPending = false

	e.dropFocus()
	e.entry.Disable()
	e.find.setDisabled(true)
	// Without a focus the Entry draws its selection only because the editor
	// keeps it visible; a disabled editor's text is not selected.
	e.entry.KeepSelectionVisible = false
	e.applyLook()
	e.shield.Show()
}

// Enable gives the editor its input, focus and look back.
func (e *Editor) Enable() {
	if !e.disabled {
		return
	}
	e.disabled = false

	e.shield.Hide()
	e.entry.KeepSelectionVisible = true
	e.find.setDisabled(false)
	e.entry.Enable()
	e.applyLook()
}

// Disabled reports whether the editor is disabled.
func (e *Editor) Disabled() bool {
	return e.disabled
}

// applyLook draws the text in the disabled colour while the editor is disabled
// and restores the syntax colours on Enable. The Entry dims its text itself,
// but the highlighter's runs carry their own colours and the semantic overlay
// paints over them, so both are taken off while disabled.
//
// The gutter and the decorator overlay draw the rest of what the editor itself
// draws in the disabled look too (#1232); see dimObjects for which parts.
func (e *Editor) applyLook() {
	if e.disabled {
		e.entry.StyleOverlayForRow = nil
	} else {
		e.entry.StyleOverlayForRow = e.semantic.runsForRow
	}
	e.applyHighlighter()
	e.entry.Refresh()
	if e.gutterW != nil {
		e.gutterW.disabled = e.disabled
		e.gutterW.Refresh()
	}
	if e.decOverlay != nil {
		e.decOverlay.disabled = e.disabled
		e.decOverlay.Refresh()
	}
}

// disabledMarkOpacity is the opacity a disabled editor draws its own marks at:
// MD3's disabled content opacity, which bellui's paint walk also dims a
// disabled widget without a look of its own to.
const disabledMarkOpacity = 0.38

// The disabled look of what the editor draws besides its text (#1232). The
// Editor tells the paint walk not to dim it (DrawsDisabledLook), so these parts
// are dimmed here or not at all:
//
//   - text (line numbers, fold placeholders) takes the disabled text colour,
//     the colour the Entry gives the code beside it;
//   - every other mark (fold chevrons, breakpoint dots, diagnostic markers and
//     squiggles, find-match washes, link underlines, the hover outline) keeps
//     its hue at 38% opacity, as the paint walk's fallback would draw it;
//   - a redaction bar stays opaque, since a translucent one would show the
//     secret under it, in its colour mixed 38% over the theme background.
//
// Only the editor's own parts are dimmed: the gutter providers of this package
// (LineNumberGutter, FoldGutter, BreakpointGutter, and a DiagnosticGutter
// without a MarkerFunc) and the span decorators the editor adds itself. A
// GutterProvider, MarkerFunc, RowDecorator or SpanDecorator the app supplies
// draws the app's content, which a disabled container leaves as the app drew
// it (belluikit/player#641).

// dimObjects redraws objs, built by one of the editor's own gutter providers,
// in the disabled look.
func dimObjects(objs []core.CanvasObject) {
	for _, o := range objs {
		switch o := o.(type) {
		case *canvas.Text:
			o.Color = theme.Color(theme.ColorNameDisabled)
		case *canvas.Rectangle:
			o.FillColor, o.StrokeColor = dimColor(o.FillColor), dimColor(o.StrokeColor)
		case *canvas.Circle:
			o.FillColor, o.StrokeColor = dimColor(o.FillColor), dimColor(o.StrokeColor)
		case *canvas.Polygon:
			o.FillColor, o.StrokeColor = dimColor(o.FillColor), dimColor(o.StrokeColor)
		case *canvas.Line:
			o.StrokeColor = dimColor(o.StrokeColor)
		}
	}
}

// dimColor returns c at 38% of its opacity, keeping its hue. nil stays nil.
func dimColor(c color.Color) color.Color {
	if c == nil {
		return nil
	}
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = uint8(float32(n.A)*disabledMarkOpacity + 0.5)
	return n
}

// dimOpaque returns the opaque colour c drawn at 38% over the theme background
// looks like: the disabled look of a redaction bar, which must stay opaque.
func dimOpaque(c color.Color) color.Color {
	fg := color.NRGBAModel.Convert(toOpaque(c)).(color.NRGBA)
	bg := color.NRGBAModel.Convert(toOpaque(theme.Color(theme.ColorNameBackground))).(color.NRGBA)
	mix := func(f, b uint8) uint8 {
		return uint8(float32(b) + (float32(f)-float32(b))*disabledMarkOpacity + 0.5)
	}
	return color.NRGBA{R: mix(fg.R, bg.R), G: mix(fg.G, bg.G), B: mix(fg.B, bg.B), A: 255}
}

// dimSpan returns span as a disabled editor draws it.
func dimSpan(span DecoratedSpan) DecoratedSpan {
	col := span.Color
	if col == nil {
		col = defaultSpanColor
	}
	if span.Kind == SpanBackground && span.Opaque {
		span.Color = dimOpaque(col)
	} else {
		span.Color = dimColor(col)
	}
	return span
}

// ownsGutterLook reports whether p is one of this package's gutter providers,
// whose marks a disabled editor dims. A CompositeGutter is not asked: its
// providers are, one by one.
func ownsGutterLook(p GutterProvider) bool {
	switch g := p.(type) {
	case *LineNumberGutter, *FoldGutter, *BreakpointGutter:
		return true
	case *DiagnosticGutter:
		return g.MarkerFunc == nil // a MarkerFunc draws the app's marker
	}
	return false
}

// renderGutterCell renders p's cell for line (1-based), in the disabled look
// when disabled and p is the editor's own.
func renderGutterCell(p GutterProvider, line int, size geo.Size, textSize float32, disabled bool) []core.CanvasObject {
	if c, ok := p.(*CompositeGutter); ok {
		return c.render(line, size, textSize, disabled)
	}
	objs := p.Render(line, size, textSize)
	if disabled && ownsGutterLook(p) {
		dimObjects(objs)
	}
	return objs
}

// dropFocus unfocuses the canvas when the focus is on one of the editor's own
// focusable parts. The Editor holds no content of the app's, so those parts
// are all there is. A widget.DisableableWidget does this in Disable, but only
// in a bellui that has #1221, and this module may be built against one that
// does not.
func (e *Editor) dropFocus() {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}
	c := app.Driver().CanvasForObject(e.entry)
	if c == nil {
		return
	}
	focused := c.Focused()
	if focused == nil {
		return
	}
	for _, own := range e.focusables() {
		if focused == own {
			c.Unfocus()
			return
		}
	}
}

// focusables lists the editor's parts that can take the keyboard focus.
func (e *Editor) focusables() []event.Focusable {
	out := []event.Focusable{e.entry}
	for _, w := range e.find.parts() {
		if f, ok := w.(event.Focusable); ok {
			out = append(out, f)
		}
	}
	return out
}

// disabledShield lies over the whole editor while it is disabled and takes
// every pointer gesture, doing nothing with it. The drivers deliver a gesture
// to the topmost object under the pointer that takes it, whatever its
// ancestors, so this is what keeps a click from focusing the Entry and a drag
// from selecting its text. Hidden while the editor is enabled, it is then
// invisible to hit testing.
type disabledShield struct {
	widget.BaseWidget
}

func newDisabledShield() *disabledShield {
	s := &disabledShield{}
	s.ExtendBaseWidget(s)
	s.Hide()
	return s
}

func (s *disabledShield) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (s *disabledShield) Tapped(*event.PointEvent)          {}
func (s *disabledShield) DoubleTapped(*event.PointEvent)    {}
func (s *disabledShield) TappedSecondary(*event.PointEvent) {}
func (s *disabledShield) LongPressed(*event.PointEvent)     {}
func (s *disabledShield) Dragged(*event.DragEvent)          {}
func (s *disabledShield) DragEnd()                          {}
func (s *disabledShield) Scrolled(*event.ScrollEvent)       {}
func (s *disabledShield) HoverIn(*event.HoverEvent)         {}
func (s *disabledShield) HoverMoved(*event.HoverEvent)      {}
func (s *disabledShield) HoverOut()                         {}
func (s *disabledShield) MouseDown(*desktop.MouseEvent)     {}
func (s *disabledShield) MouseUp(*desktop.MouseEvent)       {}
func (s *disabledShield) TouchDown(*mobile.TouchEvent)      {}
func (s *disabledShield) TouchUp(*mobile.TouchEvent)        {}
func (s *disabledShield) TouchCancel(*mobile.TouchEvent)    {}

// Cursor shows the plain arrow: a disabled editor offers no text to select.
func (s *disabledShield) Cursor() desktop.Cursor { return desktop.DefaultCursor }
