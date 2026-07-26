// Package textinput defines the neutral text-input mirror seam shared by every
// platform's native text-editing / IME integration (#768, epic #490).
//
// A [Session] binds the focused editor — any object implementing
// [core.AccessibleTexter], optionally [event.Composable] — to a native document
// [Backend], and moves state in both directions:
//
//   - push (editor → native): a (text, selection) [Snapshot] mirrored into the
//     native document whenever it changes; and
//   - edit ops (native → editor): insert / delete / replace / setSelection /
//     compose / commit, applied back through the editor's own gated,
//     undo-recorded edit paths.
//
// The seam is deliberately widget- and driver-agnostic: nothing here imports a
// concrete widget, a driver, or a platform API, and no [bellui.CanvasObject] type
// appears. Drivers own the [Backend] (the Android shadow EditText, an iOS
// UITextField document, a desktop accessibility text protocol) and drive the
// session — calling [Session.Bind] / [Session.Unbind] on focus change and
// [Session.Sync] after input events. This is Phase 0 of #768: the seam is
// defined and unit-tested, but no driver wires to it yet, so existing behavior
// is unchanged.
//
// A Session is not safe for concurrent use. Native callbacks that arrive on a
// platform UI thread must be marshalled onto the main goroutine (via bellui.Do)
// before touching the session, exactly as the drivers already marshal input —
// the seam holds no locks, matching the existing per-driver state fields it
// generalizes.
package textinput

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// Snapshot is the mirrored state of the focused editor: its full text and the
// half-open [SelStart, SelEnd) selection as rune offsets into Text. SelStart ==
// SelEnd is a bare caret at that offset. Offsets are rune indices, matching
// [core.AccessibleTexter]; a backend converts to its platform's code-unit space.
type Snapshot struct {
	Text     string
	SelStart int
	SelEnd   int
}

// View is the pull-side document view of an editor: its full text plus the
// caret and the half-open [SelStart, SelEnd) selection as rune offsets,
// captured live via [Read]. It is the read counterpart to the push [Snapshot]:
// backends that serve platform text-protocol queries rather than mirror pushes
// into a native document — the desktop accessibility text protocols — read the
// editor through [Read] instead of calling the [core.AccessibleTexter] methods
// ad-hoc, so every backend and the [Session] see one document view (#772).
type View struct {
	Text     string
	SelStart int
	SelEnd   int
	Caret    int
}

// Snapshot returns the push-side subset of the view — its text and selection.
func (v View) Snapshot() Snapshot {
	return Snapshot{Text: v.Text, SelStart: v.SelStart, SelEnd: v.SelEnd}
}

// Read captures the editor's current document view — text, selection and caret
// — in one place. It is the sole reader of a [core.AccessibleTexter]'s document
// state: [Session.Sync] takes its [Snapshot] from it, and the desktop
// accessibility text protocols read every text query through it.
func Read(ed core.AccessibleTexter) View {
	start, end := ed.AccessibleSelection()
	return View{Text: ed.AccessibleText(), SelStart: start, SelEnd: end, Caret: ed.AccessibleCaret()}
}

// ReadSelection reads only the editor's half-open [start, end) selection,
// skipping the document text. It is the read to use for a query that needs the
// selection but not the content — the desktop text protocols that keep rune
// offsets (the UIA Text pattern) — so a screen reader tracking the caret over a
// large document does not re-materialize the whole text per query (#789). Use
// [Read] where the text itself (or a rune↔code-unit conversion needing it) is
// required.
func ReadSelection(ed core.AccessibleTexter) (start, end int) {
	return ed.AccessibleSelection()
}

// The following package-level primitives are the seam's edit vocabulary over a
// [core.AccessibleTexter]. [Session]'s edit methods delegate to them (adding the
// bound-editor guard and the push), and the desktop accessibility text protocols
// call them directly, so an edit op has one definition regardless of the caller.

// InsertText replaces the editor's current selection with text — or inserts at
// the caret when nothing is selected — as one gated, undo-recorded edit.
func InsertText(ed core.AccessibleTexter, text string) {
	ed.ReplaceAccessibleSelection(text)
}

// ReplaceRange replaces the half-open [start, end) rune range with text as one
// gated, undo-recorded edit. Offsets are clamped by the editor.
func ReplaceRange(ed core.AccessibleTexter, start, end int, text string) {
	ed.SetAccessibleSelection(start, end)
	ed.ReplaceAccessibleSelection(text)
}

// ReplaceAll replaces the editor's entire content with text as one gated,
// undo-recorded edit — the whole-document replace behind the platform text
// protocols' "set value" operation.
func ReplaceAll(ed core.AccessibleTexter, text string) {
	ReplaceRange(ed, 0, len([]rune(ed.AccessibleText())), text)
}

// Select moves the editor's selection to the half-open [start, end) rune range
// (start == end places a bare caret), clamped by the editor.
func Select(ed core.AccessibleTexter, start, end int) {
	ed.SetAccessibleSelection(start, end)
}

// preeditIncluder is implemented by editors whose [core.AccessibleTexter] text
// reflects an in-progress IME preedit spliced into the document — widget.Entry
// splices the preedit through its provider so caret and selection offsets stay
// coherent with what is rendered. AccessiblePreedit reports that uncommitted
// preedit and the rune offset where it is spliced; ok is false while no preedit
// is shown. Editors whose accessible text is always committed-only (RichEntry's
// display-only preedit injection) simply do not implement the method.
//
// The seam matches it structurally (the TapThroughDismiss pattern) so widgets
// need not import this package.
//
// The push side uses it to subtract the preedit before mirroring (#1057): the
// native IME buffer holds committed text plus its own composing span, and every
// backend judges a push against that committed document — the Android side's
// composing-echo check and reseed logic are written for committed-only pushes,
// so a preedit-including push reads as "bellui holds a different document" and
// reseeds the IME buffer with the preedit baked in, the #1057 hardware-key
// doubling. The pull side ([Read]) is deliberately untouched: the desktop
// accessibility text protocols report the live text including marked/preedit
// text, per platform convention.
type preeditIncluder interface {
	AccessiblePreedit() (text string, start int, ok bool)
}

// committedSnapshot reads the editor's push [Snapshot] with any in-progress IME
// preedit subtracted, so pushes carry committed text only (#1057). Selection
// offsets inside the preedit collapse to its start (the composition anchor);
// offsets past it shift left by its length. When the editor does not report a
// preedit — or its report does not match the document (a split-brain state,
// e.g. a native rune committed mid-composition) — the snapshot is returned
// as-is rather than cutting the wrong span.
func committedSnapshot(ed core.AccessibleTexter) Snapshot {
	snap := Read(ed).Snapshot()
	p, ok := ed.(preeditIncluder)
	if !ok {
		return snap
	}
	preedit, start, active := p.AccessiblePreedit()
	if !active || preedit == "" {
		return snap
	}
	r := []rune(snap.Text)
	pr := []rune(preedit)
	end := start + len(pr)
	if start < 0 || end > len(r) || string(r[start:end]) != preedit {
		return snap
	}
	snap.Text = string(r[:start]) + string(r[end:])
	remap := func(o int) int {
		switch {
		case o >= end:
			return o - len(pr)
		case o > start:
			return start
		default:
			return o
		}
	}
	snap.SelStart, snap.SelEnd = remap(snap.SelStart), remap(snap.SelEnd)
	return snap
}

// Backend is the native document mirror a driver implements. The session pushes
// editor state to it via [Backend.SetState] and calls the session's edit-op
// methods from the native side when the user edits through the platform IME.
type Backend interface {
	// SetState mirrors the editor's current text and selection into the native
	// document. It is called on bind and whenever the pushed [Snapshot] changes
	// (see [Session.Sync]); redundant pushes are already filtered out.
	SetState(Snapshot)
}

// Session binds one focused editor to a [Backend] at a time and mirrors state
// between them. Create it once per backend with [NewSession]; rebind it to the
// focused editor on every focus change.
type Session struct {
	backend Backend

	editor core.AccessibleTexter // the bound editor; nil when nothing is bound
	comp   event.Composable      // set only when the bound editor supports IME

	last  Snapshot // last snapshot pushed to the backend
	valid bool     // whether last holds a real pushed snapshot
}

// NewSession returns a session that pushes editor state to backend. The backend
// must be non-nil.
func NewSession(backend Backend) *Session {
	return &Session{backend: backend}
}

// Bind attaches the session to the newly-focused object, taking effect only if
// it implements [core.AccessibleTexter]; any other object (or nil) unbinds the
// session. [event.Composable] is detected additionally to enable the IME ops.
//
// Bind forces the next [Session.Sync] to push even when the snapshot matches the
// last one, so focusing a different field with identical text (e.g. two empty
// entries) still repopulates the backend.
//
// Re-binding the editor that is already bound is a no-op, and in particular does
// NOT force that push: it is not "a different field", and drivers re-Bind the
// unchanged focus freely — the Android driver does so on every overlay change
// (#1016) — where a forced push would be pure churn against the native IME.
//
// The return value reports whether the bound editor's identity changed (a
// different field bound, or a bound session unbound) — false for the no-op
// re-bind. Drivers use it to tell the native input surface the editor changed,
// so it can tear down any IME session state that belongs to the previous field
// (Android: composing state, echo filters, and an unconditional restartInput on
// the seed push — #966; one Gboard editing session must never span two fields).
func (s *Session) Bind(focused any) (editorChanged bool) {
	ed, ok := focused.(core.AccessibleTexter)
	if !ok {
		wasBound := s.editor != nil
		s.Unbind()
		return wasBound
	}
	if s.editor == ed {
		return false
	}
	s.editor = ed
	s.comp, _ = focused.(event.Composable)
	s.valid = false
	return true
}

// Unbind detaches the session from the current editor. Edit ops and Sync become
// no-ops until the next [Session.Bind].
func (s *Session) Unbind() {
	s.editor, s.comp, s.valid = nil, nil, false
}

// Bound reports whether an editor is currently bound.
func (s *Session) Bound() bool { return s.editor != nil }

// composingReporter is implemented by editors that expose whether an IME
// composition is currently active (Entry/RichEntry report their composing
// flag). Matched structurally like [preeditIncluder]. The Android driver uses
// it to recognize a stale composition finalize (#1057): with hardware
// keyboards Gboard both composes each keystroke through the InputConnection
// and passes the raw key through, and the raw key commits the preedit
// editor-side (the desktop-commit TypedRune path) — so by the time Gboard
// finalizes the word with commitText, the editor is no longer composing and
// the commit's content is already in the document. Forwarding it would insert
// the whole word a second time.
type composingReporter interface {
	AccessibleComposing() bool
}

// EditorComposing reports whether the bound editor currently has an active IME
// composition. Editors that do not expose composition state — or a session
// with no composition-capable editor bound — report true, the conservative
// default: callers gate destructive decisions (dropping a commit) on a
// definitive "not composing", and an unknown state must never trigger them.
func (s *Session) EditorComposing() bool {
	cr, ok := s.comp.(composingReporter)
	if !ok {
		return true
	}
	return cr.AccessibleComposing()
}

// View captures the bound editor's live document view (see [Read]); ok is
// false when nothing is bound. Drivers use it to inspect the document around
// an incoming native edit — e.g. the Android IC-echo dedup diffs a reported
// replacement range against the pre-edit text (#1057).
func (s *Session) View() (View, bool) {
	if s.editor == nil {
		return View{}, false
	}
	return Read(s.editor), true
}

// Invalidate forces the next [Session.Sync] to push even if the snapshot is
// unchanged. Drivers call it when the editor's content may have changed outside
// the input path (e.g. a programmatic SetText) so the native mirror does not go
// stale.
func (s *Session) Invalidate() { s.valid = false }

// Sync reads the bound editor's current text and selection and, when it differs
// from the last pushed snapshot (or the session was just (re)bound or
// invalidated), pushes it to the backend. It is a no-op when nothing is bound.
//
// The pushed snapshot carries committed text only: an in-progress IME preedit
// the editor reflects in its accessible text is subtracted first (see
// [preeditIncluder], #1057), because the native mirror holds the composition
// itself and judges pushes against the committed document.
//
// This is the "push" direction; the redundant-push filter is the debounce.
// Drivers call Sync after each input event (key, composition, selection change),
// exactly where the mobile driver calls pushKeyboardState today.
func (s *Session) Sync() {
	if s.editor == nil {
		return
	}
	snap := committedSnapshot(s.editor)
	if s.valid && snap == s.last {
		return
	}
	s.last, s.valid = snap, true
	s.backend.SetState(snap)
}

// Insert replaces the current selection with text — or inserts at the caret when
// nothing is selected — as one undo-recorded edit, then pushes the new state.
// No-op when nothing is bound.
func (s *Session) Insert(text string) {
	if s.editor == nil {
		return
	}
	InsertText(s.editor, text)
	s.Sync()
}

// Delete removes the half-open [start, end) rune range as one edit, then pushes
// the new state. Offsets are clamped by the editor. No-op when nothing is bound.
func (s *Session) Delete(start, end int) {
	if s.editor == nil {
		return
	}
	ReplaceRange(s.editor, start, end, "")
	s.Sync()
}

// Replace replaces the half-open [start, end) rune range with text as one edit,
// then pushes the new state. Offsets are clamped by the editor. No-op when
// nothing is bound.
func (s *Session) Replace(start, end int, text string) {
	if s.editor == nil {
		return
	}
	ReplaceRange(s.editor, start, end, text)
	s.Sync()
}

// SetSelection moves the selection to the half-open [start, end) rune range
// (start == end places a bare caret), then pushes the new state. Offsets are
// clamped by the editor. No-op when nothing is bound.
func (s *Session) SetSelection(start, end int) {
	if s.editor == nil {
		return
	}
	Select(s.editor, start, end)
	s.Sync()
}

// Compose shows in-progress IME preedit text inline at the caret with cursor at
// the given rune offset within preedit. It does not alter the committed value,
// fire change callbacks or touch undo. No-op when the bound editor does not
// support composition (or nothing is bound).
func (s *Session) Compose(preedit string, cursor int) {
	if s.comp == nil {
		return
	}
	s.comp.Compose(&event.PreeditEvent{Text: preedit, CursorPos: cursor})
	s.Sync()
}

// Commit finalizes IME composition, inserting text as if typed (updating the
// value, undo history and change callbacks), then pushes the new state. No-op
// when the bound editor does not support composition (or nothing is bound).
func (s *Session) Commit(text string) {
	if s.comp == nil {
		return
	}
	s.comp.ComposeCommit(text)
	s.Sync()
}

// CancelCompose discards any in-progress composition without committing, then
// pushes the (unchanged committed) state. No-op when the bound editor does not
// support composition (or nothing is bound).
func (s *Session) CancelCompose() {
	if s.comp == nil {
		return
	}
	s.comp.ComposeCancel()
	s.Sync()
}
