package widget

import (
	"image/color"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/textbuf"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

const (
	bindIgnoreDelay = time.Millisecond * 100 // ignore incoming DataItem fire after we have called Set
	multiLineRows   = 3
)

// Declare conformity with interfaces
var (
	_ core.Disableable         = (*Entry)(nil)
	_ event.Draggable          = (*Entry)(nil)
	_ event.Focusable          = (*Entry)(nil)
	_ event.Composable         = (*Entry)(nil)
	_ event.PreeditPositioner  = (*Entry)(nil)
	_ event.Tappable           = (*Entry)(nil)
	_ core.Widget              = (*Entry)(nil)
	_ desktop.Mouseable        = (*Entry)(nil)
	_ desktop.Keyable          = (*Entry)(nil)
	_ mobile.Keyboardable      = (*Entry)(nil)
	_ mobile.Touchable         = (*Entry)(nil)
	_ event.Tabbable           = (*Entry)(nil)
	_ core.Accessible          = (*Entry)(nil)
	_ core.AccessibleValuer    = (*Entry)(nil)
	_ core.AccessibleStateful  = (*Entry)(nil)
	_ core.AccessibleTexter    = (*Entry)(nil)
	_ core.AccessibleConcealed = (*Entry)(nil)
	_ core.AccessibleActioner  = (*Entry)(nil)
)

// EntryStyle determines the visual variant of an entry field.
type EntryStyle int

const (
	// EntryStyleFilled is the default entry style with a visible background fill.
	EntryStyleFilled EntryStyle = iota
	// EntryStyleOutlined has no fill, with a visible border around all edges.
	EntryStyleOutlined
)

type Entry struct {
	DisableableWidget
	shortcut bellui.ShortcutHandler
	Text     string
	// Since: 1.0
	TextStyle bellui.TextStyle
	// Alignment sets the horizontal alignment of the entry text.
	// Defaults to bellui.TextAlignLeading (left).
	Alignment bellui.TextAlign
	// Style sets the MD3 visual variant (Filled or Outlined).
	Style       EntryStyle
	PlaceHolder string
	OnChanged   func(string) `json:"-"`

	// OnSubmitted will be called when the user presses Enter/Return (for a single line entry).
	// MultiLine entries will be submitted when Shift+Enter (or Return) are pressed.
	//
	// Since: 1.0
	OnSubmitted func(string) `json:"-"`

	// OnEscape will be called when the user presses Escape while the entry is
	// focused — the cancel counterpart of OnSubmitted, letting a host dismiss
	// an inline editor (pre-rebrand tracker #1052). An in-progress IME composition keeps
	// precedence: Escape discards the composition (pre-rebrand tracker #354) and reaches this
	// hook only once no composition is pending. Nil ignores the key.
	//
	// Since: 1.0
	OnEscape  func() `json:"-"`
	Password  bool
	MultiLine bool
	Wrapping  bellui.TextWrap

	// ReadOnly prevents text editing while keeping full visual styling
	// (no dimming) and allowing text selection, copy, and navigation.
	// Unlike Disable(), read-only entries are focusable and selectable.
	//
	// Since: 1.0
	ReadOnly bool

	// DeferTextSync, when true, defers synchronising the exported Text field
	// after each edit. By default every keystroke materializes the whole
	// document into Text and hands it to OnChanged, the Validator and any
	// bound value — one allocation and copy of the full document per edit,
	// the dominant per-keystroke cost once documents reach tens of megabytes.
	// With this mode on, edits update only the internal buffer (rendering,
	// caret, selection and SelectedText always reflect the live content) and
	// Text is refreshed at sync points instead: [Entry.SyncText], focus loss,
	// and Enter/Return submission (before OnSubmitted fires). OnChanged, the
	// Validator and binding writes run once per sync point — with the full
	// current text — rather than per keystroke.
	//
	// Between sync points a direct read of the Text field returns the text as
	// of the last sync (Go cannot intercept field reads); call SyncText to
	// obtain the current text. Writing Text directly while edits are pending
	// discards those edits. The mode only takes effect on the multi-line
	// single-style fast path (no folds); elsewhere edits keep the default
	// per-keystroke behavior. Off by default — the default behavior is
	// unchanged.
	//
	// Since: 1.0
	DeferTextSync bool

	// Scroll can be used to turn off the scrolling of our entry when Wrapping is WrapNone.
	//
	// Since: 1.0
	Scroll bellui.ScrollDirection

	// Set a validator that this entry will check against
	// Since: 1.0
	Validator bellui.StringValidator `json:"-"`
	// validatorInternal is the data-binding conversion check. It is deliberately
	// not the public Validator field: Bind used to claim that field, silently
	// dropping the app's validation rule, and Unbind then left the widget with
	// no validator at all. See [Entry.validate] for how the two compose.
	validatorInternal   bellui.StringValidator
	validationStatus    *validationStatus
	onValidationChanged func(error)
	validationError     error

	// If true, the Validator runs automatically on render without user interaction.
	// It will reflect any validation errors found or those explicitly set via SetValidationError().
	// Since: 1.0
	AlwaysShowValidationError bool

	// ValidateWithBorder, when true, shows validation errors via a thicker red
	// border instead of the check/error icon. The icon is hidden and no extra
	// space is reserved for it. Useful for compact widgets like SpinBox.
	ValidateWithBorder bool

	CursorRow, CursorColumn int
	// cursorUpstream tracks the caret's affinity at an LTR↔RTL direction boundary,
	// where CursorColumn alone maps to two visual x positions. false (the default)
	// renders the forward/downstream edge; true the upstream edge of the preceding
	// rune, so the caret sticks to a just-typed character or a click's near side
	// instead of jumping across the boundary. Only the rendered x is affected.
	cursorUpstream bool
	// cursorMu protects CursorRow and CursorColumn from render/event races.
	// Other mutable fields (Text, selection state, etc.) are serialized by
	// the Bellui threading model: mutations must go through bellui.Do() so they
	// execute on the main goroutine. External callers must use bellui.Do() for
	// any write to exported Entry fields.
	cursorMu        sync.Mutex
	OnCursorChanged func() `json:"-"`

	// LineHighlighter, when set, provides per-line syntax highlighting.
	// It receives a line of text and a state value (0 = initial/clear).
	// It returns styled runs for the line and the state at the end of
	// the line (to feed into the next line). The Entry caches the output
	// and only re-highlights lines affected by edits. It must be a pure
	// function of (line, state): the cache replays recorded states to skip
	// unchanged stretches of the document.
	//
	// Cost model for large documents: highlighting is computed for the
	// visible window on demand, and state chains strictly forward — the one
	// full chain is the first jump deep into freshly-set content (e.g.
	// restoring a scroll position after SetText), which must run the
	// highlighter from line 0 up to the viewport once. After SetText that
	// chain is prefilled in the background in per-frame time slices (pre-rebrand tracker #417),
	// so a deep jump only pays synchronously for the gap the prefill has not
	// reached yet. Edits stay cheap even far above the viewport: re-chaining
	// stops as soon as the state converges back into the already-highlighted
	// stretch (pre-rebrand tracker #412).
	//
	// Since: 1.0
	LineHighlighter func(line string, state int) ([]StyleRun, int) `json:"-"`

	// StyleOverlayForRow, when set, supplies additional per-row style runs
	// composited over the LineHighlighter output for that display row — e.g.
	// foreground for LSP semantic tokens computed from a whole-document
	// analysis, which the line-at-a-time LineHighlighter cannot express. It is
	// called at render time for the rows the renderer materialises (the
	// visible window on large documents); return nil for rows with no overlay.
	//
	// The returned runs tile the row from its first rune: each run covers
	// len([]rune(run.Text)) runes of the row and only that length is used —
	// the overlay never changes the text. A zero-valued run is a spacer
	// leaving its range unstyled, so a sparse overlay is spacer/token/spacer/
	// token, and the tiling may stop short of the row end (anything past it is
	// ignored). Over a covered range, a non-nil ForegroundColor or non-empty
	// ColorName replaces the base run's foreground as a unit; Bold/Italic only
	// ever switch on; a non-nil BackgroundColor replaces the base background;
	// Underline true carries UnderlineStyle and UnderlineColor with it. Unset
	// overlay fields keep the base run's values, so a semantic foreground
	// overrides lexical color without clobbering bold or an existing squiggle.
	//
	// The overlay is pure composition: it never feeds the LineHighlighter
	// state chain and never invalidates the highlight cache, so Refresh() on
	// new overlay data repaints without re-lexing. While an IME composition is
	// active the preedit is spliced into its display row — overlay offsets on
	// that one row are shifted until commit — and the preedit underline is
	// drawn over the overlay.
	//
	// Since: 1.0
	StyleOverlayForRow func(row int) []StyleRun `json:"-"`

	// Icon is displayed at the outer left of the entry.
	// It is not clickable, but can be used to indicate the purpose of the entry.
	// Since: 1.0
	Icon bellui.Resource `json:"-"`

	// OnGutterTap is called when the user clicks in the line number gutter.
	// The callback receives the 1-based line number that was clicked.
	// Only fires when ShowLineNumbers is active (i.e., LineHighlighter is set).
	OnGutterTap func(line int) `json:"-"`

	// BreakpointLines is the set of 1-based line numbers where breakpoint
	// markers (red dots) should be rendered in the gutter.
	BreakpointLines map[int]bool `json:"-"`

	// HighlightedRow is the 1-based line number to highlight with a
	// background color (e.g., current execution line in a debugger).
	// Set to 0 to disable.
	HighlightedRow int `json:"-"`

	// KeepSelectionVisible, when true, keeps the text selection highlight
	// visible even when the entry loses focus.
	KeepSelectionVisible bool `json:"-"`

	// OnScrolled is called when the Entry's internal scroll offset changes.
	// The position reports the current scroll offset (X and Y in pixels).
	// Useful for synchronising external widgets (e.g. a pinned gutter)
	// with the Entry's vertical scroll position.
	//
	// Since: 1.0
	OnScrolled func(geo.Position) `json:"-"`

	// SuppressGutter disables the built-in line number gutter and breakpoint
	// rendering even when LineHighlighter is set. Syntax highlighting still
	// works; only the gutter is suppressed. Use this when an external gutter
	// widget is managing line numbers and breakpoints.
	//
	// Since: 1.0
	SuppressGutter bool `json:"-"`

	// HiddenRanges are logical-line ranges (0-based, inclusive Start, exclusive
	// End) the Entry lays out and paints as if absent — code folding. Text(),
	// the cursor model and search positions are unaffected; only rendering and
	// vertical geometry skip them, so a folded region collapses to zero height.
	// RowPositions/RowMetrics report visible-row geometry, the scroll extent
	// shrinks by the folded height, and caret navigation skips folded rows.
	// Ranges past the end of the document are clamped/ignored. Empty == today's
	// behavior. Composes with row virtualisation (pre-rebrand tracker #119).
	//
	// Ranges are in logical lines (newline-delimited), not visual rows. When a
	// logical line wraps to several visual rows, folding that line hides all of
	// its wrapped rows (pre-rebrand tracker #145). For wrap-off content a line is a single row, so
	// the two coincide.
	//
	// Since: 1.0
	HiddenRanges []LineRange `json:"-"`

	cursorAnim *entryCursorAnimation

	dirty   bool
	focused bool
	text    RichText
	// textSynced records that SyncState just synced the segments and refreshed
	// the text, so the renderer's Refresh that BaseWidget.Refresh runs next
	// need not do it again (#488).
	textSynced bool
	// Per-line highlight cache. Kept index-aligned with the provider's logical
	// lines (noteLazyLinesEdited) and filled on demand in the lazy model
	// (ensureHighlightedThrough); eager for the dense model (pre-rebrand tracker #401).
	hlCache []hlCacheLine
	// hlComputedThrough is the count of leading lines whose highlight state
	// chain is current; ensureHighlightedThrough extends it on demand.
	hlComputedThrough int
	// hlTrusted is a short sorted list of disjoint stretches of hlCache above
	// the watermark whose entries still hold current line text and a pairwise-
	// consistent state chain: a previous walk verified them and edits since
	// then only landed outside them. When re-chaining reaches a stretch with
	// an entering state equal to the recorded stateIn, determinism makes every
	// entry in it valid, so the walk jumps over it instead of re-reading each
	// line — an edit above a deep viewport re-chains O(edit + convergence +
	// window), not O(document) (pre-rebrand tracker #412). Tracking several stretches keeps a
	// batch of scattered edits between renders bounded the same way: each edit
	// splits rather than discards the chained lines around it (pre-rebrand tracker #418).
	hlTrusted []hlStretch
	// hlWalkSteps counts per-line iterations of ensureHighlightedThrough's
	// chain walk (test hook, like lazyFullBuilds): pins the pre-rebrand tracker #412 trusted-
	// stretch jump so deep re-chains stay edit-local.
	hlWalkSteps int
	// hlPrefillRepaints counts how many times the background prefill requested a
	// text repaint because its watermark crossed the visible viewport (pre-rebrand tracker #693). A
	// test hook: it pins that a deep visible row converges from the prefill's own
	// repaint rather than depending on the animation frame cadence or a later
	// unrelated refresh.
	hlPrefillRepaints int
	// hlBuf is the provider buffer the chain was computed against; a different
	// object means the content was replaced wholesale and the chain restarts.
	hlBuf *textbuf.Buffer
	// hlPrefill drives the post-SetText background chain (pre-rebrand tracker #417): a content
	// replacement voids all prior highlight state, so this frame-tick
	// animation advances hlComputedThrough in small time-boxed slices on the
	// main goroutine until the chain reaches the end of the document. A later
	// deep jump (restore-scroll-position) then finds the watermark already
	// past it, or pays only for the remaining gap. nil when idle. The arming
	// flag swallows a tick fired synchronously from inside Animation.Start
	// (the test driver does this), which would otherwise run a highlight
	// slice inside the latency-sensitive call that armed the prefill.
	hlPrefill       *bellui.Animation
	hlPrefillArming bool
	placeholder     RichText
	content         *entryContent
	scroll          *widget.Scroll

	// useful for Form validation (as the error text should only be shown when
	// the entry is unfocused)
	onFocusChanged func(bool)

	// selectKeyDown indicates whether left shift or right shift is currently held down
	selectKeyDown bool

	sel        *selectable
	popUp      *PopUpMenu
	selHandles *entrySelectionHandles // mobile selection-handle overlay (pre-rebrand tracker #758)

	caretHandle *entryCaretHandle // mobile caret drag-handle overlay (pre-rebrand tracker #943)

	// touchPointer notes whether the most recent pointer-down was a touch;
	// touchSelection marks the current selection as touch-initiated, which is
	// what summons the mobile selection handles (a hardware-keyboard or
	// programmatic selection shows none, matching native platforms).
	touchPointer, touchSelection bool

	// OnSelectChanged is called whenever the text selection changes.
	// The callback receives the currently selected text (empty string when
	// the selection is cleared).
	OnSelectChanged func(string) `json:"-"`

	// BeforeChange, when set, is consulted before every text mutation. It
	// receives sel, the range being replaced (rune offsets into Text), and
	// next, the proposed replacement text for that range. Returning false
	// vetoes the change, leaving Text, the cursor and the undo stack untouched.
	// It fires for every interactive edit path: typing, IME commit, paste, cut,
	// delete, backspace, word-delete, newline and programmatic
	// select-and-overwrite. A nil hook imposes no restriction. The
	// whole-document resets SetText and Append are not gated.
	//
	// Since: 1.0
	BeforeChange func(sel TextRange, next string) bool `json:"-"`

	// ProtectedRanges are spans the Entry refuses to mutate. An edit overlapping
	// any range is rejected across every edit path, so a vetoed edit never
	// reaches the undo stack and cannot be re-introduced by undo/redo. A range
	// with CaretOpaque set additionally bars the caret from entering it (arrow
	// navigation, clicks and mobile taps skip over it). Offsets are rune indices
	// into Text; the caller owns reconciliation when Text changes.
	// An empty (nil) slice is exactly today's behavior.
	//
	// Protection gates new interactive edits only. Undo and Redo replay the
	// recorded document history verbatim and are not re-gated against
	// ProtectedRanges, so undoing an older legitimate edit can reconstruct text
	// that overlaps a range protected after that edit was recorded. Treat
	// ProtectedRanges as caller-managed view state reconciled alongside history.
	//
	// Since: 1.0
	ProtectedRanges []ProtectedRange `json:"-"`

	// ActionItem is a small item which is displayed at the outer right of the entry (like a password revealer)
	ActionItem      core.CanvasObject `json:"-"`
	binder          basicBinder
	conversionError error
	multiLineRows   int // override global default number of visible lines

	// undoStack stores the data necessary for undo/redo functionality
	// See entryUndoStack for implementation details.
	undoStack entryUndoStack

	// lastMouseX tracks pointer X for gutter cursor icon switching.
	// Updated by entryContent's Hoverable implementation.
	lastMouseX float32

	// skipScrollOnRefresh suppresses ensureCursorVisible during the next refresh.
	// Used by FocusGained on multi-line entries to avoid scroll jumps.
	skipScrollOnRefresh bool

	// IME composition (preedit) state. While composing, preeditText is shown
	// inline at preeditAnchor without mutating Text, OnChanged, undo or bindings;
	// see Compose/ComposeCommit/ComposeCancel.
	preeditText   string // composition text; "" = not composing
	preeditAnchor int    // rune offset into Text where the preedit is displayed
	preeditCursor int    // caret rune offset within preeditText
	// preeditApplied is the number of preedit runes currently spliced into the
	// provider's buffer at preeditAnchor (lazy fast path, pre-rebrand tracker #402); 0 when the
	// preedit is rendered via the composedDisplay string fallback instead.
	preeditApplied int
	composing      bool

	// OnCompletionSelected is called when the user accepts an item from a
	// completion popup shown via ShowCompletion.
	OnCompletionSelected func(CompletionItem)
	completion           *CompletionPopup
}

// StyleRun represents a run of text with a single color.
// Used by LineHighlighter to return per-token coloring for a line.
type StyleRun struct {
	Text            string
	ColorName       bellui.ThemeColorName // empty = default foreground
	ForegroundColor color.Color           // direct color, overrides ColorName when non-nil
	Italic          bool                  // render this run in italic

	// Bold renders this run in a bold font weight.
	//
	// Since: 1.0
	Bold bool
	// BackgroundColor, when non-nil, fills the run's cell rectangle behind the
	// text — a sub-line background (e.g. a semantic-token wash).
	//
	// Since: 1.0
	BackgroundColor color.Color
	// Underline draws a line beneath the run. UnderlineStyle selects its shape
	// (solid by default; wavy/dotted for diagnostic squiggles) and
	// UnderlineColor its color (the run's text color when nil).
	//
	// Since: 1.0
	Underline      bool
	UnderlineStyle UnderlineStyle
	UnderlineColor color.Color

	// StrikeThrough draws a line through the middle of the run — e.g. the
	// industry-standard rendering for a deprecated symbol from a semantic
	// analysis stream. StrikeColor selects its color (the run's text color
	// when nil, like UnderlineColor).
	//
	// Since: 1.0
	StrikeThrough bool
	StrikeColor   color.Color
}

// UnderlineStyle selects the shape of a [StyleRun] underline.
//
// Since: 1.0
type UnderlineStyle int

const (
	// UnderlineSolid is a straight line, e.g. for link decorations.
	UnderlineSolid UnderlineStyle = iota
	// UnderlineWavy is a sine-wave squiggle, e.g. for error/warning diagnostics.
	UnderlineWavy
	// UnderlineDotted is a dotted line, an alternative diagnostic style.
	UnderlineDotted
)

// hlCacheLine stores cached highlight output for one line.
type hlCacheLine struct {
	text     string     // the line text this cache entry was computed from
	stateIn  int        // lexer state at the start of this line
	stateOut int        // lexer state at the end of this line
	runs     []StyleRun // styled text runs for this line
}

// hlStretch is one trusted stretch [from,to) of hlCache entries: see the
// hlTrusted field on Entry (pre-rebrand tracker #412, pre-rebrand tracker #418).
type hlStretch struct {
	from, to int
}

// hlMaxTrustedStretches bounds the hlTrusted list. Dropping a stretch is
// always safe — its lines just fall back to the per-line compare walk — so
// when a splice would grow the list past this, the shortest stretch goes.
const hlMaxTrustedStretches = 8

// NewEntry creates a new single line entry widget.
func NewEntry() *Entry {
	e := &Entry{Wrapping: bellui.TextWrap(bellui.TextTruncateClip)}
	e.ExtendBaseWidget(e)
	return e
}

// NewEntryWithData returns an Entry widget connected to the specified data source.
//
// Since: 1.0
func NewEntryWithData(data binding.String) *Entry {
	entry := NewEntry()
	entry.Bind(data)

	return entry
}

// NewMultiLineEntry creates a new entry that allows multiple lines
func NewMultiLineEntry() *Entry {
	e := &Entry{MultiLine: true, Wrapping: bellui.TextWrap(bellui.TextTruncateClip)}
	e.ExtendBaseWidget(e)
	return e
}

// NewPasswordEntry creates a new entry password widget
func NewPasswordEntry() *Entry {
	e := &Entry{Password: true, Wrapping: bellui.TextWrap(bellui.TextTruncateClip)}
	e.ExtendBaseWidget(e)
	e.ActionItem = newPasswordRevealer(e)
	return e
}

// AcceptsTab returns if Entry accepts the Tab key or not.
//
// Since: 1.0
func (e *Entry) AcceptsTab() bool {
	return e.MultiLine
}

// AccessibleRole returns the appropriate text input role based on the entry's
// configuration (password, multi-line, or standard text box).
func (e *Entry) AccessibleRole() core.AccessibleRole {
	if e.Password {
		return core.RolePasswordField
	}
	if e.MultiLine {
		return core.RoleMultiLineTextBox
	}
	return core.RoleTextBox
}

// AccessibleLabel returns the placeholder text as the accessible name.
func (e *Entry) AccessibleLabel() string { return e.PlaceHolder }

// AccessibleValue returns the current text. For password entries it returns
// an empty string to avoid exposing sensitive input.
func (e *Entry) AccessibleValue() string {
	if e.Password {
		return ""
	}
	return e.Text
}

// AccessibleRange is not applicable to text entries; it returns zeros.
func (e *Entry) AccessibleRange() (min, max, current float64) { return 0, 0, 0 }

// AccessibleConcealed reports whether this entry conceals its content, i.e. it
// is a password field. Semantic auto-capture (see [core.AccessibleConcealed])
// excludes concealed entries wholesale.
func (e *Entry) AccessibleConcealed() bool { return e.Password }

// AccessibleState returns the disabled and read-only state.
func (e *Entry) AccessibleState() core.AccessibleState {
	var s core.AccessibleState
	if e.Disabled() {
		s |= core.StateDisabled
	}
	if e.ReadOnly {
		s |= core.StateReadOnly
	}
	return s
}

// AccessibleActions declares the entry's undo/redo actions so assistive
// technology can reach them without the long-press context menu (pre-rebrand tracker #1025) —
// they have no standard platform action, so backends surface them as custom
// actions (VoiceOver's actions rotor, TalkBack's actions menu, AT-SPI
// Action). Gated on availability like the context menu: none while read-only
// or on a password field, and each direction only while its history is
// non-empty. Cut/copy/paste/select-all are not declared here — they travel
// through the standard editable-text protocols instead.
func (e *Entry) AccessibleActions() []core.AccessibleAction {
	if e.isReadOnly() || e.Password {
		return nil
	}
	var acts []core.AccessibleAction
	if e.undoStack.CanUndo() {
		acts = append(acts, core.AccessibleAction{Name: core.ActionUndo, Description: lang.L("Undo")})
	}
	if e.undoStack.CanRedo() {
		acts = append(acts, core.AccessibleAction{Name: core.ActionRedo, Description: lang.L("Redo")})
	}
	return acts
}

// PerformAccessibleAction invokes the named accessible action.
func (e *Entry) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionUndo, core.ActionRedo:
		// An in-flight IME composition is display-only — outside the
		// document and the undo history. Replaying history under it leaves
		// an orphaned preedit whose position context the undo invalidated;
		// a later IME convergence then commits it at a bogus offset
		// (pre-rebrand tracker #1034). Cancel it first: an uncommitted preedit is recent
		// typing, which is what undo removes anyway.
		e.ComposeCancel()
		if name == core.ActionUndo {
			e.Undo()
		} else {
			e.Redo()
		}
	}
}

// AccessibleText returns the text assistive technology reads and navigates
// (see [core.AccessibleTexter]). It reflects the live content — ahead of the
// Text field under [Entry.DeferTextSync] — so offsets agree with the caret
// and selection. Password entries return the mask characters, never the
// secret; the rune length still matches.
func (e *Entry) AccessibleText() string {
	provider := e.textProvider()
	if e.Password {
		return strings.Repeat(passwordChar, provider.len())
	}
	return provider.String()
}

// AccessiblePreedit reports the in-progress IME preedit and the rune offset
// where it is spliced into [Entry.AccessibleText]; ok is false while no preedit
// is shown. The text-input mirror seam matches this method structurally to
// subtract the preedit from its pushes, so the native IME buffer is only ever
// seeded with committed text (pre-rebrand tracker #1057) — Entry, unlike RichEntry, splices the
// preedit through its provider, so the accessible text alone cannot tell
// committed from composing runes.
func (e *Entry) AccessiblePreedit() (text string, start int, ok bool) {
	if !e.composing || e.preeditText == "" {
		return "", 0, false
	}
	return e.preeditText, e.preeditAnchor, true
}

// AccessibleComposing reports whether an IME composition is currently active
// (a preedit is being shown — including an empty one). The text-input mirror
// seam matches this method structurally to recognize a stale composition
// finalize (pre-rebrand tracker #1057): a hardware keystroke commits the preedit as typed text
// (see TypedKey's desktop-commit path), so an IME commit arriving after that
// would insert content the document already holds.
func (e *Entry) AccessibleComposing() bool {
	return e.composing
}

// AccessibleSelection returns the selection as a half-open rune range; when
// nothing is selected both offsets are the caret position.
func (e *Entry) AccessibleSelection() (start, end int) {
	if r, ok := e.SelectionRange(); ok {
		return r.Start, r.End
	}
	caret := e.CursorTextOffset()
	return caret, caret
}

// AccessibleCaret returns the rune offset of the insertion point.
func (e *Entry) AccessibleCaret() int {
	return e.CursorTextOffset()
}

// SetAccessibleSelection selects [start, end), clamped to the content; equal
// offsets place the caret there with no selection. It is safe to call from
// any goroutine.
func (e *Entry) SetAccessibleSelection(start, end int) {
	onMain(func() {
		if start > end {
			start, end = end, start
		}
		provider := e.textProvider()
		n := provider.len()
		start = clampInt(start, 0, n)
		end = clampInt(end, 0, n)

		if start == end {
			had := false
			e.setFieldsAndRefresh(func() {
				e.CursorRow, e.CursorColumn = e.rowColFromTextPos(start)
				e.cursorUpstream = true
				had = e.clearSelection()
				e.sel.selectRow, e.sel.selectColumn = e.CursorRow, e.CursorColumn
			})
			if had {
				e.sel.notifySelectChanged()
			}
			return
		}

		srow, scol := e.rowColFromTextPos(start)
		erow, ecol := e.rowColFromTextPos(end)
		e.Select(srow, scol, erow, ecol)
	})
}

// ReplaceAccessibleSelection replaces the current selection — or inserts at
// the caret — as one gated, undo-recorded edit (the [Entry.ApplyEdits] path:
// one OnChanged, one Validator run). A no-op for read-only or disabled
// entries. It is safe to call from any goroutine.
func (e *Entry) ReplaceAccessibleSelection(text string) {
	onMain(func() {
		start, end := e.AccessibleSelection()
		e.applyEdits([]TextEdit{{Start: start, End: end, Text: text}})
	})
}

// Bind connects the specified data source to this Entry.
// The current value will be displayed and any changes in the data will cause the widget to update.
// User interactions with this Entry will set the value into the data source.
//
// The public [Entry.Validator] field is left alone: the data-binding conversion
// check is composed with it rather than replacing it, so binding a validated
// Entry keeps the app's rule and Validator may be assigned before or after Bind.
// A conversion error wins over the app's rule -- text that cannot be written to
// the bound source is invalid whatever the app's rule says about it.
//
// Since: 1.0
func (e *Entry) Bind(data binding.String) {
	e.binder.SetCallback(e.updateFromData)
	e.binder.Bind(data)

	e.validatorInternal = func(string) error {
		return e.conversionError
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (e *Entry) CreateRenderer() core.WidgetRenderer {
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	e.ExtendBaseWidget(e)

	// initialise
	e.textProvider()
	e.placeholderProvider()
	e.syncSelectable()

	// Re-arm an unfinished background highlight chain (pre-rebrand tracker #417) after a renderer
	// Destroy stopped it (pre-rebrand tracker #421): eviction also hits live, temporarily-hidden
	// widgets, and on re-show the buffer object is unchanged, so neither
	// arming seam (updateText, the fresh-buffer branch of
	// ensureHighlightedThrough) fires again — and a repaint can serve
	// already-materialized rows without consulting the highlighter, so a
	// render-path arm could not be relied on either. Idempotent when the
	// prefill is already running; a completed chain does not re-arm.
	if e.LineHighlighter != nil && e.text.lazy && e.text.buf != nil &&
		e.hlComputedThrough < e.text.buf.LineCount() {
		e.startHlPrefill()
	}

	box := canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	box.CornerRadius = th.Size(theme.SizeNameInputRadius)
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	border.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	border.CornerRadius = th.Size(theme.SizeNameInputRadius)
	cursor := canvas.NewRectangle(color.Transparent)
	cursor.Hide()

	e.cursorAnim = newEntryCursorAnimation(cursor)
	e.content = &entryContent{entry: e}
	e.content.ExtendBaseWidget(e.content)
	e.scroll = widget.NewScroll(nil)
	// A single-line text field clips its overflowing text at the edge rather
	// than fading it; suppress the scroll's edge fade-shadows so they don't dim
	// the text (or the border) at the scrolled edge (pre-rebrand tracker #357). Multi-line entries
	// keep their vertical fade-shadow as a genuine "more content" affordance.
	e.scroll.HideShadows = !e.MultiLine
	// The border is drawn after the content/scroll so its stroke always
	// composites crisply on top. The scroll viewport spans the full width and
	// shows edge fade-shadows when text overflows; drawing it under the border
	// stops those shadows from dimming the rounded border stroke/corners as the
	// scroll offset flips between edges (pre-rebrand tracker #353).
	objects := []core.CanvasObject{box}
	if e.Wrapping != bellui.TextWrapOff || e.Scroll != widget.ScrollNone {
		e.scroll.Content = e.content
		objects = append(objects, e.scroll)
	} else {
		e.scroll.Hide()
		objects = append(objects, e.content)
	}
	objects = append(objects, border)
	e.content.scroll = e.scroll
	e.scroll.OnScrolled = func(pos geo.Position) {
		e.updateTextViewport()
		if e.OnScrolled != nil {
			e.OnScrolled(pos)
		}
	}

	if e.Password && e.ActionItem == nil {
		// An entry widget has been created via struct setting manually
		// the Password field to true. Going to enable the password revealer.
		e.ActionItem = newPasswordRevealer(e)
	}

	if e.ActionItem != nil {
		objects = append(objects, e.ActionItem)
	}

	icon := canvas.NewImageFromResource(e.Icon)
	icon.FillMode = canvas.ImageFillContain
	objects = append(objects, icon)
	if e.Icon == nil {
		icon.Hide()
	}

	e.syncSegments()
	r := &entryRenderer{box: box, border: border, scroll: e.scroll, icon: icon, objects: objects, entry: e}
	r.indicator = shapeEntryChrome(box, border, e.Style != EntryStyleOutlined, false,
		th.Size(theme.SizeNameInputRadius), th.Size(theme.SizeNameInputBorder))
	return r
}

// CursorPosition returns the relative position of this Entry widget's cursor.
//
// Since: 1.0
func (e *Entry) CursorPosition() geo.Position {
	e.cursorMu.Lock()
	cursorRow, cursorCol := e.CursorRow, e.CursorColumn
	upstream := e.cursorUpstream
	e.cursorMu.Unlock()

	return e.caretRect(cursorRow, cursorCol, !upstream).Pos
}

// CaretRect returns the pixel rectangle of the caret at the logical position
// (row, col), in the Entry's content coordinate space (pre-scroll). The
// rectangle's Pos is the top-left of the caret cell (matching the position of
// the live cursor as returned by [Entry.CursorPosition]); its width is one
// character cell and its height is the row height.
//
// A zero [geo.Rectangle] is returned when (row, col) is out of range. Callers
// that render outside the scroll viewport (e.g. an anchored popover or a span
// decoration) should subtract the current scroll offset and treat positions
// outside the viewport as hidden.
//
// Since: 1.0
func (e *Entry) CaretRect(row, col int) geo.Rectangle {
	provider := e.textProvider()
	if provider == nil || row < 0 || row >= provider.rows() {
		return geo.Rectangle{}
	}
	if col < 0 || col > len(provider.row(row)) {
		return geo.Rectangle{}
	}
	if provider.rowHidden(row) {
		return geo.Rectangle{} // a folded row is not visible (pre-rebrand tracker #141)
	}
	return e.caretRect(row, col, true)
}

// PreeditCursorRectangle reports the caret rectangle in the Entry's content
// coordinate space (adjusted for the current scroll offset) so a driver can
// position the IME candidate window. It implements [event.PreeditPositioner].
//
// Since: 1.0
func (e *Entry) PreeditCursorRectangle() geo.Rectangle {
	e.cursorMu.Lock()
	row, col := e.CursorRow, e.CursorColumn
	e.cursorMu.Unlock()

	r := e.caretRect(row, col, true)
	if e.scroll != nil {
		r.Pos = r.Pos.Subtract(e.scroll.Offset)
	}
	return r
}

// caretRect is the shared implementation behind CursorPosition and CaretRect.
// It tolerates an out-of-range column (clamped by lineSizeToColumn, matching
// the historical CursorPosition behavior); callers that need strict bounds
// (CaretRect) validate before calling. downstream selects caret affinity on a
// bidi direction boundary (see lineSizeToColumn); callers without affinity
// tracking pass true.
func (e *Entry) caretRect(row, col int, downstream bool) geo.Rectangle {
	provider := e.textProvider()
	th := e.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	inputBorder := th.Size(theme.SizeNameInputBorder)
	textSize := th.Size(theme.SizeNameText)

	size := provider.lineSizeToColumn(col, row, textSize, innerPad, downstream)
	xPos := size.Width + provider.GutterWidth

	// Row top: prefer the cached per-row Y from the last layout, which already
	// accounts for folded rows collapsing to zero height (pre-rebrand tracker #141). Fall back to the
	// uniform-stride estimate before the first layout. The cached rowYPos values
	// are in the same content-coordinate base as the historical estimate
	// (innerPad - inputBorder + row*height), since the Entry's text inset height
	// is the input border.
	caretY := size.Height*float32(row) + innerPad - inputBorder
	if provider.lazy {
		// Lazy model: Y is analytic (no dense rowYPos array) and fold-aware.
		caretY, _ = provider.RowGeometry(row)
	} else if row >= 0 && row < len(provider.rowYPos) {
		caretY = provider.rowYPos[row]
	}

	// Width of one character cell at this column. At end-of-line the next-column
	// measurement does not advance, so fall back to the monospace char width.
	width := provider.lineSizeToColumn(col+1, row, textSize, innerPad, downstream).Width - size.Width
	if width <= 0 {
		width = provider.charMinSize(false, e.TextStyle, textSize).Width
	}

	// Account for text alignment offset, resolved against the active layout
	// direction so the caret tracks the text edge under RTL (no-op for LTR).
	align := lang.ResolveAlign(e.Alignment)
	if align != bellui.TextAlignLeading && e.content != nil {
		rowText := provider.row(row)
		textWidth := bellui.MeasureText(string(rowText), textSize, e.TextStyle).Width + 2*innerPad
		containerWidth := e.content.Size().Width
		if containerWidth > textWidth {
			switch align {
			case bellui.TextAlignCenter:
				xPos += (containerWidth - textWidth) / 2
			case bellui.TextAlignTrailing:
				xPos += containerWidth - textWidth
			}
		}
	}

	return geo.NewRectangle(xPos-(inputBorder/2), caretY, width, size.Height)
}

// CursorTextOffset returns how many runes into the source text the cursor is positioned at.
//
// Since: 1.0
func (e *Entry) CursorTextOffset() (pos int) {
	e.cursorMu.Lock()
	row, col := e.CursorRow, e.CursorColumn
	e.cursorMu.Unlock()
	return textPosFromRowCol(row, col, e.textProvider())
}

// Cursor returns the cursor type of this widget.
// Returns the default pointer over the gutter, I-beam over text.
func (e *Entry) Cursor() desktop.Cursor {
	if e.text.GutterWidth > 0 && e.lastMouseX >= 0 && e.lastMouseX < e.text.GutterWidth {
		return desktop.DefaultCursor
	}
	return desktop.TextCursor
}

// gutterRowAt returns the 0-based row index at the given Y coordinate,
// using the same cached row geometry as text selection hit-testing.
func (e *Entry) gutterRowAt(y float32) int {
	numRows := e.text.rows()
	// O(1) analytic inverse for the lazy model (pre-rebrand tracker #160); cached-geometry scan otherwise.
	if row := e.text.rowAtY(y); row >= 0 {
		return row
	}
	// Fallback: uniform row height.
	th := e.Theme()
	textSize := th.Size(theme.SizeNameText)
	rowH := e.text.charMinSize(false, e.TextStyle, textSize).Height
	if rowH <= 0 {
		return -1
	}
	innerPad := th.Size(theme.SizeNameInnerPadding)
	row := int((y - innerPad + e.text.inset.Height) / rowH)
	if row < 0 {
		return 0
	}
	if row >= numRows {
		return numRows - 1
	}
	return row
}

// DoubleTapped is called when this entry has been double tapped so we should select text below the pointer
func (e *Entry) DoubleTapped(_ *event.PointEvent) {
	e.focused = true
	e.syncSelectable()
	e.sel.doubleTappedAtUnixMillis = time.Now().UnixMilli()
	e.selectWordAtCursor()
	if e.touchPointer {
		e.markTouchSelection()
	}
}

// selectWordAtCursor selects the word (or whitespace run) under the caret,
// anchoring the selection at its start. Shared by DoubleTapped and LongPressed.
func (e *Entry) selectWordAtCursor() {
	row := e.textProvider().row(e.CursorRow)
	start, end := getTextWhitespaceRegion(row, e.CursorColumn, false)
	if start == -1 || end == -1 {
		return
	}

	e.setFieldsAndRefresh(func() {
		if !e.selectKeyDown {
			e.sel.selectRow = e.CursorRow
			e.sel.selectColumn = start
		}
		// Always aim to maximise the selected region
		if e.sel.selectRow > e.CursorRow || (e.sel.selectRow == e.CursorRow && e.sel.selectColumn > e.CursorColumn) {
			e.CursorColumn = start
		} else {
			e.CursorColumn = end
		}

		e.syncSelectable()
		e.sel.selecting = true
	})
}

// LongPressed implements [event.LongPressable]. A touch held on the entry
// selects the word under the finger, the native mobile gesture (pre-rebrand tracker #758); the
// release that follows opens the context menu via TappedSecondary. Desktop
// right-click is unaffected (it dispatches TappedSecondary directly).
func (e *Entry) LongPressed(ev *event.PointEvent) {
	if e.Disabled() {
		return
	}

	e.requestFocus()
	e.syncSelectable()
	e.updateMousePointer(ev.Position.Add(e.scroll.Offset), false)
	if e.Password {
		// A concealed field must not reveal word boundaries; the release
		// still opens the context menu (Paste / Select all).
		return
	}
	e.selectNearestWordAtCursor()
	e.markTouchSelection()
}

// selectNearestWordAtCursor is the long-press variant of selectWordAtCursor:
// a press landing on a whitespace/separator run selects the nearest adjacent
// word instead of the run itself, matching the native mobile gesture (pre-rebrand tracker #958).
// Double-tap keeps the run selection (desktop double-click convention).
func (e *Entry) selectNearestWordAtCursor() {
	row := e.textProvider().row(e.CursorRow)
	start, end := getTextWordRegionNearest(row, e.CursorColumn)
	if start == -1 || end == -1 {
		return
	}

	e.setFieldsAndRefresh(func() {
		// A long-press never extends an existing selection (touch has no
		// Shift), so anchor at the region start and put the caret at its
		// end. The snapped region may not contain the pressed column, so
		// selectWordAtCursor's anchor-relative maximise logic cannot apply.
		e.sel.selectRow = e.CursorRow
		e.sel.selectColumn = start
		e.CursorColumn = end
		e.syncSelectable()
		e.sel.selecting = true
	})
}

// markTouchSelection flags the current selection as touch-initiated and
// summons the selection handles for it (a no-op off mobile).
func (e *Entry) markTouchSelection() {
	e.touchSelection = true
	e.updateSelectionHandles()
}

// DragEnd is called at end of a drag event.
func (e *Entry) DragEnd() {
	e.syncSelectable()

	if e.CursorColumn == e.sel.selectColumn && e.CursorRow == e.sel.selectRow {
		e.sel.selecting = false
	}

	// A finished touch drag-select offers the context menu for the new
	// selection, like a handle adjustment (native toolbar parity, pre-rebrand tracker #945).
	// This also covers the hold-fire long-press that keeps moving: its
	// release ends as a drag, so the driver's release-time TappedSecondary
	// never fires for it. Touch-gated: desktop mouse selection is unchanged.
	if e.touchPointer && e.touchSelection && e.sel.selecting && bellui.CurrentDevice().IsMobile() {
		if _, ok := e.SelectionRange(); ok {
			rect := e.caretRect(e.CursorRow, e.CursorColumn, true)
			pos := rect.Pos.Subtract(e.scroll.Offset).AddXY(0, rect.Size.Height)
			abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super()).Add(pos)
			e.TappedSecondary(&event.PointEvent{AbsolutePosition: abs, Position: pos})
		}
	}
}

// Dragged is called when the pointer moves while a button is held down.
// It updates the selection accordingly.
func (e *Entry) Dragged(d *event.DragEvent) {
	d.Position = d.Position.Add(geo.NewPos(0, e.Theme().Size(theme.SizeNameInputBorder)))
	e.sel.dragged(d)
	e.updateMousePointer(d.Position, false)
	if e.touchPointer && e.sel.selecting {
		e.markTouchSelection()
	}
}

// ExtendBaseWidget is used by an extending widget to make use of BaseWidget functionality.
func (e *Entry) ExtendBaseWidget(wid core.Widget) {
	e.BaseWidget.ExtendBaseWidget(wid)
	e.registerShortcut()
}

// FocusGained is called when the Entry has been given focus.
func (e *Entry) FocusGained() {
	if e.MultiLine {
		// Multi-line: skip selectAll and scroll-to-cursor on focus gain.
		// Just update focused state and refresh visuals in place.
		e.dirty = true
		e.focused = true
		if e.onFocusChanged != nil {
			e.onFocusChanged(true)
		}
		e.skipScrollOnRefresh = true
		e.Refresh()
		e.skipScrollOnRefresh = false
		return
	}
	e.setFieldsAndRefresh(func() {
		e.dirty = true
		e.focused = true
	})
	if e.onFocusChanged != nil {
		e.onFocusChanged(true)
	}
	e.selectAll()
}

// FocusLost is called when the Entry has had focus removed.
func (e *Entry) FocusLost() {
	if e.composing {
		// Discard any in-progress IME composition on focus-out rather than leaving
		// it live to be implicitly committed by the next interaction (pre-rebrand tracker #354).
		e.ComposeCancel()
	}
	e.focused = false
	e.selectKeyDown = false

	// Focus loss is a deferred-mode sync point (pre-rebrand tracker #410): the composition was
	// cancelled above, so pending edits materialize into Text here and the
	// deferred OnChanged/Validator/binding write observe the final text.
	if e.text.deferredDirty {
		e.syncDeferredTextNotify()
	}

	// Stop cursor animation immediately rather than waiting for the async
	// Refresh. Without this, a timing window exists where the old entry's
	// cursor animation is still running when the new entry starts its own,
	// causing two visible cursors simultaneously.
	if e.cursorAnim != nil {
		e.cursorAnim.stop()
	}

	if e.onFocusChanged != nil {
		e.onFocusChanged(false)
	}
	e.skipScrollOnRefresh = true
	e.Refresh()
	e.skipScrollOnRefresh = false
}

// Hide hides the entry.
func (e *Entry) Hide() {
	if e.popUp != nil {
		e.popUp.Hide()
		e.popUp = nil
	}
	if e.selHandles != nil {
		e.selHandles.hide()
	}
	e.hideCaretHandle()
	e.DisableableWidget.Hide()
}

// Keyboard implements the Keyboardable interface
func (e *Entry) Keyboard() mobile.KeyboardType {
	if e.MultiLine {
		return mobile.DefaultKeyboard
	} else if e.Password {
		return mobile.PasswordKeyboard
	}

	return mobile.SingleLineKeyboard
}

// KeyDown handler for keypress events - used to store shift modifier state for text selection
func (e *Entry) KeyDown(key *event.KeyEvent) {
	if e.Disabled() {
		return
	}
	// For keyboard cursor controlled selection we now need to store shift key state and selection "start"
	// Note: selection start is where the highlight started (if the user moves the selection up or left then
	// the selectRow/Column will not match SelectionStart)
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		if !e.sel.selecting {
			e.sel.selectRow = e.CursorRow
			e.sel.selectColumn = e.CursorColumn
		}
		e.selectKeyDown = true
	}
}

// KeyUp handler for key release events - used to reset shift modifier state for text selection
func (e *Entry) KeyUp(key *event.KeyEvent) {
	if e.Disabled() {
		return
	}
	// Handle shift release for keyboard selection
	// Note: if shift is released then the user may repress it without moving to adjust their old selection
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.selectKeyDown = false
	}
}

// MinSize returns the size that this widget should not shrink below.
func (e *Entry) MinSize() geo.Size {
	e.ExtendBaseWidget(e)
	return e.BaseWidget.MinSize()
}

// IntrinsicSize reports the natural extent of the entry's text measured as if
// Wrapping and Truncation were off, plus padding — mirroring
// [RichText.IntrinsicSize] (an Entry embeds a RichText) for content autosizing.
// For a wrapping entry the embedded text's min width collapses to ~0, so use
// IntrinsicSize to size the entry (or a container around it) to its content. It
// measures the text only; the entry's own chrome (border, scrollbars) is not added.
//
// Since: 1.0
func (e *Entry) IntrinsicSize() geo.Size {
	e.ExtendBaseWidget(e)
	return e.textProvider().IntrinsicSize()
}

// MouseDown called on mouse click, this triggers a mouse click which can move the cursor,
// update the existing selection (if shift is held), or start a selection dragging operation.
func (e *Entry) MouseDown(m *desktop.MouseEvent) {
	e.touchPointer = false

	// Detect clicks in the gutter area BEFORE focus/selection to avoid side effects.
	if e.OnGutterTap != nil && e.text.GutterWidth > 0 {
		clickX := m.Position.X
		if e.scroll != nil {
			clickX += e.scroll.Offset.X
		}
		if clickX < e.text.GutterWidth {
			clickY := m.Position.Y
			if e.scroll != nil {
				clickY += e.scroll.Offset.Y
			}
			row := e.gutterRowAt(clickY)
			if row >= 0 {
				e.OnGutterTap(row + 1) // 1-based
			}
			return
		}
	}

	e.syncSelectable()

	// Update cursor position BEFORE requesting focus so that FocusGained's
	// refresh scrolls to the clicked location, not the old cursor position.
	e.updateMousePointer(m.Position.Add(e.scroll.Offset), m.Button == desktop.MouseButtonSecondary)

	e.requestFocus()

	if isTripleTap(e.sel.doubleTappedAtUnixMillis, time.Now().UnixMilli()) {
		e.sel.selectCurrentRow(false)
		e.CursorColumn = e.sel.cursorColumn
		e.Refresh()
		return
	}
	if e.selectKeyDown {
		e.sel.selecting = true
	}
	if e.sel.selecting && !e.selectKeyDown && m.Button == desktop.MouseButtonPrimary {
		e.sel.selecting = false
	}
}

// MouseUp called on mouse release
// If a mouse drag event has completed then check to see if it has resulted in an empty selection,
// if so, and if a text select key isn't held, then disable selecting
func (e *Entry) MouseUp(m *desktop.MouseEvent) {
	e.syncSelectable()
	start, _ := e.sel.selection()
	if start == -1 && e.sel.selecting && !e.selectKeyDown {
		e.sel.selecting = false
	}
}

// Redo un-does the last undo action.
//
// Like [Entry.Undo], Redo replays recorded history verbatim and is not re-gated
// against [Entry.ProtectedRanges].
//
// Since: 1.0
func (e *Entry) Redo() {
	action := e.undoStack.Redo()
	if action == nil {
		return
	}
	e.applyUndoAction(action, true)
}

// applyUndoAction replays (redo) or reverses (undo) a recorded action by
// splicing the provider directly — O(edit) through the lazy fast path instead
// of reconstructing the whole document string, which also keeps the identity
// chain intact so the next layout pass reuses the incremental row model (pre-rebrand tracker #401).
func (e *Entry) applyUndoAction(action entryUndoAction, redo bool) {
	provider := e.textProvider()
	pos := action.Apply(provider, redo)
	// Deferred text sync (pre-rebrand tracker #410): history replay defers like any other edit.
	if e.editDeferred() {
		e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos)
		e.syncSelectable()
		e.Refresh()
		return
	}
	content := provider.String()
	e.updateText(content, false)
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos)
	e.syncSelectable()
	if e.OnChanged != nil {
		e.OnChanged(content)
	}
	e.Refresh()
}

// SyncState syncs Entry's public fields to internal rendering state.
// Called automatically by BaseWidget.Refresh() before the renderer refreshes.
func (e *Entry) SyncState() {
	if len(e.text.Segments) > 0 {
		e.syncSegments()
		e.text.Refresh()
		e.textSynced = true
	}

	if e.sel != nil {
		e.sel.style = e.TextStyle
		e.sel.theme = e.Theme()
		e.sel.focussed = e.focused
		e.sel.keepSelection = e.KeepSelectionVisible
		e.sel.Refresh()
	}
}

// RefreshGutter redraws the gutter (breakpoint markers, highlighted row)
// without scrolling to the cursor position. Use this instead of Refresh()
// when only gutter visuals have changed.
func (e *Entry) RefreshGutter() {
	e.textProvider() // sync BreakpointLines / HighlightedRow to RichText
	e.text.Refresh()
}

// SelectedText returns the text currently selected in this Entry.
// If there is no selection it will return the empty string.
func (e *Entry) SelectedText() string {
	if e.sel == nil || !e.sel.selecting {
		return ""
	}
	// Reconstruct from the source text by offset rather than from visual rows: a
	// single text segment has the full text, so slicing it preserves characters that
	// word-wrap consumes at a wrap point (a space dropped from both adjacent visual
	// rows) and is exact under the lazy/windowed row model (pre-rebrand tracker #157), where rebuilding
	// the text from per-row boundaries would be both lossy and measurement-heavy.
	provider := e.textProvider()
	a := textPosFromRowCol(e.sel.selectRow, e.sel.selectColumn, provider)
	b := textPosFromRowCol(e.CursorRow, e.CursorColumn, provider)
	if a == b {
		return ""
	}
	// Read the range from the provider's buffer (no full-document decode, pre-rebrand tracker #401)
	// when it reflects the committed Text — the O(1) steady-state compare. During
	// IME composition seg.Text carries the preedit while Text does not, so fall
	// back to slicing Text directly to keep preedit out of the selection.
	if provider.String() == e.Text {
		return provider.textRange(a, b)
	}
	if a > b {
		a, b = b, a
	}
	runes := []rune(e.Text)
	if a < 0 {
		a = 0
	}
	if b > len(runes) {
		b = len(runes)
	}
	return string(runes[a:b])
}

// Select programmatically selects text from (startRow, startCol) to
// (endRow, endCol). Coordinates are 0-based. The cursor is placed at
// the end of the selection. Out-of-range values are clamped to valid bounds.
func (e *Entry) Select(startRow, startCol, endRow, endCol int) {
	e.setFieldsAndRefresh(func() {
		provider := e.textProvider()
		maxRow := provider.rows() - 1
		if maxRow < 0 {
			return
		}

		startRow = clampInt(startRow, 0, maxRow)
		endRow = clampInt(endRow, 0, maxRow)
		startCol = clampInt(startCol, 0, provider.rowLength(startRow))
		endCol = clampInt(endCol, 0, provider.rowLength(endRow))

		e.sel.selectRow = startRow
		e.sel.selectColumn = startCol
		e.CursorRow = endRow
		e.CursorColumn = endCol
		e.syncSelectable()
		e.sel.selecting = true
	})
	e.sel.notifySelectChanged()
}

// Unselect clears the current text selection, if any, leaving the cursor where
// it is. When a non-empty selection was cleared, OnSelectChanged fires with the
// empty string. It is a no-op when nothing is selected.
//
// Since: 1.0
func (e *Entry) Unselect() {
	changed := false
	e.setFieldsAndRefresh(func() {
		changed = e.clearSelection()
	})
	if changed {
		e.sel.notifySelectChanged()
	}
}

// clearSelection drops any active selection, leaving the cursor in place. It
// reports whether a non-empty selection was cleared (so the caller can decide
// whether to notify OnSelectChanged).
func (e *Entry) clearSelection() bool {
	e.syncSelectable()
	had := e.sel.selecting && e.sel.SelectedText() != ""
	e.sel.selecting = false
	return had
}

// SetIcon sets the leading icon resource for the entry.
// The icon will be displayed at the outer left of the entry, but is not clickable.
// This can be used to indicate the purpose of the entry, such as an email or password field.
//
// Since: 1.0
func (e *Entry) SetIcon(res bellui.Resource) {
	onMain(func() {
		e.Icon = res
		e.Refresh()
	})
}

// SetMinRowsVisible forces a multi-line entry to show `count` number of rows without scrolling.
// This is not a validation or requirement, it just impacts the minimum visible size.
// Use this carefully as Bellui apps can run on small screens so you may wish to add a scroll container if
// this number is high. Default is 3.
//
// Since: 1.0
func (e *Entry) SetMinRowsVisible(count int) {
	onMain(func() {
		e.multiLineRows = count
		e.Refresh()
	})
}

// RowMetrics returns the Y offset of the first text row and the vertical
// stride between consecutive rows. External widgets (such as a pinned
// gutter or row decorator overlay) use these values to align with the
// Entry's text content.
//
// After the first layout pass the values come from the cached RichText
// geometry and are exact. Before layout a theme-derived fallback is used.
//
// Since: 1.0
func (e *Entry) RowMetrics() (topOffset, stride float32) {
	// Lazy model (pre-rebrand tracker #157): uniform stride, analytic Y — serve from RowGeometry of the
	// first visible row instead of the (absent) dense rowYPos array.
	if e.text.lazy {
		stride = e.text.stride()
		topOffset, _ = e.text.RowGeometry(e.text.firstVisibleAtOrAfter(0))
		if stride <= 0 {
			th := e.Theme()
			stride = e.text.charMinSize(false, e.TextStyle, th.Size(theme.SizeNameText)).Height
		}
		return
	}
	// Use cached geometry from the last layout pass when available. With folded
	// rows (pre-rebrand tracker #141) the stride is the uniform visible-row advance and the offset is
	// the first non-folded row's top, so a stale fold on row 0 does not yield a
	// zero stride. Without folds this matches rowYPos[1]-rowYPos[0] exactly.
	if len(e.text.rowYPos) >= 2 {
		first := e.text.firstVisibleAtOrAfter(0)
		if first >= len(e.text.rowYPos) {
			first = 0
		}
		topOffset = e.text.rowYPos[first]
		stride = e.text.stride()
		if stride <= 0 {
			stride = e.text.rowYPos[1] - e.text.rowYPos[0]
		}
		return
	}
	// Fallback before first layout.
	th := e.Theme()
	textSize := th.Size(theme.SizeNameText)
	topOffset = th.Size(theme.SizeNameInnerPadding) - th.Size(theme.SizeNameInputBorder)
	stride = e.text.charMinSize(false, e.TextStyle, textSize).Height
	return
}

// RowPositions returns the exact Y position and height for each row,
// as computed during the last layout pass. External gutter and decorator
// widgets should prefer these over RowMetrics to avoid floating-point
// drift on long documents.
//
// Returns nil before the first layout pass.
//
// Since: 1.0
func (e *Entry) RowPositions() (yPositions []float32, heights []float32) {
	// Lazy model (pre-rebrand tracker #157): no dense geometry array — synthesise positions analytically
	// (uniform stride, fold-aware). O(rows); external gutters on very large documents
	// should prefer the O(1) RowMetrics. Matches the dense rowYPos/rowHeight values.
	if e.text.lazy {
		n := e.text.rows()
		if n == 0 {
			return nil, nil
		}
		yPositions = make([]float32, n)
		heights = make([]float32, n)
		for i := 0; i < n; i++ {
			yPositions[i], heights[i] = e.text.RowGeometry(i)
		}
		return
	}
	if len(e.text.rowYPos) == 0 {
		return nil, nil
	}
	// Return positions relative to the scroll viewport (RichText internal
	// coordinates). Callers that render outside the Entry should pad
	// themselves to match the Entry's inputBorder offset.
	yPositions = make([]float32, len(e.text.rowYPos))
	copy(yPositions, e.text.rowYPos)
	heights = make([]float32, len(e.text.rowHeight))
	copy(heights, e.text.rowHeight)
	return
}

// updateTextViewport feeds the current scroll viewport to the virtualised text
// provider and, when the materialised row window changes, triggers a cheap
// window-sized re-render. No-op unless row virtualisation is active (pre-rebrand tracker #119). It
// must not recompute row bounds, scroll, or re-enter Entry.Refresh.
func (e *Entry) updateTextViewport() {
	if e.scroll == nil || !e.text.canVirtualize() {
		return
	}
	if e.text.SetVisibleRange(e.scroll.Offset.Y, e.scroll.Size().Height) {
		cache.Renderer(&e.text).Refresh()
		// The selection overlay is windowed to the same band, so rebuild it for
		// the new window as we scroll.
		if e.sel != nil && e.sel.selecting {
			e.sel.Refresh()
		}
	}
}

// SetPlaceHolder sets the text that will be displayed if the entry is otherwise empty.
// It is safe to call from any goroutine.
func (e *Entry) SetPlaceHolder(text string) {
	onMain(func() {
		e.Theme() // setup theme cache before locking
		e.PlaceHolder = text
		e.placeholderProvider().Segments[0].(*TextSegment).Text = text
		e.placeholder.updateRowBounds()
		e.placeholderProvider().Refresh()
	})
}

// SetText manually sets the text of the Entry to the given text value.
// Calling SetText resets all undo history.
// It is safe to call from any goroutine.
func (e *Entry) SetText(text string) {
	onMain(func() {
		e.hlCache = nil // force full re-highlight for the new content
		e.hlComputedThrough = 0
		e.setText(text, false)
	})
}

func (e *Entry) setText(text string, fromBinding bool) {
	e.Theme() // setup theme cache before locking
	// A wholesale text replacement invalidates any selection's offsets, so drop
	// it before re-laying-out — otherwise a stale anchor survives and the next
	// edit replaces a phantom range (pre-rebrand tracker #154).
	cleared := e.clearSelection()
	e.updateTextAndRefresh(text, fromBinding)
	e.updateCursorAndSelection()
	if cleared {
		e.sel.notifySelectChanged()
	}

	e.undoStack.Clear()
}

// Append appends the text to the end of the entry.
//
// Since: 1.0
func (e *Entry) Append(text string) {
	onMain(func() {
		provider := e.textProvider()
		provider.insertAt(provider.len(), []rune(text))
		e.undoStack.Clear()
		// Deferred text sync (pre-rebrand tracker #410): programmatic appends defer too, so bulk
		// appends to a huge document stay O(edit) per call.
		if e.editDeferred() {
			e.Refresh()
			return
		}
		content := provider.String()
		changed := e.updateText(content, false)
		cb := e.OnChanged

		if changed {
			e.validate()
			if cb != nil {
				cb(content)
			}
		}
		e.Refresh()
	})
}

// Tapped is called when this entry has been tapped. We update the cursor position in
// device-specific callbacks (MouseDown() and TouchDown()).
func (e *Entry) Tapped(ev *event.PointEvent) {
	if bellui.CurrentDevice().IsMobile() && e.sel.selecting {
		e.sel.selecting = false
	}
	// A tap that collapsed a selection re-reveals the caret handle armed by
	// the preceding TouchDown (pre-rebrand tracker #943); a no-op when no handle is armed.
	e.updateCaretHandle()
}

// TappedSecondary is called when right or alternative tap is invoked.
//
// Opens the PopUpMenu with `Paste` item to paste text from the clipboard.
func (e *Entry) TappedSecondary(pe *event.PointEvent) {
	if e.Disabled() && e.Password {
		return // no popup options for a disabled concealed field
	}

	e.requestFocus()
	entryPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super())
	e.showContextMenu(entryPos.Add(pe.Position))
}

// showContextMenu builds and presents the edit context menu (Cut / Copy / Paste
// / Select all, plus Undo / Redo when available) at the given absolute canvas
// position.
func (e *Entry) showContextMenu(pos geo.Position) {
	super := e.super()
	app := bellui.CurrentApp()
	clipboard := app.Clipboard()
	typedShortcut := super.(bellui.Shortcutable).TypedShortcut
	cutItem := bellui.NewMenuItem(lang.L("Cut"), func() {
		typedShortcut(&bellui.ShortcutCut{Clipboard: clipboard})
	})
	copyItem := bellui.NewMenuItem(lang.L("Copy"), func() {
		typedShortcut(&bellui.ShortcutCopy{Clipboard: clipboard})
	})
	pasteItem := bellui.NewMenuItem(lang.L("Paste"), func() {
		typedShortcut(&bellui.ShortcutPaste{Clipboard: clipboard})
	})
	selectAllItem := bellui.NewMenuItem(lang.L("Select all"), func() {
		e.selectAll()
		if e.touchPointer {
			e.markTouchSelection() // Select all from the touch context menu gets handles
			// Re-present the menu over the now-full selection so Cut/Copy are
			// immediately usable, matching native iOS/Android; without this the
			// menu just vanished and re-summoning it needed a fresh tap that
			// moved the caret / lost the selection (pre-rebrand tracker #1128). The current menu is
			// dismissing as part of this tap, so re-show on the next event cycle.
			bellui.Do(func() { e.showContextMenu(pos) })
		}
	})

	menuItems := make([]*bellui.MenuItem, 0, 6)
	if e.isReadOnly() {
		menuItems = append(menuItems, copyItem, selectAllItem)
	} else if e.Password {
		menuItems = append(menuItems, pasteItem, selectAllItem)
	} else {
		canUndo, canRedo := e.undoStack.CanUndo(), e.undoStack.CanRedo()
		if canUndo {
			undoItem := bellui.NewMenuItem(lang.L("Undo"), e.Undo)
			menuItems = append(menuItems, undoItem)
		}
		if canRedo {
			redoItem := bellui.NewMenuItem(lang.L("Redo"), e.Redo)
			menuItems = append(menuItems, redoItem)
		}
		if canUndo || canRedo {
			menuItems = append(menuItems, bellui.NewMenuItemSeparator())
		}
		menuItems = append(menuItems, cutItem, copyItem, pasteItem, selectAllItem)
	}

	c := app.Driver().CanvasForObject(super)
	if c == nil {
		return
	}
	e.popUp = NewPopUpMenu(bellui.NewMenu("", menuItems...), c)
	e.popUp.ShowAtPosition(pos)
}

// TouchDown is called when this entry gets a touch down event on mobile device, we ensure we have focus.
//
// Since: 1.0
func (e *Entry) TouchDown(ev *mobile.TouchEvent) {
	now := time.Now().UnixMilli()
	e.touchPointer = true
	e.syncSegments()
	if !e.Disabled() {
		e.requestFocus()
	}
	if isTripleTap(e.sel.doubleTappedAtUnixMillis, now) {
		e.sel.selectCurrentRow(false)
		e.CursorColumn = e.sel.cursorColumn
		e.Refresh()
		return
	}

	// Like MouseDown, translate the entry-relative touch position into content
	// coordinates so the caret lands correctly in a scrolled entry.
	e.updateMousePointer(ev.Position.Add(e.scroll.Offset), false)

	// A plain touch tap placing the caret summons the draggable caret
	// handle (pre-rebrand tracker #943); a tap that grows into a drag-select or long-press
	// selection hides it again as the selection handles take over.
	e.showCaretHandle()
}

// TouchUp is called when this entry gets a touch up event on mobile device.
//
// Since: 1.0
func (e *Entry) TouchUp(*mobile.TouchEvent) {
}

// TouchCancel is called when this entry gets a touch cancel event on mobile device (app was removed from focus).
//
// Since: 1.0
func (e *Entry) TouchCancel(*mobile.TouchEvent) {
}

// TypedKey receives key input events when the Entry widget is focused.
// entryKeyHandlers dispatches a navigation/edit key to its handler. Returning
// false aborts TypedKey before the shared text-update/refresh tail. Keeping the
// table here (rather than a large switch in TypedKey) holds that method's
// cyclomatic complexity under the CI limit (pre-rebrand tracker #293).
var entryKeyHandlers = map[event.KeyName]func(e *Entry, provider *RichText, multiLine bool) bool{
	event.KeyBackspace: func(e *Entry, p *RichText, _ bool) bool { return e.typedKeyBackspace(p) },
	event.KeyDelete:    func(e *Entry, p *RichText, _ bool) bool { return e.typedKeyDelete(p) },
	event.KeyReturn:    func(e *Entry, p *RichText, ml bool) bool { e.typedKeyReturn(p, ml); return true },
	event.KeyEnter:     func(e *Entry, p *RichText, ml bool) bool { e.typedKeyReturn(p, ml); return true },
	event.KeyTab:       func(e *Entry, _ *RichText, _ bool) bool { e.typedKeyTab(); return true },
	event.KeyUp:        func(e *Entry, p *RichText, _ bool) bool { e.typedKeyUp(p); return true },
	event.KeyDown:      func(e *Entry, p *RichText, _ bool) bool { e.typedKeyDown(p); return true },
	event.KeyLeft:      func(e *Entry, p *RichText, _ bool) bool { e.typedKeyLeft(p); return true },
	event.KeyRight:     func(e *Entry, p *RichText, _ bool) bool { e.typedKeyRight(p); return true },
	event.KeyEnd:       func(e *Entry, p *RichText, _ bool) bool { e.typedKeyEnd(p); return true },
	event.KeyHome:      func(e *Entry, _ *RichText, _ bool) bool { e.typedKeyHome(); return true },
	event.KeyPageUp:    func(e *Entry, p *RichText, _ bool) bool { e.typedKeyPageUp(p); return true },
	event.KeyPageDown:  func(e *Entry, p *RichText, _ bool) bool { e.typedKeyPageDown(p); return true },
}

// HandlesEscape reports whether Escape belongs to this entry rather than to
// the pop-up or dialog around it: while an IME composition is in flight there
// is a composition to reject, and a host that set OnEscape has its own meaning
// for the key. Otherwise Escape falls through and closes the surface the entry
// sits in.
//
// Since: 1.0
func (e *Entry) HandlesEscape() bool {
	if e.Disabled() {
		return false
	}
	return e.composing || e.OnEscape != nil
}

func (e *Entry) TypedKey(key *event.KeyEvent) {
	if e.Disabled() {
		return
	}
	// Keyboard interaction dismisses the mobile caret drag-handle (pre-rebrand tracker #943).
	e.hideCaretHandle()
	if key.Name == event.KeyEscape {
		if e.composing {
			// Esc discards an in-progress IME composition without committing,
			// giving the user a way to reject the suggested composition (pre-rebrand tracker #354).
			e.ComposeCancel()
		} else if e.OnEscape != nil {
			// The cancel counterpart of OnSubmitted — lets a host dismiss an
			// inline editor (pre-rebrand tracker #1052).
			e.OnEscape()
		}
		return
	}
	// ReadOnly: allow navigation and selection keys, block text mutation.
	if e.ReadOnly {
		switch key.Name {
		case event.KeyBackspace, event.KeyDelete, event.KeyReturn, event.KeyEnter, event.KeyTab:
			return
		}
	}
	if e.cursorAnim != nil {
		e.cursorAnim.interrupt()
	}
	provider := e.textProvider()
	multiLine := e.MultiLine

	// Check shift state via driver modifiers (more reliable than KeyDown events).
	// Assert the narrower KeyModifierDriver (not desktop.Driver) so the mobile
	// driver — which tracks hardware-keyboard modifiers but is not a full desktop
	// driver — also drives Shift+Arrow selection (pre-rebrand tracker #1083); glfw satisfies both.
	if dd, ok := bellui.CurrentApp().Driver().(desktop.KeyModifierDriver); ok {
		e.selectKeyDown = dd.CurrentKeyModifiers()&event.KeyModifierShift != 0
		if e.selectKeyDown && !e.sel.selecting {
			e.sel.selectRow = e.CursorRow
			e.sel.selectColumn = e.CursorColumn
		}
	}
	if e.selectKeyDown || e.sel.selecting {
		if e.selectingKeyHandler(key) {
			e.Refresh()
			return
		}
	}

	// Navigation and edit keys default the caret to forward affinity; the insert
	// path (TypedRune/paste) overrides it to upstream where relevant.
	e.cursorUpstream = false

	handler, ok := entryKeyHandlers[key.Name]
	if !ok {
		return
	}
	// A handler returning false aborts before the text-update/refresh tail
	// (e.g. backspace/delete at a boundary that changed nothing).
	if !handler(e, provider, multiLine) {
		return
	}

	// Skip folded rows (pre-rebrand tracker #141), biased to the direction of travel, so caret
	// navigation never lands on a hidden line.
	e.skipHiddenRowForKey(key.Name)
	// Keep the caret out of caret-opaque protected spans (pre-rebrand tracker #140), biased to the
	// same direction so the span is skipped in a single keypress.
	e.clampCaretFromOpaqueForKey(key.Name)

	// Deferred text sync (pre-rebrand tracker #410): an editing key's splice stayed in the buffer
	// (for a pure navigation key this simply skips a no-op materialization
	// while a sync is pending); notifications wait for the next sync point.
	if e.editDeferred() {
		if e.CursorRow == e.sel.selectRow && e.CursorColumn == e.sel.selectColumn {
			e.sel.selecting = false
		}
		e.Refresh()
		return
	}
	content := provider.String()
	changed := e.updateText(content, false)
	if e.CursorRow == e.sel.selectRow && e.CursorColumn == e.sel.selectColumn {
		e.sel.selecting = false
	}
	cb := e.OnChanged
	if changed {
		e.validate()
		if cb != nil {
			cb(content)
		}
	}
	e.Refresh()
}

// Undo un-does the last modifying user-action.
//
// Undo replays the recorded document history verbatim; it is not re-gated
// against [Entry.ProtectedRanges], so it may reconstruct text overlapping a
// range protected after the undone edit was recorded. Protection applies to new
// interactive edits, not history replay.
//
// Since: 1.0
func (e *Entry) Undo() {
	action := e.undoStack.Undo()
	if action == nil {
		return
	}
	e.applyUndoAction(action, false)
}

func (e *Entry) typedKeyUp(provider *RichText) {
	if e.CursorRow > 0 {
		e.CursorRow--
	} else {
		e.CursorColumn = 0
	}

	rowLength := provider.rowLength(e.CursorRow)
	if e.CursorColumn > rowLength {
		e.CursorColumn = rowLength
	}
	e.syncSelectable()
}

func (e *Entry) typedKeyDown(provider *RichText) {
	rowLength := provider.rowLength(e.CursorRow)

	if e.CursorRow < provider.rows()-1 {
		e.CursorRow++
		rowLength = provider.rowLength(e.CursorRow)
	} else {
		e.CursorColumn = rowLength
	}

	if e.CursorColumn > rowLength {
		e.CursorColumn = rowLength
	}
	e.syncSelectable()
}

func (e *Entry) typedKeyLeft(provider *RichText) {
	// Visual movement: in RTL/mixed rows the Left arrow follows the visual order,
	// not the logical column. Non-RTL rows fall through to the logical step.
	if col, moved, atEdge := provider.visualColNeighbor(e.CursorRow, e.CursorColumn, -1); moved {
		e.CursorColumn = col
		e.syncSelectable()
		return
	} else if atEdge {
		if e.MultiLine && e.CursorRow > 0 {
			e.CursorRow--
			e.CursorColumn = provider.rowLength(e.CursorRow)
		}
		e.syncSelectable()
		return
	}

	if e.CursorColumn > 0 {
		e.CursorColumn--
	} else if e.MultiLine && e.CursorRow > 0 {
		e.CursorRow--
		e.CursorColumn = provider.rowLength(e.CursorRow)
	}
	e.syncSelectable()
}

func (e *Entry) typedKeyRight(provider *RichText) {
	if col, moved, atEdge := provider.visualColNeighbor(e.CursorRow, e.CursorColumn, +1); moved {
		e.CursorColumn = col
		e.syncSelectable()
		return
	} else if atEdge {
		if e.MultiLine && e.CursorRow < provider.rows()-1 {
			e.CursorRow++
			e.CursorColumn = 0
		}
		e.syncSelectable()
		return
	}

	if e.MultiLine {
		rowLength := provider.rowLength(e.CursorRow)
		if e.CursorColumn < rowLength {
			e.CursorColumn++
		} else if e.CursorRow < provider.rows()-1 {
			e.CursorRow++
			e.CursorColumn = 0
		}
	} else if e.CursorColumn < provider.len() {
		e.CursorColumn++
	}
	e.syncSelectable()
}

func (e *Entry) typedKeyHome() {
	e.CursorColumn = 0
}

func (e *Entry) typedKeyEnd(provider *RichText) {
	if e.MultiLine {
		e.CursorColumn = provider.rowLength(e.CursorRow)
	} else {
		e.CursorColumn = provider.len()
	}
}

// typedKeyBackspace deletes the character before the cursor. It returns false
// when there is nothing to delete, signalling the caller to return early.
func (e *Entry) typedKeyBackspace(provider *RichText) bool {
	isEmpty := provider.len() == 0 || (e.CursorColumn == 0 && e.CursorRow == 0)
	if isEmpty {
		return false
	}

	pos := e.CursorTextOffset()
	if !e.editAllowed(pos-1, pos, "") {
		return false
	}
	deletedText := provider.deleteFromTo(pos-1, pos)
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos - 1)
	e.syncSelectable()
	e.undoStack.MergeOrAdd(&entryModifyAction{
		Delete:   true,
		Position: pos - 1,
		Text:     deletedText,
	})
	return true
}

// typedKeyDelete deletes the character after the cursor. It returns false when
// there is nothing to delete, signalling the caller to return early.
func (e *Entry) typedKeyDelete(provider *RichText) bool {
	pos := e.CursorTextOffset()
	if provider.len() == 0 || pos == provider.len() {
		return false
	}
	if !e.editAllowed(pos, pos+1, "") {
		return false
	}

	deletedText := provider.deleteFromTo(pos, pos+1)
	e.undoStack.MergeOrAdd(&entryModifyAction{
		Delete:   true,
		Position: pos,
		Text:     deletedText,
	})
	return true
}

// pageRowCount is how many rows one PageUp/PageDown step moves the caret: a
// viewport screenful less one row of overlap, matching common editors. Falls
// back to a single row before the scroll viewport has a measured height (e.g.
// before the first layout), so paging still advances.
func (e *Entry) pageRowCount() int {
	if e.scroll == nil {
		return 1
	}
	_, stride := e.RowMetrics()
	viewH := e.scroll.Size().Height
	if stride <= 0 || viewH <= 0 {
		return 1
	}
	rows := int(viewH / stride)
	if rows > 1 {
		rows-- // keep one line of context across the page boundary
	}
	return max(rows, 1)
}

func (e *Entry) typedKeyPageUp(provider *RichText) {
	if e.MultiLine {
		e.CursorRow = max(0, e.CursorRow-e.pageRowCount())
		e.CursorColumn = min(e.CursorColumn, provider.rowLength(e.CursorRow))
	} else {
		e.CursorColumn = 0
	}
	e.syncSelectable()
}

func (e *Entry) typedKeyPageDown(provider *RichText) {
	if e.MultiLine {
		last := provider.rows() - 1
		e.CursorRow = min(last, e.CursorRow+e.pageRowCount())
		e.CursorColumn = min(e.CursorColumn, provider.rowLength(e.CursorRow))
	} else {
		e.CursorColumn = provider.len()
	}
	e.syncSelectable()
}

// typedKeyDocStart jumps the caret to the very beginning of the document.
func (e *Entry) typedKeyDocStart() {
	e.CursorRow = 0
	e.CursorColumn = 0
	e.syncSelectable()
}

// typedKeyDocEnd jumps the caret to the very end of the document.
func (e *Entry) typedKeyDocEnd(provider *RichText) {
	if e.MultiLine {
		last := provider.rows() - 1
		e.CursorRow = last
		e.CursorColumn = provider.rowLength(last)
	} else {
		e.CursorColumn = provider.len()
	}
	e.syncSelectable()
}

// handler for Ctrl+[backspace/delete] - delete the word
// to the left or right of the cursor
func (e *Entry) deleteWord(right bool) {
	provider := e.textProvider()
	cursorRow, cursorCol := e.CursorRow, e.CursorColumn

	// start, end relative to text row
	start, end := getTextWhitespaceRegion(provider.row(cursorRow), cursorCol, true)
	if right {
		start = cursorCol
	} else {
		end = cursorCol
	}
	if start == -1 || end == -1 {
		return
	}

	// convert start, end to absolute text position
	b := provider.rowBoundary(cursorRow)
	if b != nil {
		start += b.begin
		end += b.begin
	}

	if !e.editAllowed(start, end, "") {
		return
	}

	erased := provider.deleteFromTo(start, end)
	e.undoStack.MergeOrAdd(&entryModifyAction{
		Delete:   true,
		Position: start,
		Text:     erased,
	})

	if !right {
		e.CursorColumn = cursorCol - (end - start)
	}
	// Deferred text sync (pre-rebrand tracker #410): the word-delete splice stayed in the buffer.
	if e.editDeferred() {
		e.Refresh()
		return
	}
	e.updateTextAndRefresh(provider.String(), false)
}

func (e *Entry) typedKeyTab() {
	if dd, ok := bellui.CurrentApp().Driver().(desktop.Driver); ok {
		if dd.CurrentKeyModifiers()&event.KeyModifierShift != 0 {
			return // don't insert a tab when Shift+Tab typed
		}
	}
	e.TypedRune('\t')
}

// TypedRune receives text input events when the Entry widget is focused.
// isReadOnly returns true if the entry should not accept text mutations.
// This covers both Disabled (dimmed + non-editable) and ReadOnly (full
// styling + non-editable).
func (e *Entry) isReadOnly() bool {
	return e.Disabled() || e.ReadOnly
}

func (e *Entry) TypedRune(r rune) {
	// Typing dismisses the mobile caret drag-handle (pre-rebrand tracker #943).
	e.hideCaretHandle()
	if e.composing {
		// Desktop IME commit: the finalized character arrives as a normal char
		// event while the inline preedit is still active (committed text is not
		// delivered via ComposeCommit on the desktop driver). Route it through the
		// commit path so the preedit glyphs are dropped and the caret restored to
		// the anchor first — otherwise the committed text is inserted next to the
		// still-visible composition, appending instead of replacing it (pre-rebrand tracker #354).
		e.ComposeCommit(string(r))
		return
	}
	e.insertRunesAtCursor([]rune{r})
}

// insertRunesAtCursor inserts runes at the caret as if typed: it replaces any
// active selection, updates Text, records a single undo action, validates and
// fires OnChanged. It is the shared implementation behind TypedRune and IME
// commit (ComposeCommit). A no-op for read-only entries or an empty slice.
func (e *Entry) insertRunesAtCursor(runes []rune) {
	if e.isReadOnly() || len(runes) == 0 {
		return
	}

	e.syncSelectable()
	if e.popUp != nil {
		e.popUp.Hide()
	}

	// Gate the edit (selection replacement or caret insertion) before mutating.
	delStart, delEnd := e.editTargetForInsert()
	if !e.editAllowed(delStart, delEnd, string(runes)) {
		return
	}

	// if we've typed a character and we're selecting then replace the selection with the character
	cb := e.OnChanged
	if e.sel.selecting {
		e.eraseSelection()
	}

	pos := e.CursorTextOffset()

	provider := e.textProvider()
	provider.insertAt(pos, runes)

	// Deferred text sync (pre-rebrand tracker #410): the splice stayed in the buffer, so skip the
	// document materialization and its notifications until the next sync point.
	deferred := e.editDeferred()
	var content string
	if !deferred {
		content = provider.String()
		e.updateText(content, false)
	}
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos + len(runes))
	// Stick the caret to the character just typed (upstream), so at a direction
	// boundary it stays beside the new glyph instead of jumping to the far side.
	e.cursorUpstream = true
	e.syncSelectable()

	e.undoStack.MergeOrAdd(&entryModifyAction{
		Position: pos,
		Text:     runes,
	})

	if !deferred {
		e.validate()
		if cb != nil {
			cb(content)
		}
	}
	e.Refresh()
}

// composedDisplay returns the text to lay out and render: the committed Text
// with any in-progress IME preedit spliced in at preeditAnchor. When not
// composing it is simply Text.
//
// When the preedit was spliced into the provider's buffer (the lazy fast path,
// pre-rebrand tracker #402) this returns the buffer's memoized materialization — the same string
// object seg.Text already holds, so syncSegments performs a no-op store and
// the incremental row model survives every composition update. The string
// concatenation below is the fallback for non-lazy providers.
func (e *Entry) composedDisplay() string {
	if !e.composing || e.preeditText == "" {
		return e.Text
	}
	if e.preeditApplied > 0 {
		return e.text.String()
	}
	r := []rune(e.Text)
	if e.preeditAnchor < 0 {
		e.preeditAnchor = 0
	}
	if e.preeditAnchor > len(r) {
		e.preeditAnchor = len(r)
	}
	return string(r[:e.preeditAnchor]) + e.preeditText + string(r[e.preeditAnchor:])
}

// applyPreeditToProvider splices the preedit runes into the provider's buffer
// in place of the previously applied ones, so composition updates ride the
// incremental fast path instead of rebuilding the whole row model per event
// (pre-rebrand tracker #402). Reports false when the fast path is unavailable (non-lazy provider,
// folds, or an external write broke the identity chain mid-composition); the
// caller then relies on the composedDisplay string fallback.
func (e *Entry) applyPreeditToProvider(runes []rune) bool {
	provider := e.textProvider()
	if !provider.lazyCanEdit() {
		// An external rewrite mid-composition replaced the buffer content —
		// any previously applied range is gone with it.
		e.preeditApplied = 0
		return false
	}
	provider.lazySpliceText(e.preeditAnchor, e.preeditAnchor+e.preeditApplied, runes)
	e.preeditApplied = len(runes)
	return true
}

// removeAppliedPreedit splices previously applied preedit runes back out of
// the provider's buffer. A no-op when none are applied.
func (e *Entry) removeAppliedPreedit() {
	if e.preeditApplied == 0 {
		return
	}
	provider := e.textProvider()
	if provider.lazyCanEdit() {
		provider.lazySpliceText(e.preeditAnchor, e.preeditAnchor+e.preeditApplied, nil)
	}
	e.preeditApplied = 0
}

// Compose sets or replaces the inline IME preedit (composition) text shown at
// the caret. It does not modify Text, fire OnChanged, touch the undo stack or
// write to a bound value — only ComposeCommit does. An empty PreeditEvent.Text
// clears the visible preedit without committing.
//
// Compose implements the [event.Composable] interface; platform drivers call it
// while an input method is composing.
//
// Since: 1.0
func (e *Entry) Compose(ev *event.PreeditEvent) {
	if ev == nil || e.isReadOnly() {
		return
	}
	onMain(func() {
		if !e.composing {
			// First update of a composition: anchor at the caret, replacing any
			// selection just as typing would.
			if e.sel != nil && e.sel.selecting {
				e.eraseSelectionAndUpdate()
			}
			e.preeditAnchor = e.CursorTextOffset()
			e.composing = true
		}
		e.preeditText = ev.Text
		newRunes := []rune(ev.Text)
		pc := ev.CursorPos
		preeditLen := len(newRunes)
		if pc < 0 {
			pc = 0
		}
		if pc > preeditLen {
			pc = preeditLen
		}
		e.preeditCursor = pc

		// Splice the preedit into the provider's buffer so this update rides the
		// incremental fast path (pre-rebrand tracker #402); falls back to the composedDisplay string
		// path for non-lazy providers.
		e.applyPreeditToProvider(newRunes)

		e.syncSegments()
		e.textProvider().updateRowBounds()
		// Selection geometry would be offset by the inserted preedit glyphs, so
		// suppress it for the composition lifetime.
		if e.sel != nil {
			e.sel.selecting = false
		}
		e.CursorRow, e.CursorColumn = e.rowColFromTextPos(e.preeditAnchor + e.preeditCursor)
		e.cursorUpstream = true
		e.syncSelectable()
		e.Refresh()
	})
}

// ComposeCommit finalizes an IME composition, inserting text as if it had been
// typed: Text, the undo history and OnChanged are all updated as a single edit.
// An empty string commits nothing and behaves like ComposeCancel.
//
// ComposeCommit implements the [event.Composable] interface.
//
// Since: 1.0
func (e *Entry) ComposeCommit(text string) {
	onMain(func() {
		composing := e.composing
		anchor := e.preeditAnchor
		applied := e.preeditApplied

		// A selection made after composition started (a mid-composition
		// long-press word-select, a handle drag) must survive the commit:
		// the committed text lands at the composition anchor — where the
		// preedit was shown — never over that newer selection. Left active,
		// the anchor-restored caret below would pair with the selection's
		// anchor into an accidental giant range that the insert then
		// replaces (pre-rebrand tracker #945 device finding: committing after a long-press
		// deleted everything between the selection and the anchor).
		selStart, selEnd := -1, -1
		if composing && e.sel != nil && e.sel.selecting {
			selStart, selEnd = e.sel.selection()
			e.sel.selecting = false
		}

		// Likewise a bare caret placed OUTSIDE the preedit after composition
		// started (an external mid-composition caret move — on iOS a VoiceOver
		// tap lands on the native view and is forwarded here, pre-rebrand tracker #1118) must
		// survive the commit: the committed text still lands at the anchor,
		// and the caret returns to where the user put it, remapped below the
		// same way a preserved selection is. A caret inside the preedit (the
		// normal composing state) keeps the caret-after-committed convention.
		// The caret is expressed in display coordinates, which include the
		// preedit glyphs whether they were spliced into the provider (applied)
		// or rendered via the composedDisplay fallback — so the preedit span is
		// its rune length, not preeditApplied.
		span := len([]rune(e.preeditText))
		curPos := -1
		if composing && selStart < 0 {
			cur := textPosFromRowCol(e.CursorRow, e.CursorColumn, e.textProvider())
			if cur < anchor || cur > anchor+span {
				curPos = cur
			}
		}

		e.removeAppliedPreedit()
		e.clearPreedit()
		if composing {
			// Drop the preedit glyphs from the layout and move the caret back to
			// the anchor so the committed text lands where the preedit was shown.
			e.syncSegments()
			e.textProvider().updateRowBounds()
			e.CursorRow, e.CursorColumn = e.rowColFromTextPos(anchor)
			e.syncSelectable()
		}
		if text != "" {
			e.insertRunesAtCursor([]rune(text))
		} else if composing {
			e.Refresh()
		}

		// Remap a preserved selection or caret from the preedit-including display
		// space it was captured in: `removed` preedit runes came out at the
		// anchor and the committed runes went in there. The selection branch
		// (pre-rebrand tracker #945) was captured against the spliced provider, so its removed
		// length is preeditApplied; the caret branch uses the display span.
		n := len([]rune(text))
		adjustBy := func(o, removed int) int {
			if o >= anchor+removed {
				o -= removed
			} else if o > anchor {
				o = anchor
			}
			if o > anchor {
				o += n
			}
			return o
		}
		if selStart >= 0 {
			selStart, selEnd = adjustBy(selStart, applied), adjustBy(selEnd, applied)
			if selEnd > selStart {
				e.sel.selectRow, e.sel.selectColumn = e.rowColFromTextPos(selStart)
				e.CursorRow, e.CursorColumn = e.rowColFromTextPos(selEnd)
				e.sel.selecting = true
				e.syncSelectable()
				e.sel.notifySelectChanged()
			}
			e.Refresh()
		} else if curPos >= 0 {
			e.CursorRow, e.CursorColumn = e.rowColFromTextPos(adjustBy(curPos, span))
			e.syncSelectable()
			e.Refresh()
		}
	})
}

// ComposeCancel discards any in-progress IME composition without committing,
// restoring the caret to where composition began. Text is untouched.
//
// ComposeCancel implements the [event.Composable] interface.
//
// Since: 1.0
func (e *Entry) ComposeCancel() {
	onMain(func() {
		if !e.composing {
			return
		}
		e.cancelComposition()
		e.Refresh()
	})
}

// cancelComposition tears down an in-progress IME composition without
// committing: the preedit glyphs leave the layout and the caret returns to the
// anchor. Unlike ComposeCancel it neither refreshes nor synchronizes to the
// main goroutine — callers do both. No-op when not composing.
func (e *Entry) cancelComposition() {
	if !e.composing {
		return
	}
	anchor := e.preeditAnchor
	applied := e.preeditApplied

	// A selection made after composition started survives the cancel; its
	// offsets only shift when it sat beyond the spliced preedit (the
	// ComposeCommit pattern, without an insert).
	selStart, selEnd := -1, -1
	if e.sel != nil && e.sel.selecting {
		selStart, selEnd = e.sel.selection()
		e.sel.selecting = false
	}

	e.removeAppliedPreedit()
	e.clearPreedit()
	e.syncSegments()
	e.textProvider().updateRowBounds()
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(anchor)
	e.syncSelectable()

	if selStart >= 0 {
		adjust := func(o int) int {
			if o >= anchor+applied {
				return o - applied
			}
			if o > anchor {
				return anchor
			}
			return o
		}
		selStart, selEnd = adjust(selStart), adjust(selEnd)
		if selEnd > selStart {
			e.sel.selectRow, e.sel.selectColumn = e.rowColFromTextPos(selStart)
			e.CursorRow, e.CursorColumn = e.rowColFromTextPos(selEnd)
			e.sel.selecting = true
			e.syncSelectable()
			e.sel.notifySelectChanged()
		}
	}
}

// clearPreedit resets composition state. It does not refresh or touch Text.
func (e *Entry) clearPreedit() {
	e.composing = false
	e.preeditText = ""
	e.preeditCursor = 0
}

// styleRunsForRow returns the styled runs for a display row: the
// LineHighlighter output from HighlightRunsForRow, then the host style
// overlay (StyleOverlayForRow), then an underline over the IME preedit range
// while composing — so syntax highlighting, semantic overlay and preedit
// underline compose in that order.
func (e *Entry) styleRunsForRow(row int) []StyleRun {
	base := e.HighlightRunsForRow(row)
	base = e.overlayRunsForRow(row, base)
	if !e.composing || e.preeditText == "" {
		return base
	}

	provider := e.textProvider()
	if row < 0 || row >= provider.rows() {
		return base
	}
	rowRunes := provider.row(row)
	rowStart := textPosFromRowCol(row, 0, provider)
	pStart := e.preeditAnchor
	pEnd := e.preeditAnchor + len([]rune(e.preeditText))

	// Intersect the preedit span with this row (row-local rune coordinates).
	lo := pStart - rowStart
	hi := pEnd - rowStart
	if lo < 0 {
		lo = 0
	}
	if hi > len(rowRunes) {
		hi = len(rowRunes)
	}
	if lo >= hi {
		return base // preedit is not on this row
	}

	// The renderer requires the runs to tile the whole row; synthesize a base
	// run covering it when there is no syntax highlighting.
	if len(base) == 0 {
		base = []StyleRun{{Text: string(rowRunes)}}
	}
	return underlineSubrange(base, lo, hi)
}

// overlayRunsForRow composites the host style overlay (StyleOverlayForRow,
// pre-rebrand tracker #1079) onto the base runs for a display row. The overlay's runs tile the row
// from rune 0 by Text length; each styled run is merged over its range via
// overlaySubrange, zero-valued runs are spacers. When the row has no base runs
// (no LineHighlighter) a base run covering the row is synthesized, since the
// renderer requires runs to tile the whole row.
func (e *Entry) overlayRunsForRow(row int, base []StyleRun) []StyleRun {
	if e.StyleOverlayForRow == nil {
		return base
	}
	overlay := e.StyleOverlayForRow(row)
	if len(overlay) == 0 {
		return base
	}
	provider := e.textProvider()
	if row < 0 || row >= provider.rows() {
		return base
	}

	out := base
	idx := 0
	for _, ov := range overlay {
		lo, hi := idx, idx+len([]rune(ov.Text))
		idx = hi
		if hi <= lo || !styleRunStyled(ov) {
			continue // spacer
		}
		if len(out) == 0 {
			out = []StyleRun{{Text: string(provider.row(row))}}
		}
		out = overlaySubrange(out, lo, hi, ov)
	}
	return out
}

// styleRunStyled reports whether a run carries any styling — a zero-valued
// overlay run is a spacer that leaves its range untouched.
func styleRunStyled(r StyleRun) bool {
	return r.ForegroundColor != nil || r.ColorName != "" || r.Bold || r.Italic ||
		r.BackgroundColor != nil || r.Underline || r.StrikeThrough
}

// mergeStyleRun applies ov's set fields onto dst, keeping dst's values where
// ov is unset: a non-nil ForegroundColor or non-empty ColorName replaces the
// foreground as a unit (so an overlay ColorName is not shadowed by a stale
// base direct color), Bold/Italic only ever switch on, a set Underline
// carries its style and color with it, and a set StrikeThrough carries its
// color with it.
func mergeStyleRun(dst *StyleRun, ov StyleRun) {
	if ov.ForegroundColor != nil || ov.ColorName != "" {
		dst.ForegroundColor = ov.ForegroundColor
		dst.ColorName = ov.ColorName
	}
	if ov.Bold {
		dst.Bold = true
	}
	if ov.Italic {
		dst.Italic = true
	}
	if ov.BackgroundColor != nil {
		dst.BackgroundColor = ov.BackgroundColor
	}
	if ov.Underline {
		dst.Underline = true
		dst.UnderlineStyle = ov.UnderlineStyle
		dst.UnderlineColor = ov.UnderlineColor
	}
	if ov.StrikeThrough {
		dst.StrikeThrough = true
		dst.StrikeColor = ov.StrikeColor
	}
}

// underlineSubrange splits runs at the [lo, hi) rune boundaries (measured across
// the concatenated run text) and marks the covered pieces as underlined. Runs
// outside the range are returned unchanged.
func underlineSubrange(runs []StyleRun, lo, hi int) []StyleRun {
	return overlaySubrange(runs, lo, hi, StyleRun{Underline: true, UnderlineStyle: UnderlineSolid})
}

// overlaySubrange splits runs at the [lo, hi) rune boundaries (measured across
// the concatenated run text) and merges ov's set fields into the covered
// pieces (mergeStyleRun). Runs outside the range are returned unchanged.
func overlaySubrange(runs []StyleRun, lo, hi int, ov StyleRun) []StyleRun {
	out := make([]StyleRun, 0, len(runs)+2)
	idx := 0
	for _, run := range runs {
		rl := []rune(run.Text)
		runStart := idx
		runEnd := idx + len(rl)
		idx = runEnd

		cuts := []int{runStart}
		if lo > runStart && lo < runEnd {
			cuts = append(cuts, lo)
		}
		if hi > runStart && hi < runEnd {
			cuts = append(cuts, hi)
		}
		cuts = append(cuts, runEnd)

		for i := 0; i+1 < len(cuts); i++ {
			s, en := cuts[i], cuts[i+1]
			piece := run
			piece.Text = string(rl[s-runStart : en-runStart])
			if s >= lo && en <= hi {
				mergeStyleRun(&piece, ov)
			}
			out = append(out, piece)
		}
	}
	return out
}

// TypedShortcut implements the Shortcutable interface
func (e *Entry) TypedShortcut(shortcut core.Shortcut) {
	e.shortcut.TypedShortcut(shortcut)
}

// AddShortcut registers a handler for a keyboard shortcut on this Entry.
// The handler fires via TypedShortcut when the Entry has focus.
func (e *Entry) AddShortcut(shortcut core.Shortcut, handler func(core.Shortcut)) {
	e.shortcut.AddShortcut(shortcut, handler)
}

// Unbind disconnects any configured data source from this Entry.
// The current value will remain at the last value of the data source.
//
// Any [Entry.Validator] the app installed is left in place -- Unbind only drops
// the data-binding conversion check it installed itself.
//
// Since: 1.0
func (e *Entry) Unbind() {
	e.validatorInternal = nil
	e.binder.Unbind()
}

// copyToClipboard copies the current selection to a given clipboard.
// This does nothing if it is a concealed entry.
func (e *Entry) copyToClipboard(clipboard bellui.Clipboard) {
	if !e.sel.selecting || e.Password {
		return
	}

	clipboard.SetContent(e.sel.SelectedText())
}

// cutToClipboard copies the current selection to a given clipboard and then removes the selected text.
// This does nothing if it is a concealed entry.
func (e *Entry) cutToClipboard(clipboard bellui.Clipboard) {
	if !e.sel.selecting || e.Password {
		return
	}

	e.copyToClipboard(clipboard)
	e.eraseSelectionAndUpdate()
	// Deferred text sync (pre-rebrand tracker #410): notifications wait for the sync point.
	if e.text.deferredDirty {
		e.Refresh()
		return
	}
	content := e.Text
	cb := e.OnChanged

	e.validate()
	if cb != nil {
		cb(content)
	}
	e.Refresh()
}

// eraseSelection deletes the selected text and moves the cursor but does not update the text field.
func (e *Entry) eraseSelection() bool {
	if e.isReadOnly() {
		return false
	}

	provider := e.textProvider()
	posA, posB := e.sel.selection()

	if posA == posB {
		return false
	}

	erasedText := provider.deleteFromTo(posA, posB)
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(posA)
	e.syncSelectable()
	e.sel.selectRow, e.sel.selectColumn = e.CursorRow, e.CursorColumn
	e.sel.selecting = false

	e.undoStack.MergeOrAdd(&entryModifyAction{
		Delete:   true,
		Position: posA,
		Text:     erasedText,
	})

	return true
}

// eraseSelectionAndUpdate removes the current selected region and moves the cursor.
// It also updates the text if something has been erased.
func (e *Entry) eraseSelectionAndUpdate() {
	// Gate standalone selection deletion (cut, selection+delete/backspace, the
	// pre-erase before a newline). The select-then-insert paths (typing, paste)
	// gate the whole replacement and call eraseSelection directly, so they do
	// not double-fire the hook here.
	if e.sel != nil && e.sel.selecting {
		posA, posB := e.sel.selection()
		if posA != posB && !e.editAllowed(posA, posB, "") {
			return
		}
	}
	if e.eraseSelection() {
		// Deferred text sync (pre-rebrand tracker #410): the erase splice stayed in the buffer.
		if e.editDeferred() {
			return
		}
		e.updateText(e.textProvider().String(), false)
	}
}

// pasteFromClipboard inserts text from the clipboard content,
// starting from the cursor position.
func (e *Entry) pasteFromClipboard(clipboard bellui.Clipboard) {
	if e.isReadOnly() {
		return
	}
	e.syncSelectable()

	// Clipboard-flavor negotiation (pre-rebrand tracker #581). Entry is a plain-text surface: it
	// deliberately degrades the richer clipboard flavors rather than pretending
	// to support them. Editable rich-text/image paste is a separate widget
	// tracked in pre-rebrand tracker #648 and is intentionally NOT built here.
	//
	// HTML flavor: Entry never parses or strips markup. Per the clipboard
	// contract (clipboard.go), SetHTML always writes a plain-text fallback in the
	// same transaction and Content() reports it, so the plain Content() read
	// below already yields the correct text for an HTML-carrying clipboard — no
	// explicit HTMLClipboard handling is needed on a plain-text surface.
	text := clipboard.Content()

	// Image flavor: an image-only clipboard carries no plain text
	// (Content() == ""). A plain Entry has nowhere to put a raster image, so
	// pasting one is a documented no-op — not an error (editable image paste
	// into rich content lives in pre-rebrand tracker #648). Asserted explicitly so the intent is
	// clear and the early return is robust to later changes in the plain-text
	// branch below.
	if text == "" && !e.sel.selecting {
		if ic, ok := clipboard.(bellui.ImageClipboard); ok {
			if _, has := ic.Image(); has {
				return // Nothing plain-text to paste; image paste deferred to pre-rebrand tracker #648.
			}
		}
	}

	if text == "" {
		if !e.sel.selecting {
			return // Nothing to paste into the text content.
		}
		posA, posB := e.sel.selection()
		if posA == posB || !e.editAllowed(posA, posB, "") {
			return
		}
		if !e.eraseSelection() {
			return
		}
		// The erase is a text change like any other selection deletion, so run
		// the standard update-and-notify tail. Leaving Text stale here made the
		// following refresh restore the pre-erase content from it while the
		// undo stack kept the recorded deletion — reverting the erase on screen
		// and corrupting the history (pre-rebrand tracker #427).
		if e.editDeferred() { // deferred lazy-Text mode (pre-rebrand tracker #410)
			e.Refresh()
			return
		}
		content := e.textProvider().String()
		e.updateText(content, false)
		cb := e.OnChanged
		e.validate()
		if cb != nil {
			cb(content)
		}
		e.Refresh()
		return
	}

	if !e.MultiLine {
		// format clipboard content to be compatible with single line entry
		text = strings.Replace(text, "\n", " ", -1)
	}

	// Gate the whole replacement (selection -> pasted text) before mutating.
	delStart, delEnd := e.editTargetForInsert()
	if !e.editAllowed(delStart, delEnd, text) {
		return
	}

	if e.sel.selecting {
		e.eraseSelection()
	}

	runes := []rune(text)
	pos := e.CursorTextOffset()
	provider := e.textProvider()
	provider.insertAt(pos, runes)

	e.undoStack.Add(&entryModifyAction{
		Position: pos,
		Text:     runes,
	})
	// Deferred text sync (pre-rebrand tracker #410): the paste splice stayed in the buffer.
	deferred := e.editDeferred()
	var content string
	if !deferred {
		content = provider.String()
		e.updateText(content, false)
	}
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos + len(runes))
	// Stick the caret to the end of the pasted text (upstream affinity).
	e.cursorUpstream = true
	e.syncSelectable()

	if !deferred {
		cb := e.OnChanged
		e.validate()
		if cb != nil {
			cb(content) // We know that the text has changed.
		}
	}

	e.Refresh() // placing the cursor (and refreshing) happens last
}

// placeholderProvider returns the placeholder text handler for this entry
func (e *Entry) placeholderProvider() *RichText {
	e.placeholder.ExtendBaseWidget(&e.placeholder)
	if len(e.placeholder.Segments) > 0 {
		return &e.placeholder
	}

	e.placeholder.Scroll = widget.ScrollNone
	e.placeholder.inset = geo.NewSize(0, e.Theme().Size(theme.SizeNameInputBorder))

	style := RichTextStyleInline
	style.ColorName = theme.ColorNamePlaceHolder
	style.TextStyle = e.TextStyle

	e.placeholder.Segments = []RichTextSegment{
		&TextSegment{
			Style: style,
			Text:  e.PlaceHolder,
		},
	}

	return &e.placeholder
}

func (e *Entry) registerShortcut() {
	e.shortcut.AddShortcut(&bellui.ShortcutUndo{}, func(se core.Shortcut) {
		e.Undo()
	})
	e.shortcut.AddShortcut(&bellui.ShortcutRedo{}, func(se core.Shortcut) {
		e.Redo()
	})
	e.shortcut.AddShortcut(&bellui.ShortcutCut{}, func(se core.Shortcut) {
		cut := se.(*bellui.ShortcutCut)
		e.cutToClipboard(cut.Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutCopy{}, func(se core.Shortcut) {
		cpy := se.(*bellui.ShortcutCopy)
		e.copyToClipboard(cpy.Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutPaste{}, func(se core.Shortcut) {
		paste := se.(*bellui.ShortcutPaste)
		e.pasteFromClipboard(paste.Clipboard)
	})
	e.shortcut.AddShortcut(&bellui.ShortcutSelectAll{}, func(se core.Shortcut) {
		e.selectAll()
	})

	moveWord := func(s core.Shortcut) {
		row := e.textProvider().row(e.CursorRow)
		start, end := getTextWhitespaceRegion(row, e.CursorColumn, true)
		if start == -1 || end == -1 {
			return
		}

		e.setFieldsAndRefresh(func() {
			if s.(*desktop.CustomShortcut).KeyName == event.KeyLeft {
				if e.CursorColumn == 0 {
					if e.CursorRow > 0 {
						e.CursorRow--
						e.CursorColumn = len(e.textProvider().row(e.CursorRow))
					}
				} else {
					e.CursorColumn = start
				}
			} else {
				if e.CursorColumn == len(e.textProvider().row(e.CursorRow)) {
					if e.CursorRow < e.textProvider().rows()-1 {
						e.CursorRow++
						e.CursorColumn = 0
					}
				} else {
					e.CursorColumn = end
				}
			}
			e.syncSelectable()
		})
	}
	selectMoveWord := func(se core.Shortcut) {
		if !e.sel.selecting {
			e.sel.selectColumn = e.CursorColumn
			e.sel.selectRow = e.CursorRow
			e.sel.selecting = true
		}
		moveWord(se)
	}
	unselectMoveWord := func(se core.Shortcut) {
		e.sel.selecting = false
		moveWord(se)
	}

	moveWordModifier := event.KeyModifierShortcutDefault
	if runtime.GOOS == "darwin" {
		moveWordModifier = event.KeyModifierAlt

		// Cmd+left, Cmd+right shortcuts behave like Home and End keys on Mac OS
		shortcutHomeEnd := func(s core.Shortcut) {
			e.sel.selecting = false
			if s.(*desktop.CustomShortcut).KeyName == event.KeyLeft {
				e.typedKeyHome()
			} else {
				e.typedKeyEnd(e.textProvider())
			}
			e.Refresh()
		}
		e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: event.KeyModifierSuper}, shortcutHomeEnd)
		e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: event.KeyModifierSuper}, shortcutHomeEnd)
	}

	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: moveWordModifier}, unselectMoveWord)
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: moveWordModifier | event.KeyModifierShift}, selectMoveWord)
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: moveWordModifier}, unselectMoveWord)
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: moveWordModifier | event.KeyModifierShift}, selectMoveWord)

	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyBackspace, Modifier: moveWordModifier},
		func(core.Shortcut) { e.deleteWord(false) })
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: event.KeyDelete, Modifier: moveWordModifier},
		func(core.Shortcut) { e.deleteWord(true) })

	// Document-jump shortcuts: Ctrl+Home/Ctrl+End (Cmd+ArrowUp/ArrowDown on
	// macOS) move the caret to the very start/end of the document, with Shift
	// variants extending the selection. This complements PageUp/PageDown, which
	// page by a viewport screenful (pre-rebrand tracker #377).
	docJump := func(toEnd, selecting bool) func(core.Shortcut) {
		return func(core.Shortcut) {
			if selecting {
				if !e.sel.selecting {
					e.sel.selectColumn = e.CursorColumn
					e.sel.selectRow = e.CursorRow
					e.sel.selecting = true
				}
			} else {
				e.sel.selecting = false
			}
			if toEnd {
				e.typedKeyDocEnd(e.textProvider())
			} else {
				e.typedKeyDocStart()
			}
			e.Refresh()
		}
	}
	docStartKey, docEndKey := event.KeyHome, event.KeyEnd
	if runtime.GOOS == "darwin" {
		docStartKey, docEndKey = event.KeyUp, event.KeyDown
	}
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: docStartKey, Modifier: event.KeyModifierShortcutDefault}, docJump(false, false))
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: docStartKey, Modifier: event.KeyModifierShortcutDefault | event.KeyModifierShift}, docJump(false, true))
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: docEndKey, Modifier: event.KeyModifierShortcutDefault}, docJump(true, false))
	e.shortcut.AddShortcut(&desktop.CustomShortcut{KeyName: docEndKey, Modifier: event.KeyModifierShortcutDefault | event.KeyModifierShift}, docJump(true, true))
}

func (e *Entry) requestFocus() {
	impl := e.super()
	if c := bellui.CurrentApp().Driver().CanvasForObject(impl); c != nil {
		c.Focus(impl.(event.Focusable))
	}
}

// Obtains row,col from a given textual position
// expects a read or write lock to be held by the caller
func (e *Entry) rowColFromTextPos(pos int) (row int, col int) {
	return rowColFromTextPos(pos, e.textProvider(), e.Wrapping)
}

// selectAll selects all text in entry
func (e *Entry) selectAll() {
	if e.textProvider().len() == 0 {
		return
	}
	e.setFieldsAndRefresh(func() {
		e.sel.selectRow = 0
		e.sel.selectColumn = 0

		lastRow := e.textProvider().rows() - 1
		e.CursorColumn = e.textProvider().rowLength(lastRow)
		e.CursorRow = lastRow
		e.syncSelectable()
		e.sel.selecting = true
	})
	e.sel.notifySelectChanged()
}

// selectingKeyHandler performs keypress action in the scenario that a selection
// is either a) in progress or b) about to start
// returns true if the keypress has been fully handled
func (e *Entry) selectingKeyHandler(key *event.KeyEvent) bool {
	if e.selectKeyDown && !e.sel.selecting {
		switch key.Name {
		case event.KeyUp, event.KeyDown,
			event.KeyLeft, event.KeyRight,
			event.KeyEnd, event.KeyHome,
			event.KeyPageUp, event.KeyPageDown:
			e.sel.selecting = true
		}
	}

	if !e.sel.selecting {
		return false
	}

	switch key.Name {
	case event.KeyBackspace, event.KeyDelete:
		// clears the selection -- return handled
		e.eraseSelectionAndUpdate()
		// Deferred text sync (pre-rebrand tracker #410): notifications wait for the sync point.
		if e.text.deferredDirty {
			e.Refresh()
			return true
		}
		content := e.Text
		cb := e.OnChanged

		e.validate()
		if cb != nil {
			cb(content)
		}
		e.Refresh()
		return true
	case event.KeyReturn, event.KeyEnter:
		if e.MultiLine {
			// clear the selection -- return unhandled to add the newline
			e.setFieldsAndRefresh(e.eraseSelectionAndUpdate)
		}
		return false
	}

	if !e.selectKeyDown {
		switch key.Name {
		case event.KeyLeft:
			// seek to the start of the selection -- return handled
			selectStart, _ := e.sel.selection()
			e.CursorRow, e.CursorColumn = e.rowColFromTextPos(selectStart)
			e.syncSelectable()
			e.sel.selecting = false
			return true
		case event.KeyRight:
			// seek to the end of the selection -- return handled
			_, selectEnd := e.sel.selection()
			e.CursorRow, e.CursorColumn = e.rowColFromTextPos(selectEnd)
			e.syncSelectable()
			e.sel.selecting = false
			return true
		case event.KeyUp, event.KeyDown, event.KeyEnd, event.KeyHome, event.KeyPageUp, event.KeyPageDown:
			// cursor movement without left or right shift -- clear selection and return unhandled
			e.sel.selecting = false
			return false
		}
	}

	return false
}

// updateHighlightCache incrementally re-highlights lines affected by text changes.
// Called from syncSegments when a LineHighlighter is set.
//
// In the lazy single-segment model the cache is filled on demand instead:
// HighlightRunsForRow drives ensureHighlightedThrough for the rows the renderer
// actually shows, so a SetText of a huge document highlights one screenful, not
// every line (pre-rebrand tracker #401). Edits keep the cache aligned via noteLazyLinesEdited.
func (e *Entry) updateHighlightCache() {
	if e.LineHighlighter == nil {
		e.hlCache = nil
		e.hlComputedThrough = 0
		e.hlTrusted = nil
		e.stopHlPrefill()
		return
	}
	if e.text.lazy && e.text.buf != nil {
		return // windowed: built and verified on demand in ensureHighlightedThrough
	}

	lines := strings.Split(e.Text, "\n")
	// Grow or shrink cache to match line count.
	for len(e.hlCache) < len(lines) {
		e.hlCache = append(e.hlCache, hlCacheLine{stateIn: -1})
	}
	e.hlCache = e.hlCache[:len(lines)]

	// Re-highlight from the first line whose content or input state changed.
	state := 0
	for i, line := range lines {
		if e.hlCache[i].stateIn == state && e.hlCache[i].text == line && e.hlCache[i].runs != nil {
			// Cache hit — same state and same text, skip.
			state = e.hlCache[i].stateOut
			continue
		}
		// Cache miss — re-highlight from here.
		runs, newState := e.LineHighlighter(line, state)
		e.hlCache[i] = hlCacheLine{text: line, stateIn: state, stateOut: newState, runs: runs}
		state = newState
		// If next line's cached state and text match, we can stop (convergence).
		if i+1 < len(lines) && i+1 < len(e.hlCache) &&
			e.hlCache[i+1].stateIn == state && e.hlCache[i+1].text == lines[i+1] && e.hlCache[i+1].runs != nil {
			break
		}
	}
	// The eager loop leaves every line chained; keep the lazy watermark in step
	// in case the widget later switches to the lazy model. Everything is
	// prefix-verified, so no separate trusted stretches remain.
	e.hlComputedThrough = len(lines)
	e.hlTrusted = nil
}

// noteLazyLinesEdited keeps hlCache index-aligned with the provider's logical
// lines after an incremental edit replaced lines [a..z] with newCount lines,
// and lowers the computed watermark so ensureHighlightedThrough re-chains from
// the edit. Wired to RichText.onLazyLinesEdited (pre-rebrand tracker #401).
func (e *Entry) noteLazyLinesEdited(a, z, newCount int) {
	if e.LineHighlighter == nil || len(e.hlCache) == 0 {
		return
	}
	end := z + 1
	if a > len(e.hlCache) {
		a = len(e.hlCache)
	}
	if end > len(e.hlCache) {
		end = len(e.hlCache)
	}
	if end-a != newCount {
		fresh := make([]hlCacheLine, newCount)
		for i := range fresh {
			fresh[i].stateIn = -1
		}
		e.hlCache = spliceSlice(e.hlCache, a, end, fresh)
	} else {
		for i := a; i < end; i++ {
			e.hlCache[i] = hlCacheLine{stateIn: -1}
		}
	}

	delta := newCount - (end - a)
	oldW := e.hlComputedThrough
	if e.hlComputedThrough > a {
		e.hlComputedThrough = a
	}
	if e.hlComputedThrough < oldW {
		// The edit lowered the watermark, so a tail of rows now needs
		// re-chaining. Since the synchronous chain is capped (pre-rebrand tracker #167), the
		// background prefill (pre-rebrand tracker #417) is the path that reconverges rows far below
		// the watermark — a deep viewport can no longer pull them all in one
		// render — so (re-)arm it here in case a prior chain had already
		// completed and stopped it. Idempotent when still running; a no-op when
		// no driver is attached or the chain is complete.
		e.startHlPrefill()
	}

	// Keep the trusted stretches aligned with the splice (pre-rebrand tracker #412): shift each
	// stretch above the edit, split one the edit lands inside — both halves
	// stay text-current and chain-consistent, so scattered batch edits between
	// renders each leave their own stretch instead of discarding half (pre-rebrand tracker #418).
	if len(e.hlTrusted) == 0 && oldW <= a {
		return // nothing tracked and nothing newly chained below the edit
	}
	next := make([]hlStretch, 0, len(e.hlTrusted)+1)
	// The already-chained lines between the edit and the old watermark stay
	// text-current and chain-consistent ([0, watermark) is splice-free by
	// construction): record them as a trusted stretch a later re-chain can
	// jump over, so an edit far above a deep viewport re-runs the highlighter
	// only until the state converges (pre-rebrand tracker #412). Every existing stretch sits above
	// the old watermark, so prepending keeps the list sorted and disjoint.
	if from, to := a+newCount, oldW+delta; to > from {
		next = append(next, hlStretch{from, to})
	}
	for _, s := range e.hlTrusted {
		switch {
		case s.from >= end: // wholly above the splice: shift
			next = append(next, hlStretch{s.from + delta, s.to + delta})
		case s.to <= a: // wholly below the splice: untouched
			next = append(next, s)
		default: // the splice lands inside: keep both surviving halves
			if s.from < a {
				next = append(next, hlStretch{s.from, a})
			}
			if s.to > end {
				next = append(next, hlStretch{a + newCount, s.to + delta})
			}
		}
	}
	for len(next) > hlMaxTrustedStretches {
		shortest := 0
		for i, s := range next {
			if s.to-s.from < next[shortest].to-next[shortest].from {
				shortest = i
			}
		}
		next = append(next[:shortest], next[shortest+1:]...)
	}
	e.hlTrusted = next
}

// ensureHighlightedThrough computes the forward-chained highlight state for
// logical lines [hlComputedThrough, target] in the lazy model, reusing cached
// entries whose (stateIn, text) still match so an edit re-runs the highlighter
// only until the state chain converges. Line text is read from the provider's
// buffer — no document-wide split (pre-rebrand tracker #401). Once the chain converges into a
// trusted stretch left by an earlier edit, the walk jumps over it wholesale,
// so re-chaining to a deep viewport touches O(edits + convergence + window)
// lines even after scattered batch edits between renders (pre-rebrand tracker #412, pre-rebrand tracker #418); only
// the very first deep jump after a content replacement (SetText)
// has to chain from line 0, since no prior state is known for fresh content —
// and the background prefill (pre-rebrand tracker #417) usually gets there first, leaving only
// the gap above its watermark to chain synchronously.
func (e *Entry) ensureHighlightedThrough(target int) {
	if e.LineHighlighter == nil || !e.text.lazy || e.text.buf == nil {
		return
	}
	p := &e.text
	// A content replacement rebuilds the provider's buffer object (SetText,
	// external write); restart the chain — cached entries still shortcut
	// unchanged prefixes via the per-line compare below, but the trusted
	// stretch is void: its text-currency was verified against the old buffer,
	// so a deep jump into fresh content chains from line 0 (pre-rebrand tracker #412). Kick off
	// the background prefill so that first chain rarely happens synchronously:
	// idle frame ticks advance the watermark until it reaches the end (pre-rebrand tracker #417).
	if e.hlBuf != p.buf {
		e.hlBuf = p.buf
		e.hlComputedThrough = 0
		e.hlTrusted = nil
		e.startHlPrefill()
	}
	L := p.buf.LineCount()
	for len(e.hlCache) < L {
		e.hlCache = append(e.hlCache, hlCacheLine{stateIn: -1})
	}
	e.hlCache = e.hlCache[:L]
	// The list is sorted, so truncation strands stretches from the tail in.
	for n := len(e.hlTrusted); n > 0; n = len(e.hlTrusted) {
		s := &e.hlTrusted[n-1]
		if s.to > L {
			s.to = L
		}
		if s.from < s.to {
			break
		}
		e.hlTrusted = e.hlTrusted[:n-1]
	}
	if target >= L {
		target = L - 1
	}
	i := e.hlComputedThrough
	// Bound the synchronous chain (pre-rebrand tracker #167). A render request for a row far below the
	// highlight watermark — a fold that collapses a huge region so its footer row
	// jumps into view, or a scroll that lands deep before the prefill has caught up
	// — would otherwise chain the whole prefix in one blocking pass (auto-folding a
	// 437k-row @dataset hitched ~1.15s). Cap how far a single call chains: the
	// requested row renders unstyled for now, and the background prefill (pre-rebrand tracker #417) —
	// whose slice target stays within hlPrefillSliceLines of the watermark, well
	// under this cap, so it is never clamped — converges the remainder over frames.
	// Its running animation keeps repainting the window, so the deep row picks up
	// its highlight progressively instead of freezing the apply.
	if target > i+maxSyncHighlightChain {
		target = i + maxSyncHighlightChain
	}
	if i > target {
		return
	}
	state := 0
	if i > 0 {
		state = e.hlCache[i-1].stateOut
	}
	ti := 0 // hlTrusted is sorted and i only grows, so this never rescans
	for i <= target {
		// Jump over a trusted stretch (pre-rebrand tracker #412): its entries hold current text
		// and a pairwise-consistent chain, so an entering state that matches
		// the recorded one revalidates the whole stretch at once. With several
		// stretches tracked (pre-rebrand tracker #418) each is jumped as the walk reaches it.
		for ti < len(e.hlTrusted) && e.hlTrusted[ti].to <= i {
			ti++
		}
		if ti < len(e.hlTrusted) && i >= e.hlTrusted[ti].from && e.hlCache[i].stateIn == state {
			state = e.hlCache[e.hlTrusted[ti].to-1].stateOut
			i = e.hlTrusted[ti].to
			ti++
			continue
		}
		e.hlWalkSteps++
		text := p.lineText(i)
		if c := &e.hlCache[i]; c.runs != nil && c.stateIn == state && c.text == text {
			state = c.stateOut
			i++
			continue
		}
		runs, newState := e.LineHighlighter(text, state)
		e.hlCache[i] = hlCacheLine{text: text, stateIn: state, stateOut: newState, runs: runs}
		state = newState
		i++
	}
	// A jump may have carried the chain past target; everything below i is
	// now prefix-verified, so only stretch remainders above it stay.
	e.hlComputedThrough = i
	for len(e.hlTrusted) > 0 && e.hlTrusted[0].to <= i {
		e.hlTrusted = e.hlTrusted[1:]
	}
	if len(e.hlTrusted) > 0 && e.hlTrusted[0].from < i {
		e.hlTrusted[0].from = i
	}
}

// Background highlight prefill (pre-rebrand tracker #417): after a content replacement the state
// chain restarts at line 0, so the first deep jump would have to chain the
// whole document synchronously. hlPrefillTick runs once per rendered frame
// (driven by a repeat-forever animation, i.e. on the main goroutine between
// input events) and extends the chain in small slices under a time budget, so
// it never delays a keystroke; when a deep jump arrives it pays only for the
// gap still above the watermark. Highlighting work is identical either way —
// prefill just moves it off the latency-sensitive path.
const hlPrefillSliceLines = 256

// maxSyncHighlightChain caps how many lines a single synchronous
// ensureHighlightedThrough call will chain when a render request lands far below
// the highlight watermark (pre-rebrand tracker #167). It sits well above any realistic viewport jump
// (a viewport is tens of rows) so ordinary scrolling and editing still highlight
// synchronously and exactly, and only the pathological far jump — a collapsed
// fold's footer, a deep scroll before the prefill has caught up — is bounded,
// converging afterward through the background prefill. It must exceed
// hlPrefillSliceLines so a prefill slice, whose target stays within that many
// lines of the watermark, is never itself clamped.
//
// Tuned down from 20000 (pre-rebrand tracker #693): the one-time synchronous apply of a collapsed
// huge fold scales with this cap (~7µs/line, so ~139ms at 20000 vs ~55ms here),
// and the prefill now repaints the deep row itself as its watermark reaches the
// viewport (repaintIfPrefillReachedViewport), so convergence no longer depends on
// the cap being large. Kept above the deepest fixed deep-jump test (pre-rebrand tracker #412) so that
// exact-chaining assertion stays valid without churn.
const maxSyncHighlightChain = 8000

// hlPrefillFrameBudget bounds the highlighting work done per frame tick — a
// few thousand lines of a typical highlighter, i.e. a 500k-line document
// finishes in a few seconds while leaving most of a 60 fps frame for input
// and paint. It is a var so tests can zero it, pinning each tick to exactly
// one slice regardless of machine speed.
var hlPrefillFrameBudget = 5 * time.Millisecond

// startHlPrefill arms the prefill driver; a no-op when it is already running.
// Called on the main goroutine from updateText after a wholesale content
// replacement, and from ensureHighlightedThrough when it detects a fresh
// buffer (an external swap that did not pass through updateText).
func (e *Entry) startHlPrefill() {
	if e.hlPrefill != nil {
		return
	}
	if a := bellui.CurrentApp(); a == nil || a.Driver() == nil {
		return
	}
	anim := &bellui.Animation{
		Duration:    time.Second, // irrelevant: Tick fires every frame while running
		RepeatCount: bellui.AnimationRepeatForever,
		Curve:       bellui.AnimationLinear,
		Tick:        func(float32) { e.hlPrefillTick() },
	}
	e.hlPrefill = anim
	e.hlPrefillArming = true
	anim.Start()
	e.hlPrefillArming = false
}

func (e *Entry) stopHlPrefill() {
	if e.hlPrefill == nil {
		return
	}
	e.hlPrefill.Stop()
	e.hlPrefill = nil
}

// hlPrefillTick advances the highlight chain by bounded slices until the
// per-frame budget is spent, stopping the driver once the watermark reaches
// the end of the document (or the lazy highlight path no longer applies).
// Edits below the watermark simply lower it — the next tick resumes from the
// edit and re-converges over the trusted stretch (pre-rebrand tracker #412), so no explicit
// restart is needed; a SetText swaps the buffer and the tick re-chains from
// line 0 against the new content.
func (e *Entry) hlPrefillTick() {
	if e.hlPrefillArming {
		return // synchronous tick from inside Start; wait for a real frame
	}
	// Repaint the text if this tick's newly-highlighted lines cover a visible row
	// (pre-rebrand tracker #693). The prefill advances only the highlight *state*; without an explicit
	// repaint, dirty-region culling leaves a deep visible row (e.g. a collapsed
	// fold's footer above the watermark) showing unstyled until some unrelated
	// refresh, since the running animation does not mark the entry dirty. Bounding
	// it to the watermark's passage through the viewport keeps repaints rare.
	watermarkBefore := e.hlComputedThrough
	defer e.repaintIfPrefillReachedViewport(watermarkBefore)

	deadline := time.Now().Add(hlPrefillFrameBudget)
	for {
		p := &e.text
		if e.LineHighlighter == nil || !p.lazy || p.buf == nil {
			e.stopHlPrefill()
			return
		}
		if e.hlBuf != p.buf {
			// Fresh buffer: the stale watermark must not size this slice's
			// target. Chaining line 0 resets the bookkeeping (and re-arms are
			// no-ops); the next iteration continues with real slices.
			e.ensureHighlightedThrough(0)
			continue
		}
		L := p.buf.LineCount()
		if e.hlComputedThrough >= L {
			e.stopHlPrefill()
			return
		}
		e.ensureHighlightedThrough(min(e.hlComputedThrough+hlPrefillSliceLines, L) - 1)
		if !time.Now().Before(deadline) {
			return // resume on the next frame
		}
	}
}

// repaintIfPrefillReachedViewport refreshes the text region when the prefill
// watermark advanced from `before` to now across (or into) the visible rows, so
// those rows re-render with their freshly-computed highlight (pre-rebrand tracker #693). It is a
// no-op when the watermark did not advance (an edit lowered it, or the buffer
// reset), or when the newly-highlighted band [before, now) does not overlap the
// viewport — so the passage of the watermark past the viewport costs at most a
// few O(window) repaints, and a watermark climbing far below or above the
// viewport costs none.
func (e *Entry) repaintIfPrefillReachedViewport(before int) {
	now := e.hlComputedThrough
	if now <= before {
		return
	}
	visStart, visEnd := e.text.visibleRowRange()
	if visEnd <= visStart {
		return
	}
	if before < visEnd && now > visStart {
		e.hlPrefillRepaints++
		e.text.Refresh()
	}
}

// HighlightRunsForRow returns the styled runs for a given row index.
// Returns nil if no highlighter is active or the row is out of range.
// The gutter/highlighter path runs wrap-off, so visual rows and logical lines
// coincide; in the lazy model the requested row is highlighted on demand.
func (e *Entry) HighlightRunsForRow(row int) []StyleRun {
	if row < 0 {
		return nil
	}
	e.ensureHighlightedThrough(row)
	if row >= len(e.hlCache) {
		return nil
	}
	// A deep render jump caps the synchronous chain (pre-rebrand tracker #167), so the row may sit
	// above the verified watermark. Its cache entry is then either untouched
	// (nil, unstyled) or a leftover from a prior pass whose upstream highlighter
	// state has since changed (a trusted stretch the capped walk never reached to
	// revalidate) — returning that would paint stale colours. Report unstyled
	// until the background prefill converges the watermark past this row.
	// hlComputedThrough is the count of chained lines, i.e. the first uncomputed
	// index, so a verified row satisfies row < hlComputedThrough.
	if row >= e.hlComputedThrough {
		return nil
	}
	return e.hlCache[row].runs
}

func (e *Entry) syncSegments() {
	colName := theme.ColorNameForeground
	wrap := e.textWrap()
	disabled := e.Disabled()
	if disabled {
		colName = theme.ColorNameDisabled
	}

	text := e.textProvider()
	text.Wrapping = wrap

	// Enable row virtualisation (pre-rebrand tracker #119, pre-rebrand tracker #139) for multi-line content that lives
	// inside the scroll container — the single-segment, uniform-height case where
	// a large document would otherwise materialise (and shape) every row. This
	// now covers word/break wrapping too: wrapped rows share the same uniform
	// stride, so only the visible window is materialised while rowBounds still
	// holds every wrapped fragment for caret/selection geometry.
	scrollActive := e.Wrapping != bellui.TextWrapOff || e.Scroll != widget.ScrollNone
	text.virtualize = e.MultiLine && scrollActive

	// Deferred text sync (pre-rebrand tracker #410): pushed to the provider like virtualize, so the
	// lazy splice path knows whether to skip the per-edit materialization.
	text.deferText = e.DeferTextSync

	// Push folded ranges (pre-rebrand tracker #141) to the provider; it normalises and clamps them to
	// the current row count, so ranges left dangling by a shrunken document are
	// reconciled here on every text change.
	text.setHiddenRanges(e.HiddenRanges)

	// Always use a single segment for editing (cursor, selection, row bounds).
	// Syntax highlighting is stored in hlCache and applied at render time.
	e.updateHighlightCache()

	// The editing segment shows the committed Text with any in-progress IME
	// preedit spliced in at the anchor. Text itself is never mutated, so undo,
	// OnChanged and bindings are unaffected by composition.
	display := e.composedDisplay()

	if len(text.Segments) != 1 {
		text.Segments = []RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: display}}
		text.rowBoundsVersion++ // new segment list — invalidate row bounds
	}
	textSegment := text.Segments[0].(*TextSegment)
	if textSegment.Text != display || textSegment.Style.ColorName != colName ||
		textSegment.Style.concealed != e.Password || textSegment.Style.TextStyle != e.TextStyle ||
		textSegment.Style.Alignment != e.Alignment {
		text.rowBoundsVersion++ // content or style changed — invalidate row bounds
	}
	textSegment.Text = display
	textSegment.Style.ColorName = colName
	textSegment.Style.concealed = e.Password
	textSegment.Style.TextStyle = e.TextStyle
	textSegment.Style.Alignment = e.Alignment

	colName = theme.ColorNamePlaceHolder
	if disabled {
		colName = theme.ColorNameDisabled
	}

	placeholder := e.placeholderProvider()
	placeholder.Wrapping = wrap

	if len(placeholder.Segments) > 0 {
		textSegment := placeholder.Segments[0].(*TextSegment)
		textSegment.Style.ColorName = colName
		textSegment.Style.TextStyle = e.TextStyle
		textSegment.Style.Alignment = e.Alignment
		textSegment.Text = e.PlaceHolder
	}
}

func (e *Entry) syncSelectable() {
	if e.sel == nil {
		e.sel = &selectable{theme: e.Theme(), provider: e.textProvider(), focus: e, password: e.Password, style: e.TextStyle}
		e.sel.ExtendBaseWidget(e.sel)
	}

	e.sel.cursorRow, e.sel.cursorColumn = e.CursorRow, e.CursorColumn
	e.sel.keepSelection = e.KeepSelectionVisible
	e.sel.onSelectChanged = e.OnSelectChanged
	// Keep the selection's alignment in step with the Entry so the highlight
	// rectangles (and drag hit-testing, via alignOffsetX) track the text edge
	// under center/trailing alignment — e.g. a right-aligned NumericEntry. Without
	// this the selectable defaults to leading and draws the highlight on the left
	// while the text sits on the right (pre-rebrand tracker #781).
	e.sel.alignment = e.Alignment
}

// textProvider returns the text handler for this entry
func (e *Entry) textProvider() *RichText {
	e.text.ExtendBaseWidget(&e.text)
	// Always keep RowHighlighter and ShowLineNumbers in sync.
	if e.LineHighlighter != nil {
		e.text.RowHighlighter = e.styleRunsForRow
		e.text.ShowLineNumbers = !e.SuppressGutter
		if e.text.onLazyLinesEdited == nil {
			e.text.onLazyLinesEdited = e.noteLazyLinesEdited
		}
	}
	// Sync debugger fields to the underlying RichText (suppressed with gutter).
	if e.SuppressGutter {
		e.text.BreakpointLines = nil
		e.text.HighlightedRow = 0
	} else {
		e.text.BreakpointLines = e.BreakpointLines
		e.text.HighlightedRow = e.HighlightedRow
	}
	if len(e.text.Segments) > 0 {
		return &e.text
	}

	if e.Text != "" {
		e.dirty = true
	}

	e.text.Scroll = widget.ScrollNone
	e.text.inset = geo.NewSize(0, e.Theme().Size(theme.SizeNameInputBorder))
	e.text.Segments = []RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: e.composedDisplay()}}
	// Wire the highlight cache (and preedit underline) to the RichText renderer.
	e.text.RowHighlighter = e.styleRunsForRow
	return &e.text
}

// textWrap calculates the wrapping that we should apply.
func (e *Entry) textWrap() bellui.TextWrap {
	if e.Wrapping == bellui.TextWrap(bellui.TextTruncateClip) { // this is now the default - but we scroll around this large content
		return bellui.TextWrapOff
	}

	if !e.MultiLine && (e.Wrapping == bellui.TextWrapBreak || e.Wrapping == bellui.TextWrapWord) {
		bellui.LogError("Entry cannot wrap single line", nil)
		e.Wrapping = bellui.TextWrap(bellui.TextTruncateClip)
		return bellui.TextWrapOff
	}
	return e.Wrapping
}

func (e *Entry) updateCursorAndSelection() {
	e.CursorRow, e.CursorColumn = e.truncatePosition(e.CursorRow, e.CursorColumn)

	e.syncSelectable()
	e.sel.selectRow, e.sel.selectColumn = e.truncatePosition(e.sel.selectRow, e.sel.selectColumn)
}

func (e *Entry) updateFromData(data binding.DataItem) {
	if data == nil {
		return
	}
	textSource, ok := data.(binding.String)
	if !ok {
		return
	}

	val, err := textSource.Get()
	e.conversionError = err
	e.validate()
	if err != nil {
		return
	}
	e.setText(val, true)
}

func (e *Entry) truncatePosition(row, col int) (int, int) {
	// Use the provider length, not e.Text, so an in-progress IME preedit (which
	// is shown in the provider but not in e.Text) keeps the caret in range.
	if e.textProvider().len() == 0 {
		return 0, 0
	}
	newRow := row
	newCol := col
	if row >= e.textProvider().rows() {
		newRow = e.textProvider().rows() - 1
	}
	rowLength := e.textProvider().rowLength(newRow)
	if (newCol >= rowLength) || (newRow < row) {
		newCol = rowLength
	}
	return newRow, newCol
}

func (e *Entry) updateMousePointer(p geo.Position, rightClick bool) {
	row, col, downstream := e.sel.getRowCol(p)

	if !rightClick || !e.sel.selecting {
		e.CursorRow = row
		e.CursorColumn = col
		// Land the caret on the side of the direction boundary that was clicked.
		e.cursorUpstream = !downstream

		e.syncSelectable()
		// A click inside a caret-opaque span snaps to the nearer boundary.
		e.clampCaretFromOpaque(0)
		row, col = e.CursorRow, e.CursorColumn
	}

	if !e.sel.selecting {
		e.sel.selectRow = row
		e.sel.selectColumn = col
	}

	r := cache.Renderer(e.content)
	if r != nil {
		r.(*entryContentRenderer).moveCursor()
	}
}

// updateText updates the internal text to the given value.
// It assumes that a lock exists on the widget.
func (e *Entry) updateText(text string, fromBinding bool) bool {
	changed := e.Text != text
	e.Text = text
	e.syncSegments()
	e.text.updateRowBounds()

	// A wholesale replacement voids the highlight chain: Entry.SetText resets
	// the cache before reaching here (the buffer object may survive when the
	// new text is value-identical), and a binding write swaps the buffer
	// without resetting. Arm the background prefill (pre-rebrand tracker #417) at this seam rather
	// than only from the render path — a repaint can serve already-materialized
	// rows without consulting the highlighter, so a render-side hook is not
	// guaranteed to run. Ordinary edits keep their cache (noteLazyLinesEdited)
	// and don't arm.
	if e.LineHighlighter != nil && e.text.lazy && e.text.buf != nil &&
		(e.hlBuf != e.text.buf || (e.hlComputedThrough == 0 && e.hlCache == nil)) {
		e.startHlPrefill()
	}

	if e.Text != "" {
		e.dirty = true
	}

	if changed && !fromBinding {
		if e.binder.dataListenerPair.listener != nil {
			e.binder.SetCallback(nil)
			e.binder.CallWithData(e.writeData)
			e.binder.SetCallback(e.updateFromData)
		}
	}
	return changed
}

// updateTextAndRefresh updates the internal text to the given value then refreshes it.
// This should not be called under a property lock
func (e *Entry) updateTextAndRefresh(text string, fromBinding bool) {
	var callback func(string)

	changed := e.updateText(text, fromBinding)

	if changed {
		callback = e.OnChanged
	}

	e.validate()
	if callback != nil {
		callback(text)
	}
	e.Refresh()
}

// editDeferred finishes an interactive edit whose provider splice was deferred
// (DeferTextSync, pre-rebrand tracker #410): it reports true when the buffer is ahead of Text so
// the caller skips the materializing tail (updateText, Validator, OnChanged,
// binding write) — all deferred to the next sync point. False means the edit
// took the normal path (mode off, or the fast path did not apply) and the
// caller must run the default tail.
func (e *Entry) editDeferred() bool {
	if !e.text.deferredDirty {
		return false
	}
	e.dirty = true
	return true
}

// syncDeferredText is the deferred-mode (pre-rebrand tracker #410) materialization: it re-anchors
// the provider's identity chain to the buffer's current content and updates
// Text (and, via updateText, any bound value). It reports the current text and
// whether it differed from the last-synced Text. Cheap no-op when nothing is
// pending. During an IME composition the preedit runes spliced into the buffer
// for display are excluded — they are not committed text — and the identity
// chain is left deferred until the composition ends.
func (e *Entry) syncDeferredText() (string, bool) {
	provider := e.textProvider()
	if !provider.deferredDirty {
		return e.Text, false
	}
	if e.preeditApplied > 0 {
		n := provider.buf.Len()
		content := provider.buf.Substring(0, e.preeditAnchor) +
			provider.buf.Substring(e.preeditAnchor+e.preeditApplied, n)
		return content, e.updateText(content, false)
	}
	provider.lazyMaterialize()
	content := provider.String()
	return content, e.updateText(content, false)
}

// syncDeferredTextNotify runs a full deferred-mode sync point: materialize the
// pending text and, when it changed, run the Validator and fire OnChanged once
// with the full current text (the binding write happens inside updateText).
func (e *Entry) syncDeferredTextNotify() string {
	content, changed := e.syncDeferredText()
	if changed {
		e.validate()
		if cb := e.OnChanged; cb != nil {
			cb(content)
		}
	}
	return content
}

// SyncText returns the entry's current text, first synchronising any edits the
// [Entry.DeferTextSync] mode left pending: the Text field is refreshed and, if
// the text changed since the last sync point, the Validator, OnChanged and any
// bound value observe it now. When the mode is off (the default) Text is always
// current and SyncText simply returns it.
// It is safe to call from any goroutine.
//
// Since: 1.0
func (e *Entry) SyncText() string {
	var text string
	onMain(func() {
		text = e.syncDeferredTextNotify()
	})
	return text
}

func (e *Entry) writeData(data binding.DataItem) {
	if data == nil {
		return
	}
	textTarget, ok := data.(binding.String)
	if !ok {
		return
	}
	curValue, err := textTarget.Get()
	if err == nil && curValue == e.Text {
		e.conversionError = nil
		return
	}
	e.conversionError = textTarget.Set(e.Text)
}

func (e *Entry) typedKeyReturn(provider *RichText, multiLine bool) {
	onSubmitted := e.OnSubmitted
	selectDown := e.selectKeyDown
	text := e.Text
	// Submission is a deferred-mode sync point (pre-rebrand tracker #410): OnSubmitted must see
	// the current text, and the pending OnChanged/Validator run first.
	if onSubmitted != nil && (!multiLine || selectDown) && e.text.deferredDirty {
		text = e.syncDeferredTextNotify()
	}

	if !multiLine {
		// Single line doesn't support newline.
		// Call submitted callback, if any.
		if onSubmitted != nil {
			onSubmitted(text)
		}
		return
	} else if selectDown && onSubmitted != nil {
		// Multiline supports newline, unless shift is held and OnSubmitted is set.
		onSubmitted(text)
		return
	}
	s := []rune("\n")
	pos := e.CursorTextOffset()
	if !e.editAllowed(pos, pos, "\n") {
		return
	}
	provider.insertAt(pos, s)
	e.undoStack.MergeOrAdd(&entryModifyAction{
		Position: pos,
		Text:     s,
	})
	e.CursorColumn = 0
	e.CursorRow++
	e.syncSelectable()
}

func (e *Entry) setFieldsAndRefresh(f func()) {
	f()

	impl := e.super()
	if impl == nil {
		return
	}
	impl.Refresh()
}

var _ core.WidgetRenderer = (*entryRenderer)(nil)

type entryRenderer struct {
	box, border *canvas.Rectangle
	indicator   float32 // the filled style's indicator thickness, from shapeEntryChrome
	scroll      *widget.Scroll
	icon        *canvas.Image

	objects []core.CanvasObject
	entry   *Entry
}

func (r *entryRenderer) Destroy() {
	// Stop the background highlight prefill: its animation closure would
	// otherwise keep a discarded Entry (rope buffer, hlCache) alive and
	// ticking until the chain reaches the end of the document (pre-rebrand tracker #421). Destroy
	// also fires on cache eviction of a live, temporarily-hidden widget, so
	// CreateRenderer re-arms an unfinished chain on re-show.
	r.entry.stopHlPrefill()
	if r.entry.popUp != nil {
		r.entry.popUp.Hide()
		r.entry.popUp = nil
	}
}

// RoundedClipCornerRadius implements the rounded clipping interface (the
// same seam Card uses), so the scroll bar — whose square-cornered strip
// otherwise paints past the rounded outline, obvious since the pre-rebrand tracker #449
// grab-target work — and any content at the corners stay inside the drawn
// shape (pre-rebrand tracker #467).
//
// The clip applies at the widget bounds, but the visible box is inset by one
// input border. Offsetting the box's rounded rect outward by that border
// gives exactly the widget rect with radius+border (same arc centers), so
// this radius makes the clip's arc concentric with the box's; the remaining
// one-border band between the two arcs is covered by the border stroke,
// which draws on top (pre-rebrand tracker #353). The radius is the theme's,
// which the box's top corners take in either style.
//
// The clip has one radius for all four corners, so it also rounds the filled
// style's square bottom corners, which MD3 leaves square (#494).
func (r *entryRenderer) RoundedClipCornerRadius() float32 {
	th := r.entry.Theme()
	return th.Size(theme.SizeNameInputRadius) + th.Size(theme.SizeNameInputBorder)
}

func (r *entryRenderer) trailingInset() float32 {
	th := r.entry.Theme()
	xInset := float32(0)

	if r.entry.ActionItem != nil {
		xInset = r.entry.ActionItem.MinSize().Width
	}

	if r.entry.hasValidator() && !r.entry.ValidateWithBorder {
		iconSpace := th.Size(theme.SizeNameInlineIcon) + th.Size(theme.SizeNameLineSpacing)
		if r.entry.ActionItem == nil {
			xInset = iconSpace + th.Size(theme.SizeNameInnerPadding)
		} else {
			xInset += iconSpace
		}
	}

	return xInset
}

func (r *entryRenderer) leadingInset() float32 {
	th := r.entry.Theme()
	xInset := float32(0)

	if r.entry.Icon != nil {
		xInset = th.Size(theme.SizeNameInlineIcon) + th.Size(theme.SizeNameLineSpacing)
	}

	return xInset
}

func (r *entryRenderer) Layout(size geo.Size) {
	th := r.entry.Theme()
	borderSize := th.Size(theme.SizeNameInputBorder)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	innerPad := th.Size(theme.SizeNameInnerPadding)
	inputBorder := th.Size(theme.SizeNameInputBorder)

	layoutEntryChrome(r.box, r.border, size, borderSize, r.indicator)

	pad := theme.InputBorderSize()
	actionIconSize := geo.NewSize(0, size.Height-pad*2)
	if r.entry.ActionItem != nil {
		actionIconSize.Width = r.entry.ActionItem.MinSize().Width
		r.entry.ActionItem.Resize(actionIconSize)
		r.entry.ActionItem.Move(geo.NewPos(size.Width-actionIconSize.Width-pad, pad))
	}

	validatorIconSize := geo.NewSize(0, 0)
	if r.entry.hasValidator() || r.entry.AlwaysShowValidationError {
		validatorIconSize = geo.NewSquareSize(iconSize)

		r.ensureValidationSetup()
		r.entry.validationStatus.Resize(validatorIconSize)

		if r.entry.ActionItem == nil {
			r.entry.validationStatus.Move(geo.NewPos(size.Width-validatorIconSize.Width-innerPad, innerPad))
		} else {
			r.entry.validationStatus.Move(geo.NewPos(size.Width-validatorIconSize.Width-actionIconSize.Width, innerPad))
		}
	}

	if r.entry.Icon != nil {
		r.icon.Resize(geo.NewSquareSize(iconSize))
		r.icon.Move(geo.NewPos(innerPad, innerPad))
	}

	r.entry.textProvider().inset = geo.NewSize(0, inputBorder)
	r.entry.placeholderProvider().inset = geo.NewSize(0, inputBorder)
	// The viewport sits inside the background box on all four sides: it was
	// always inset vertically, but horizontally it used to span the full
	// widget, leaving the scroll bar riding over the border stroke and
	// poking an inputBorder past the box's edge (pre-rebrand tracker #467).
	entrySize := size.Subtract(geo.NewSize(r.trailingInset()+r.leadingInset()+inputBorder*2, inputBorder*2))
	entryYOffset := inputBorder
	// Vertically center text content for single-line entries that are taller than min height.
	if !r.entry.MultiLine {
		contentH := r.entry.textProvider().MinSize().Height
		available := size.Height - inputBorder*2
		if contentH < available {
			entryYOffset = (size.Height - contentH) / 2
		}
	}
	entryPos := geo.NewPos(r.leadingInset()+inputBorder, entryYOffset)

	prov := r.entry.textProvider()
	r.entry.cursorMu.Lock()
	curRow, curCol := r.entry.CursorRow, r.entry.CursorColumn
	r.entry.cursorMu.Unlock()
	textPos := textPosFromRowCol(curRow, curCol, prov)
	selectPos := textPosFromRowCol(r.entry.sel.selectRow, r.entry.sel.selectColumn, prov)
	if r.entry.Wrapping == bellui.TextWrapOff && r.entry.Scroll == widget.ScrollNone {
		r.entry.content.Resize(entrySize)
		r.entry.content.Move(entryPos)
	} else {
		r.scroll.Resize(entrySize)
		r.scroll.Move(entryPos)
	}

	r.entry.cursorMu.Lock()
	curRow, curCol = r.entry.CursorRow, r.entry.CursorColumn
	r.entry.cursorMu.Unlock()
	resizedTextPos := textPosFromRowCol(curRow, curCol, prov)
	if textPos != resizedTextPos {
		r.entry.setFieldsAndRefresh(func() {
			r.entry.cursorMu.Lock()
			r.entry.CursorRow, r.entry.CursorColumn = r.entry.rowColFromTextPos(textPos)
			r.entry.sel.cursorRow, r.entry.sel.cursorColumn = r.entry.CursorRow, r.entry.CursorColumn
			r.entry.cursorMu.Unlock()

			if r.entry.sel.selecting {
				r.entry.sel.selectRow, r.entry.sel.selectColumn = r.entry.rowColFromTextPos(selectPos)
			}
		})
	}

	// The scroll viewport is now sized: re-window the virtualised text for the
	// initial layout and any resize (OnScrolled alone misses both). (pre-rebrand tracker #119)
	r.entry.updateTextViewport()
}

// MinSize calculates the minimum size of an entry widget.
// This is based on the contained text with a standard amount of padding added.
// If MultiLine is true then we will reserve space for at leasts 3 lines
func (r *entryRenderer) MinSize() geo.Size {
	if rend := cache.Renderer(r.entry.content); rend != nil {
		rend.(*entryContentRenderer).updateScrollDirections()
	}

	th := r.entry.Theme()
	minSize := geo.Size{}

	if r.scroll.Direction == widget.ScrollNone {
		minSize = r.entry.content.MinSize().AddWidthHeight(0, th.Size(theme.SizeNameInputBorder)*2)
	} else {
		innerPadding := th.Size(theme.SizeNameInnerPadding)
		textSize := th.Size(theme.SizeNameText)
		charMin := r.entry.placeholderProvider().charMinSize(r.entry.Password, r.entry.TextStyle, textSize)
		minSize = charMin.Add(geo.NewSquareSize(innerPadding))

		if r.entry.MultiLine {
			count := r.entry.multiLineRows
			if count <= 0 {
				count = multiLineRows
			}

			minSize.Height = charMin.Height*float32(count) + innerPadding
		}

		minSize = minSize.AddWidthHeight(innerPadding*2, innerPadding)
	}

	iconSpace := th.Size(theme.SizeNameInlineIcon) + th.Size(theme.SizeNameLineSpacing)
	if r.entry.ActionItem != nil {
		minSize.Width += iconSpace
	}
	if r.entry.hasValidator() && !r.entry.ValidateWithBorder {
		minSize.Width += iconSpace
	}
	if r.entry.Icon != nil {
		minSize.Width += iconSpace
	}

	return minSize
}

func (r *entryRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *entryRenderer) Refresh() {
	content := r.entry.content
	focusedAppearance := r.entry.focused && !r.entry.Disabled()
	scroll := r.entry.Scroll
	wrapping := r.entry.Wrapping

	// BaseWidget.Refresh runs SyncState first, which has just synced and
	// refreshed the text (its Refresh updates the row bounds too). A renderer
	// refreshed directly has not.
	if r.entry.textSynced {
		r.entry.textSynced = false
	} else {
		r.entry.syncSegments()
		r.entry.text.updateRowBounds()
		r.entry.text.Refresh()
	}
	r.entry.placeholder.updateRowBounds()
	// Syntax highlighting is applied in textRenderer.Refresh() via
	// applySyntaxColors() using the RowHighlighter callback.
	r.entry.placeholder.Refresh()

	th := r.entry.Theme()
	inputBorder := th.Size(theme.SizeNameInputBorder)

	r.refreshScrollWrapping(content, wrapping, scroll, inputBorder)
	r.entry.updateCursorAndSelection()

	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.refreshBoxAndBorder(th, v, focusedAppearance)

	if r.entry.ActionItem != nil {
		r.entry.ActionItem.Refresh()
	}

	r.refreshValidationStyle(th, v)
	r.indicator = shapeEntryChrome(r.box, r.border, r.entry.Style != EntryStyleOutlined,
		focusedAppearance || r.entry.showsValidationError(),
		th.Size(theme.SizeNameInputRadius), inputBorder)
	layoutEntryChrome(r.box, r.border, r.entry.Size(), inputBorder, r.indicator)

	if r.entry.Icon != nil {
		r.icon.Resource = r.entry.Icon
		r.icon.Refresh()
		r.icon.Show()
	} else {
		r.icon.Hide()
	}

	r.entry.sel.Hidden = !r.entry.focused && !r.entry.KeepSelectionVisible
	r.entry.updateSelectionHandles()
	r.entry.updateCaretHandle()

	cache.Renderer(r.entry.content).Refresh()
	canvas.Refresh(r.entry.super())
}

// refreshScrollWrapping corrects the scroll wrappers if the wrap mode changed.
// The viewport rect must match entryRenderer.Layout: inset by one input
// border on every side of the background box (pre-rebrand tracker #467).
func (r *entryRenderer) refreshScrollWrapping(content core.CanvasObject, wrapping bellui.TextWrap, scroll widget.ScrollDirection, inputBorder float32) {
	entrySize := r.entry.Size().Subtract(geo.NewSize(r.trailingInset()+r.leadingInset()+inputBorder*2, inputBorder*2))
	entryPos := geo.NewPos(r.leadingInset()+inputBorder, inputBorder)
	if wrapping == bellui.TextWrapOff && scroll == widget.ScrollNone && r.scroll.Content != nil {
		r.scroll.Hide()
		r.scroll.Content = nil
		content.Move(entryPos)
		content.Resize(entrySize)

		for i, o := range r.objects {
			if o == r.scroll {
				r.objects[i] = content
				break
			}
		}
	} else if (wrapping != bellui.TextWrapOff || scroll != widget.ScrollNone) && r.scroll.Content == nil {
		r.scroll.Content = content
		content.Move(geo.NewPos(0, 0))
		r.scroll.Move(entryPos)
		r.scroll.Resize(entrySize)
		r.scroll.Show()

		for i, o := range r.objects {
			if o == content {
				r.objects[i] = r.scroll
				break
			}
		}
	}
}

// refreshBoxAndBorder applies the background fill and border styling.
func (r *entryRenderer) refreshBoxAndBorder(th bellui.Theme, v bellui.ThemeVariant, focusedAppearance bool) {
	style := internaltheme.RenderStyleForWidget(r.entry)
	isOutlined := r.entry.Style == EntryStyleOutlined

	if isOutlined {
		r.box.FillColor = color.Transparent
	} else {
		r.box.FillColor = th.Color(theme.ColorNameInputBackground, v)
	}
	// shapeEntryChrome sets the corner radii, once the border is styled.

	if focusedAppearance {
		r.border.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		if isOutlined || style == internaltheme.RenderStyleModern {
			r.border.StrokeWidth = th.Size(theme.SizeNameInputBorder) * 2
		}
		return
	}

	if isOutlined || style == internaltheme.RenderStyleModern {
		r.border.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	}
	switch {
	case r.entry.Disabled():
		r.border.StrokeColor = th.Color(theme.ColorNameDisabled, v)
	case isOutlined:
		r.border.StrokeColor = th.Color(theme.ColorNameOutline, v)
	default:
		r.border.StrokeColor = th.Color(theme.ColorNameInputBorder, v)
	}
}

// showsValidationError reports whether the entry shows its validation error.
func (e *Entry) showsValidationError() bool {
	return (e.hasValidator() || e.AlwaysShowValidationError) && !e.Disabled() &&
		(e.dirty || e.AlwaysShowValidationError) && e.validationError != nil
}

// refreshValidationStyle applies validation-driven border or status styling.
func (r *entryRenderer) refreshValidationStyle(th bellui.Theme, v bellui.ThemeVariant) {
	if !r.entry.hasValidator() && !r.entry.AlwaysShowValidationError {
		if r.entry.validationStatus != nil {
			r.entry.validationStatus.Hide()
		}
		return
	}

	hasError := r.entry.showsValidationError()
	if r.entry.ValidateWithBorder {
		// Show validation via border styling: thicker + red on error
		if hasError {
			r.border.StrokeColor = th.Color(theme.ColorNameError, v)
			r.border.StrokeWidth = th.Size(theme.SizeNameInputBorder) * 2
		} else {
			r.border.StrokeWidth = th.Size(theme.SizeNameInputBorder)
		}
		if r.entry.validationStatus != nil {
			r.entry.validationStatus.Hide()
		}
		return
	}

	if hasError && !r.entry.focused {
		r.border.StrokeColor = th.Color(theme.ColorNameError, v)
	}
	r.ensureValidationSetup()
	r.entry.validationStatus.Refresh()
}

func (r *entryRenderer) ensureValidationSetup() {
	if r.entry.validationStatus == nil {
		r.entry.validationStatus = newValidationStatus(r.entry)
		r.objects = append(r.objects, r.entry.validationStatus)
		r.Layout(r.entry.Size())

		r.entry.validate()
		r.Refresh()
	}
}

var _ core.Widget = (*entryContent)(nil)

type entryContent struct {
	BaseWidget

	entry  *Entry
	scroll *widget.Scroll
}

func (e *entryContent) CreateRenderer() core.WidgetRenderer {
	e.ExtendBaseWidget(e)

	provider := e.entry.textProvider()
	placeholder := e.entry.placeholderProvider()
	if provider.len() != 0 {
		placeholder.Hide()
	}
	objects := []core.CanvasObject{placeholder, provider, e.entry.cursorAnim.cursor}

	r := &entryContentRenderer{
		e.entry.cursorAnim.cursor, objects,
		provider, placeholder, e,
	}
	r.updateScrollDirections()
	r.Layout(e.Size())
	return r
}

// DragEnd is called at end of a drag event.
func (e *entryContent) DragEnd() {
	// we need to propagate the focus, top level widget handles focus APIs
	e.entry.requestFocus()

	e.entry.DragEnd()
}

// Dragged is called when the pointer moves while a button is held down.
// It updates the selection accordingly.
func (e *entryContent) Dragged(d *event.DragEvent) {
	e.entry.Dragged(d)
}

// HoverIn tracks pointer X for gutter cursor icon switching.
func (e *entryContent) HoverIn(m *event.HoverEvent) {
	e.entry.lastMouseX = m.Position.X
}

// HoverMoved tracks pointer X for gutter cursor icon switching.
func (e *entryContent) HoverMoved(m *event.HoverEvent) {
	e.entry.lastMouseX = m.Position.X
}

// HoverOut resets pointer tracking.
func (e *entryContent) HoverOut() {
	e.entry.lastMouseX = -1
}

var _ core.WidgetRenderer = (*entryContentRenderer)(nil)

type entryContentRenderer struct {
	cursor  *canvas.Rectangle
	objects []core.CanvasObject

	provider, placeholder *RichText
	content               *entryContent
}

func (r *entryContentRenderer) Destroy() {
	r.content.entry.cursorAnim.stop()
	// The selection is laid out with the text but is one of Objects only while
	// it is selecting, so a release of the tree does not reach its renderer:
	// left cached, it outlived the entry's (#720).
	if sel := r.content.entry.sel; sel != nil {
		cache.DestroyRenderer(sel)
	}
}

func (r *entryContentRenderer) Layout(size geo.Size) {
	r.provider.Resize(size)
	r.placeholder.Resize(size)
}

func (r *entryContentRenderer) MinSize() geo.Size {
	r.content.Theme() // setup theme cache before locking
	minSize := r.content.entry.placeholderProvider().MinSize()

	if r.content.entry.textProvider().len() > 0 {
		minSize = r.content.entry.text.MinSize()
	}

	return minSize
}

func (r *entryContentRenderer) Objects() []core.CanvasObject {
	// Objects are generated dynamically force selection rectangles to appear underneath the text
	if r.content.entry.sel.selecting {
		return append([]core.CanvasObject{r.content.entry.sel}, r.objects...)
	}
	return r.objects
}

func (r *entryContentRenderer) Refresh() {
	provider := r.content.entry.textProvider()
	placeholder := r.content.entry.placeholderProvider()
	focused := r.content.entry.focused
	focusedAppearance := focused && !r.content.entry.Disabled()
	r.updateScrollDirections()

	if provider.len() == 0 {
		placeholder.Show()
	} else if placeholder.Visible() {
		placeholder.Hide()
	}

	th := r.content.entry.Theme()
	if focusedAppearance {
		settings := bellui.CurrentApp().Settings()
		if settings.ShowAnimations() {
			r.content.entry.cursorAnim.start()
		} else {
			r.cursor.FillColor = th.Color(theme.ColorNamePrimary, settings.ThemeVariant())
		}
		r.cursor.Show()
	} else {
		r.content.entry.cursorAnim.stop()
		r.cursor.Hide()
	}
	r.moveCursor()

	canvas.Refresh(r.content)
}

func (r *entryContentRenderer) ensureCursorVisible() {
	if r.content.entry.skipScrollOnRefresh {
		return
	}
	th := r.content.entry.Theme()
	lineSpace := th.Size(theme.SizeNameLineSpacing)

	letter := bellui.MeasureText("e", th.Size(theme.SizeNameText), r.content.entry.TextStyle)
	padX := letter.Width*2 + lineSpace
	padY := letter.Height - lineSpace
	cx := r.cursor.Position().X
	cy := r.cursor.Position().Y
	cx1 := cx - padX
	cy1 := cy - padY
	cx2 := cx + r.cursor.Size().Width + padX
	cy2 := cy + r.cursor.Size().Height + padY
	offset := r.content.scroll.Offset
	size := r.content.scroll.Size()

	if offset.X <= cx1 && cx2 < offset.X+size.Width &&
		offset.Y <= cy1 && cy2 < offset.Y+size.Height {
		return
	}

	move := geo.NewDelta(0, 0)
	if cx1 < offset.X {
		move.DX -= offset.X - cx1
	} else if cx2 >= offset.X+size.Width {
		move.DX += cx2 - (offset.X + size.Width)
	}
	if cy1 < offset.Y {
		move.DY -= offset.Y - cy1
	} else if cy2 >= offset.Y+size.Height {
		move.DY += cy2 - (offset.Y + size.Height)
	}
	if r.content.scroll.Content != nil {
		r.content.scroll.ScrollToOffset(r.content.scroll.Offset.Add(move))
		// ScrollToOffset fires OnScrolled (which re-windows) only when the offset
		// moved; re-window here too so cursor-driven scrolling (keyboard nav,
		// paging) always sees the current text, moved or not.
		r.content.entry.updateTextViewport()
	}
}

func (r *entryContentRenderer) moveCursor() {
	// build r.selection[] if the user has made a selection
	r.content.entry.sel.Refresh()

	th := r.content.entry.Theme()
	textSize := th.Size(theme.SizeNameText)
	inputBorder := th.Size(theme.SizeNameInputBorder)

	lineHeight := r.content.entry.text.charMinSize(r.content.entry.Password, r.content.entry.TextStyle, textSize).Height
	r.cursor.Resize(geo.NewSize(inputBorder, lineHeight))
	r.cursor.Move(r.content.entry.CursorPosition())

	callback := r.content.entry.OnCursorChanged
	r.ensureCursorVisible()

	if callback != nil {
		callback()
	}
}

func (r *entryContentRenderer) updateScrollDirections() {
	if r.content.scroll == nil { // not scrolling
		return
	}

	switch r.content.entry.Wrapping {
	case bellui.TextWrapOff:
		r.content.scroll.Direction = r.content.entry.Scroll
	case bellui.TextWrap(bellui.TextTruncateClip): // this is now the default - but we scroll
		r.content.scroll.Direction = widget.ScrollBoth
	default: // bellui.TextWrapBreak, bellui.TextWrapWord
		r.content.scroll.Direction = widget.ScrollVerticalOnly
	}
}

// getTextWhitespaceRegion returns the start/end markers for selection highlight on starting from col
// and expanding to the start and end of the whitespace or text underneath the specified position.
// Pass `true` for `expand` if you want whitespace selection to extend to the neighboring words.
func getTextWhitespaceRegion(row []rune, col int, expand bool) (int, int) {
	if len(row) == 0 || col < 0 {
		return -1, -1
	}

	// If the click position exceeds the length of text then snap it to the end
	if col >= len(row) {
		col = len(row) - 1
	}

	// maps: " fi-sh 日本語本語日  \t "
	// into: " -- -- ------   "
	space := func(r rune) rune {
		// If this rune is a typical word separator then classify it as whitespace
		if isWordSeparator(r) {
			return ' '
		}
		return '-'
	}
	toks := strings.Map(space, string(row))
	c := byte(' ')

	startCheck := col
	endCheck := col
	if expand {
		if col > 0 && toks[col-1] == ' ' { // ignore the prior whitespace then count
			startCheck = strings.LastIndexByte(toks[:startCheck], '-')
			if startCheck == -1 {
				startCheck = 0
			}
		}
		if toks[col] == ' ' { // ignore the current whitespace then count
			endCheck = col + strings.IndexByte(toks[endCheck:], '-')
		}
	} else if toks[col] == ' ' {
		c = byte('-')
	}

	// LastIndexByte + 1 ensures that the position of the unwanted character ' ' is excluded
	// +1 also has the added side effect whereby if ' ' isn't found then -1 is snapped to 0
	start := strings.LastIndexByte(toks[:startCheck], c) + 1

	// IndexByte will find the position of the next unwanted character, this is to be the end
	// marker for the selection
	end := -1
	if endCheck != -1 {
		end = strings.IndexByte(toks[endCheck:], c)
	}

	if end == -1 {
		end = len(toks) // snap end to len(toks) if it results in -1
	} else {
		end += endCheck // otherwise include the text slice position
	}
	return start, end
}

// getTextWordRegionNearest returns the selection region for a long-press at
// col: the word under the position or — when the position lands on a run of
// whitespace/separators — the nearest adjacent word, because the native
// mobile long-press never selects the space itself (pre-rebrand tracker #958). The word whose
// edge is closer to the pressed column wins; ties go to the preceding word.
// A row consisting only of separators keeps the run selection.
func getTextWordRegionNearest(row []rune, col int) (int, int) {
	start, end := getTextWhitespaceRegion(row, col, false)
	if start == -1 || end == -1 {
		return start, end
	}

	probe := col
	if probe >= len(row) { // snapped the same way by getTextWhitespaceRegion
		probe = len(row) - 1
	}
	if !isWordSeparator(row[probe]) {
		return start, end // pressed on a word: keep it
	}

	left := start > 0       // a word ends just before the run
	right := end < len(row) // another begins just past it
	switch {
	case !left && !right:
		return start, end // nothing to snap to
	case left && (!right || probe-start <= end-1-probe):
		return getTextWhitespaceRegion(row, start-1, false)
	default:
		return getTextWhitespaceRegion(row, end, false)
	}
}

func isWordSeparator(r rune) bool {
	return unicode.IsSpace(r) ||
		strings.ContainsRune("`~!@#$%^&*()-=+[{]}\\|;:'\",.<>/?", r)
}

// entryUndoAction represents a single user action that can be undone
type entryUndoAction interface {
	// Apply performs the recorded action on the provider — redo replays the
	// original edit, undo (redo=false) reverses it — and returns the caret rune
	// offset after the application. Splicing the provider directly keeps the
	// per-action cost O(edit) instead of O(document) (pre-rebrand tracker #401).
	Apply(p *RichText, redo bool) (caretPos int)
}

// entryMergeableUndoAction is like entryUndoAction, but the undoStack
// can try to merge it with the next action (see TryMerge).
// This is useful because it allows grouping together actions like
// entering every single characters in a word. We don't want to have to
// undo every single character addition.
type entryMergeableUndoAction interface {
	entryUndoAction
	// TryMerge attempts to merge the current action
	// with the next action. It returns true if successful.
	// If it fails, the undoStack will simply add the next
	// item without merging.
	TryMerge(next entryMergeableUndoAction) bool
}

// Declare conformity with entryMergeableUndoAction interface
var _ entryMergeableUndoAction = (*entryModifyAction)(nil)

// entryModifyAction implements entryMergeableUndoAction.
// It represents the insertion/deletion of a single string at a
// position (e.g. "Hello" => "Hello, world", or "Hello" => "He").
type entryModifyAction struct {
	// Delete is true if this action deletes Text, and false if it inserts Text
	Delete bool
	// Position represents the start position of Text
	Position int
	// Text is the text that is inserted or deleted at Position
	Text []rune
}

// Apply splices the recorded edit into the provider. Undoing a deletion (or
// redoing an insertion) re-inserts Text at Position with the caret after it;
// the reverse operations delete the same range with the caret at Position —
// matching the caret placement the string-replay implementation had.
func (i *entryModifyAction) Apply(p *RichText, redo bool) int {
	insert := i.Delete != redo
	if insert {
		p.insertAt(i.Position, i.Text)
		return i.Position + len(i.Text)
	}
	p.deleteFromTo(i.Position, i.Position+len(i.Text))
	return i.Position
}

func (i *entryModifyAction) TryMerge(other entryMergeableUndoAction) bool {
	if other, ok := other.(*entryModifyAction); ok {
		// Don't merge two different types of modifyAction
		if i.Delete != other.Delete {
			return false
		}

		// Don't merge two separate words
		wordSeparators := func(s []rune) (num int, onlyWordSeparators bool) {
			onlyWordSeparators = true
			for _, r := range s {
				if isWordSeparator(r) {
					num++
					onlyWordSeparators = false
				}
			}
			return num, onlyWordSeparators
		}
		selfNumWS, _ := wordSeparators(i.Text)
		otherNumWS, otherOnlyWS := wordSeparators(other.Text)
		if !((selfNumWS == 0 && otherNumWS == 0) ||
			(selfNumWS > 0 && otherOnlyWS)) {
			return false
		}

		if i.Delete {
			if i.Position == other.Position+len(other.Text) {
				i.Position = other.Position
				i.Text = append(other.Text, i.Text...)
				return true
			}
		} else {
			if i.Position+len(i.Text) == other.Position {
				i.Text = append(i.Text, other.Text...)
				return true
			}
		}
		return false
	}
	return false
}

// entryUndoStack stores the information necessary for textual undo/redo functionality.
type entryUndoStack struct {
	// items is the stack for storing the history of user actions.
	items []entryUndoAction
	// index is the size of the current effective undo stack.
	// items[index-1] and below are the possible undo actions.
	// items[index] and above are the possible redo actions.
	index int
}

// Undo pops the most recent action off the stack and returns it (nil when the
// stack is empty). The caller applies it via entryUndoAction.Apply.
func (u *entryUndoStack) Undo() entryUndoAction {
	if !u.CanUndo() {
		return nil
	}
	u.index--
	return u.items[u.index]
}

// Redo advances the stack to the next redoable action and returns it (nil when
// there is none). The caller applies it via entryUndoAction.Apply.
func (u *entryUndoStack) Redo() entryUndoAction {
	if !u.CanRedo() {
		return nil
	}
	action := u.items[u.index]
	u.index++
	return action
}

// Returns true if an undo action is available
func (u *entryUndoStack) CanUndo() bool {
	return u.index != 0
}

// Returns true if an redo action is available
func (u *entryUndoStack) CanRedo() bool {
	return u.index != len(u.items)
}

// Adds the action to the stack, which can later be undone by calling Undo()
func (u *entryUndoStack) Add(a entryUndoAction) {
	u.items = u.items[:u.index]
	u.items = append(u.items, a)
	u.index++
}

// Tries to merge the action with the last item on the undo stack.
// If it can't be merged, it calls Add().
func (u *entryUndoStack) MergeOrAdd(a entryUndoAction) {
	u.items = u.items[:u.index]
	if u.index == 0 {
		u.Add(a)
		return
	}
	ma, ok := a.(entryMergeableUndoAction)
	if !ok {
		u.Add(a)
		return
	}
	mprev, ok := u.items[u.index-1].(entryMergeableUndoAction)
	if !ok {
		u.Add(a)
		return
	}
	if !mprev.TryMerge(ma) {
		u.Add(a)
		return
	}
}

// Removes all items from the undo stack
func (u *entryUndoStack) Clear() {
	u.items = nil
	u.index = 0
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
