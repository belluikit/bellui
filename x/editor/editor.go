package editor

import (
	"image/color"
	"log"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Editor is a composable code editor widget for Bellui applications.
//
// It provides pluggable syntax highlighting via [Highlighter], a customizable
// gutter via [GutterProvider], row-level decorations via [RowDecorator], and
// a built-in find bar.
//
// The gutter is rendered as a separate widget that does not scroll
// horizontally, matching the behavior of production code editors.
//
// The Editor manages its own scrolling, overlays and pointer-capture layer, so
// it must own its viewport: do not wrap it in an outer [container.Scroll]. An
// outer scroll intercepts pointer gestures, so the editor never receives clicks
// or focus (#111). Place the Editor directly in its parent (it fills the space
// given) and let it scroll internally; it logs a warning if it detects an outer
// scrolling ancestor at layout time.
type Editor struct {
	widget.BaseWidget

	// OnChanged is called whenever the text content changes — per user edit by
	// default; batched to sync points under [Editor.DeferTextSync].
	OnChanged func(string)

	// OnRun is called when the user presses Cmd+R / Ctrl+R.
	OnRun func()

	// OnSave is called when the user presses Cmd+S / Ctrl+S.
	OnSave func()

	// OnCursorChanged is called when the cursor position changes.
	OnCursorChanged func(row, col int)

	// Gutter provides the gutter rendering to the left of the text.
	// Set to nil to hide the gutter. Use [NewCompositeGutter] to combine
	// multiple providers (e.g. line numbers + breakpoints).
	Gutter GutterProvider

	// Decorators provide row-level background or overlay rendering.
	// They are applied in order.
	Decorators []RowDecorator

	// SpanDecorators provide character-range (span) decorations: squiggles,
	// sub-line backgrounds, boxes, and opaque redaction overlays. They are
	// applied in order, over the row decorators.
	SpanDecorators []SpanDecorator

	// Highlighter provides syntax highlighting for the editor content.
	// Set to nil for plain text editing.
	Highlighter Highlighter

	// KeyMap configures the "smart editing" interaction shortcuts (completion,
	// signature help, overload cycling). NewEditor populates it with
	// [DefaultKeyMap]; a host may override bindings before the editor is first
	// shown. Find/save/run and navigation shortcuts are not part of the keymap.
	KeyMap KeyMap

	// MinRows sets the minimum visible rows. Default is 20.
	// Set to a smaller value (e.g. 5) for compact editors.
	MinRows int

	// MaxCompletionRows caps the visible rows in the completion popup; extra
	// matches scroll behind the window ([widget.CompletionPopup.MaxVisibleRows]).
	// 0 uses the built-in default (8). The popup renders in the canvas overlay,
	// so it overflows a compact editor into the surrounding window naturally;
	// this is the knob that keeps a very long list bounded.
	MaxCompletionRows int

	// AutoGrow makes the editor grow vertically to fit its content.
	// When true, MinRowsVisible adjusts to match the number of lines.
	// MinAutoGrowRows and MaxAutoGrowRows bound the range.
	AutoGrow        bool
	MinAutoGrowRows int // default 3
	MaxAutoGrowRows int // default 50, 0 = unlimited

	// AutoIndent governs the editor's automatic indentation. On Enter the new
	// line starts with the current line's leading whitespace (or whatever
	// [Editor.IndentProvider] returns); Tab/Shift-Tab indent and dedent the
	// current line or selected block. NewEditor enables it. With it off, Enter
	// inserts a bare newline and Tab a literal tab (the Entry defaults).
	AutoIndent bool
	// IndentProvider is the optional language rule for AutoIndent. nil inherits
	// the current line's indentation (the language-agnostic default).
	IndentProvider IndentProvider
	// TabWidth is the width of one indentation level in spaces. NewEditor
	// defaults it to 4. It defines the indent unit passed to an IndentProvider.
	TabWidth int
	// UseSpaces selects the indent unit: TabWidth spaces when true, a single tab
	// when false. NewEditor enables it (spaces).
	UseSpaces bool

	// DeferTextSync defers the per-keystroke materialization of the whole
	// document into the Entry's Text field (fyne#410), flattening keystroke
	// latency on multi-megabyte documents. With it on, OnChanged fires at sync
	// points — a newline or deletion, a caret row change, undo/redo, focus
	// loss, [Editor.Text] — with the full current text, rather than on every
	// keystroke. Features that need current text per edit (completion or
	// signature providers, protected ranges, masked redactions) force a sync
	// on every edit, so they behave exactly as with the mode off; active
	// folds disable the fork's deferred path entirely. Default off — behavior
	// is byte-identical. Set before the editor is shown, or toggle at runtime
	// via [Editor.SetDeferTextSync]. See defersync.go for the full contract.
	DeferTextSync bool

	// internal widgets
	entry       *editorEntry
	gutterW     *gutterWidget
	decOverlay  *decoratorOverlay
	find        *findBar
	searchSpans *SearchMatchSpanDecorator // span-level find-match highlights
	topBar      *bellui.Container         // find bar wrapper (shown/hidden)
	hbox        *bellui.Container         // gutter + entry layout
	root        *bellui.Container         // topBar + hbox
	floatLayer  *bellui.Container         // floats popovers over the text

	// activePopovers holds the currently shown popovers. At most one is
	// interactive (completion or hover); a non-interactive companion (the
	// signature-help card) may be shown alongside it — see showPopoverAt.
	activePopovers []*popover
	completion     *completionController // completion popup state (nil-safe)
	hover          *hoverController      // hover card state (nil-safe)
	hoverCap       *hoverCapture         // transparent pointer-move capture layer
	hoverOutline   *StaticSpanDecorator  // outlines the hovered range
	signature      *signatureController  // signature-help card state (nil-safe)

	diagnostics          *diagnosticSpanDecorator // internal severity-squiggle overlay
	diagGutter           *DiagnosticGutter        // gutter markers, fed from SetDiagnostics
	onDiagnosticsChanged func([]Diagnostic)       // problems-panel sync hook (nil-safe)

	// lastDiagSummary gates the screen-reader diagnostics announcement to
	// actual changes ("" = never announced / no diagnostics); announce is the
	// live-region seam, nil = bellui.AnnounceToScreenReader (test hook).
	lastDiagSummary string
	announce        func(text string, assertive bool)

	link                *linkController      // Cmd/Ctrl-hover link affordance (nil-safe)
	linkDeco            *StaticSpanDecorator // underlines the navigable symbol
	linkModWatchStop    func()               // removes the driver modifier listener (#155)
	onRequestNavigation func(row, col int)   // Cmd/Ctrl+click "go to definition" hook

	protected *protectedController // read-only ranges enforced via entry.BeforeChange
	redaction *redactionController // opaque masking bars + caret-bar via entry.ProtectedRanges

	fold       *foldController // code folding via entry.HiddenRanges + placeholder overlay
	foldGutter *FoldGutter     // internal fold-chevron gutter, wired to fold state

	semantic *semanticOverlay // whole-document foreground overlay via entry.StyleOverlayForRow (#175)

	chordPending bool        // Ctrl/Cmd+K seen, awaiting the chord's second key
	chordTimer   *time.Timer // resets chordPending if the second key never comes

	shortcutsInstalled bool // guards one-time KeyMap shortcut registration
	scrollGuardChecked bool // guards the one-time outer-scroll warning (#111)

	findVisible        bool
	suppressChange     bool // true during programmatic SetText
	suppressCompletion bool // true while applying a completion edit

	// anchorText is the text snapshot the next OnChanged is diffed against to
	// recover the edit for anchor-tracking (#130); see anchor.go.
	anchorText string

	// deferredSyncPending flags that a gated edit needs a sync point (#129);
	// deferredLastRow/deferredLastLines are the caret row and live line count
	// at the last consumed sync, so a row change or a newline added/removed
	// also syncs (#149). All live in consumeDeferredSync / noteDeferredEdit.
	deferredSyncPending bool
	deferredLastRow     int
	deferredLastLines   int

	scrollX, scrollY float32 // last known Entry scroll offset
}

// NewEditor creates a new code editor with default settings.
func NewEditor() *Editor {
	e := &Editor{
		Gutter:     NewLineNumberGutter(),
		KeyMap:     DefaultKeyMap(),
		AutoIndent: true,
		TabWidth:   defaultTabWidth,
		UseSpaces:  true,
	}
	e.ExtendBaseWidget(e)

	// Editor entry.
	e.entry = newEditorEntry()
	e.entry.TextStyle = bellui.TextStyle{Monospace: true}
	e.entry.SetMinRowsVisible(20)
	e.entry.KeepSelectionVisible = true

	e.entry.OnChanged = func(s string) {
		e.syncExternalWidgets()
		e.updateAutoGrow(s)
		// Anchor-track (#130): shift protected/redaction/fold ranges through the
		// edit so line-count changes no longer need a host re-publish. Runs
		// before the redaction reconcile so the caret-bar offsets are re-derived
		// from the already-shifted spans.
		e.trackAnchors(s)
		// Keep the masked-span caret-bar offsets aligned with the new text.
		if e.redaction != nil {
			e.redaction.reconcile()
		}
		// Drive completion off genuine user edits only — not programmatic
		// SetText (suppressChange) nor our own accept edit (suppressCompletion).
		if e.completion != nil && !e.suppressChange && !e.suppressCompletion {
			e.completion.onTextChanged()
		}
		// Signature help refreshes on the same user edits, after completion so the
		// completion list anchors first and the card places as its companion (#79).
		if e.signature != nil && !e.suppressChange && !e.suppressCompletion {
			e.signature.onTextChanged()
		}
		// A keyboard-invoked hover card is anchored to the cursor, not the
		// pointer; an edit invalidates it.
		if e.hover != nil && e.hover.keyboardShown {
			e.hover.dismiss()
		}
		if e.OnChanged != nil && !e.suppressChange {
			e.OnChanged(s)
		}
	}

	e.entry.OnCursorChanged = func() {
		// Deferred-sync pump (#129): fires after every edit (the fork calls this
		// from every content refresh), so flagged edits and row changes
		// materialize here.
		e.consumeDeferredSync()
		// Dismiss a keyboard-invoked hover card when the cursor moves off its
		// symbol (the pointer-move dismiss does not apply to the keyboard path).
		if e.hover != nil && e.hover.keyboardShown {
			e.hover.dismiss()
		}
		if e.OnCursorChanged != nil {
			e.OnCursorChanged(e.entry.CursorRow, e.entry.CursorColumn)
		}
	}

	// Sync gutter and decorator overlay with the entry's scroll offset.
	e.entry.OnScrolled = func(pos geo.Position) {
		e.scrollX, e.scrollY = pos.X, pos.Y
		if e.gutterW != nil {
			e.gutterW.scrollY = pos.Y
		}
		if e.decOverlay != nil {
			e.decOverlay.scrollY = pos.Y
		}
		e.syncExternalWidgets()
		e.repositionPopover()
	}

	// Float layer hosts anchored popovers (completion, hover, signature help)
	// over the text. Its layout hook re-anchors the active popover on resize.
	e.floatLayer = container.New(&popoverLayer{onLayout: func(geo.Size) { e.repositionPopover() }})

	// Popover key/mouse/focus routing. The Entry keeps focus while a popover is
	// open; these hooks give the active popover first dibs on keys and dismiss
	// it on Escape, outside click, or focus loss.
	e.entry.onKey = func(ev *event.KeyEvent) bool {
		// The completion popup gets first dibs (#143): it renders in the canvas
		// overlay in KeepHostFocus mode, so the Entry stays focused and the
		// editor offers each key via HandleKey — arrows/Enter/Tab/Escape are
		// consumed, PageUp/PageDown only while its documentation panel shows,
		// everything else falls through to the paths below.
		if e.completion != nil && e.completion.handleKey(ev) {
			return true
		}
		// Float-layer popovers next. Offer the key to any popover that handles
		// keys (the hover card); the signature companion sets no handler and is
		// key-transparent.
		if len(e.activePopovers) > 0 {
			for _, p := range e.activePopovers {
				if p.keyHandler != nil && p.keyHandler(ev) {
					return true
				}
			}
			if ev.Name == event.KeyEscape {
				e.dismissPopover()
				return true
			}
		}
		// Auto-indent on Enter, after popovers so an open completion popup accepts
		// with Enter first. Consumes the key only when it inserts an indented
		// newline; otherwise falls through to the Entry's bare newline.
		if isEnterKey(ev) && e.handleEnterAutoIndent() {
			return true
		}
		// Tab / Shift-Tab block indent & dedent, likewise after popovers (an open
		// completion popup accepts with Tab first). Consumes Tab when AutoIndent
		// is on, replacing the Entry's literal-tab insert.
		if ev.Name == event.KeyTab && e.handleTabIndent(e.shiftHeld()) {
			return true
		}
		// Modifier-less KeyMap bindings run here: the platform driver only
		// synthesizes CustomShortcuts for modified chords, so a bare key (e.g.
		// the default F12 go-to-definition) never reaches TypedShortcut. Modified
		// chords go through installKeyMapShortcuts instead. Ordered after
		// popovers so an open completion popup keeps first claim on the key.
		if gd := e.KeyMap.GoToDefinition; gd.Modifier == 0 && gd.KeyName != "" && ev.Name == gd.KeyName {
			e.GoToDefinitionAtCursor()
			return true
		}
		return false
	}
	e.entry.onMouseDown = func(m *desktop.MouseEvent) {
		if len(e.activePopovers) > 0 && !e.popoverContains(m.Position) {
			e.dismissPopover()
		}
		// Cmd/Ctrl+click fires the go-to-definition hook.
		e.link.onMouseDown(m)
	}
	e.entry.onFocusLost = func() { e.dismissPopover() }

	// Track the platform shortcut modifier (Cmd/Ctrl) so a Cmd-hover lights the
	// link underline without needing a mouse move.
	e.entry.onKeyDown = func(k *event.KeyEvent) { e.link.onKeyEvent(k.Name, true) }
	e.entry.onKeyUp = func(k *event.KeyEvent) { e.link.onKeyEvent(k.Name, false) }

	// Electric-character reindent (#136): after the Entry inserts a typed rune,
	// an IndentProvider that also implements ReindentProvider may re-indent the
	// line (e.g. dedent an "else:" clause). Inert without such a provider.
	e.entry.onRune = e.handleElectricRune

	// Chord handler: Ctrl/Cmd+K then Ctrl/Cmd+I shows hover at the cursor (#29),
	// matching VS Code's ⌘K ⌘I. Consumes the leading K; any other shortcut clears
	// a pending chord and falls through.
	e.entry.onShortcut = e.handleEditorShortcut

	// Protected-range controller (inert until SetProtectedRanges adds ranges).
	// BeforeChange is the single gate the fork funnels every interactive edit
	// path through; allow() returns true immediately when no ranges are set.
	// Masked redactions veto through the same gate (after protection, so a
	// protected-range hit reports its callback first) to keep edits off a masked
	// span's edges — the CaretOpaque field alone permits boundary inserts (#108).
	e.protected = newProtectedController(e)
	e.entry.BeforeChange = func(sel widget.TextRange, _ string) bool {
		if !e.protected.allow(sel) {
			return false
		}
		if e.redaction != nil && !e.redaction.allow(sel) {
			return false
		}
		// The edit will apply: flag a per-edit deferred sync when an active
		// feature needs it (#129). No-op with DeferTextSync off.
		e.noteDeferredEdit()
		return true
	}

	// Redaction controller (inert until SetRedactions adds spans). It owns the
	// Entry's CaretOpaque ProtectedRanges and draws the opaque masking bars.
	e.redaction = newRedactionController(e)

	// Fold controller (inert until SetFoldRegions/Fold/AutoFold add regions). It
	// drives the Entry's HiddenRanges from the collapsed regions and feeds the
	// fold gutter and placeholder overlay. The internal fold gutter is bridged to
	// its foldable/collapsed queries and toggle so a host can compose it directly.
	e.fold = newFoldController(e)
	e.foldGutter = newFoldGutter(e.fold.foldableAtHeader, e.fold.collapsedAtHeader, e.fold.toggle)

	// Whole-document foreground overlay for LSP semantic tokens (#175), inert
	// until SetSemanticTokens publishes any. It composites over the line
	// Highlighter via the fork's pull-based StyleOverlayForRow, so wiring it
	// unconditionally costs nothing until tokens exist.
	e.semantic = newSemanticOverlay()
	e.entry.StyleOverlayForRow = e.semantic.runsForRow

	// Completion controller (inert until SetCompletionProvider installs one).
	e.completion = newCompletionController(e)

	// Link controller (Cmd/Ctrl-hover go-to-definition affordance) + its underline
	// decoration (inert until SetLinkProvider / OnRequestNavigation are set).
	e.link = newLinkController(e)
	e.linkDeco = &StaticSpanDecorator{}

	// Hover controller + its transparent pointer-capture layer and range
	// outline (all inert until SetHoverProvider installs a provider). The same
	// capture layer drives the link affordance and owns the cursor — a pointer
	// when a link is lit, the text I-beam otherwise.
	e.hover = newHoverController(e)
	e.hoverOutline = &StaticSpanDecorator{}
	e.hoverCap = newHoverCapture(
		func(pos geo.Position) { e.hover.onMouseMoved(pos); e.link.onMouseMoved(pos) },
		func() { e.hover.onMouseOut(); e.link.onMouseOut() },
		func() desktop.Cursor {
			if e.link.active() {
				return desktop.PointerCursor
			}
			return desktop.TextCursor
		},
	)

	// Signature-help controller (inert until SetSignatureHelpProvider installs
	// a provider).
	e.signature = newSignatureController(e)

	// Diagnostics squiggle overlay + gutter markers (inert until SetDiagnostics
	// adds entries). The gutter is fed automatically from SetDiagnostics; the
	// host composes it into its CompositeGutter via Editor.DiagnosticGutter.
	e.diagnostics = &diagnosticSpanDecorator{}
	e.diagGutter = NewDiagnosticGutter()

	// Span decorator that highlights find matches; fed by the find bar.
	e.searchSpans = &SearchMatchSpanDecorator{Active: -1}

	// Find bar. The text source synchronises deferred edits (#129) so a search
	// always runs against the live document.
	e.find = newFindBar(
		func() string { return e.docText() },
		func(sr, sc, er, ec int) {
			e.fold.revealRows(sr, er) // unfold so a match inside a fold is exposed
			e.entry.Select(sr, sc, er, ec)
		},
		func() { e.HideFind() },
		func(matches []TextRange, active int) {
			e.searchSpans.Matches = matches
			e.searchSpans.Active = active
			e.RefreshDecorators()
		},
	)
	e.find.entry.addFindShortcuts(
		func() { e.ShowFind() },
		func() { e.FindNext() },
		func() { e.FindPrev() },
	)

	// Cmd+F / Cmd+G / Shift+Cmd+G on the main entry.
	e.entry.addFindShortcuts(
		func() { e.ShowFind() },
		func() { e.FindNext() },
		func() { e.FindPrev() },
	)

	// The completion / signature-help / overload-cycle shortcuts are configurable
	// via e.KeyMap and installed lazily in CreateRenderer, so a host can rebind
	// them after construction. See installKeyMapShortcuts.

	// Cmd+R on the main entry — run cell.
	e.entry.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyR,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) {
		if e.OnRun != nil {
			e.OnRun()
		}
	})

	// Cmd+S on the main entry — save notebook.
	e.entry.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyS,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) {
		if e.OnSave != nil {
			e.OnSave()
		}
	})

	// Build layout. The gutter is added in applyGutter when Gutter is set.
	e.topBar = container.NewStack() // initially empty
	e.hbox = container.NewBorder(nil, nil, nil, nil, e.entry)
	e.root = container.NewBorder(nil, nil, nil, nil, e.hbox)

	return e
}

// CreateRenderer implements core.Widget.
func (e *Editor) CreateRenderer() core.WidgetRenderer {
	// Apply MinRows if set (default 20)
	if e.MinRows > 0 {
		e.entry.SetMinRowsVisible(e.MinRows)
	}
	e.installKeyMapShortcuts()
	e.entry.DeferTextSync = e.DeferTextSync
	e.applyHighlighter()
	e.applyGutter()
	e.applyDecorators()
	// Force the entry to rebuild its highlight cache now that
	// LineHighlighter is set. Must run before syncExternalWidgets
	// so that RowPositions() returns populated data.
	e.entry.Refresh()
	e.syncExternalWidgets()
	// Subscribe the link affordance to driver modifier changes (#155) for the
	// editor's on-canvas lifetime; Destroy removes it and a re-show re-arms.
	if e.linkModWatchStop == nil && e.link != nil {
		e.linkModWatchStop = e.link.watchModifiers()
	}
	return &editorRenderer{WidgetRenderer: widget.NewSimpleRenderer(e.root), e: e}
}

// editorRenderer wraps the simple renderer to release canvas-lifetime
// resources: the driver modifier-change listener (#155) is removed on Destroy
// and re-registered by the next CreateRenderer on re-show.
type editorRenderer struct {
	core.WidgetRenderer
	e *Editor
}

func (r *editorRenderer) Destroy() {
	if r.e.linkModWatchStop != nil {
		r.e.linkModWatchStop()
		r.e.linkModWatchStop = nil
	}
	r.WidgetRenderer.Destroy()
}

// installKeyMapShortcuts registers the configurable interaction shortcuts
// (completion, signature help, overload cycling) from e.KeyMap onto the entry.
// It runs once, from CreateRenderer, so a host can customize e.KeyMap after
// construction but before the editor is shown. A binding left at its zero value
// (empty key) is skipped, leaving that action unbound. Find/save/run shortcuts
// are registered eagerly in NewEditor and are not part of the keymap.
func (e *Editor) installKeyMapShortcuts() {
	if e.shortcutsInstalled {
		return
	}
	e.shortcutsInstalled = true

	add := func(sc desktop.CustomShortcut, fn func()) {
		if sc.KeyName == "" {
			return // unbound: skip
		}
		s := sc // stable address for the shortcut registry
		e.entry.Entry.AddShortcut(&s, func(_ core.Shortcut) { fn() })
	}
	add(e.KeyMap.Completion, func() { e.completion.invoke() })
	add(e.KeyMap.SignatureHelp, func() { e.ShowSignatureHelpAtCursor() })
	add(e.KeyMap.CycleOverloadNext, func() { e.signature.cycle(1) })
	add(e.KeyMap.CycleOverloadPrev, func() { e.signature.cycle(-1) })
	// GoToDefinition only reaches the shortcut registry when rebound to a
	// modified chord; the modifier-less default (F12) is handled in the onKey
	// dispatcher instead (add skips it there as a no-firing registration).
	add(e.KeyMap.GoToDefinition, func() { e.GoToDefinitionAtCursor() })
}

// Resize lays the editor out and, once, checks whether a host has nested it in
// an outer clipping/scrolling ancestor such as a container.Scroll. The Editor
// manages its own scrolling, overlays and pointer capture and expects to own its
// viewport; an outer scroll intercepts pointer gestures so clicks never reach the
// editor and it never focuses (#111). The check only logs a one-time warning and
// never alters layout.
func (e *Editor) Resize(size geo.Size) {
	e.BaseWidget.Resize(size)
	e.warnIfNestedInScroll()
}

// warnIfNestedInScroll logs a single warning when the editor is hosted inside an
// outer clipping/scrolling container (see [Editor.Resize] and #111). It is a
// no-op until the editor is attached to a canvas — retried on later resizes —
// and performs its check at most once.
func (e *Editor) warnIfNestedInScroll() {
	if e.scrollGuardChecked {
		return
	}
	app := bellui.CurrentApp()
	if app == nil {
		return
	}
	d := app.Driver()
	if d == nil {
		return
	}
	c := d.CanvasForObject(e.entry)
	if c == nil {
		return // not on a canvas yet; a later Resize will retry
	}
	e.scrollGuardChecked = true
	// Within the editor's own composition nothing clips the entry, so a clipping
	// ancestor here means a host wrapped the editor in an outer scroll/clip.
	if _, clipped := driver.ClipBoundsForObject(c, e.entry); clipped {
		log.Println("fycoder: Editor is hosted inside an outer clipping/scrolling " +
			"container (e.g. container.Scroll); it manages its own scrolling, overlays " +
			"and pointer capture, so an outer scroll intercepts clicks and the editor " +
			"never focuses. Give the Editor its own viewport (no outer container.Scroll). " +
			"See github.com/belluikit/bellui/x/editor/issues/111.")
	}
}

// syncExternalWidgets pushes the Entry's row metrics to the gutter widget and
// decorator overlay and repaints them. Line counts are no longer pushed: both
// widgets read the live O(1) [widget.Entry.LineCount] via their getLineCount
// hook (#149), so nothing here forces a deferred-sync materialization (#129)
// and this stays cheap from OnScrolled.
func (e *Editor) syncExternalWidgets() {
	topOff, stride := e.entry.RowMetrics()

	if e.gutterW != nil {
		e.gutterW.topOffset = topOff
		e.gutterW.stride = stride
		e.gutterW.Refresh()
	}
	if e.decOverlay != nil {
		e.decOverlay.topOffset = topOff
		e.decOverlay.stride = stride
		e.decOverlay.Refresh()
	}
}

// Text returns the current editor content. Under [Editor.DeferTextSync] it is
// a sync point: pending edits materialize and OnChanged observers run first.
func (e *Editor) Text() string {
	return e.docText()
}

// SetText replaces the editor content as a document SWAP: it does not fire
// OnChanged (only user edits trigger the callback), clears the undo history,
// and leaves anchored ranges (#130) for the host to re-publish. For a
// programmatic REWRITE with user-edit semantics — quick fixes, refactors,
// formatting — use [Editor.ApplyEdits]/[Editor.ApplyEdit] instead (#160).
func (e *Editor) SetText(s string) {
	e.suppressChange = true
	e.entry.SetText(s)
	e.suppressChange = false
	e.updateAutoGrow(s)
	e.syncExternalWidgets()
}

// updateAutoGrow adjusts MinRowsVisible to fit the content when AutoGrow is enabled.
func (e *Editor) updateAutoGrow(s string) {
	if !e.AutoGrow {
		return
	}
	lines := strings.Count(s, "\n") + 1

	minRows := e.MinAutoGrowRows
	if minRows <= 0 {
		minRows = 3
	}
	maxRows := e.MaxAutoGrowRows
	if maxRows <= 0 {
		maxRows = 50
	}

	rows := lines
	if rows < minRows {
		rows = minRows
	}
	if rows > maxRows {
		rows = maxRows
	}
	e.entry.SetMinRowsVisible(rows)
	// Refresh the editor so gutter and decorators resize to match.
	e.Refresh()
}

// CursorPos returns the current cursor position (0-based row, column).
func (e *Editor) CursorPos() (row, col int) {
	return e.entry.CursorRow, e.entry.CursorColumn
}

// SetCursorPos moves the cursor to the given position (0-based).
func (e *Editor) SetCursorPos(row, col int) {
	e.entry.CursorRow = row
	e.entry.CursorColumn = col
}

// SelectedText returns the currently selected text, or empty string.
func (e *Editor) SelectedText() string {
	return e.entry.SelectedText()
}

// ShowFind displays the find bar and focuses the search entry.
// If text is selected, it pre-fills the search field.
func (e *Editor) ShowFind() {
	if !e.findVisible {
		e.findVisible = true
		e.root.Objects = []core.CanvasObject{e.hbox}
		e.root.Add(e.find.Content())
		e.root.Layout = layout.NewBorderLayout(e.find.Content(), nil, nil, nil)
		e.root.Refresh()
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(e.entry)
	e.find.Focus(c, e.entry.SelectedText())
}

// HideFind hides the find bar and returns focus to the editor.
func (e *Editor) HideFind() {
	if !e.findVisible {
		return
	}
	e.findVisible = false
	e.root.Objects = []core.CanvasObject{e.hbox}
	e.root.Layout = layout.NewBorderLayout(nil, nil, nil, nil)
	e.root.Refresh()
	e.find.ClearLabel()

	// Clear span-level match highlights.
	e.searchSpans.Matches = nil
	e.searchSpans.Active = -1
	e.RefreshDecorators()

	if c := bellui.CurrentApp().Driver().CanvasForObject(e.entry); c != nil {
		c.Focus(e.entry)
	}
}

// FindVisible returns whether the find bar is currently shown.
func (e *Editor) FindVisible() bool {
	return e.findVisible
}

// FindNext moves to the next search match.
func (e *Editor) FindNext() {
	e.find.FindNext()
}

// FindPrev moves to the previous search match.
func (e *Editor) FindPrev() {
	e.find.FindPrev()
}

// Focus gives keyboard focus to the editor's text area.
// Keyboard shortcuts (Cmd+F, Cmd+S, etc.) require focus to fire.
func (e *Editor) Focus() {
	if c := bellui.CurrentApp().Driver().CanvasForObject(e.entry); c != nil {
		c.Focus(e.entry)
	}
}

// SetReadOnly sets the editor to read-only mode.
// Uses Entry.ReadOnly instead of Disable() so text colors and background
// remain normal — syntax highlighting stays visible.
func (e *Editor) SetReadOnly(ro bool) {
	e.entry.ReadOnly = ro
}

// applyHighlighter wires the Highlighter to the Entry's LineHighlighter.
func (e *Editor) applyHighlighter() {
	if e.Highlighter == nil {
		e.entry.LineHighlighter = nil
		return
	}
	h := e.Highlighter
	e.entry.LineHighlighter = func(line string, state int) ([]widget.StyleRun, int) {
		runs, newState := h.Highlight(line, state)
		// Convert editor.StyleRun to widget.StyleRun.
		// Resolve ColorName to a direct ForegroundColor via themeColor()
		// because Bellui's theme doesn't know fycoder's custom color names.
		wRuns := make([]widget.StyleRun, len(runs))
		for i, r := range runs {
			var fg color.Color
			if r.ForegroundColor != nil {
				fg = r.ForegroundColor
			} else if r.ColorName != "" {
				fg = themeColor(r.ColorName)
			}
			wRuns[i] = widget.StyleRun{
				Text:            r.Text,
				ForegroundColor: fg,
				Italic:          r.Italic,
				Bold:            r.Bold,
				BackgroundColor: r.BackgroundColor,
				Underline:       r.Underline,
				UnderlineStyle:  r.UnderlineStyle.toWidget(),
				UnderlineColor:  r.UnderlineColor,
			}
		}
		return wRuns, newState
	}
}

// applyGutter configures the gutter widget from the Gutter field.
// When a gutter is set, the Entry's built-in gutter is suppressed.
func (e *Editor) applyGutter() {
	if e.Gutter == nil {
		e.gutterW = nil
		e.entry.SuppressGutter = false
		e.rebuildHbox()
		return
	}

	e.entry.SuppressGutter = true
	e.gutterW = newGutterWidget(e.Gutter)
	e.gutterW.getLineCount = e.lineCount
	e.gutterW.getRowPositions = func() ([]float32, []float32) {
		return e.entry.RowPositions()
	}
	// Viewport-windowed geometry keeps the gutter O(viewport) on large documents
	// (returns nil to defer to the exact path below the size threshold).
	e.gutterW.getRowWindow = e.rowWindow
	e.gutterW.getRowAtContentY = e.rowAtContentY
	e.rebuildHbox()
}

// applyDecorators configures the decorator overlay from the Decorators field.
// The internal search-match decorator is always appended so the find bar can
// highlight matches even on an otherwise bare editor.
func (e *Editor) applyDecorators() {
	spanDecs := append(append([]SpanDecorator{}, e.SpanDecorators...), e.searchSpans)
	if e.diagnostics != nil {
		spanDecs = append(spanDecs, e.diagnostics)
	}
	if e.hoverOutline != nil {
		spanDecs = append(spanDecs, e.hoverOutline)
	}
	if e.linkDeco != nil {
		spanDecs = append(spanDecs, e.linkDeco)
	}
	// Redaction bars paint last so the opaque mask covers the text and every
	// other span decoration within its range.
	if e.redaction != nil {
		spanDecs = append(spanDecs, e.redaction)
	}

	e.decOverlay = newDecoratorOverlay(e.Decorators, spanDecs)
	e.decOverlay.getLineCount = e.lineCount
	e.decOverlay.getRowPositions = func() ([]float32, []float32) {
		return e.entry.RowPositions()
	}
	e.decOverlay.getRangeRects = e.RangeRects
	e.decOverlay.getRowWindow = e.rowWindow
	// Fold placeholders: the collapsed-region ellipsis/summary drawn inline after
	// each folded header line, positioned via the same caret geometry as popovers.
	e.decOverlay.foldPlaceholders = e.fold.placeholders
	e.decOverlay.getCaretRect = e.PositionAt
	e.rebuildHbox()
}

// rebuildHbox reconstructs the gutter + editor + overlay layout.
func (e *Editor) rebuildHbox() {
	// The center content stacks, bottom to top: the entry, the decorator
	// overlay (which has no input interfaces, so mouse events pass through to
	// the entry below), and the float layer hosting popovers.
	objs := []core.CanvasObject{e.entry}
	if e.decOverlay != nil {
		objs = append(objs, e.decOverlay)
	}
	// The hover-capture layer sits above the text (to receive pointer moves) but
	// below the float layer (so popovers still get their own hover events). It
	// implements only Hoverable, so taps/drags pass through to the Entry. It is
	// inset by the scroll bar strips so the bar underneath still receives the
	// hover that expands it (fyne hover goes to the topmost hoverable, #151).
	if e.hoverCap != nil {
		objs = append(objs, container.New(hoverCapLayout{}, e.hoverCap))
	}
	if e.floatLayer != nil {
		objs = append(objs, e.floatLayer)
	}
	center := container.NewStack(objs...)

	if e.gutterW != nil {
		// Clip the gutter content (Bellui doesn't clip widget bounds),
		// then pad to match the Entry's inputBorder viewport offset.
		// Clipping inside padding ensures line numbers at negative Y
		// or beyond the viewport height are hidden.
		clipped := container.NewScroll(e.gutterW)
		clipped.Direction = container.ScrollNone
		pad := theme.InputBorderSize()
		paddedGutter := container.New(
			layout.NewCustomPaddedLayout(pad, pad, 0, 0),
			clipped,
		)
		e.hbox.Objects = []core.CanvasObject{paddedGutter, center}
		e.hbox.Layout = layout.NewBorderLayout(nil, nil, paddedGutter, nil)
	} else {
		e.hbox.Objects = []core.CanvasObject{center}
		e.hbox.Layout = layout.NewBorderLayout(nil, nil, nil, nil)
	}
}

// RefreshGutter triggers a repaint of the gutter.
func (e *Editor) RefreshGutter() {
	if e.gutterW != nil {
		e.gutterW.Refresh()
	}
}

// RefreshDecorators triggers a repaint of all row and span decorators. Call
// it after mutating a decorator's state (e.g. a StaticSpanDecorator's spans).
// Adding the first decorator to a previously bare editor requires a full
// refresh instead, since the overlay is created lazily.
func (e *Editor) RefreshDecorators() {
	if e.decOverlay != nil {
		e.decOverlay.Refresh()
	}
}

// showPopoverAt displays p anchored to the character cell at (row, col). The
// popover flips above/below and clamps left/right to stay within the editor
// viewport, and tracks the anchor as the editor scrolls or resizes.
//
// At most one interactive popover (completion/hover) is shown at a time:
// showing a new interactive popover dismisses any other interactive one. A
// non-interactive companion (the signature-help card) coexists with the
// interactive popover so call hints and the completion list can show together
// (#79); when the two cannot be placed without overlapping, the interactive one
// yields (see layoutPopovers).
//
// It reports false (showing nothing) when the anchor cell is not currently
// visible — callers should treat that as "cannot anchor here".
func (e *Editor) showPopoverAt(p *popover, row, col int) bool {
	anchor := e.PositionAt(row, col)
	if anchor.IsZero() {
		return false
	}
	p.anchorRow, p.anchorCol = row, col

	// Rebuild the active set: drop p if already present (re-show without firing
	// its onDismiss) and any other popover of the same interactivity class, keep
	// the complementary one, then append p on top.
	kept := e.activePopovers[:0]
	for _, q := range e.activePopovers {
		if q == p {
			continue
		}
		if q.interactive == p.interactive {
			if q.onDismiss != nil {
				q.onDismiss()
			}
			continue
		}
		kept = append(kept, q)
	}
	e.activePopovers = append(kept, p)

	// One interactive surface at a time across both hosting models (#143): an
	// interactive float-layer popover (the hover card) also closes an open
	// overlay completion popup, as showing completion closes the hover card.
	if p.interactive && e.completion != nil {
		e.completion.dismiss()
	}

	// The object set changed (p added, same-class dropped), so place and sync.
	e.layoutPopovers()
	e.refreshFloatLayer()
	return true
}

// dismissInteractivePopovers dismisses any interactive float-layer popover
// (the hover card), leaving companions in place — the completion popup calls
// it when showing, mirroring showPopoverAt's one-interactive-surface rule.
func (e *Editor) dismissInteractivePopovers() {
	for _, p := range append([]*popover(nil), e.activePopovers...) {
		if p.interactive {
			e.dismissPopoverObj(p)
		}
	}
}

// dismissPopover hides all active popovers — the overlay completion popup
// included — and fires each float-layer popover's onDismiss. It backs the
// whole-surface dismissals: Escape fallback, outside click, and focus loss.
func (e *Editor) dismissPopover() {
	if e.completion != nil {
		e.completion.dismiss()
	}
	ps := e.activePopovers
	e.activePopovers = nil
	e.refreshFloatLayer()
	for _, p := range ps {
		if p.onDismiss != nil {
			p.onDismiss()
		}
	}
}

// dismissPopoverObj hides a single popover (if shown), leaving any companion in
// place, and fires its onDismiss. It is how a controller dismisses its own
// surface without disturbing the other — e.g. completion closing while the
// signature card stays up at a call site.
func (e *Editor) dismissPopoverObj(p *popover) {
	if e.removePopover(p) {
		e.refreshFloatLayer()
	}
}

// removePopover drops p from the active set and fires its onDismiss, reporting
// whether it was present. It does NOT touch the float layer — callers refresh
// once after a batch of changes, which keeps the float layer's Layout hook (it
// calls repositionPopover) from re-entering a Refresh cycle.
func (e *Editor) removePopover(p *popover) bool {
	idx := -1
	for i, q := range e.activePopovers {
		if q == p {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	e.activePopovers = append(e.activePopovers[:idx], e.activePopovers[idx+1:]...)
	if p.onDismiss != nil {
		p.onDismiss()
	}
	return true
}

// repositionPopover re-anchors the active popovers after a scroll or resize,
// dismissing any whose anchor cell has scrolled out of the viewport. The
// overlay completion popup follows the same contract via its own Reposition
// (#143). It is also the float layer's Layout hook, so it must refresh the
// layer only when the object set actually changes — otherwise Refresh →
// Layout → reposition loops.
func (e *Editor) repositionPopover() {
	if e.completion != nil {
		e.completion.reposition()
	}
	if len(e.activePopovers) == 0 {
		return
	}
	changed := false
	for _, p := range append([]*popover(nil), e.activePopovers...) {
		if e.PositionAt(p.anchorRow, p.anchorCol).IsZero() {
			changed = e.removePopover(p) || changed
		}
	}
	changed = e.layoutPopovers() || changed
	if changed {
		e.refreshFloatLayer()
	}
}

// layoutPopovers places every active popover at its anchor and, when the
// interactive popover and the signature companion would overlap (too little
// vertical room for both), drops the companion so the interactive popover wins —
// completion stays the primary surface, matching the long-standing
// signature-yields-to-completion rule. It returns whether it changed the active
// set, and never refreshes the float layer itself (see repositionPopover).
func (e *Editor) layoutPopovers() (changed bool) {
	bounds := e.popoverBounds()
	for _, p := range e.activePopovers {
		anchor := e.PositionAt(p.anchorRow, p.anchorCol)
		p.resizeAndPlace(anchor, bounds)
	}
	if inter, comp := e.interactivePopover(), e.companionPopover(); inter != nil && comp != nil && inter.overlaps(comp) {
		return e.removePopover(comp)
	}
	// The overlay completion popup is the interactive surface when it is open
	// (#143): a companion that cannot be placed clear of it yields the same
	// way, via the popup's OnMoved-reported rectangle in editor-local space.
	if comp := e.companionPopover(); comp != nil && e.completion != nil &&
		e.completion.visible() && comp.overlapsRect(e.completion.rect) {
		return e.removePopover(comp)
	}
	return false
}

// refreshFloatLayer syncs the float layer's objects with the active popovers.
func (e *Editor) refreshFloatLayer() {
	if e.floatLayer == nil {
		return
	}
	objs := make([]core.CanvasObject, 0, len(e.activePopovers))
	for _, p := range e.activePopovers {
		objs = append(objs, p.obj)
	}
	e.floatLayer.Objects = objs
	e.floatLayer.Refresh()
}

// popoverShown reports whether p is currently in the active set.
func (e *Editor) popoverShown(p *popover) bool {
	for _, q := range e.activePopovers {
		if q == p {
			return true
		}
	}
	return false
}

// interactivePopover returns the active interactive popover (completion/hover),
// or nil when none is shown.
func (e *Editor) interactivePopover() *popover {
	for _, p := range e.activePopovers {
		if p.interactive {
			return p
		}
	}
	return nil
}

// companionPopover returns the active non-interactive companion (the signature
// card), or nil when none is shown.
func (e *Editor) companionPopover() *popover {
	for _, p := range e.activePopovers {
		if !p.interactive {
			return p
		}
	}
	return nil
}

// popoverContains reports whether the viewport-space point falls within any
// active popover — used to tell a click on a popover from an outside click.
func (e *Editor) popoverContains(pos geo.Position) bool {
	for _, p := range e.activePopovers {
		if p.contains(pos) {
			return true
		}
	}
	return false
}

// popoverBounds returns the usable region for placing a popover, in the editor's
// viewport-local coordinate space (the same space [Editor.PositionAt] and the
// float layer use). It is the host canvas's content area translated by the
// editor's offset within the canvas, so a popover may overflow a compact or
// embedded editor into the surrounding window rather than being clamped to the
// editor's own bounds (see #19). Nothing in the editor's own widget tree clips,
// so an in-tree popover positioned beyond the editor still renders and stays
// interactive; a host that wraps the editor in a container.Scroll will clip the
// popover to that scroll's viewport (still far larger than a compact editor).
//
// It falls back to the editor viewport when no canvas/driver is available (tests
// and before the first show).
func (e *Editor) popoverBounds() geo.Rectangle {
	fallback := geo.Rectangle{Size: e.entry.Size()}
	app := bellui.CurrentApp()
	if app == nil {
		return fallback
	}
	d := app.Driver()
	if d == nil {
		return fallback
	}
	c := d.CanvasForObject(e.entry)
	if c == nil {
		return fallback
	}
	cs := c.Size()
	if cs.Width <= 0 || cs.Height <= 0 {
		return fallback
	}
	// Canvas origin expressed in editor-local coordinates is -off; the canvas
	// extends cs from there.
	off := d.AbsolutePositionForObject(e.entry)
	bounds := geo.NewRectangle(-off.X, -off.Y, cs.Width, cs.Height)

	// If an ancestor clips the editor — e.g. a host that wraps it in a
	// container.Scroll — prefer that clip region so the popover isn't positioned
	// into an area the scroll then clips (#73). Clip bounds are absolute canvas
	// coordinates; translate into editor-local space (− off) and intersect.
	if clip, ok := driver.ClipBoundsForObject(c, e.entry); ok {
		clipLocal := geo.NewRectangle(clip.Pos.X-off.X, clip.Pos.Y-off.Y, clip.Size.Width, clip.Size.Height)
		bounds = intersectRect(bounds, clipLocal)
	}
	return bounds
}

// intersectRect returns the overlap of a and b. With no overlap the result has
// zero/negative extent, which popoverPosition clamps to the min corner.
func intersectRect(a, b geo.Rectangle) geo.Rectangle {
	x0 := max(a.Pos.X, b.Pos.X)
	y0 := max(a.Pos.Y, b.Pos.Y)
	x1 := min(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y1 := min(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	return geo.NewRectangle(x0, y0, x1-x0, y1-y0)
}

// popoverVisible reports whether any anchored popover is currently shown. It is
// primarily a hook for tests and hosts coordinating their own overlays.
func (e *Editor) popoverVisible() bool {
	return len(e.activePopovers) > 0
}

// RefreshHighlighter re-wires the highlighter (call after changing
// the Highlighter field).
func (e *Editor) RefreshHighlighter() {
	e.applyHighlighter()
	e.entry.Refresh()
}

// chordTimeout bounds how long a pending chord (Ctrl/Cmd+K) waits for its second
// key before resetting.
const chordTimeout = 1 * time.Second

// handleEditorShortcut is the editorEntry's onShortcut hook: it offers the
// shortcut to the editor's interactive surfaces before the Entry's own handling.
// It returns true to consume the shortcut.
func (e *Editor) handleEditorShortcut(s core.Shortcut) bool {
	// Undo/redo replay history without passing the BeforeChange gate, so the
	// deferred-sync classifier never sees them (#129): flag a sync here (the
	// shortcut falls through to the Entry, and the flag is consumed after the
	// replayed edit's refresh). Harmless no-op when nothing ends up pending.
	if e.DeferTextSync {
		switch s.(type) {
		case *bellui.ShortcutUndo, *bellui.ShortcutRedo:
			e.deferredSyncPending = true
		}
	}
	if e.handleDocScrollShortcut(s) {
		return true
	}
	return e.handleChordShortcut(s)
}

// handleDocScrollShortcut scrolls the completion documentation panel on
// Ctrl/Cmd+Down / Ctrl/Cmd+Up (#89). It consumes the shortcut only when the
// panel is open and actually scrolls, so the keys fall through to normal
// document navigation whenever completion is closed or the panel can't scroll.
func (e *Editor) handleDocScrollShortcut(s core.Shortcut) bool {
	cs, ok := s.(*desktop.CustomShortcut)
	if !ok || cs.Modifier != event.KeyModifierShortcutDefault || e.completion == nil {
		return false
	}
	switch cs.KeyName {
	case event.KeyDown:
		return e.completion.scrollDoc(1)
	case event.KeyUp:
		return e.completion.scrollDoc(-1)
	}
	return false
}

// handleChordShortcut implements the Ctrl/Cmd+K Ctrl/Cmd+I "show hover" chord
// (#29). It returns true to consume the shortcut. The leading K is always
// consumed (and starts the pending window); a following I while pending fires
// ShowHoverAtCursor; any other shortcut clears the pending chord and falls
// through to normal handling.
func (e *Editor) handleChordShortcut(s core.Shortcut) bool {
	cs, ok := s.(*desktop.CustomShortcut)
	if !ok || cs.Modifier != event.KeyModifierShortcutDefault {
		e.cancelChord()
		return false
	}
	switch cs.KeyName {
	case event.KeyK:
		e.beginChord()
		return true
	case event.KeyI:
		if e.chordPending {
			e.cancelChord()
			e.ShowHoverAtCursor()
			return true
		}
	}
	e.cancelChord()
	return false
}

func (e *Editor) beginChord() {
	e.chordPending = true
	if e.chordTimer != nil {
		e.chordTimer.Stop()
	}
	e.chordTimer = time.AfterFunc(chordTimeout, func() {
		bellui.Do(func() { e.chordPending = false })
	})
}

func (e *Editor) cancelChord() {
	e.chordPending = false
	if e.chordTimer != nil {
		e.chordTimer.Stop()
		e.chordTimer = nil
	}
}

// editorEntry extends widget.Entry with Escape and find shortcut support, plus
// hooks the editor uses to route key, mouse, and focus events to an active
// popover without the popover stealing focus.
type editorEntry struct {
	widget.Entry
	onEscape func()

	// onKey is consulted before the default key handling. It returns true if the
	// event was consumed (e.g. by an open popover).
	onKey func(*event.KeyEvent) bool
	// onMouseDown reports a mouse-down (with its modifier/button) before the Entry
	// handles it.
	onMouseDown func(*desktop.MouseEvent)
	// onKeyDown / onKeyUp observe raw key press/release (including modifier keys)
	// before the Entry handles them — used to track the Cmd/Ctrl link modifier.
	onKeyDown func(*event.KeyEvent)
	onKeyUp   func(*event.KeyEvent)
	// onShortcut is consulted before the Entry's own shortcut handling; it returns
	// true to consume the shortcut (used for the keyboard "show hover" chord).
	onShortcut func(core.Shortcut) bool
	// onRune observes a typed rune after the Entry has inserted it — the seam
	// for electric-character reindent (#136), which must see the post-insert line.
	onRune func(rune)
	// onFocusLost fires when the Entry loses keyboard focus.
	onFocusLost func()
}

func newEditorEntry() *editorEntry {
	e := &editorEntry{}
	e.MultiLine = true
	e.ExtendBaseWidget(e)
	return e
}

func (e *editorEntry) TypedKey(ev *event.KeyEvent) {
	if e.onKey != nil && e.onKey(ev) {
		return
	}
	if ev.Name == event.KeyEscape && e.onEscape != nil {
		e.onEscape()
		return
	}
	e.Entry.TypedKey(ev)
}

func (e *editorEntry) TypedShortcut(s core.Shortcut) {
	if e.onShortcut != nil && e.onShortcut(s) {
		return
	}
	e.Entry.TypedShortcut(s)
}

// TypedRune lets the Entry insert the rune first, then reports it to onRune so
// electric reindent sees the line with the character already in place. During
// an IME composition the Entry routes the rune to the commit path itself;
// electric reindent still runs against the committed result.
func (e *editorEntry) TypedRune(r rune) {
	e.Entry.TypedRune(r)
	if e.onRune != nil {
		e.onRune(r)
	}
}

func (e *editorEntry) MouseDown(m *desktop.MouseEvent) {
	if e.onMouseDown != nil {
		e.onMouseDown(m)
	}
	e.Entry.MouseDown(m)
}

func (e *editorEntry) KeyDown(k *event.KeyEvent) {
	if e.onKeyDown != nil {
		e.onKeyDown(k)
	}
	e.Entry.KeyDown(k)
}

func (e *editorEntry) KeyUp(k *event.KeyEvent) {
	if e.onKeyUp != nil {
		e.onKeyUp(k)
	}
	e.Entry.KeyUp(k)
}

func (e *editorEntry) FocusLost() {
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
	e.Entry.FocusLost()
}

func (e *editorEntry) addFindShortcuts(onFind, onFindNext, onFindPrev func()) {
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyF,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) { onFind() })
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyG,
		Modifier: event.KeyModifierSuper,
	}, func(_ core.Shortcut) { onFindNext() })
	e.Entry.AddShortcut(&desktop.CustomShortcut{
		KeyName:  event.KeyG,
		Modifier: event.KeyModifierSuper | event.KeyModifierShift,
	}, func(_ core.Shortcut) { onFindPrev() })
}
