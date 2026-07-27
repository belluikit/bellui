package editor

import (
	"context"
	"image/color"
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func editorWithWords(t *testing.T, words ...string) *Editor {
	t.Helper()
	e := renderedPopoverEditor(t, "")
	e.SetCompletionProvider(&WordListCompletionProvider{Words: words})
	return e
}

func typeRunes(e *Editor, s string) {
	for _, r := range s {
		e.entry.TypedRune(r)
	}
}

func shownLabels(e *Editor) []string {
	out := make([]string, len(e.completion.shown))
	for i, it := range e.completion.shown {
		out[i] = completionLabel(it)
	}
	return out
}

func TestCompletion_TypingShowsAndFilters(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "print", "println", "property", "range", "return")

	typeRunes(e, "pr")
	if !e.completion.visible() {
		t.Fatal("completion popup should be visible after typing 'pr'")
	}
	got := shownLabels(e)
	want := map[string]bool{"print": true, "println": true, "property": true}
	if len(got) != len(want) {
		t.Fatalf("want %d matches, got %v", len(want), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected match %q in %v", g, got)
		}
	}

	// Live-narrow: 'i' excludes "property".
	typeRunes(e, "i")
	got = shownLabels(e)
	want = map[string]bool{"print": true, "println": true}
	if len(got) != len(want) {
		t.Fatalf("after 'pri' want %d matches, got %v", len(want), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected match %q in %v", g, got)
		}
	}
}

func TestCompletion_PopupHasPositiveSize(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "alpha", "alpine", "beta")
	typeRunes(e, "al")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	if sz := e.completion.popup.MinSize(); sz.Width <= 0 || sz.Height <= 0 {
		t.Errorf("popup should size to its content, got %v", sz)
	}
}

func TestCompletion_AcceptReplacesPrefix(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "print", "println")

	typeRunes(e, "pr")
	// Top match is "print"; move down to "println" and accept with Enter.
	e.completion.handleKey(&event.KeyEvent{Name: event.KeyDown})
	e.completion.handleKey(&event.KeyEvent{Name: event.KeyReturn})

	if e.completion.visible() {
		t.Error("popup should close after accept")
	}
	if e.Text() != "println" {
		t.Errorf("accept should replace the prefix: want %q, got %q", "println", e.Text())
	}
	if r, c := e.CursorPos(); r != 0 || c != len("println") {
		t.Errorf("cursor should sit after the inserted text (0,%d), got (%d,%d)", len("println"), r, c)
	}
}

func TestCompletion_AcceptFiresOnChanged(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "println")
	var last string
	e.OnChanged = func(s string) { last = s }

	typeRunes(e, "pr")
	e.completion.handleKey(&event.KeyEvent{Name: event.KeyEnter})

	if last != "println" {
		t.Errorf("accept should fire OnChanged with the new text: got %q", last)
	}
}

func TestCompletion_TabAccepts(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "range")
	typeRunes(e, "ra")
	e.completion.handleKey(&event.KeyEvent{Name: event.KeyTab})
	if e.Text() != "range" {
		t.Errorf("Tab should accept: want %q, got %q", "range", e.Text())
	}
}

func TestCompletion_EscapeDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "print")
	typeRunes(e, "pr")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	// Escape routes through the popover infra (the key handler declines it).
	e.entry.onKey(&event.KeyEvent{Name: event.KeyEscape})
	if e.completion.visible() {
		t.Error("Escape should dismiss completion")
	}
	if e.Text() != "pr" {
		t.Errorf("dismiss must not alter text, got %q", e.Text())
	}
}

func TestCompletion_DismissesWhenPrefixEmpties(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "print")
	typeRunes(e, "p")
	if !e.completion.visible() {
		t.Fatal("popup should be visible after 'p'")
	}
	// Delete the prefix → completion dismisses.
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	if e.completion.visible() {
		t.Error("emptying the identifier prefix should dismiss completion")
	}
}

func TestCompletion_InvokeShowsAll(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "alpha", "beta", "gamma")
	// No prefix typed; explicit invoke (Ctrl+Space) shows everything.
	e.completion.invoke()
	if !e.completion.visible() {
		t.Fatal("invoke should show the popup with no prefix")
	}
	if len(e.completion.shown) != 3 {
		t.Errorf("invoke should show all 3 words, got %v", shownLabels(e))
	}
}

// itemProvider returns a fixed list of CompletionItems verbatim, so a test can
// exercise SortText/label ordering the WordListCompletionProvider cannot express.
type itemProvider struct{ items []CompletionItem }

func (p *itemProvider) Complete(_ context.Context, _, _ int, _ CompletionTrigger) ([]CompletionItem, error) {
	return p.items, nil
}

// TestCompletion_ScoredLessTieBreak pins the SortProxy comparator (#165): score
// wins first, then SortText, then label — the string tie-breaks resolved by the
// active locale's collation (lang.Compare) rather than raw byte order.
func TestCompletion_ScoredLessTieBreak(t *testing.T) {
	test.NewTempApp(t)

	hi := scoredCompletion{item: CompletionItem{Label: "z"}, score: 10}
	lo := scoredCompletion{item: CompletionItem{Label: "a"}, score: 1}
	if !scoredCompletionLess(hi, lo) {
		t.Error("higher score must sort first regardless of label")
	}

	// Equal score: SortText decides ahead of the label.
	a := scoredCompletion{item: CompletionItem{Label: "zzz", SortText: "1"}, score: 5}
	b := scoredCompletion{item: CompletionItem{Label: "aaa", SortText: "2"}, score: 5}
	if !scoredCompletionLess(a, b) {
		t.Error("equal score must tie-break on SortText before label")
	}

	// Equal score and SortText: the label decides.
	c := scoredCompletion{item: CompletionItem{Label: "apple"}, score: 5}
	d := scoredCompletion{item: CompletionItem{Label: "banana"}, score: 5}
	if !scoredCompletionLess(c, d) {
		t.Error("equal score and SortText must tie-break on label")
	}
}

// TestCompletion_SortTextOrdersEqualScores drives the ordering end-to-end through
// the data/model pipeline: with an empty prefix every candidate scores 0, so the
// displayed order is the SortText tie-break.
func TestCompletion_SortTextOrdersEqualScores(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetCompletionProvider(&itemProvider{items: []CompletionItem{
		{Label: "gamma", SortText: "3"},
		{Label: "alpha", SortText: "1"},
		{Label: "beta", SortText: "2"},
	}})

	e.completion.invoke() // no prefix → all score 0 → order is SortText
	got := shownLabels(e)
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %q, want %q (full order %v)", i, got[i], want[i], got)
		}
	}
}

func TestCompletion_NoProviderUnchanged(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	typeRunes(e, "pr")
	if e.completion.visible() {
		t.Error("no popup should appear without a provider")
	}
	if e.Text() != "pr" {
		t.Errorf("typing should behave normally, got %q", e.Text())
	}
}

func TestCompletion_ClearingProviderDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "print")
	typeRunes(e, "pr")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	e.SetCompletionProvider(nil)
	if e.completion.visible() {
		t.Error("clearing the provider should dismiss completion")
	}
}

func TestCompletion_TriggerCharacterOpensWithEmptyPrefix(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "field", "method", "value")
	e.SetCompletionTriggerCharacters('.')

	// Typing "obj" then "." ends the identifier (empty prefix), but "." is a
	// trigger character so completion opens anchored at the cursor.
	typeRunes(e, "obj.")
	if !e.completion.visible() {
		t.Fatal("trigger character '.' should open completion with an empty prefix")
	}
	if len(e.completion.shown) != 3 {
		t.Errorf("member completion should show all 3 candidates, got %v", shownLabels(e))
	}

	// Continuing to type narrows the open list against the new prefix.
	typeRunes(e, "me")
	if got := shownLabels(e); len(got) != 1 || got[0] != "method" {
		t.Errorf("typing after the trigger should filter members: got %v", got)
	}
}

func TestCompletion_TriggerCharacterUnconfiguredDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "field", "method")
	// No trigger characters declared: '.' behaves as before (empty prefix dismisses).
	typeRunes(e, "obj.")
	if e.completion.visible() {
		t.Error("without configured trigger characters, '.' should not open completion")
	}
}

func TestCompletion_ClearingTriggerCharacters(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "field")
	e.SetCompletionTriggerCharacters('.')
	e.SetCompletionTriggerCharacters() // clear
	typeRunes(e, "obj.")
	if e.completion.visible() {
		t.Error("clearing trigger characters should restore the default dismiss behavior")
	}
}

// triggerWordProvider is a WordListCompletionProvider that also declares its own
// trigger characters via CompletionTriggerProvider.
type triggerWordProvider struct {
	WordListCompletionProvider
	triggers []rune
}

func (p *triggerWordProvider) TriggerCharacters() []rune { return p.triggers }

func TestCompletion_ProviderDeclaredTriggerCharacters(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	// No editor-level triggers set; the provider declares "." itself.
	e.SetCompletionProvider(&triggerWordProvider{
		WordListCompletionProvider: WordListCompletionProvider{Words: []string{"field", "method"}},
		triggers:                   []rune{'.'},
	})

	typeRunes(e, "obj.")
	if !e.completion.visible() {
		t.Fatal("a provider-declared trigger character should open completion with no host wiring")
	}
}

func TestCompletion_TriggerCharactersUnionEditorAndProvider(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetCompletionProvider(&triggerWordProvider{
		WordListCompletionProvider: WordListCompletionProvider{Words: []string{"x"}},
		triggers:                   []rune{'.'},
	})
	e.SetCompletionTriggerCharacters('>')

	if !e.completion.isTriggerChar('.') {
		t.Error("provider-declared '.' should be a trigger character")
	}
	if !e.completion.isTriggerChar('>') {
		t.Error("editor-set '>' should be a trigger character")
	}
	if e.completion.isTriggerChar('x') {
		t.Error("an undeclared character should not trigger")
	}

	// Clearing the provider drops its declared triggers but keeps the editor set.
	e.SetCompletionProvider(nil)
	if e.completion.isTriggerChar('.') {
		t.Error("clearing the provider should drop its declared triggers")
	}
	if !e.completion.isTriggerChar('>') {
		t.Error("clearing the provider must not clear the editor-set triggers")
	}
}

// callSiteEditor builds an editor (with completion + signature providers, and
// "(" as a completion trigger) in a window tall enough that, with the cursor
// mid-document, the signature card has room above the line and the completion
// list room below — so both can be shown at once.
func callSiteEditor(t *testing.T) *Editor {
	t.Helper()
	e := NewEditor()
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"arg1", "arg2"}})
	e.SetCompletionTriggerCharacters('(')
	e.SetSignatureHelpProvider(&FuncSignatureProvider{
		Funcs: map[string]SignatureInformation{"foo": {Label: "foo(a, b)"}},
		Text:  func() string { return e.Text() },
	})
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(600, 600))
	return e
}

func TestCompletion_SignatureAndCompletionCoexistAtCallSite(t *testing.T) {
	test.NewTempApp(t)
	e := callSiteEditor(t)

	// Place the call well down the document so the card has ample room above the
	// line and the list ample room below (in the 600px-tall window).
	e.SetText(strings.Repeat("\n", 18))
	e.SetCursorPos(14, 0)
	typeRunes(e, "foo(")

	if !e.signature.visible() {
		t.Fatal("signature card should be visible at the call site")
	}
	if !e.completion.visible() {
		t.Fatal("completion should show alongside the signature card (#79)")
	}
	if e.signature.pop.placement != placeAbove {
		t.Errorf("signature card should sit above the line, got %v", e.signature.pop.placement)
	}
	// The overlay popup places below the caret when there is room: its reported
	// editor-local rect starts at or below the anchor cell's bottom edge.
	anchor := e.PositionAt(e.completion.anchorRow, e.completion.anchorCol)
	if r := e.completion.rect; r.IsZero() || r.Pos.Y < anchor.Pos.Y+anchor.Size.Height {
		t.Errorf("completion popup should sit below the line, rect %v vs anchor %v", r, anchor)
	}
}

func TestCompletion_CompletionWinsWhenNoRoomForBoth(t *testing.T) {
	test.NewTempApp(t)
	e := callSiteEditor(t)

	// Call on the top row: no room above, so the card would flip below and collide
	// with the completion list — the companion yields and completion stays primary.
	typeRunes(e, "foo(")
	if !e.completion.visible() {
		t.Error("completion should win in tight space")
	}
	if e.signature.visible() {
		t.Error("the signature companion should yield when it cannot be placed clear of completion")
	}
}

func TestCompletion_ProviderTriggersReadDynamically(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	// Provider installed before it knows its triggers (e.g. pre-LSP-handshake).
	p := &triggerWordProvider{
		WordListCompletionProvider: WordListCompletionProvider{Words: []string{"field"}},
		triggers:                   nil,
	}
	e.SetCompletionProvider(p)

	typeRunes(e, "obj.")
	if e.completion.visible() {
		t.Fatal("'.' must not trigger before the provider declares it")
	}

	// Async capabilities arrive: the provider now reports ".".
	p.triggers = []rune{'.'}
	typeRunes(e, "x.")
	if !e.completion.visible() {
		t.Error("provider triggers should be read on demand — no reinstall needed (#80)")
	}
}

func TestCompletion_ParenTriggerCompletesWithoutSignatureProvider(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "arg1", "arg2")
	// "(" registered as a completion trigger and NO signature provider: completion
	// owns "(" and opens argument completion.
	e.SetCompletionTriggerCharacters('(')

	typeRunes(e, "foo(")
	if !e.completion.visible() {
		t.Error("without a signature provider, '(' should open completion")
	}
}

// docProvider returns a fixed candidate list, used to exercise the documentation
// side panel (#24).
type docProvider struct{ items []CompletionItem }

func (p *docProvider) Complete(_ context.Context, _, _ int, _ CompletionTrigger) ([]CompletionItem, error) {
	return p.items, nil
}

// countingProvider records how many times it is queried so a test can assert an
// edit did (or did not) trigger completion.
type countingProvider struct {
	items []CompletionItem
	calls int
}

func (p *countingProvider) Complete(_ context.Context, _, _ int, _ CompletionTrigger) ([]CompletionItem, error) {
	p.calls++
	return p.items, nil
}

// Regression for the downstream symptom of #108. Before masked spans were
// atomic, a boundary insert at a masked secret's edge was the one edit inside
// the secret region the editor allowed, so every keystroke there fired
// OnChanged -> a completion query, flashing a popup over the secret per
// keystroke. With the edge frozen the edit is vetoed, OnChanged never fires, and
// completion is never queried.
func TestCompletion_MaskedSecretEdgeDoesNotTrigger(t *testing.T) {
	const sample = `key = "ABCDE"` // secret "ABCDE" at columns [7, 12)
	e := renderedPopoverEditor(t, sample)
	p := &countingProvider{items: []CompletionItem{{Label: "ABCDEF"}, {Label: "ABCDEFG"}}}
	e.SetCompletionProvider(p)
	e.SetRedactions([]TextRange{{StartRow: 0, StartCol: 7, EndRow: 0, EndCol: 12}})

	for _, col := range []int{7, 12} { // start edge, then end edge
		e.entry.CursorRow, e.entry.CursorColumn = 0, col
		e.entry.TypedRune('A')
	}

	if e.Text() != sample {
		t.Errorf("typing at a masked edge must be vetoed, got %q", e.Text())
	}
	if p.calls != 0 {
		t.Errorf("a vetoed masked-edge edit must not query completion, got %d queries", p.calls)
	}
	if e.completion.visible() {
		t.Error("no completion popup should show for a vetoed masked-edge edit")
	}

	// Sanity: the provider is live — an edit clear of the secret still queries it,
	// so calls==0 above reflects the veto, not a dead provider.
	e.entry.CursorRow, e.entry.CursorColumn = 0, len([]rune(sample))
	e.entry.TypedRune('A')
	if p.calls == 0 {
		t.Fatal("an edit outside the masked span should query completion (wiring check)")
	}
}

// Documentation-panel rendering (content, markdown parsing, min height,
// selection-follow) is the fork popup's own behavior, tested in
// widget/completion_popup_internal_test.go (fyne#435). fycoder asserts its
// side of the contract: docs are mapped through to the popup, and the paging
// keys are consumed exactly when a documentation panel is showing.

func TestCompletion_DocPanelHiddenWithoutDocs(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithWords(t, "alpha", "alpine") // WordList items carry no Documentation
	typeRunes(e, "al")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	if e.completion.handleKey(&event.KeyEvent{Name: event.KeyPageDown}) {
		t.Error("PageDown must fall through when no candidate has documentation")
	}
	if e.completion.scrollDoc(1) {
		t.Error("the doc-scroll chord must fall through when no candidate has documentation")
	}
}

func TestCompletion_DocPanelShows(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	longDoc := strings.Repeat("a line of documentation\n\n", 60)
	e.SetCompletionProvider(&docProvider{items: []CompletionItem{
		{Label: "alpha", InsertText: "alpha", Documentation: longDoc},
		{Label: "alpine", InsertText: "alpine", Documentation: longDoc},
	}})

	typeRunes(e, "al")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	// The panel widens the popup beyond the same list without docs.
	withDocs := e.completion.popup.MinSize().Width
	e.completion.dismiss()
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"alpha", "alpine"}})
	typeRunes(e, "p") // "alp": refilters and re-shows without docs
	if plain := e.completion.popup.MinSize().Width; withDocs <= plain {
		t.Errorf("a doc panel should widen the popup: %v (docs) vs %v (plain)", withDocs, plain)
	}
}

func TestCompletion_DocPanelKeyboardScroll(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	longDoc := strings.Repeat("a line of documentation\n\n", 60)
	e.SetCompletionProvider(&docProvider{items: []CompletionItem{
		{Label: "alpha", InsertText: "alpha", Documentation: longDoc},
	}})

	typeRunes(e, "al")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}
	if !e.completion.handleKey(&event.KeyEvent{Name: event.KeyPageDown}) {
		t.Fatal("PageDown should page the doc panel when it shows")
	}
	if e.completion.selected != 0 {
		t.Errorf("scrolling the doc panel must not move the list selection, got %d", e.completion.selected)
	}
	if !e.completion.handleKey(&event.KeyEvent{Name: event.KeyPageUp}) {
		t.Error("PageUp should page the doc panel when it shows")
	}
}

func TestCompletion_DocPanelShortcutScroll(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	longDoc := strings.Repeat("a line of documentation\n\n", 60)
	e.SetCompletionProvider(&docProvider{items: []CompletionItem{
		{Label: "alpha", InsertText: "alpha", Documentation: longDoc},
	}})

	typeRunes(e, "al")
	if !e.completion.visible() {
		t.Fatal("popup should be visible")
	}

	down := &desktop.CustomShortcut{KeyName: event.KeyDown, Modifier: event.KeyModifierShortcutDefault}

	// The chord routes via onShortcut → handleDocScrollShortcut → scrollDoc,
	// consumed exactly while the doc panel shows (#89).
	if !e.handleDocScrollShortcut(down) {
		t.Error("Ctrl/Cmd+Down should page the doc panel while it shows")
	}
	if e.completion.selected != 0 {
		t.Errorf("scrolling must not move the list selection, got %d", e.completion.selected)
	}

	// With completion closed the shortcut falls through (does not consume).
	e.completion.dismiss()
	if e.handleDocScrollShortcut(down) {
		t.Error("doc-scroll shortcut must fall through when completion is closed")
	}
}

func TestFuzzyMatch(t *testing.T) {
	if _, ok := fuzzyMatch("", "anything"); !ok {
		t.Error("empty query should match everything")
	}
	if _, ok := fuzzyMatch("pr", "print"); !ok {
		t.Error("'pr' should match 'print'")
	}
	if _, ok := fuzzyMatch("xyz", "print"); ok {
		t.Error("'xyz' should not match 'print'")
	}
	prefix, _ := fuzzyMatch("pr", "print")  // prefix match
	scatter, _ := fuzzyMatch("pt", "print") // subsequence, not a prefix
	if prefix <= scatter {
		t.Errorf("a prefix match should outscore a scattered one: %d vs %d", prefix, scatter)
	}
}

func TestWordBeforeCursor(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foo bar_baz")
	e.SetCursorPos(0, 11)
	prefix, _, startCol, _, endCol := e.wordBeforeCursor()
	if prefix != "bar_baz" || startCol != 4 || endCol != 11 {
		t.Errorf("word = %q [%d,%d), want \"bar_baz\" [4,11)", prefix, startCol, endCol)
	}

	// Just after "foo" (col 3) the prefix is "foo".
	e.SetCursorPos(0, 3)
	if p, _, _, _, _ := e.wordBeforeCursor(); p != "foo" {
		t.Errorf("word before col 3 should be \"foo\", got %q", p)
	}

	// With a space immediately before the cursor (col 4) the prefix is empty.
	e.SetCursorPos(0, 4)
	if p, _, _, _, _ := e.wordBeforeCursor(); p != "" {
		t.Errorf("word before col 4 should be empty, got %q", p)
	}
}

func TestOffsetRowColRoundTrip(t *testing.T) {
	text := "abc\ndef\nghij"
	cases := []struct{ row, col int }{{0, 0}, {0, 3}, {1, 0}, {1, 2}, {2, 4}}
	for _, tc := range cases {
		off := rowColToOffset(text, tc.row, tc.col)
		if r, c := offsetToRowCol(text, off); r != tc.row || c != tc.col {
			t.Errorf("(%d,%d) → off %d → (%d,%d)", tc.row, tc.col, off, r, c)
		}
	}
}

// TestCompletion_FlipsAboveAtEditorBottom covers the editor-boundary flip: an
// editor that does not reach the bottom of its window still has canvas room
// below it, so the popup's own canvas-keyed flip would open the list downwards
// past the editor's bottom edge, over whatever sits beneath. Anchored on the
// last visible line, it must open ABOVE the current line instead.
func TestCompletion_FlipsAboveAtEditorBottom(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText(strings.Repeat("word\n", 40))
	e.SetCompletionProvider(&WordListCompletionProvider{
		Words: []string{"alpha", "alpine", "amber", "anchor", "ankle", "answer", "antler", "anvil"},
	})

	// The editor occupies the top third of a tall window: plenty of canvas below
	// it, none of it the editor's.
	filler := canvas.NewRectangle(color.NRGBA{A: 255})
	filler.SetMinSize(geo.NewSize(0, 400))
	w := test.NewWindow(container.NewBorder(nil, filler, nil, nil, e))
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 600))

	// Put the caret on the last row the editor shows, then type a prefix.
	rows := int(e.entry.Size().Height / e.gutterW.stride)
	e.SetCursorPos(rows-1, 0)
	typeRunes(e, "an")

	if !e.completion.visible() {
		t.Fatal("completion popup should be visible after typing 'an'")
	}
	anchor := e.PositionAt(e.completion.anchorRow, e.completion.anchorCol)
	r := e.completion.rect
	if r.IsZero() {
		t.Fatal("popup reported no placement")
	}
	if r.Pos.Y+r.Size.Height > anchor.Pos.Y+0.5 {
		t.Errorf("popup should sit above the anchor line near the editor's bottom edge: "+
			"popup %v, anchor %v, editor height %v", r, anchor, e.entry.Size().Height)
	}
}

// TestCompletion_OverflowsCompactEditor keeps the deliberate escape hatch from
// #19: a list taller than a compact editor has room on NEITHER side, so it still
// opens downwards and spills into the surrounding window rather than being
// squeezed above the caret.
func TestCompletion_OverflowsCompactEditor(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.MinRows = 3
	e.SetText("word\nword\nword\n")
	e.SetCompletionProvider(&WordListCompletionProvider{
		Words: []string{"alpha", "alpine", "amber", "anchor", "ankle", "answer", "antler", "anvil"},
	})

	filler := canvas.NewRectangle(color.NRGBA{A: 255})
	filler.SetMinSize(geo.NewSize(0, 400))
	w := test.NewWindow(container.NewBorder(nil, filler, nil, nil, e))
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 600))

	e.SetCursorPos(1, 0)
	typeRunes(e, "an")
	if !e.completion.visible() {
		t.Fatal("completion popup should be visible after typing 'an'")
	}
	anchor := e.PositionAt(e.completion.anchorRow, e.completion.anchorCol)
	if r := e.completion.rect; r.Pos.Y < anchor.Pos.Y {
		t.Errorf("a list taller than a compact editor should still open downwards, got %v vs anchor %v", r, anchor)
	}
}
