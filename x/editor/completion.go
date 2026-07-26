package editor

import (
	"context"
	"strings"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/widget"
)

// CompletionKind classifies a completion item so a host can render an icon or
// group results. The values mirror the common LSP completion-item kinds but
// carry no LSP dependency.
type CompletionKind int

const (
	CompletionText CompletionKind = iota
	CompletionKeyword
	CompletionFunction
	CompletionMethod
	CompletionVariable
	CompletionField
	CompletionClass
	CompletionModule
	CompletionSnippet
	CompletionConstant
)

// CompletionTrigger describes why completion was requested.
type CompletionTrigger int

const (
	// CompletionInvoked is an explicit request, e.g. Ctrl+Space.
	CompletionInvoked CompletionTrigger = iota
	// CompletionAutomatic fires while the user types an identifier.
	CompletionAutomatic
	// CompletionTriggerCharacter fires on a language trigger character (e.g. ".").
	CompletionTriggerCharacter
)

// CompletionItem is a single candidate offered by a [CompletionProvider].
type CompletionItem struct {
	// Label is the text shown in the list. When empty, InsertText is shown.
	Label string
	// Detail is a short, dimmed annotation shown to the right (e.g. a type).
	Detail string
	// Documentation is longer help text shown in the side panel beside the list.
	// It is rendered as plain text by default; set DocumentationMarkdown to treat
	// it as markdown.
	Documentation string
	// DocumentationMarkdown reports that Documentation is markdown (rendered with
	// formatting) rather than plain text (rendered verbatim). Defaults to plain
	// text so a doc containing markdown-significant characters is not misrendered.
	DocumentationMarkdown bool
	// InsertText is the text inserted on accept. When empty, Label is inserted.
	InsertText string
	// Replace is the document range replaced on accept. When empty, the editor
	// replaces the identifier prefix the user has typed at the trigger position.
	Replace TextRange
	// Kind classifies the item for icons/grouping.
	Kind CompletionKind
	// SortText overrides alphabetical ordering among equally-scored items.
	SortText string
}

// CompletionProvider supplies completion candidates at a document position.
// It is optional and LSP-shaped but carries no LSP types: a host can back it
// with a static word list, ctags, or a language server bridge.
//
// Complete is called on the UI goroutine and should return promptly; the editor
// shows the results it returns. (Truly asynchronous providers arrive with the
// Phase 7 LSP layer.)
type CompletionProvider interface {
	Complete(ctx context.Context, row, col int, trigger CompletionTrigger) ([]CompletionItem, error)
}

// CompletionTriggerProvider is an optional interface a [CompletionProvider] may
// also implement to declare its own trigger characters — for example an LSP
// bridge reporting the server's completionProvider.triggerCharacters. The
// declared characters are unioned with any set via
// [Editor.SetCompletionTriggerCharacters], so a host can use either or both.
//
// TriggerCharacters is consulted on demand (per edit), so a provider that learns
// its triggers asynchronously — e.g. an LSP bridge after the initialize
// handshake — can report them later without reinstalling the provider (#80).
type CompletionTriggerProvider interface {
	TriggerCharacters() []rune
}

// SetCompletionProvider installs (or clears, with nil) the completion provider.
// With no provider set, editor behavior is unchanged. If the provider also
// implements [CompletionTriggerProvider], its declared trigger characters take
// effect (consulted on demand).
func (e *Editor) SetCompletionProvider(p CompletionProvider) {
	e.completion.provider = p
	if p == nil {
		e.completion.dismiss()
	}
}

// SetCompletionTriggerCharacters declares the characters that open completion
// when typed (e.g. '.' for member access, '(' for arguments). Typing a trigger
// character queries the provider with [CompletionTriggerCharacter] at the cursor
// — even with no identifier prefix — and anchors the popover there. Passing no
// runes clears the set, leaving only the typed-identifier and Ctrl+Space paths.
func (e *Editor) SetCompletionTriggerCharacters(chars ...rune) {
	e.completion.triggerChars = runeSet(chars)
}

// runeSet builds a lookup set from runes, returning nil for an empty input so an
// absent set and a cleared one read the same.
func runeSet(chars []rune) map[rune]bool {
	if len(chars) == 0 {
		return nil
	}
	set := make(map[rune]bool, len(chars))
	for _, r := range chars {
		set[r] = true
	}
	return set
}

// isTriggerChar reports whether r opens completion — declared either on the
// editor ([Editor.SetCompletionTriggerCharacters]) or by the provider
// ([CompletionTriggerProvider]). The two sources are unioned, and the provider
// is consulted on demand so its triggers may change after install (#80).
func (c *completionController) isTriggerChar(r rune) bool {
	if c.triggerChars[r] {
		return true
	}
	if tp, ok := c.provider.(CompletionTriggerProvider); ok {
		for _, t := range tp.TriggerCharacters() {
			if t == r {
				return true
			}
		}
	}
	return false
}

// WordListCompletionProvider is a simple [CompletionProvider] backed by a fixed
// list of words. It returns every word as a candidate and lets the editor filter
// live against the typed prefix — handy for demos, tests, and keyword/snippet
// dictionaries.
type WordListCompletionProvider struct {
	Words []string
	// Kind classifies every item (defaults to CompletionText).
	Kind CompletionKind
}

// Complete returns one item per configured word.
func (p *WordListCompletionProvider) Complete(_ context.Context, _, _ int, _ CompletionTrigger) ([]CompletionItem, error) {
	items := make([]CompletionItem, 0, len(p.Words))
	for _, w := range p.Words {
		items = append(items, CompletionItem{Label: w, InsertText: w, Kind: p.Kind})
	}
	return items, nil
}

// maxVisibleCompletions bounds the popup height; extra matches scroll.
const maxVisibleCompletions = 8

// completionController owns completion state (session anchor, provider result
// set, fuzzy filtering) and drives the fork's [widget.CompletionPopup] (#143).
// The popup renders in the canvas overlay stack in KeepHostFocus mode
// (fyne#442): the Entry keeps keyboard focus — caret, focus-loss logic and
// window shortcuts survive the popup being open — and the editor's onKey
// dispatcher offers keys via [widget.CompletionPopup.HandleKey]. The bounded
// list window (fyne#436), documentation side panel (fyne#435) and the
// companion-anchoring OnMoved signal (fyne#437) are the popup's own.
type completionController struct {
	e        *Editor
	provider CompletionProvider

	popup *widget.CompletionPopup

	// rect is the popup's last reported placement in the editor's local
	// coordinate space (from OnMoved, zero while hidden) — the signature
	// companion avoids it in layoutPopovers.
	rect geo.Rectangle

	all   []CompletionItem // last full result set from the provider
	shown []CompletionItem // filtered + sorted, currently displayed

	// source → filter → sort is the fork's data/model pipeline (fyne#539)
	// behind the popup: each keystroke re-scores c.all into source, the
	// FilterProxy keeps the fuzzy-matching survivors (one O(n) re-index), and
	// the SortProxy orders them by score with a locale-aware tie-break
	// (lang.Compare, fyne#504) instead of a raw byte comparison. The popup
	// still renders through SetItems, so this is the data pipeline only — the
	// identity-stable *highlight* awaits the popup consuming a SelectionModel
	// directly (the larger rework in #165).
	source *model.SliceModel[scoredCompletion]
	sorted *model.SortProxy[scoredCompletion]

	selected             int
	anchorRow, anchorCol int // identifier start anchoring the active session

	triggerChars map[rune]bool // editor-set characters that open completion (e.g. '.')
}

// scoredCompletion pairs a provider item with its fuzzy score against the active
// prefix so the filter and sort proxies operate on precomputed values rather than
// re-running fuzzyMatch per comparison.
type scoredCompletion struct {
	item    CompletionItem
	score   int
	matched bool
}

// scoredCompletionLess orders scored items best-first: higher fuzzy score wins,
// ties break on SortText then label using the active locale's collation
// (lang.Compare via model.StringLess semantics) rather than raw byte order.
func scoredCompletionLess(a, b scoredCompletion) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.item.SortText != b.item.SortText {
		return lang.Compare(a.item.SortText, b.item.SortText) < 0
	}
	return lang.Compare(completionLabel(a.item), completionLabel(b.item)) < 0
}

func newCompletionController(e *Editor) *completionController {
	c := &completionController{e: e, anchorRow: -1, anchorCol: -1}
	c.source = model.NewSliceModel[scoredCompletion]()
	filtered := model.NewFilterProxy[scoredCompletion](c.source)
	filtered.SetPredicate(func(s scoredCompletion) bool { return s.matched })
	c.sorted = model.NewSortProxy[scoredCompletion](filtered)
	c.sorted.SetLess(scoredCompletionLess)
	return c
}

// visible reports whether the completion popup is currently shown. It tracks
// the OnMoved rectangle (non-zero exactly between Show and Hide, fyne#437)
// rather than BaseWidget.Visible, which a fresh, never-shown widget reports
// as true.
func (c *completionController) visible() bool {
	return c.popup != nil && !c.rect.IsZero()
}

// onTextChanged is called after a user edit. It opens, refilters, or dismisses
// completion based on the identifier under the cursor.
func (c *completionController) onTextChanged() {
	if c.provider == nil {
		return
	}
	prefix, row, startCol, _, _ := c.e.wordBeforeCursor()
	if prefix == "" {
		// A trigger character (e.g. "." for members, "(" for arguments) opens
		// completion with an empty prefix, anchored at the cursor. The signature
		// card, if any, shows alongside (#79). Otherwise an empty prefix dismisses.
		if c.isTriggerChar(c.e.charBeforeCursor()) {
			c.query(c.e.entry.CursorRow, c.e.entry.CursorColumn, CompletionTriggerCharacter)
			return
		}
		c.dismiss()
		return
	}
	if !c.visible() || row != c.anchorRow || startCol != c.anchorCol {
		c.query(row, startCol, CompletionAutomatic)
	} else {
		c.filterAndShow(prefix)
	}
}

// invoke forces a completion query at the cursor (e.g. Ctrl+Space), showing all
// candidates even when no prefix has been typed.
func (c *completionController) invoke() {
	if c.provider == nil {
		return
	}
	_, row, startCol, _, _ := c.e.wordBeforeCursor()
	c.query(row, startCol, CompletionInvoked)
}

// query asks the provider for candidates at the cursor and shows the results
// filtered by the current identifier prefix.
func (c *completionController) query(startRow, startCol int, trigger CompletionTrigger) {
	row, col := c.e.entry.CursorRow, c.e.entry.CursorColumn
	items, err := c.provider.Complete(context.Background(), row, col, trigger)
	if err != nil || len(items) == 0 {
		c.dismiss()
		return
	}
	c.all = items
	c.anchorRow, c.anchorCol = startRow, startCol
	prefix, _, _, _, _ := c.e.wordBeforeCursor()
	c.filterAndShow(prefix)
}

// filterAndShow fuzzy-filters the last result set against prefix and displays
// the survivors, dismissing when nothing matches.
func (c *completionController) filterAndShow(prefix string) {
	scored := make([]scoredCompletion, 0, len(c.all))
	for _, it := range c.all {
		s, ok := fuzzyMatch(prefix, completionLabel(it))
		scored = append(scored, scoredCompletion{item: it, score: s, matched: ok})
	}
	// Set re-indexes the FilterProxy and re-sorts the SortProxy once, in that
	// order, via the listeners they registered on the source.
	c.source.Set(scored)
	if c.sorted.Len() == 0 {
		c.dismiss()
		return
	}

	c.shown = c.shown[:0]
	for i := 0; i < c.sorted.Len(); i++ {
		c.shown = append(c.shown, c.sorted.Item(i).item)
	}
	c.selected = 0
	c.show()
}

// show (re)displays the popup with the current matches, selecting the top. It
// anchors at the session's identifier start; a session whose anchor is not
// visible (scrolled away) dismisses instead.
func (c *completionController) show() {
	if !c.ensureUI() {
		return
	}
	limit := maxVisibleCompletions
	if c.e.MaxCompletionRows > 0 {
		limit = c.e.MaxCompletionRows
	}
	c.popup.MaxVisibleRows = limit

	// SetItems keeps an in-range selection; an empty set first forces the
	// selection back to the top for the refreshed list, matching a fresh show.
	c.selected = 0
	c.popup.SetItems(nil)
	c.popup.SetItems(forkCompletionItems(c.shown))

	// Acceptance flows through the popup (Enter/Tab via HandleKey, Space, or a
	// tap all call its trigger): capture this show's items and anchor, because
	// the popup hides — and OnMoved's zero rect resets the session — before
	// OnSelected fires.
	shown := append([]CompletionItem(nil), c.shown...)
	row, col := c.anchorRow, c.anchorCol
	c.popup.OnSelected = func(i int, _ widget.CompletionItem) {
		c.selected = i
		c.acceptItem(shown, i, row, col)
	}

	anchor, ok := c.caretAnchor()
	if !ok {
		c.dismiss()
		return
	}
	if c.visible() {
		c.popup.Reposition(anchor)
	} else {
		// One interactive surface at a time: an open hover card yields, as it
		// did when completion shared the float-layer popover class.
		c.e.dismissInteractivePopovers()
		c.popup.ShowAtCaret(anchor)
	}
}

// caretAnchor returns the canvas-space cell rectangle of the session anchor —
// the coordinate space [widget.CompletionPopup.ShowAtCaret] expects. ok is
// false when the anchor is scrolled out of view or the editor is not on a
// canvas yet.
func (c *completionController) caretAnchor() (geo.Rectangle, bool) {
	r := c.e.PositionAt(c.anchorRow, c.anchorCol)
	if r.IsZero() {
		return geo.Rectangle{}, false
	}
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return geo.Rectangle{}, false
	}
	off := app.Driver().AbsolutePositionForObject(c.e.entry)
	r.Pos = r.Pos.Add(off)
	return r, true
}

// reposition re-anchors a visible popup after a scroll or resize, dismissing
// when the anchor cell has left the viewport — the same contract the editor's
// repositionPopover applies to float-layer popovers.
func (c *completionController) reposition() {
	if !c.visible() {
		return
	}
	anchor, ok := c.caretAnchor()
	if !ok {
		c.dismiss()
		return
	}
	c.popup.Reposition(anchor)
}

// ensureUI lazily builds the fork completion popup on the editor's canvas. It
// reports false before the editor is shown (no canvas to overlay on).
func (c *completionController) ensureUI() bool {
	if c.popup != nil {
		return true
	}
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return false
	}
	cv := app.Driver().CanvasForObject(c.e.entry)
	if cv == nil {
		return false
	}
	p := widget.NewCompletionPopup(cv, nil)
	// The Entry keeps keyboard focus (fyne#442): caret, focus-loss logic and
	// window shortcuts survive the popup; the editor's onKey dispatcher offers
	// keys through HandleKey instead.
	p.KeepHostFocus = true
	// OnSelected is (re)wired per show — it captures that show's items and
	// anchor (see show).
	p.OnMoved = c.onPopupMoved
	c.popup = p
	return true
}

// onPopupMoved observes the popup's placement (fyne#437): the rectangle is
// translated into the editor's local space for the signature companion to
// avoid (see layoutPopovers), and the zero rectangle — the popup hid, whatever
// hid it (Escape, outside tap, an accepted item, dismiss) — resets the session
// and re-places the companion. The signature card itself stays, matching the
// old dismissPopoverObj semantics where closing completion left it up.
func (c *completionController) onPopupMoved(r geo.Rectangle) {
	if r.IsZero() {
		c.rect = geo.Rectangle{}
		c.reset()
		c.e.repositionPopover()
		return
	}
	if app := bellui.CurrentApp(); app != nil && app.Driver() != nil {
		off := app.Driver().AbsolutePositionForObject(c.e.entry)
		r.Pos = r.Pos.Subtract(off)
	}
	c.rect = r
	c.e.repositionPopover()
}

// scrollDoc pages the popup's documentation panel by dir (±1), leaving the
// list selection unchanged (#86). It reports whether it consumed the key —
// false when no documentation panel is showing, so the keystroke falls
// through to its normal handling. The paging itself is the popup's (fyne#435).
func (c *completionController) scrollDoc(dir float32) bool {
	if !c.visible() {
		return false
	}
	name := event.KeyPageDown
	if dir < 0 {
		name = event.KeyPageUp
	}
	return c.popup.HandleKey(&event.KeyEvent{Name: name})
}

// handleKey offers a key to the popup ([widget.CompletionPopup.HandleKey]):
// arrows navigate, Enter/Tab accept (through OnSelected), Escape dismisses,
// PageUp/PageDown page the documentation panel when it shows. Everything else
// reports false and falls through to the Entry.
func (c *completionController) handleKey(ev *event.KeyEvent) bool {
	return c.popup != nil && c.popup.HandleKey(ev)
}

// acceptItem applies the accepted item's replacement. items and the anchor are
// the accepting show's snapshot: the popup hides — and the session resets via
// OnMoved's zero rectangle — before OnSelected delivers the acceptance.
func (c *completionController) acceptItem(items []CompletionItem, i, anchorRow, anchorCol int) {
	if i < 0 || i >= len(items) {
		return
	}
	it := items[i]
	rng := it.Replace
	if rng.IsEmpty() {
		rng = TextRange{
			StartRow: anchorRow, StartCol: anchorCol,
			EndRow: c.e.entry.CursorRow, EndCol: c.e.entry.CursorColumn,
		}
	}
	insert := it.InsertText
	if insert == "" {
		insert = it.Label
	}
	c.e.replaceRange(rng, insert)
}

// dismiss hides the popup if shown and resets state (the popup's Hide resets
// the session through OnMoved's zero rectangle).
func (c *completionController) dismiss() {
	if c.visible() {
		c.popup.Hide()
		return
	}
	c.reset()
}

func (c *completionController) reset() {
	c.all = nil
	c.shown = nil
	c.selected = 0
	c.anchorRow, c.anchorCol = -1, -1
}

// completionLabel returns the text used for display and matching.
func completionLabel(it CompletionItem) string {
	if it.Label != "" {
		return it.Label
	}
	return it.InsertText
}

// forkCompletionItems maps fycoder's completion items to the fork popup's
// display type. Label/InsertText fold into the shown text via completionLabel;
// Replace/Kind/SortText stay host-side (acceptance and ordering are the
// controller's).
func forkCompletionItems(items []CompletionItem) []widget.CompletionItem {
	out := make([]widget.CompletionItem, len(items))
	for i, it := range items {
		out[i] = widget.CompletionItem{
			Text:                  completionLabel(it),
			Detail:                it.Detail,
			Documentation:         it.Documentation,
			DocumentationMarkdown: it.DocumentationMarkdown,
		}
	}
	return out
}

// fixedSizeLayout reports a fixed minimum size and stretches its children to
// fill, letting a popover size itself to a value the controller computes.
type fixedSizeLayout struct {
	size geo.Size
}

func (l *fixedSizeLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return l.size
}

func (l *fixedSizeLayout) Layout(objs []core.CanvasObject, s geo.Size) {
	for _, o := range objs {
		o.Resize(s)
		o.Move(geo.NewPos(0, 0))
	}
}

// isIdentRune reports whether r can be part of an identifier being completed.
func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordBeforeCursor returns the identifier ending at the cursor: its text and the
// half-open range [start, cursor) on the cursor's row. The prefix is empty when
// the character before the cursor is not an identifier character.
func (e *Editor) wordBeforeCursor() (prefix string, startRow, startCol, endRow, endCol int) {
	row, col := e.entry.CursorRow, e.entry.CursorColumn
	lines := strings.Split(e.docText(), "\n")
	if row < 0 || row >= len(lines) {
		return "", row, col, row, col
	}
	runes := []rune(lines[row])
	if col > len(runes) {
		col = len(runes)
	}
	start := col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	return string(runes[start:col]), row, start, row, col
}

// charBeforeCursor returns the rune immediately before the cursor, or 0 when the
// cursor is at the start of a line or out of range. Used to detect signature
// trigger characters like "(" and ",".
func (e *Editor) charBeforeCursor() rune {
	row, col := e.entry.CursorRow, e.entry.CursorColumn
	lines := strings.Split(e.docText(), "\n")
	if row < 0 || row >= len(lines) {
		return 0
	}
	runes := []rune(lines[row])
	if col <= 0 || col > len(runes) {
		return 0
	}
	return runes[col-1]
}

// replaceRange applies completion acceptance through the public programmatic
// edit path (#160): a single undo step, one OnChanged, protection-gated, with
// the caret left at the end of the inserted text.
func (e *Editor) replaceRange(r TextRange, with string) {
	e.ApplyEdit(r, with)
}

// rowColToOffset converts a 0-based (row, col) to a rune offset into text.
func rowColToOffset(text string, row, col int) int {
	lines := strings.Split(text, "\n")
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += len([]rune(lines[i])) + 1 // +1 for the newline
	}
	return off + col
}

// offsetToRowCol converts a rune offset into text to a 0-based (row, col).
func offsetToRowCol(text string, off int) (row, col int) {
	runes := []rune(text)
	if off > len(runes) {
		off = len(runes)
	}
	for i := 0; i < off; i++ {
		if runes[i] == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	return row, col
}

// fuzzyMatch reports whether query is a (case-insensitive) subsequence of target
// and, if so, a score that rewards prefix, start, and consecutive matches.
// An empty query matches everything with score 0.
func fuzzyMatch(query, target string) (score int, ok bool) {
	if query == "" {
		return 0, true
	}
	q := strings.ToLower(query)
	t := strings.ToLower(target)

	qi := 0
	prev := -2
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if t[ti] != q[qi] {
			continue
		}
		score++
		if ti == 0 {
			score += 10
		}
		if ti == prev+1 {
			score += 5
		}
		prev = ti
		qi++
	}
	if qi != len(q) {
		return 0, false
	}
	if strings.HasPrefix(t, q) {
		score += 20
	}
	score -= len(t) / 10 // gently prefer shorter candidates
	return score, true
}
