package editor

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
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

// Disable makes the editor visible but inert (see the note above). Open
// popovers close, a focus inside the editor is dropped, and the find bar, if
// shown, stays shown and disabled. It is safe before the editor is drawn.
// DrawsDisabledLook reports that the paint walk must not dim a disabled
// Editor (bellui's core.DisabledLook, belluikit/bellui#1223): its Entry and find bar are disabled and draw their own disabled look.
func (*Editor) DrawsDisabledLook() bool { return true }

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
func (e *Editor) applyLook() {
	if e.disabled {
		e.entry.StyleOverlayForRow = nil
	} else {
		e.entry.StyleOverlayForRow = e.semantic.runsForRow
	}
	e.applyHighlighter()
	e.entry.Refresh()
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
