package textinput

import (
	"testing"

	"github.com/belluikit/bellui/event"

	"github.com/stretchr/testify/assert"
)

// fakeEditor is a minimal but faithful core.AccessibleTexter: a rune buffer with
// a selection that honors the same clamp/splice contract Entry does, so tests
// assert real resulting text and selection rather than recorded mock calls.
type fakeEditor struct {
	runes      []rune
	selS, selE int
}

func newEditor(text string) *fakeEditor {
	r := []rune(text)
	return &fakeEditor{runes: r, selS: len(r), selE: len(r)}
}

func (f *fakeEditor) AccessibleText() string { return string(f.runes) }

func (f *fakeEditor) AccessibleSelection() (int, int) { return f.selS, f.selE }

func (f *fakeEditor) AccessibleCaret() int { return f.selE }

func (f *fakeEditor) SetAccessibleSelection(start, end int) {
	if start > end {
		start, end = end, start
	}
	n := len(f.runes)
	f.selS = clamp(start, 0, n)
	f.selE = clamp(end, 0, n)
}

func (f *fakeEditor) ReplaceAccessibleSelection(text string) {
	repl := []rune(text)
	next := make([]rune, 0, len(f.runes)-(f.selE-f.selS)+len(repl))
	next = append(next, f.runes[:f.selS]...)
	next = append(next, repl...)
	next = append(next, f.runes[f.selE:]...)
	caret := f.selS + len(repl)
	f.runes, f.selS, f.selE = next, caret, caret
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// composingEditor adds IME composition on top of fakeEditor, recording the calls
// the session routes to it. It satisfies event.Composable (which embeds
// event.Focusable).
type composingEditor struct {
	fakeEditor
	preedits  []event.PreeditEvent
	commits   []string
	cancelled int
}

func newComposingEditor(text string) *composingEditor {
	return &composingEditor{fakeEditor: *newEditor(text)}
}

func (c *composingEditor) Compose(p *event.PreeditEvent) { c.preedits = append(c.preedits, *p) }
func (c *composingEditor) ComposeCommit(s string)        { c.commits = append(c.commits, s) }
func (c *composingEditor) ComposeCancel()                { c.cancelled++ }

// event.Focusable, unused by the seam but required to satisfy Composable.
func (c *composingEditor) FocusGained()             {}
func (c *composingEditor) FocusLost()               {}
func (c *composingEditor) TypedRune(rune)           {}
func (c *composingEditor) TypedKey(*event.KeyEvent) {}

// preeditEditor mimics Entry's composition model on top of composingEditor: the
// preedit is spliced into the accessible text at its anchor (so AccessibleText
// mixes committed and composing runes, #1057) and reported via
// AccessiblePreedit, with the caret inside the preedit.
type preeditEditor struct {
	composingEditor
	preedit       string
	preeditAnchor int
	composingNow  bool // reported via AccessibleComposing
}

func (p *preeditEditor) AccessibleComposing() bool { return p.composingNow }

func newPreeditEditor(text string) *preeditEditor {
	return &preeditEditor{composingEditor: *newComposingEditor(text)}
}

// setPreedit splices preedit into the document at the current anchor (replacing
// any previous preedit) and parks the caret after it, like Entry.Compose.
func (p *preeditEditor) setPreedit(preedit string) {
	if p.preedit == "" {
		p.preeditAnchor = p.selE // anchor at the caret, like Entry
	}
	r := p.runes
	old := len([]rune(p.preedit))
	next := make([]rune, 0, len(r)-old+len([]rune(preedit)))
	next = append(next, r[:p.preeditAnchor]...)
	next = append(next, []rune(preedit)...)
	next = append(next, r[p.preeditAnchor+old:]...)
	p.runes = next
	p.preedit = preedit
	caret := p.preeditAnchor + len([]rune(preedit))
	p.selS, p.selE = caret, caret
}

// commitPreedit finalizes the composition: the spliced runes stay as committed
// text and the preedit report clears, like Entry.ComposeCommit.
func (p *preeditEditor) commitPreedit() {
	caret := p.preeditAnchor + len([]rune(p.preedit))
	p.preedit = ""
	p.selS, p.selE = caret, caret
}

func (p *preeditEditor) AccessiblePreedit() (string, int, bool) {
	if p.preedit == "" {
		return "", 0, false
	}
	return p.preedit, p.preeditAnchor, true
}

// recordingBackend captures every pushed snapshot.
type recordingBackend struct{ states []Snapshot }

func (b *recordingBackend) SetState(s Snapshot) { b.states = append(b.states, s) }

func (b *recordingBackend) last() Snapshot { return b.states[len(b.states)-1] }

func TestSession_BindNonTexterUnbinds(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)

	s.Bind("not an editor")
	assert.False(t, s.Bound())

	s.Sync() // no editor bound
	assert.Empty(t, b.states)
}

func TestSession_SyncPushesAndDedups(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")

	s.Bind(ed)
	assert.True(t, s.Bound())

	s.Sync()
	assert.Equal(t, []Snapshot{{Text: "hello", SelStart: 5, SelEnd: 5}}, b.states)

	s.Sync() // unchanged → deduped
	assert.Len(t, b.states, 1)
}

func TestSession_BindForcesPushEvenWhenIdentical(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)

	s.Bind(newEditor(""))
	s.Sync()
	assert.Len(t, b.states, 1)

	// A second, distinct empty field: identical snapshot must still push.
	s.Bind(newEditor(""))
	s.Sync()
	assert.Len(t, b.states, 2)
}

// Re-binding the already-bound editor must not force a push: drivers re-Bind the
// unchanged focus freely (the Android driver does so on every overlay change,
// #1016), and a forced redundant push is churn against the native IME.
func TestSession_RebindSameEditorDoesNotForcePush(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")

	s.Bind(ed)
	s.Sync()
	assert.Len(t, b.states, 1)

	s.Bind(ed)
	s.Sync()
	assert.Len(t, b.states, 1, "re-binding the same editor must not re-push")

	// The redundant-push filter still tracks real edits after a re-bind.
	ed.runes = []rune("hello world")
	s.Sync()
	assert.Len(t, b.states, 2)
}

// Bind reports editor-identity changes (#966): drivers use the signal to reset
// per-editor native IME session state (Android composing flag + echo filters,
// and the unconditional restartInput on the seed push), so one IME session can
// never span two fields.
func TestSession_BindReportsEditorChange(t *testing.T) {
	s := NewSession(&recordingBackend{})
	a, b := newEditor("a"), newEditor("b")

	assert.True(t, s.Bind(a), "first bind is an editor change")
	assert.False(t, s.Bind(a), "re-binding the same editor is not")
	assert.True(t, s.Bind(b), "a different editor is")
	assert.True(t, s.Bind("not an editor"), "unbinding a bound session is")
	assert.False(t, s.Bind(nil), "unbinding an unbound session is not")
	assert.True(t, s.Bind(a), "binding after unbind is")
}

func TestSession_Insert(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("")
	s.Bind(ed)

	s.Insert("hi")
	assert.Equal(t, "hi", ed.AccessibleText())
	assert.Equal(t, Snapshot{Text: "hi", SelStart: 2, SelEnd: 2}, b.last())
}

func TestSession_InsertReplacesSelection(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("abcd")
	s.Bind(ed)

	s.SetSelection(1, 3) // select "bc"
	s.Insert("X")
	assert.Equal(t, "aXd", ed.AccessibleText())
	assert.Equal(t, Snapshot{Text: "aXd", SelStart: 2, SelEnd: 2}, b.last())
}

func TestSession_Delete(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")
	s.Bind(ed)

	s.Delete(1, 3) // remove "el"
	assert.Equal(t, "hlo", ed.AccessibleText())
	assert.Equal(t, Snapshot{Text: "hlo", SelStart: 1, SelEnd: 1}, b.last())
}

func TestSession_Replace(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")
	s.Bind(ed)

	s.Replace(0, 5, "world")
	assert.Equal(t, "world", ed.AccessibleText())
	assert.Equal(t, Snapshot{Text: "world", SelStart: 5, SelEnd: 5}, b.last())
}

func TestSession_SetSelectionClampsAndPushes(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hi")
	s.Bind(ed)

	s.SetSelection(-4, 99) // clamped to [0, 2)
	assert.Equal(t, Snapshot{Text: "hi", SelStart: 0, SelEnd: 2}, b.last())
}

func TestSession_ComposeCommitCancelRouteToComposable(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newComposingEditor("")
	s.Bind(ed)

	s.Compose("ni", 2)
	assert.Equal(t, []event.PreeditEvent{{Text: "ni", CursorPos: 2}}, ed.preedits)

	s.Commit("你")
	assert.Equal(t, []string{"你"}, ed.commits)

	s.CancelCompose()
	assert.Equal(t, 1, ed.cancelled)
}

func TestSession_ComposeNoOpWithoutComposable(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("x") // plain AccessibleTexter, no Composable
	s.Bind(ed)

	// Must not panic and must not mutate the editor.
	s.Compose("ni", 2)
	s.Commit("你")
	s.CancelCompose()
	assert.Equal(t, "x", ed.AccessibleText())
}

// The pushed snapshot must carry committed text only (#1057): an editor that
// splices its IME preedit into the accessible text (Entry) reports the span via
// AccessiblePreedit and the seam subtracts it, so the native IME buffer is
// never reseeded with composing runes baked in.
func TestSession_SyncSubtractsPreedit(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newPreeditEditor("x")
	s.Bind(ed)

	s.Sync()
	assert.Equal(t, Snapshot{Text: "x", SelStart: 1, SelEnd: 1}, b.last())

	ed.setPreedit("ni") // document reads "xni", caret 3
	assert.Equal(t, "xni", ed.AccessibleText())
	s.Sync()
	// Committed-only push: preedit gone, caret collapsed to the anchor — which
	// equals the last push, so nothing new is sent at all.
	assert.Len(t, b.states, 1, "composing must not generate a divergent push")

	ed.setPreedit("nih") // composition grows; committed document unchanged
	s.Sync()
	assert.Len(t, b.states, 1)

	ed.commitPreedit() // "nih" becomes committed text
	s.Sync()
	assert.Equal(t, Snapshot{Text: "xnih", SelStart: 4, SelEnd: 4}, b.last())
}

// Selection offsets past the subtracted preedit shift left; offsets inside it
// collapse to the composition anchor.
func TestSession_SyncSubtractsPreeditRemapsSelection(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newPreeditEditor("ab")
	ed.SetAccessibleSelection(1, 1)
	s.Bind(ed)

	ed.setPreedit("ni") // document "anib", preedit at [1,3)
	ed.selS, ed.selE = 0, 4
	s.Sync()
	assert.Equal(t, Snapshot{Text: "ab", SelStart: 0, SelEnd: 2}, b.last())

	ed.selS, ed.selE = 2, 2 // inside the preedit → anchor
	s.Sync()
	assert.Equal(t, Snapshot{Text: "ab", SelStart: 1, SelEnd: 1}, b.last())
}

// A preedit report that does not match the document — the split-brain states
// #1057 fix B targets — must not cut the wrong span: the push falls back to the
// live text as-is.
func TestSession_SyncPreeditMismatchPushesAsIs(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newPreeditEditor("x")
	s.Bind(ed)

	ed.setPreedit("ni")
	ed.runes = []rune("xno") // document diverged from the preedit report
	s.Sync()
	assert.Equal(t, Snapshot{Text: "xno", SelStart: 3, SelEnd: 3}, b.last())

	// Out-of-bounds anchor likewise falls back.
	ed.runes = []rune("xni")
	ed.preeditAnchor = 2 // span [2,4) exceeds the document
	s.Sync()
	assert.Equal(t, Snapshot{Text: "xni", SelStart: 3, SelEnd: 3}, b.last())
}

// EditorComposing reports the bound editor's composition state, defaulting to
// true whenever it cannot know — the Android driver drops an IME commit only
// on a definitive "not composing" (#1057), and an unknown editor must never
// lose a commit.
func TestSession_EditorComposing(t *testing.T) {
	s := NewSession(&recordingBackend{})

	assert.True(t, s.EditorComposing(), "unbound session must report composing (conservative)")

	s.Bind(newComposingEditor("")) // Composable without AccessibleComposing
	assert.True(t, s.EditorComposing(), "editor without the reporter must report composing")

	ed := newPreeditEditor("")
	s.Bind(ed)
	assert.False(t, s.EditorComposing())
	ed.composingNow = true
	assert.True(t, s.EditorComposing())
}

func TestSession_OpsNoOpWhenUnbound(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")
	s.Bind(ed)
	s.Unbind()
	assert.False(t, s.Bound())

	s.Insert("x")
	s.Delete(0, 1)
	s.Replace(0, 1, "y")
	s.SetSelection(0, 1)
	assert.Equal(t, "hello", ed.AccessibleText())
	assert.Empty(t, b.states)
}

// The package-level primitives below are the seam's shared document-view read
// and edit vocabulary — the same code the desktop accessibility text protocols
// call directly (#772). Session's methods delegate to them, so these lock the
// contract both callers rely on.

func TestRead_CapturesDocumentView(t *testing.T) {
	ed := newEditor("hello")
	ed.SetAccessibleSelection(1, 4) // select "ell"; caret at end (selE)

	v := Read(ed)
	assert.Equal(t, View{Text: "hello", SelStart: 1, SelEnd: 4, Caret: 4}, v)
	// The push subset drops the caret.
	assert.Equal(t, Snapshot{Text: "hello", SelStart: 1, SelEnd: 4}, v.Snapshot())
}

// countingEditor wraps fakeEditor to count AccessibleText reads, proving
// ReadSelection does not touch the document (#789).
type countingEditor struct {
	fakeEditor
	textReads int
}

func (c *countingEditor) AccessibleText() string {
	c.textReads++
	return c.fakeEditor.AccessibleText()
}

func TestReadSelection_DoesNotReadText(t *testing.T) {
	ed := &countingEditor{fakeEditor: *newEditor("hello world")}
	ed.SetAccessibleSelection(2, 7)

	start, end := ReadSelection(ed)
	assert.Equal(t, 2, start)
	assert.Equal(t, 7, end)
	assert.Zero(t, ed.textReads, "ReadSelection must not materialize the document")

	// Read, by contrast, does read the text.
	_ = Read(ed)
	assert.Equal(t, 1, ed.textReads)
}

func TestInsertText_ReplacesSelectionOrCaret(t *testing.T) {
	ed := newEditor("abcd")
	ed.SetAccessibleSelection(1, 3) // "bc"
	InsertText(ed, "X")
	assert.Equal(t, "aXd", ed.AccessibleText())

	InsertText(ed, "Y") // nothing selected → insert at caret
	assert.Equal(t, "aXYd", ed.AccessibleText())
}

func TestReplaceRange_And_Select(t *testing.T) {
	ed := newEditor("hello")
	ReplaceRange(ed, 1, 3, "X") // remove "el", insert "X"
	assert.Equal(t, "hXlo", ed.AccessibleText())

	Select(ed, 0, 4)
	s, e := ed.AccessibleSelection()
	assert.Equal(t, 0, s)
	assert.Equal(t, 4, e)
}

func TestReplaceAll_ReplacesWholeDocument(t *testing.T) {
	ed := newEditor("hello")
	ed.SetAccessibleSelection(2, 2) // caret mid-doc must not matter
	ReplaceAll(ed, "world!")
	assert.Equal(t, "world!", ed.AccessibleText())

	ReplaceAll(ed, "") // clear
	assert.Equal(t, "", ed.AccessibleText())
}

func TestSession_InvalidateForcesPush(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")
	s.Bind(ed)

	s.Sync()
	assert.Len(t, b.states, 1)

	s.Invalidate() // e.g. after a programmatic SetText that kept the same value
	s.Sync()
	assert.Len(t, b.states, 2)
}

// TestSession_SyncPushesOutOfBandEditorChange pins the #763 invariant: when the
// bound editor's text changes out of band — a programmatic SetText, a binding
// write, undo/redo, autocomplete — with no intervening input op, the next Sync
// (which the driver calls every dirty paint frame) reads the live editor and
// pushes the new state so the native mirror never goes stale. This is the seam
// half of the fix; the driver half is the per-frame syncKeyboardState() call.
func TestSession_SyncPushesOutOfBandEditorChange(t *testing.T) {
	b := &recordingBackend{}
	s := NewSession(b)
	ed := newEditor("hello")
	s.Bind(ed)

	s.Sync()
	assert.Equal(t, []Snapshot{{Text: "hello", SelStart: 5, SelEnd: 5}}, b.states)

	// Editor mutated directly, as a programmatic SetText would (no seam edit op,
	// no Invalidate) — the value genuinely differs from the last push.
	ed.runes = []rune("hello world")
	ed.selS, ed.selE = 11, 11

	s.Sync()
	assert.Len(t, b.states, 2, "next Sync must push the out-of-band change")
	assert.Equal(t, Snapshot{Text: "hello world", SelStart: 11, SelEnd: 11}, b.last())
}
