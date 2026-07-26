package widget

import (
	"image"
	"image/color"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// RichEntry is an editable rich-text surface: text with per-run styling laid out
// and rendered by [RichText], edited through a segment-aware document model
// (#648, Phase 1).
//
// Unlike [Entry] — which is flat-string / single-segment to the bone — RichEntry
// stores its content as an ordered list of [TextSegment]s and [ImageSegment]
// atoms, and addresses the caret and selection with a position model in which
// each image occupies exactly one position. It covers text editing (typing,
// deletion, caret navigation, selection), per-run styling via
// [RichEntry.ApplyStyle], undo/redo, IME composition, flavor-aware clipboard, and
// both block and inline images ([RichEntry.InsertImage] / [RichEntry.InsertInlineImage],
// #665). Word/character wrapping is opt-in via [RichEntry.Wrapping] (#653; the
// default is off, one rendered row per '\n'-separated line).
//
// Since: 2.18
type RichEntry struct {
	BaseWidget

	// OnChanged is called, on the main goroutine, after any edit that changes the
	// content.
	//
	// Since: 2.18
	OnChanged func()

	// OnContentChanged is called, on the main goroutine, after any edit that
	// changes the content, with the current document segments. The slice is the
	// live backing store — treat it as read-only (the same contract as
	// [RichEntry.Segments]). Use this instead of OnChanged + Segments() when a
	// host wants the rich content on every change without a separate read.
	//
	// Since: 2.18
	OnContentChanged func([]RichTextSegment)

	// Wrapping selects how logical lines that exceed the width are laid out. The
	// default [bellui.TextWrapOff] keeps each '\n'-separated line on one rendered row
	// (horizontal scrolling is the caller's concern); [bellui.TextWrapWord] and
	// [bellui.TextWrapBreak] soft-wrap a long line onto multiple visual rows, with
	// the caret and selection mapping following the wrap (#653).
	//
	// Since: 2.18
	Wrapping bellui.TextWrap

	// Style selects the MD3 visual chrome, matching [Entry.Style]: [EntryStyleFilled]
	// (the default) draws a filled rounded-rect background; [EntryStyleOutlined] is
	// transparent with an always-visible outline. In both styles the border adopts
	// the primary color while focused.
	//
	// Since: 2.18
	Style EntryStyle

	// TypingStyle is the style applied to newly typed text. Its zero value means
	// "inherit the style at the caret" (typing inside a bold run stays bold); set
	// it to type in a specific style regardless of the surrounding runs.
	//
	// Since: 2.18
	TypingStyle RichTextStyle

	doc      *richEntryDoc
	provider RichText

	caret     int // global rune offset of the caret
	anchor    int // selection anchor offset (the fixed end of a selection)
	selecting bool

	selectKeyDown bool
	focused       bool

	// IME composition state (#658). The preedit is shown inline at the caret but
	// never touches the document, undo or change callbacks until commit.
	composing     bool
	preeditText   string
	preeditCursor int // caret offset within preeditText, in runes

	sel        *selectable
	cursorAnim *entryCursorAnimation
	shortcut   bellui.ShortcutHandler
	undo       richEntryUndoStack

	// Mobile touch-selection state (#888, parity with Entry's #758): selHandles
	// is the draggable-handle overlay shown while a focused editor has a live
	// touch-initiated selection; touchPointer notes whether the most recent
	// pointer-down was a touch; touchSelection marks the selection as
	// touch-initiated (keyboard/programmatic selections summon no handles).
	// See richentry_handles.go.
	selHandles                   *entrySelectionHandles
	touchPointer, touchSelection bool

	// caretHandle is the mobile caret drag-handle overlay (#999, parity with
	// Entry's #943): a single draggable dot summoned after a plain touch tap so
	// the insertion point can be repositioned without selecting. See
	// richentry_caret_handle.go.
	caretHandle *richEntryCaretHandle

	// axText memoizes AccessibleText's U+FFFC rune stream so the mobile
	// text-input mirror's per-paint-frame sync (#783) does not rebuild the whole
	// document string every frame; invalidated only on committed-content changes
	// (afterEditNotify + SetSegments), not on repaint/preedit/restyle (#794).
	// Main-goroutine-owned, like the rest of the widget's state.
	axText      string
	axTextValid bool
}

// NewRichEntry creates a new empty rich-text editor.
//
// Since: 2.18
func NewRichEntry() *RichEntry {
	e := &RichEntry{doc: newRichEntryDoc()}
	e.ExtendBaseWidget(e)
	e.registerShortcuts()
	return e
}

// SetSegments replaces the editor content with the given rich-text segments and
// moves the caret to the start. Text segments are forced inline; non-text
// segments are rendered but not yet caret-addressable (Phase 1).
//
// Since: 2.18
func (e *RichEntry) SetSegments(segs []RichTextSegment) {
	e.doc.setSegments(segs)
	e.caret, e.anchor, e.selecting = 0, 0, false
	e.undo.clear()        // external content replacement is not undoable
	e.axTextValid = false // committed content replaced; drop the AccessibleText memo (#794)
	e.syncProvider()
	e.Refresh()
}

// Segments returns the current document segments. The slice is the live backing
// store — treat it as read-only and use the editing methods to mutate content.
//
// Since: 2.18
func (e *RichEntry) Segments() []RichTextSegment {
	return e.doc.segments()
}

// String returns the plain-text content of the editor.
//
// Since: 2.18
func (e *RichEntry) String() string {
	return e.doc.String()
}

// SelectedText returns the currently selected text, or the empty string when
// nothing is selected.
//
// Since: 2.18
func (e *RichEntry) SelectedText() string {
	from, to, ok := e.selection()
	if !ok {
		return ""
	}
	return e.doc.textBetween(from, to)
}

// ApplyStyle applies a style to the current selection, splitting runs at the
// selection boundaries. With no selection it sets [RichEntry.TypingStyle] so the
// next typed text uses the style.
//
// Since: 2.18
func (e *RichEntry) ApplyStyle(style RichTextStyle) {
	if from, to, ok := e.selection(); ok {
		e.mutate(from, to, func() { e.doc.applyStyle(from, to, style) })
		return
	}
	e.TypingStyle = style
}

// InsertImage inserts an in-memory image at the caret as one undo step. The image
// is a block that occupies its own row; the caret lands after it. Any current
// selection is replaced.
//
// Since: 2.18
func (e *RichEntry) InsertImage(img image.Image) {
	e.insertAtom(&ImageSegment{Image: img})
}

// InsertImageResource is like [RichEntry.InsertImage] for a resource-backed image
// (e.g. bundled bytes).
//
// Since: 2.18
func (e *RichEntry) InsertImageResource(res bellui.Resource) {
	e.insertAtom(&ImageSegment{Resource: res})
}

// InsertInlineImage inserts an in-memory image that flows *within* the current
// text row (like a large glyph) rather than on its own block row, as one undo
// step. The image occupies one caret position; any current selection is
// replaced (#665).
//
// Since: 2.18
func (e *RichEntry) InsertInlineImage(img image.Image) {
	e.insertAtom(&ImageSegment{Image: img, InlineImage: true})
}

// InsertInlineImageResource is like [RichEntry.InsertInlineImage] for a
// resource-backed image.
//
// Since: 2.18
func (e *RichEntry) InsertInlineImageResource(res bellui.Resource) {
	e.insertAtom(&ImageSegment{Resource: res, InlineImage: true})
}

// insertAtom inserts a non-text atom segment at the caret as one undo step.
func (e *RichEntry) insertAtom(seg RichTextSegment) {
	from, to := e.caret, e.caret
	if f, t, ok := e.selection(); ok {
		from, to = f, t
	}
	e.mutate(from, to, func() {
		e.deleteSelectionRaw()
		e.caret = e.doc.insertAtom(e.caret, seg)
		e.anchor, e.selecting = e.caret, false
	})
}

// Undo reverts the last edit, restoring the caret. It is a no-op when there is
// nothing to undo.
//
// Since: 2.18
func (e *RichEntry) Undo() {
	if ed := e.undo.undo(); ed != nil {
		e.caret = ed.apply(e.doc, false)
		e.anchor, e.selecting = e.caret, false
		e.afterEditNotify()
	}
}

// Redo re-applies the last undone edit. It is a no-op when there is nothing to
// redo.
//
// Since: 2.18
func (e *RichEntry) Redo() {
	if ed := e.undo.redo(); ed != nil {
		e.caret = ed.apply(e.doc, true)
		e.anchor, e.selecting = e.caret, false
		e.afterEditNotify()
	}
}

// --- editing primitives ---

func (e *RichEntry) effectiveStyle() RichTextStyle {
	if !richStyleEqual(e.TypingStyle, RichTextStyle{}) {
		return e.TypingStyle
	}
	return e.doc.styleAt(e.caret)
}

// selection returns the ordered half-open selection range and whether a
// non-empty selection exists.
func (e *RichEntry) selection() (from, to int, ok bool) {
	if !e.selecting || e.anchor == e.caret {
		return 0, 0, false
	}
	from, to = e.anchor, e.caret
	if from > to {
		from, to = to, from
	}
	return from, to, true
}

// mutate performs a document edit that replaces the styled content of [from, to)
// with whatever `do` leaves in its place, recording it as one reversible undo
// step. `do` must mutate the document and set the caret but must not itself
// notify (mutate does). The replaced range is derived from the document length
// change, so mutate works for insert, delete, replace and restyle uniformly.
func (e *RichEntry) mutate(from, to int, do func()) {
	caretBefore := e.caret
	lenBefore := e.doc.len()
	before := e.doc.segmentsBetween(from, to)

	do()

	afterEnd := from + (to - from) + (e.doc.len() - lenBefore)
	after := e.doc.segmentsBetween(from, afterEnd)
	e.undo.add(&richEntryEdit{
		at: from, before: before, after: after,
		caretBefore: caretBefore, caretAfter: e.caret,
	})
	e.afterEditNotify()
}

// deleteSelectionRaw removes the current selection without notifying or
// recording undo — a building block used inside mutate.
func (e *RichEntry) deleteSelectionRaw() bool {
	from, to, ok := e.selection()
	if !ok {
		return false
	}
	e.doc.deleteRange(from, to)
	e.caret, e.anchor, e.selecting = from, from, false
	return true
}

// deleteSelection removes the current selection as one undo step.
func (e *RichEntry) deleteSelection() bool {
	from, to, ok := e.selection()
	if !ok {
		return false
	}
	e.mutate(from, to, func() { e.deleteSelectionRaw() })
	return true
}

func (e *RichEntry) insertRunes(runes []rune) {
	if len(runes) == 0 {
		return
	}
	from, to := e.caret, e.caret
	if f, t, ok := e.selection(); ok {
		from, to = f, t
	}
	e.mutate(from, to, func() {
		// Delete any selection first, then resolve the style at the true insertion
		// point (the collapsed caret) — otherwise typing over a selection would
		// inherit the style of a character inside the region being removed.
		e.deleteSelectionRaw()
		style := e.effectiveStyle()
		e.caret = e.doc.insertText(e.caret, runes, style)
		e.anchor, e.selecting = e.caret, false
	})
}

// afterEditNotify refreshes the provider, repaints and fires OnChanged after a
// content change (interactive edit, undo or redo).
func (e *RichEntry) afterEditNotify() {
	e.axTextValid = false // committed content changed; drop the AccessibleText memo (#783/#794)
	e.interruptCursor()
	// e.Refresh() re-syncs the provider synchronously (BaseWidget.Refresh ->
	// richEntryRenderer.Refresh -> syncProvider), so a direct syncProvider here
	// would rebuild + re-lay-out the whole provider a second time per keystroke —
	// O(document) wasted work on a large doc (#896).
	e.Refresh()
	if e.OnChanged != nil {
		e.OnChanged()
	}
	if e.OnContentChanged != nil {
		e.OnContentChanged(e.doc.segments())
	}
}

// interruptCursor keeps the caret solid while the user is actively typing or
// navigating, resuming the blink after a short idle (mirrors Entry).
func (e *RichEntry) interruptCursor() {
	if e.cursorAnim != nil {
		e.cursorAnim.interrupt()
	}
}

// moveCaret moves the caret to offset, extending the selection when extend is
// true and collapsing it otherwise.
func (e *RichEntry) moveCaret(offset int, extend bool) {
	if offset < 0 {
		offset = 0
	}
	if n := e.doc.len(); offset > n {
		offset = n
	}
	if extend {
		if !e.selecting {
			e.anchor = e.caret
			e.selecting = true
		}
	} else {
		e.selecting = false
		e.anchor = offset
	}
	e.caret = offset
	e.interruptCursor()
	e.Refresh()
}

// --- Focusable ---

// FocusGained is called when the RichEntry has been given focus.
func (e *RichEntry) FocusGained() {
	e.focused = true
	e.Refresh()
}

// FocusLost is called when the RichEntry has had focus removed.
func (e *RichEntry) FocusLost() {
	if e.composing {
		e.ComposeCancel()
	}
	e.focused = false
	e.selectKeyDown = false
	e.Refresh()
}

// TypedRune receives text input events and inserts the rune at the caret. While
// an IME composition is active, a typed rune commits it (the desktop char
// callback path).
func (e *RichEntry) TypedRune(r rune) {
	// Typing dismisses the mobile caret drag-handle (#999).
	e.hideCaretHandle()
	if e.composing {
		e.ComposeCommit(string(r))
		return
	}
	e.insertRunes([]rune{r})
}

// TypedKey receives key input events for editing and navigation.
func (e *RichEntry) TypedKey(key *event.KeyEvent) {
	// Keyboard interaction dismisses the mobile caret drag-handle (#999).
	e.hideCaretHandle()
	if e.composing {
		if key.Name == event.KeyEscape {
			e.ComposeCancel()
		}
		return // other keys are owned by the IME while composing
	}
	extend := e.selectKeyDown
	switch key.Name {
	case event.KeyBackspace:
		if e.deleteSelection() {
			return
		}
		if e.caret == 0 {
			return
		}
		at := e.caret
		e.mutate(at-1, at, func() {
			e.doc.deleteRange(at-1, at)
			e.caret, e.anchor = at-1, at-1
		})
	case event.KeyDelete:
		if e.deleteSelection() {
			return
		}
		if e.caret >= e.doc.len() {
			return // nothing to delete at the document end
		}
		at := e.caret
		e.mutate(at, at+1, func() { e.doc.deleteRange(at, at+1) })
	case event.KeyReturn, event.KeyEnter:
		e.insertRunes([]rune{'\n'})
	case event.KeyLeft:
		if !extend {
			if from, _, ok := e.selection(); ok {
				e.moveCaret(from, false)
				return
			}
		}
		e.moveCaret(e.caret-1, extend)
	case event.KeyRight:
		if !extend {
			if _, to, ok := e.selection(); ok {
				e.moveCaret(to, false)
				return
			}
		}
		e.moveCaret(e.caret+1, extend)
	case event.KeyUp:
		row, col := e.doc.offsetToRowCol(e.caret)
		if row == 0 {
			e.moveCaret(0, extend)
			return
		}
		e.moveCaret(e.doc.rowColToOffset(row-1, col), extend)
	case event.KeyDown:
		row, col := e.doc.offsetToRowCol(e.caret)
		if row+1 >= e.doc.rowCount() {
			e.moveCaret(e.doc.len(), extend) // last row: go to end of document
			return
		}
		e.moveCaret(e.doc.rowColToOffset(row+1, col), extend)
	case event.KeyHome:
		row, _ := e.doc.offsetToRowCol(e.caret)
		e.moveCaret(e.doc.rowColToOffset(row, 0), extend)
	case event.KeyEnd:
		row, _ := e.doc.offsetToRowCol(e.caret)
		e.moveCaret(e.doc.rowColToOffset(row, e.doc.len()), extend)
	}
}

// --- desktop.Keyable (shift tracking for selection) ---

// KeyDown records shift-key presses so cursor navigation extends the selection.
func (e *RichEntry) KeyDown(key *event.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		if !e.selecting {
			e.anchor = e.caret
		}
		e.selectKeyDown = true
	}
}

// KeyUp clears the shift-key state.
func (e *RichEntry) KeyUp(key *event.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.selectKeyDown = false
	}
}

// --- Tappable ---

// Tapped focuses the editor and places the caret at the tapped position.
func (e *RichEntry) Tapped(ev *event.PointEvent) {
	e.requestFocus()
	e.syncSel()
	row, col, _ := e.sel.getRowCol(ev.Position)
	e.moveCaret(e.doc.rowColToOffset(row, col), false)
	// A plain touch tap placing the caret summons the draggable caret handle
	// (#999); a drag-select or long-press instead starts a selection, which
	// updateCaretHandle then hides in favor of the selection handles.
	e.showCaretHandle()
}

func (e *RichEntry) requestFocus() {
	impl := e.super()
	if c := bellui.CurrentApp().Driver().CanvasForObject(impl); c != nil {
		if f, ok := impl.(event.Focusable); ok {
			c.Focus(f)
		}
	}
}

// Cursor shows the text I-beam while hovering the editor.
func (e *RichEntry) Cursor() desktop.Cursor {
	return desktop.TextCursor
}

// --- DoubleTappable (word selection) ---

// DoubleTapped selects the word under the pointer, mirroring Entry. On mobile,
// where there is no Shift key, double-tap is a primary way to begin a selection.
func (e *RichEntry) DoubleTapped(ev *event.PointEvent) {
	e.requestFocus()
	e.syncSel()
	row, col, _ := e.sel.getRowCol(ev.Position)
	start, end := getTextWhitespaceRegion(e.provider.row(row), col, false)
	if start == -1 || end == -1 {
		return
	}
	e.anchor = e.doc.rowColToOffset(row, start)
	e.caret = e.doc.rowColToOffset(row, end)
	e.selecting = e.anchor != e.caret
	e.interruptCursor()
	if e.selecting && e.touchPointer {
		e.markTouchSelection() // a mobile double-tap selection gets handles (#888)
	}
	e.Refresh()
}

// --- Draggable (pointer / touch range selection) ---

// Dragged extends a text selection as the pointer (or touch) moves. The first
// event of a drag anchors the selection at the drag's start position; later
// events move the caret end. This is the only selection path on mobile, which
// has no Shift key (#750).
func (e *RichEntry) Dragged(d *event.DragEvent) {
	e.requestFocus()
	e.syncSel()
	if !e.selecting {
		start := d.Position.Subtract(d.Dragged)
		sr, sc, _ := e.sel.getRowCol(start)
		e.anchor = clampInt(e.doc.rowColToOffset(sr, sc), 0, e.doc.len())
		e.selecting = true
	}
	row, col, _ := e.sel.getRowCol(d.Position)
	e.caret = clampInt(e.doc.rowColToOffset(row, col), 0, e.doc.len())
	e.interruptCursor()
	if e.touchPointer && e.selecting {
		e.markTouchSelection() // a touch drag-select gets handles (#888)
	}
	e.Refresh()
}

// DragEnd finalizes a drag selection, collapsing it when the drag selected
// nothing (a press-drag that returned to its origin).
func (e *RichEntry) DragEnd() {
	if e.anchor == e.caret {
		e.selecting = false
		e.Refresh()
	}

	// A finished touch drag-select offers the context menu for the new
	// selection, like a handle adjustment (native toolbar parity, #945).
	// This also covers the hold-fire long-press that keeps moving: its
	// release ends as a drag, so the driver's release-time TappedSecondary
	// never fires for it. Touch-gated: desktop mouse selection is unchanged.
	if e.touchPointer && e.touchSelection && e.selecting && bellui.CurrentDevice().IsMobile() {
		if _, _, ok := e.selection(); ok {
			row, col := e.doc.offsetToRowCol(e.caret)
			rect := e.selectionHandleCaretRect(row, col)
			pos := rect.Pos.AddXY(0, rect.Size.Height)
			abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super()).Add(pos)
			e.TappedSecondary(&event.PointEvent{AbsolutePosition: abs, Position: pos})
		}
	}
}

// --- clipboard shortcuts ---

// TypedShortcut handles registered keyboard shortcuts (copy/cut/paste/select-all).
func (e *RichEntry) TypedShortcut(s core.Shortcut) {
	e.shortcut.TypedShortcut(s)
}

func (e *RichEntry) registerShortcuts() {
	e.shortcut.AddShortcut(&bellui.ShortcutCopy{}, func(se core.Shortcut) {
		e.copyToClipboard(se.(*bellui.ShortcutCopy).Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutCut{}, func(se core.Shortcut) {
		e.cutToClipboard(se.(*bellui.ShortcutCut).Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutPaste{}, func(se core.Shortcut) {
		e.pasteFromClipboard(se.(*bellui.ShortcutPaste).Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutSelectAll{}, func(core.Shortcut) {
		e.selectAll()
	})
	e.shortcut.AddShortcut(&bellui.ShortcutUndo{}, func(core.Shortcut) {
		e.Undo()
	})
	e.shortcut.AddShortcut(&bellui.ShortcutRedo{}, func(core.Shortcut) {
		e.Redo()
	})
}

func (e *RichEntry) selectAll() {
	if e.doc.len() == 0 {
		return
	}
	e.anchor, e.caret, e.selecting = 0, e.doc.len(), true
	e.Refresh()
}

// copyToClipboard writes the selection to the clipboard, negotiating flavors
// (#666): a lone in-memory image goes to the image flavor; styled text goes to
// the HTML flavor (with a plain-text fallback); otherwise plain text.
func (e *RichEntry) copyToClipboard(clipboard bellui.Clipboard) {
	from, to, ok := e.selection()
	if !ok {
		return
	}
	segs := e.doc.segmentsBetween(from, to)
	plain := e.doc.textBetween(from, to)

	if len(segs) == 1 {
		if img, isImg := segs[0].(*ImageSegment); isImg && img.Image != nil {
			if ic, has := clipboard.(bellui.ImageClipboard); has {
				ic.SetImage(img.Image)
				return
			}
		}
	}
	if hc, has := clipboard.(bellui.HTMLClipboard); has {
		hc.SetHTML(segsToHTML(segs), plain)
		return
	}
	clipboard.SetContent(plain)
}

func (e *RichEntry) cutToClipboard(clipboard bellui.Clipboard) {
	if _, _, ok := e.selection(); !ok {
		return
	}
	e.copyToClipboard(clipboard)
	e.deleteSelection() // records its own undo step and notifies
}

// pasteFromClipboard inserts the clipboard content at the caret, negotiating
// flavors (#657): an image is inserted as a block atom; HTML is parsed to styled
// segments; otherwise plain text (line endings normalized to '\n'). Each paste is
// one undo step.
func (e *RichEntry) pasteFromClipboard(clipboard bellui.Clipboard) {
	if ic, has := clipboard.(bellui.ImageClipboard); has {
		if img, ok := ic.Image(); ok && img != nil {
			e.InsertImage(img)
			return
		}
	}
	if hc, has := clipboard.(bellui.HTMLClipboard); has {
		if h, ok := hc.HTML(); ok && h != "" {
			if segs := flattenHTMLToInline(NewRichTextFromHTML(h).Segments); len(segs) > 0 {
				e.insertSegments(segs)
				return
			}
		}
	}
	text := strings.ReplaceAll(clipboard.Content(), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	e.insertRunes([]rune(text))
}

// insertSegments splices a list of styled segments (from a rich paste) at the
// caret as one undo step, replacing any selection.
func (e *RichEntry) insertSegments(segs []RichTextSegment) {
	if len(segs) == 0 {
		return
	}
	from, to := e.caret, e.caret
	if f, t, ok := e.selection(); ok {
		from, to = f, t
	}
	e.mutate(from, to, func() {
		e.deleteSelectionRaw()
		e.caret = e.doc.insertSegments(e.caret, segs)
		e.anchor, e.selecting = e.caret, false
	})
}

// Keyboard returns the type of keyboard to show on mobile devices.
func (e *RichEntry) Keyboard() mobile.KeyboardType {
	return mobile.DefaultKeyboard
}

// Hide hides the editor, dismissing its mobile selection handles with it.
func (e *RichEntry) Hide() {
	if e.selHandles != nil {
		e.selHandles.hide()
	}
	e.hideCaretHandle()
	e.BaseWidget.Hide()
}

// --- IME composition (event.Composable / PreeditPositioner, #658) ---

// Compose sets or replaces the inline preedit text shown at the caret. It does
// not change the document, undo history or change callbacks.
func (e *RichEntry) Compose(ev *event.PreeditEvent) {
	if !e.composing {
		e.deleteSelection() // replace any selection when composition starts
		e.composing = true
	}
	e.preeditText = ev.Text
	e.preeditCursor = clampInt(ev.CursorPos, 0, len([]rune(ev.Text)))
	if e.preeditText == "" {
		e.composing = false
	}
	e.interruptCursor()
	e.syncProvider()
	e.Refresh()
}

// ComposeCommit finalizes composition, inserting the committed text as one edit.
func (e *RichEntry) ComposeCommit(text string) {
	e.clearPreedit()
	if text == "" {
		e.syncProvider()
		e.Refresh()
		return
	}
	e.insertRunes([]rune(text))
}

// ComposeCancel discards any in-progress composition without committing.
func (e *RichEntry) ComposeCancel() {
	if !e.composing {
		return
	}
	e.clearPreedit()
	e.syncProvider()
	e.Refresh()
}

func (e *RichEntry) clearPreedit() {
	e.composing = false
	e.preeditText = ""
	e.preeditCursor = 0
}

// PreeditCursorRectangle reports the caret rectangle so a driver can position the
// IME candidate window. It implements [event.PreeditPositioner].
func (e *RichEntry) PreeditCursorRectangle() geo.Rectangle {
	pos, size := e.caretRect()
	return geo.NewRectangle(pos.X, pos.Y, size.Width, size.Height)
}

// --- rendering ---

// displaySegments returns the segments the provider should render: the document
// content, plus the underlined preedit run injected at the caret while composing
// (the document itself is never modified by composition).
func (e *RichEntry) displaySegments() []RichTextSegment {
	if !e.composing || e.preeditText == "" {
		return e.doc.segments()
	}
	pre := &TextSegment{
		Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Underline: true}},
		Text:  e.preeditText,
	}
	segs := e.doc.segments()
	out := make([]RichTextSegment, 0, len(segs)+2)
	cum, inserted := 0, false
	for _, seg := range segs {
		n := segPositions(seg)
		if !inserted && e.caret >= cum && e.caret <= cum+n {
			local := e.caret - cum
			if ts, ok := seg.(*TextSegment); ok {
				r := []rune(ts.Text)
				if left := string(r[:local]); left != "" {
					out = append(out, &TextSegment{Style: ts.Style, Text: left})
				}
				out = append(out, pre)
				if right := string(r[local:]); right != "" {
					out = append(out, &TextSegment{Style: ts.Style, Text: right})
				}
				inserted = true
				cum += n
				continue
			}
			// atom boundary
			if local == 0 {
				out = append(out, pre)
				inserted = true
			}
			out = append(out, seg)
			if local > 0 && !inserted {
				out = append(out, pre)
				inserted = true
			}
			cum += n
			continue
		}
		out = append(out, seg)
		cum += n
	}
	if !inserted {
		out = append(out, pre)
	}
	return out
}

// caretDoc returns the document and caret position the caret geometry is computed
// against: normally the real document, or a display document (with the preedit
// spliced in) while composing so the caret tracks inside the preedit.
func (e *RichEntry) caretDoc() (*richEntryDoc, int) {
	if e.composing && e.preeditText != "" {
		// Build the caret doc from the *same* segment slice the provider laid out
		// (provider.Segments, set by syncProvider to displaySegments), not a fresh
		// displaySegments() call — buildWrapRows keys rows to segments by identity,
		// so a fresh preedit-spliced slice would not match the provider's rowBounds.
		disp := &richEntryDoc{segs: e.provider.Segments}
		if e.Wrapping != bellui.TextWrapOff {
			// Follow the wrap of the composed display so the candidate window is
			// positioned on the correct visual row of a wrapped document (#891).
			disp.setWrapRows(disp.buildWrapRows(&e.provider))
		}
		return disp, e.caret + e.preeditCursor
	}
	return e.doc, e.caret
}

func (e *RichEntry) syncProvider() {
	e.provider.ExtendBaseWidget(&e.provider)
	e.provider.Scroll = widget.ScrollNone
	e.provider.Segments = e.displaySegments()
	e.provider.Wrapping = e.Wrapping
	e.provider.Refresh()
	e.rebuildWrapRows()
}

// rebuildWrapRows refreshes the wrapping-aware row model from the provider's
// laid-out rowBounds when wrapping is on, so the caret/selection mapping follows
// soft-wrap splits (#653); with wrapping off it reverts to the fast newline-count
// model. It is skipped during IME composition, whose display document (preedit
// spliced in) is not d.segs — the caret then rides the composition temp doc
// (caretDoc), which is wrap-off; live wrapping resumes on commit.
func (e *RichEntry) rebuildWrapRows() {
	if e.Wrapping == bellui.TextWrapOff || e.composing {
		e.doc.setWrapRows(nil)
		return
	}
	e.doc.setWrapRows(e.doc.buildWrapRows(&e.provider))
}

func (e *RichEntry) syncSel() {
	if e.sel == nil {
		// host = e makes the selectable forward pointer events to this editor,
		// which owns the caret/selection model — otherwise the selectable child
		// shadows RichEntry's own Tapped/Dragged handlers and clicks never move
		// the caret (keyboard-only navigation).
		e.sel = &selectable{theme: e.Theme(), provider: &e.provider, focus: e, host: e}
		e.sel.ExtendBaseWidget(e.sel)
	}
	e.sel.theme = e.Theme()
	e.sel.focussed = e.focused
	e.sel.selecting = e.selecting && !e.composing // selection is suppressed while composing
	e.sel.cursorRow, e.sel.cursorColumn = e.doc.offsetToRowCol(e.caret)
	e.sel.selectRow, e.sel.selectColumn = e.doc.offsetToRowCol(e.anchor)
}

// rowHasInlineAtom reports whether the given rendered row contains an inline
// non-text atom (an inline image), whose width the text-only column space does
// not account for — so its caret geometry needs the segment walk (#665). A block
// image is not inline and takes the whole-row left-edge path instead.
func (e *RichEntry) rowHasInlineAtom(row int) bool {
	b := e.provider.rowBoundary(row)
	if b == nil {
		return false
	}
	for _, seg := range b.segments {
		if _, isText := seg.(*TextSegment); isText {
			continue
		}
		if seg.Inline() {
			return true
		}
	}
	return false
}

// caretXInlineWalk computes the caret x for a row that flows inline image atoms
// among text: it walks the row's segments in order, advancing by each text
// slice's measured width and each atom's rendered width, until it has consumed
// `col` positions (#665). This is the atom-aware replacement for
// lineSizeToColumn, which measures through images as if they were zero width.
func (e *RichEntry) caretXInlineWalk(row, col int, textSize, leadingX float32) float32 {
	innerPad := e.Theme().Size(theme.SizeNameInnerPadding)
	if x, ok := e.provider.inlineColumnX(row, col, textSize, innerPad); ok {
		return x + e.provider.GutterWidth
	}
	return leadingX
}

// caretRect returns the caret rectangle in the editor's content coordinates.
func (e *RichEntry) caretRect() (pos geo.Position, size geo.Size) {
	// During composition the provider renders the preedit inline, so the caret is
	// mapped against a display document (doc + preedit) at the preedit cursor.
	cd, caret := e.caretDoc()
	row, col := cd.offsetToRowCol(caret)
	return e.caretRectAt(row, col, cd.rowIsImage(row))
}

// caretRectAt returns the caret rectangle at an arbitrary (row, column) in the
// editor's content coordinates — the geometry shared by the caret visual and
// the mobile selection-handle edges (#888).
func (e *RichEntry) caretRectAt(row, col int, imageRow bool) (pos geo.Position, size geo.Size) {
	th := e.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	inputBorder := th.Size(theme.SizeNameInputBorder)
	textSize := th.Size(theme.SizeNameText)

	y, h := e.provider.RowGeometry(row)
	leadingX := innerPad - e.provider.inset.Width + e.provider.GutterWidth
	var x float32
	switch {
	case e.rowHasInlineAtom(row):
		// The row mixes text and inline image atoms, whose widths RichText's
		// text-only column space ignores. Under bidi the visual layout resolves
		// the position (#913); otherwise walk the row's segments LTR (#665).
		if bx, ok := e.provider.bidiInlineColumnX(row, col, innerPad, true); ok {
			x = bx + e.provider.GutterWidth
		} else {
			x = e.caretXInlineWalk(row, col, textSize, leadingX)
		}
	case imageRow:
		// A block image has no text column; sit the caret at the row's left edge
		// (before the image), matching where col 0 of a text row renders.
		x = leadingX
	case e.provider.rowLength(row) == 0:
		// An empty row (e.g. an empty field, or a blank line) has no glyphs, so its
		// only caret sits at the leading content edge. Measuring the row would return
		// the same leading x in software, but some GPU/mobile text paths report a
		// nonzero advance for an empty run and drop the caret at the trailing edge
		// (#749) — placing it directly is both correct and quirk-proof.
		x = leadingX
	default:
		before := e.provider.lineSizeToColumn(col, row, textSize, innerPad, true)
		x = before.Width + e.provider.GutterWidth
		if h <= 0 {
			h = before.Height
		}
	}
	if h <= 0 {
		h = e.provider.charMinSize(false, bellui.TextStyle{}, textSize).Height
	}
	return geo.NewPos(x-inputBorder/2, y), geo.NewSize(inputBorder, h)
}

// CreateRenderer is a private method to Bellui which links this widget to its
// renderer.
func (e *RichEntry) CreateRenderer() core.WidgetRenderer {
	e.ExtendBaseWidget(e)
	e.syncProvider()
	e.syncSel()

	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	box := canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	box.CornerRadius = th.Size(theme.SizeNameInputRadius)
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	border.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	border.CornerRadius = th.Size(theme.SizeNameInputRadius)

	cursor := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	cursor.Hide()
	e.cursorAnim = newEntryCursorAnimation(cursor)

	r := &richEntryRenderer{entry: e, box: box, border: border, cursor: cursor}
	// box behind the content; border stroked last so it always composites crisply
	// on top of the text/selection at the rounded edges (mirrors Entry).
	r.objects = []core.CanvasObject{box, e.sel, &e.provider, cursor, border}
	return r
}

var _ core.WidgetRenderer = (*richEntryRenderer)(nil)

type richEntryRenderer struct {
	entry       *RichEntry
	box, border *canvas.Rectangle
	cursor      *canvas.Rectangle
	objects     []core.CanvasObject
}

func (r *richEntryRenderer) Destroy() {
	if r.entry.cursorAnim != nil {
		r.entry.cursorAnim.stop()
	}
}

func (r *richEntryRenderer) Layout(size geo.Size) {
	th := r.entry.Theme()
	borderSize := th.Size(theme.SizeNameInputBorder)

	// 0.5 is removed so on low DPI it rounds down on the trailing edge (as Entry).
	r.border.Resize(geo.NewSize(size.Width-borderSize-.5, size.Height-borderSize-.5))
	r.border.StrokeWidth = borderSize
	r.border.Move(geo.NewSquareOffsetPos(borderSize / 2))
	r.box.Resize(size.Subtract(geo.NewSquareSize(borderSize * 2)))
	r.box.Move(geo.NewSquareOffsetPos(borderSize))

	// The content keeps the full-size coordinate space (the caret geometry is
	// computed against it); the provider's own inner padding (> the border) keeps
	// the text clear of the border stroke.
	r.entry.provider.Resize(size)
	r.entry.sel.Resize(size)

	// A width change rewraps the content, so the wrapping-aware row model must be
	// rebuilt from the freshly laid-out provider (Layout is not routed through
	// syncProvider) so the caret mapping tracks the new wrap points (#653).
	r.entry.rebuildWrapRows()
}

func (r *richEntryRenderer) MinSize() geo.Size {
	borderSize := r.entry.Theme().Size(theme.SizeNameInputBorder)
	return r.entry.provider.MinSize().AddWidthHeight(borderSize*2, borderSize*2)
}

func (r *richEntryRenderer) Objects() []core.CanvasObject {
	return r.objects
}

// refreshChrome updates the background box and border to reflect the style and
// focus state, mirroring Entry's refreshBoxAndBorder (minus validation/disabled,
// which RichEntry does not model yet).
func (r *richEntryRenderer) refreshChrome() {
	e := r.entry
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(e)
	isOutlined := e.Style == EntryStyleOutlined
	borderSize := th.Size(theme.SizeNameInputBorder)

	if isOutlined {
		r.box.FillColor = color.Transparent
	} else {
		r.box.FillColor = th.Color(theme.ColorNameInputBackground, v)
	}
	r.box.CornerRadius = th.Size(theme.SizeNameInputRadius)
	r.border.CornerRadius = r.box.CornerRadius
	r.border.StrokeWidth = borderSize

	if e.focused {
		r.border.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		if isOutlined || style == internaltheme.RenderStyleModern {
			r.border.StrokeWidth = borderSize * 2
		}
	} else if isOutlined {
		r.border.StrokeColor = th.Color(theme.ColorNameOutline, v)
	} else {
		r.border.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	}
	r.box.Refresh()
	r.border.Refresh()
}

func (r *richEntryRenderer) Refresh() {
	e := r.entry
	e.syncProvider()
	e.syncSel()
	e.sel.Refresh()
	r.refreshChrome()

	if e.focused {
		if bellui.CurrentApp().Settings().ShowAnimations() {
			e.cursorAnim.start()
		} else {
			r.cursor.FillColor = e.Theme().Color(theme.ColorNamePrimary, bellui.CurrentApp().Settings().ThemeVariant())
		}
		r.cursor.Show()
	} else {
		e.cursorAnim.stop()
		r.cursor.Hide()
	}

	pos, size := e.caretRect()
	r.cursor.Move(pos)
	r.cursor.Resize(size)

	// The central mobile selection-handle seam (#888): every selection or
	// focus mutation funnels through this Refresh (a no-op off mobile).
	e.updateSelectionHandles()
	// The caret drag-handle (#999) rides the same central Refresh seam.
	e.updateCaretHandle()

	canvas.Refresh(e.super())
}
