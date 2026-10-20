package widget

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/belluikit/typesetting/di"
	"github.com/belluikit/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/textbuf"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

const passwordChar = "•"

var (
	_ core.Widget                        = (*RichText)(nil)
	_ core.Accessible                    = (*RichText)(nil)
	_ core.AccessibleVirtualNodeProvider = (*RichText)(nil)
	_ core.AccessibleTexter              = (*RichText)(nil)
)

// AccessibleRole returns [core.RoleLabel].
func (t *RichText) AccessibleRole() core.AccessibleRole { return core.RoleLabel }

// AccessibleLabel returns the concatenated plain text of all segments.
func (t *RichText) AccessibleLabel() string { return t.String() }

// AccessibleText returns the concatenated segment text for assistive
// technologies (pre-rebrand tracker #556). Selection offsets index this text; around non-text
// block segments (images, separators) — which contribute rows but no runes —
// the mapping is approximate.
func (t *RichText) AccessibleText() string { return t.String() }

// AccessibleSelection returns the user selection as a half-open rune range;
// when nothing is selected both offsets are the cursor position. Always
// (0, 0) while the rich text is not selectable.
func (t *RichText) AccessibleSelection() (start, end int) {
	if !t.isSelectable() || t.selection == nil {
		return 0, 0
	}
	return t.selection.accessibleSelection()
}

// AccessibleCaret returns the rune offset of the selection cursor.
func (t *RichText) AccessibleCaret() int {
	if !t.isSelectable() || t.selection == nil {
		return 0
	}
	return t.selection.accessibleCaret()
}

// SetAccessibleSelection selects [start, end), clamped to the content; equal
// offsets clear the selection. A no-op while the rich text is not
// selectable. It is safe to call from any goroutine.
func (t *RichText) SetAccessibleSelection(start, end int) {
	if !t.isSelectable() || t.selection == nil {
		return
	}
	onMain(func() {
		t.selection.setAccessibleSelection(start, end, t.Wrapping)
	})
}

// ReplaceAccessibleSelection is a no-op: rich text is not editable.
func (t *RichText) ReplaceAccessibleSelection(string) {}

// AccessibleVirtualNodes reports one RoleHeading node per heading segment
// (pre-rebrand tracker #533), so screen readers can navigate rendered markdown/HTML by
// headings. Headings have no CanvasObject of their own — their text is
// painted by the shared row machinery — so each node's bounds are the
// union of the segment's laid-out row visuals, read from the visual cache
// without touching the layout engine. Segments not yet rendered contribute
// no node.
func (t *RichText) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	var nodes []core.AccessibleVirtualNode

	t.visualMu.Lock()
	defer t.visualMu.Unlock()
	for _, seg := range t.Segments {
		ts, ok := seg.(*TextSegment)
		if !ok || ts.Style.HeadingLevel == 0 {
			continue
		}

		var rect geo.Rectangle
		found := false
		for _, vis := range t.visualCache[seg] {
			if !vis.Visible() {
				continue
			}
			r := geo.Rectangle{Pos: vis.Position(), Size: vis.Size()}
			if !found {
				rect = r
				found = true
				continue
			}
			x1 := geo.Min(rect.Pos.X, r.Pos.X)
			y1 := geo.Min(rect.Pos.Y, r.Pos.Y)
			x2 := geo.Max(rect.Pos.X+rect.Size.Width, r.Pos.X+r.Size.Width)
			y2 := geo.Max(rect.Pos.Y+rect.Size.Height, r.Pos.Y+r.Size.Height)
			rect = geo.Rectangle{Pos: geo.NewPos(x1, y1), Size: geo.NewSize(x2-x1, y2-y1)}
		}
		if !found {
			continue
		}

		// Row visuals are positioned in content coordinates; when the
		// RichText virtualizes behind its internal scroller the widget-
		// relative position must subtract the scroll offset, and rows
		// scrolled out of the viewport are dropped like clipped real
		// nodes (pre-rebrand tracker #545).
		if t.scr != nil {
			rect.Pos = geo.NewPos(rect.Pos.X-t.scr.Offset.X, rect.Pos.Y-t.scr.Offset.Y)
			viewport := t.scr.Size()
			if rect.Pos.Y+rect.Size.Height < 0 || rect.Pos.Y > viewport.Height {
				continue
			}
		}

		nodes = append(nodes, core.AccessibleVirtualNode{
			AccessibleNode: core.AccessibleNode{
				Role:  core.RoleHeading,
				Label: ts.Text,
				Level: ts.Style.HeadingLevel,
			},
			Pos:  rect.Pos,
			Size: rect.Size,
			ID:   seg,
		})
	}
	return nodes
}

// RichText represents the base element for a rich text-based widget.
//
// Since: 1.0
type RichText struct {
	BaseWidget
	Segments []RichTextSegment
	Wrapping bellui.TextWrap
	Scroll   bellui.ScrollDirection

	// The truncation mode of the text
	//
	// Since: 1.0
	Truncation bellui.TextTruncation

	// AllowRemoteContent opts this RichText into loading image segments whose
	// source is a remote (http/https) or local-file URI. It is false by default
	// so that rendering untrusted HTML/Markdown cannot, with no user
	// interaction, trigger network requests (SSRF, unbounded downloads) or read
	// local files (pre-rebrand tracker #285). When false, such ImageSegments render a broken-image
	// placeholder instead of being fetched. Set it to true only for content you
	// trust (e.g. local documentation or your own markdown).
	//
	// Since: 1.0
	AllowRemoteContent bool

	// Virtualize opts a scrollable RichText (Scroll != ScrollNone) into row
	// virtualisation: only the rows whose vertical band intersects the viewport
	// are materialised and shaped, so a very large document (thousands of rows)
	// scrolls without laying out every row up front (pre-rebrand tracker #158). It has no effect when
	// Scroll is ScrollNone, or on the truncation/single-row configurations that
	// cannot virtualise (see canVirtualize).
	//
	// Off by default: virtualisation trades exact, eager measurement for a
	// windowed model whose vertical (and, for wrap-off content, horizontal) scroll
	// extent is estimated from the materialised window and refines as you scroll
	// (pre-rebrand tracker #157/#170 jitter). That is a worthwhile trade only for genuinely large
	// documents, so it is opt-in rather than automatic for every scrolling
	// RichText. Entry enables the equivalent path internally regardless of this
	// field.
	Virtualize bool

	// RowHighlighter, when set, returns styled runs for a row. Used by Entry
	// for syntax highlighting without modifying the segment model.
	RowHighlighter func(row int) []StyleRun

	// ShowLineNumbers renders line numbers in the gutter area.
	// GutterWidth is computed and used by Layout, CursorPosition, and getRowCol.
	ShowLineNumbers bool
	GutterWidth     float32 // computed; read by Entry for cursor offset

	// BreakpointLines is the set of 1-based line numbers where breakpoint
	// markers (red dots) should be rendered in the gutter.
	BreakpointLines map[int]bool

	// HighlightedRow is the 1-based line number to highlight with a
	// background color (e.g., current execution line in a debugger).
	// Set to 0 to disable.
	HighlightedRow int

	inset     geo.Size      // this varies due to how the widget works (entry with scroller vs others with padding)
	rowBounds []rowBoundary // cache for boundaries
	scr       *widget.Scroll
	prop      *canvas.Rectangle // used to apply text minsize to the scroller `scr`, if present (upstream pre-rebrand tracker #2464)

	visualMu    sync.Mutex
	visualCache map[RichTextSegment]map[int]core.CanvasObject

	// Performance caches to avoid redundant work on Refresh.
	markdownCache       string                // last markdown string passed to ParseMarkdown
	rowBoundsWidth      float32               // width when rowBounds was last computed
	rowBoundsVersion    int                   // bumped when segments change
	rowBoundsComputed   int                   // version when rowBounds was last computed
	rowBoundsTruncation bellui.TextTruncation // truncation when rowBounds was last computed
	rowBoundsWrapping   bellui.TextWrap       // wrapping when rowBounds was last computed
	lastContentFP       uint64                // content fingerprint at last SyncState — detects external changes

	// Memoized multi-segment String() result (pre-rebrand tracker #560): the returned object
	// keeps its identity while the content version is unchanged, so
	// identity-keyed consumers (the AX text index, pre-rebrand tracker #555) hit their caches
	// instead of re-indexing per query. Guarded by the same staleness
	// heuristics as the row model (rowBoundsVersion + contentFingerprint).
	strMemo    string
	strMemoOk  bool
	strMemoVer int
	strMemoFP  uint64

	// The last charMinSize result and what it was measured for (#492). Every
	// min-size pass asks each label for it, and looking it up in the global
	// font metrics cache cost about a third of the pass in the labels scene.
	charMemoKey charSizeKey
	charMemo    geo.Size
	charMemoOk  bool

	// Selectable enables text selection on this RichText when the window's
	// SelectableMode is SelectableAll. Default false — internal RichText
	// widgets (e.g. inside Button) leave this false so they remain non-selectable.
	//
	// Since: 1.0
	Selectable bool

	selection *focusSelectable // overlay for text selection (created in CreateRenderer)

	// Cached row geometry from Layout, used by selection hit-testing.
	rowYPos   []float32 // Y position of each row (absolute, from top)
	rowHeight []float32 // height of each row

	// Per-row measured-height cache (pre-rebrand tracker #158 Step 1). Dense (full-materialisation)
	// path only: avoids re-measuring (re-shaping) unchanged rows on every
	// calculateMin/Layout after the first pass. Parallel to rowBounds; a NaN
	// entry means "not yet measured". Rebuilt (all NaN) whenever the key
	// (rowBoundsVersion, measurement width, theme inner-pad/line-spacing/text-size)
	// changes. Rows containing async-sized "volatile" segments (images, tables)
	// are never stored here — they are always measured fresh, so an image that
	// resizes after load is never served a stale height.
	rowHeightCache            []float32
	rowHeightCacheVer         int     // rowBoundsVersion the cache was built for
	rowHeightCacheWidth       float32 // measurement width the cached heights are valid for
	rowHeightCacheInnerPad    float32 // theme guard: innerPadding
	rowHeightCacheLineSpacing float32 // theme guard: lineSpacing
	rowHeightCacheTextSize    float32 // theme guard: text size

	// styleHeightCache memoises the line height of ASCII text runs by their
	// (size, style), so the variable-height prefix build (pre-rebrand tracker #158) measures each
	// distinct style once instead of re-shaping every row (pre-rebrand tracker #313). Sound only for
	// ASCII text: a non-ASCII run may shape with a fallback face whose line metrics
	// differ, so those rows are always measured fresh. Reset with rowHeightCache.
	styleHeightCache map[rtHeightKey]float32

	// denseIncr is the edit-incremental dense build state (pre-rebrand tracker #921): a snapshot +
	// per-segment checkpoints of the last dense build, letting the next rebuild
	// re-shape only the changed segment range and the renderer re-render only
	// the resulting dirty rows. Nil whenever the incremental path is ineligible.
	denseIncr *denseIncrState

	// Row virtualisation (pre-rebrand tracker #119). When virtualize is set — by Entry for
	// scrollable, non-wrapping, single-segment content — only the rows whose Y
	// band intersects the viewport (+buffer) are materialised and shaped. Row
	// geometry for every row is still filled analytically (uniform stride) so
	// selection/cursor/gutter consumers are unaffected. See canVirtualize.
	virtualize   bool
	winStart     int     // first materialised row (inclusive)
	winEnd       int     // last materialised row (exclusive)
	winRows      []int   // exact materialised logical rows (skips folded rows)
	viewTop      float32 // viewport top in content coordinates
	viewHeight   float32 // viewport height; 0 = unknown, use a bootstrap window
	virtWidth    float32 // cached widest-row width for the virtual MinSize
	virtWidthVer int     // rowBoundsVersion virtWidth was computed for
	virtWidthOK  bool    // whether virtWidth has been computed at least once

	// Code folding (pre-rebrand tracker #141). hiddenRanges are the resolved visual-row ranges laid
	// out and painted as absent: zero height, not materialised. The text model,
	// row count and search positions are unaffected — only vertical geometry and
	// rendering skip them. Empty == no folding. Normalised (sorted, merged,
	// Start>=0). Derived from hiddenLogical via rowToLine by applyHiddenRanges.
	hiddenRanges []LineRange
	// hiddenLogical is the caller's fold request in logical-line coordinates
	// (Entry.HiddenRanges). For wrapped content a logical line spans several
	// visual rows; rowToLine translates the request into hiddenRanges (pre-rebrand tracker #145).
	// For wrap-off single-segment content one logical line is one visual row, so
	// the translation is the identity and behaviour matches the original F4.
	hiddenLogical []LineRange
	// rowToLine maps each visual row to its 0-based logical line (newline count
	// before the row's start). Non-decreasing. Built by buildRowToLine for
	// single-segment content; nil for multi-segment, where folds stay row-based.
	rowToLine []int

	// Lazy/windowed wrap measurement (pre-rebrand tracker #157). Active only when canVirtualize();
	// then rowBounds stays nil and the row model is served from these per-logical-
	// line structures, so a large wrapped document shapes only the lines spanning
	// the viewport instead of every line up front. The Phase-A uniform-stride
	// invariant (single segment, truncation off) makes row Y analytic, so only the
	// wrapped-row COUNT per logical line is unknown until that line is measured;
	// unmeasured lines carry a cheap ceil(runeLen/estCharsPerRow) estimate that
	// refines to exact on measurement (accepted scroll-extent jitter, pre-rebrand tracker #157).
	lazy           bool            // lazy row model is current (set by updateRowBounds)
	buf            *textbuf.Buffer // canonical content buffer + logical-line index for the single segment (pre-rebrand tracker #380/#400)
	lazyStr        string          // the segment Text value buf currently reflects (identity anchor)
	lineMeasured   []bool          // line i's wrapped rows are exact (vs estimated)
	lineWrap       [][]rowBoundary // line i's exact wrapped sub-rows, LINE-RELATIVE offsets (nil until measured)
	lineRowCount   []int           // visual rows for line i (exact if measured, else estimate); nil ⇒ lazy model not built
	lineStartRow   []int           // prefix sum: first visual row of line i (len == lines+1)
	prefixDirty    bool            // lineStartRow needs rebuild after a rowCount change
	estCharsPerRow float32         // estimate denominator (avg chars per wrapped row)
	lazyMaxWidth   float32         // wrap/measure width the lazy model was built for
	lineScratch    []rune          // reusable scratch for per-line wrap measurement (ensureLineMeasured)
	lazyFullBuilds int             // full content (re)builds of buf — test hook for identity-chain regressions (pre-rebrand tracker #400)
	vwLine         int             // widest logical line (wrap-off virtual width cache, pre-rebrand tracker #403)
	vwLen          int             // rune length of vwLine when cached
	vwDirty        bool            // widest-line cache needs a rescan

	// Deferred text sync (pre-rebrand tracker #410). When deferText is set — by Entry when its
	// DeferTextSync mode is on — lazySpliceText skips the per-edit document
	// materialization: seg.Text and lazyStr keep the last-synced string object
	// (so the identity gate still detects external writes) while buf runs
	// ahead with the real content. deferredDirty records that state; it is
	// cleared when lazyMaterialize re-anchors the pair, or when a full rebuild
	// resets buf from seg.Text (an external write wins over deferred edits).
	deferText     bool // skip materialization in lazySpliceText (set by Entry)
	deferredDirty bool // buf holds edits not yet reflected in seg.Text/lazyStr

	// onLazyLinesEdited, when set, is called after an incremental edit replaced
	// logical lines [a..z] with newCount lines, so per-line caches layered above
	// the provider (Entry's highlight cache) can splice in step (pre-rebrand tracker #401).
	onLazyLinesEdited func(a, z, newCount int)

	// Variable-height multi-segment virtualisation (pre-rebrand tracker #158 Step 2). Mutually
	// exclusive with lazy: lazy is the single-segment uniform-stride model;
	// varHeight is the multi-segment model where each visual row can have a
	// different height (images, tables, code blocks, blockquotes). rowBounds is
	// still built eagerly (all boundaries known), but only the visible window's
	// objects are materialised and only in-window volatile rows are measured;
	// per-row Y comes from rowYPrefix (a prefix sum of measured/estimated row
	// heights) so RowGeometry/rowAtY are O(1)/O(log n) without a dense array.
	varHeight         bool      // variable-height model is current (set by updateRowBounds)
	rowYPrefix        []float32 // prefix sum: rowYPrefix[i] = top of row i (folds+spacing baked in); len rows+1
	prefixHeightDirty bool      // rowYPrefix needs rebuild after a height/fold change
	prefixHeightWidth float32   // content width rowYPrefix was built for
	prefixHeightVer   int       // rowBoundsVersion rowYPrefix was built for
	volatileEstAvg    float32   // running mean of measured volatile-row heights (estimate source)
	volatileEstCount  int       // number of volatile rows folded into volatileEstAvg

	// Windowed multi-segment build (pre-rebrand tracker #317). The block-lazy model is the cold-build
	// analogue of the single-segment lazy model (pre-rebrand tracker #157) for the variable-height
	// multi-segment case: instead of shaping every row up front (the O(n) HarfBuzz
	// cost), updateRowBoundsBlockLazy groups segments into logical *blocks* with a
	// cheap structural pass (no shaping) and estimates each block's visual-row count
	// and height. A block is shaped on demand (ensureBlockMeasured) only when the
	// viewport — or a caret/selection/search op via rowBoundary — addresses it,
	// replacing the estimate and refining the prefix sums. Mutually exclusive with
	// lazy and varHeight; declines when folds are active (those keep the proven dense
	// varHeight path). rowBounds stays nil; the row model is served from these
	// block-level structures. See richtext_blocklazy.go.
	blockLazy           bool            // block-lazy multi-segment model is current
	blocks              []richBlock     // structural blocks (no shaping)
	blockMeasured       []bool          // block i's rows/heights are exact (vs estimated)
	blockRows           [][]rowBoundary // block i's exact sub-rows (nil until measured)
	blockRowHeights     [][]float32     // block i's exact per-row heights (nil until measured)
	blockRowCount       []int           // visual rows for block i (exact if measured, else estimate)
	blockHeight         []float32       // block i's content height excluding trailing spacing
	blockEstRowH        []float32       // estimated per-row height for an unmeasured block i
	blockStartRow       []int           // prefix sum: first visual row of block i (len blocks+1)
	blockTopY           []float32       // prefix sum: top Y of block i within content (len blocks+1)
	blockPrefixDirty    bool            // block prefixes need rebuild after a count/height change
	blockMaxWidth       float32         // wrap/measure width the block model was built for
	blockInnerPad       float32         // innerPadding the block model was built for
	blockEstCharsPerRow float32         // estimate denominator (avg chars per wrapped row)

	// namedFonts is the per-RichText allowlist of custom fonts that parsed
	// HTML/Markdown may request by family name (pre-rebrand tracker #496). Keys are normalised
	// (trimmed, lowercased) so matching is case- and whitespace-insensitive.
	// Populated only by RegisterNamedFont. Untrusted parsed content can only
	// ever select a font present here (consistent with pre-rebrand tracker #285: no font-file
	// loading from content, allowlist-only); an unknown family falls through
	// to the default behaviour with a nil FontSource.
	namedFonts map[string]bellui.Resource
}

// virtualRowBuffer is the number of extra rows materialised above and below the
// viewport so a small scroll does not immediately require a re-window.
const virtualRowBuffer = 8

// varHeightMinRows is the visual-row count above which multi-segment content
// switches to the variable-height virtual window (pre-rebrand tracker #158). Below it, small
// documents stay on the simple full-materialisation path: windowing and the
// volatile-row estimate jitter are pure overhead at small sizes.
const varHeightMinRows = 2000

// NewRichText returns a new RichText widget that renders the given text and segments.
// If no segments are specified it will be converted to a single segment using the default text settings.
//
// Since: 1.0
func NewRichText(segments ...RichTextSegment) *RichText {
	t := &RichText{Segments: segments}
	t.Scroll = widget.ScrollNone
	t.ExtendBaseWidget(t)
	return t
}

// NewRichTextWithText returns a new RichText widget that renders the given text.
// The string will be converted to a single text segment using the default text settings.
//
// Since: 1.0
func NewRichTextWithText(text string) *RichText {
	return NewRichText(&TextSegment{
		Style: RichTextStyleInline,
		Text:  text,
	})
}

// RegisterNamedFont adds a custom font to this RichText's per-widget allowlist
// under the given family name, so that parsed HTML/Markdown content can select
// it via CSS font-family / <font face> (pre-rebrand tracker #496). The name is matched case- and
// whitespace-insensitively (it is trimmed and lowercased before storage and
// lookup). Registering an empty name is a no-op; registering a nil resource
// removes any existing entry.
//
// This is the only way parsed (potentially untrusted) content can choose a
// custom font: the parser will never load a font file named in the content,
// only select from fonts the application registered here (consistent with the
// untrusted-content security model, pre-rebrand tracker #285). A requested family that is not
// registered falls through to the default behaviour (nil FontSource).
//
// Since: 1.0
func (t *RichText) RegisterNamedFont(name string, res bellui.Resource) {
	key := normalizeFontName(name)
	if key == "" {
		return
	}
	if res == nil {
		delete(t.namedFonts, key)
		return
	}
	if t.namedFonts == nil {
		t.namedFonts = make(map[string]bellui.Resource)
	}
	t.namedFonts[key] = res
}

// lookupNamedFont resolves a normalized family name against this RichText's
// allowlist, returning nil when unregistered (or when there is no allowlist).
func (t *RichText) lookupNamedFont(name string) bellui.Resource {
	if t == nil || len(t.namedFonts) == 0 {
		return nil
	}
	return t.namedFonts[normalizeFontName(name)]
}

// normalizeFontName trims surrounding whitespace and lowercases a font family
// name so lookups are case- and whitespace-insensitive.
func normalizeFontName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// isSelectable returns whether this RichText should support text selection,
// considering both the Selectable property and the window-level SelectableMode.
func (t *RichText) isSelectable() bool {
	if !t.Selectable {
		return false // no window mode makes a non-selectable RichText selectable
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(t)
	if sc, ok := c.(selectableCanvas); ok {
		switch sc.SelectableMode() {
		case bellui.SelectableAll:
			return t.Selectable
		case bellui.SelectableNone:
			return false
		}
	}
	return t.Selectable
}

// SelectedText returns the text currently selected in this RichText.
// Only works when Selectable is true and the window's SelectableMode permits it.
//
// Since: 1.0
func (t *RichText) SelectedText() string {
	if !t.isSelectable() || t.selection == nil {
		return ""
	}
	return t.selection.SelectedText()
}

// updateVirtualViewport feeds the internal scroll container's viewport to the
// virtualised row model and, when the materialised window changes, triggers a
// window-sized re-render. It is the standalone-RichText equivalent of
// Entry.updateTextViewport (an Entry drives its own external scroll), letting a
// scrollable RichText/Label window a huge document (pre-rebrand tracker #158). No-op unless
// virtualisation is active. SetVisibleRange only reports a change when the window
// actually moved, so the Refresh→Layout→here loop converges after the window
// stabilises (matching the Entry path).
func (t *RichText) updateVirtualViewport() {
	if t.scr == nil || !t.canVirtualize() {
		return
	}
	if t.SetVisibleRange(t.scr.Offset.Y, t.scr.Size().Height) {
		cache.Renderer(t).Refresh()
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (t *RichText) CreateRenderer() core.WidgetRenderer {
	t.prop = canvas.NewRectangle(color.Transparent)
	if t.scr == nil && t.Scroll != widget.ScrollNone {
		t.scr = widget.NewScroll(&bellui.Container{Layout: layout.NewStackLayout(), Objects: []core.CanvasObject{
			t.prop, &bellui.Container{},
		}})
		if t.Virtualize {
			// Opted in: the scrollable RichText drives its own virtual window from
			// the scroll offset (pre-rebrand tracker #158); large multi-segment documents then window
			// via varHeight. Without opt-in the scroll container shows the fully
			// materialised content, identical to the before pre-rebrand tracker #158 behaviour (pre-rebrand tracker #169).
			t.virtualize = true
			t.scr.OnScrolled = func(geo.Position) { t.updateVirtualViewport() }
		}
	}

	t.selection = &focusSelectable{}
	t.selection.ExtendBaseWidget(t.selection)
	t.selection.focus = t.selection
	t.selection.theme = t.Theme()
	t.selection.provider = t

	t.ExtendBaseWidget(t)
	r := &textRenderer{obj: t}

	t.updateRowBounds() // set up the initial text layout etc
	r.Refresh()
	return r
}

// MinSize calculates the minimum size of a rich text widget.
// This is based on the contained text with a standard amount of padding added.
func (t *RichText) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// IntrinsicSize reports the size the text wants when given unlimited width: its
// natural extent measured as if Wrapping and Truncation were off, plus the
// standard padding.
//
// It complements [RichText.MinSize], which for a wrapping or truncating widget
// reports a width of ~0 — such a widget can shrink to fit any width by wrapping
// or clipping, so its minimum width carries no information about the content. Use
// IntrinsicSize to size a container, popover, or tooltip to its content: measure
// the natural width, cap it as desired, then read the wrapped height back from
// MinSize at that capped width.
//
// When Wrapping and Truncation are already off, IntrinsicSize is identical to
// MinSize.
func (t *RichText) IntrinsicSize() geo.Size {
	t.ExtendBaseWidget(t)
	if t.Wrapping == bellui.TextWrapOff && t.Truncation == bellui.TextTruncateOff {
		return t.MinSize()
	}
	// Prefer the side-effect-free natural measurement for the common single
	// text-segment content (Label, single-segment Entry/RichText): it computes the
	// unwrapped extent directly without mutating or repainting the widget.
	if size, ok := t.naturalSize(); ok {
		return size
	}
	return t.intrinsicSizeViaRefresh()
}

// naturalSize measures the unwrapped extent of single text-segment content
// (the only shape Label and Entry ever produce, and the common RichText case)
// without touching the live render state — no row-bounds mutation, no repaint,
// no scroll/virtual-window reset. It mirrors the dense calculateMin arithmetic
// for one TextSegment in wrap-off / truncation-off mode and reports the content
// extent directly from the text, so it is well-defined even when the widget
// itself scrolls or virtualises (where MinSize collapses to the scroll minimum)
// — exactly the case an Entry presents. ok is false for content it does not
// handle (multiple/non-text segments or active folds, which change the extent),
// so the caller falls back to intrinsicSizeViaRefresh.
//
// It measures every logical line, so it is O(content) — acceptable because
// IntrinsicSize is a content-autosizing query (an exact widest-line answer
// inherently requires examining every line), never the layout hot path.
func (t *RichText) naturalSize() (geo.Size, bool) {
	if len(t.Segments) != 1 || len(t.hiddenRanges) > 0 {
		return geo.Size{}, false
	}
	seg, ok := t.Segments[0].(*TextSegment)
	if !ok {
		return geo.Size{}, false
	}

	th := t.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)
	style := seg.Style.TextStyle
	textSize := seg.size()
	source := seg.Style.FontSource

	// One row per logical (newline-delimited) line when wrapping is off: width is
	// the widest line, height the sum of line heights plus inter-row line spacing
	// (added only between non-inline rows, exactly as calculateMin gates it).
	lines := strings.Split(seg.Text, "\n")
	var width, height float32
	for _, line := range lines {
		m, _ := bellui.MeasureTextFull(line, textSize, style, source)
		width = geo.Max(width, m.Width)
		height += m.Height
	}
	if !seg.Inline() && len(lines) > 1 {
		height += lineSpacing * float32(len(lines)-1)
	}
	if height == 0 {
		height = t.charMinSize(false, bellui.TextStyle{}, th.Size(theme.SizeNameText)).Height
	}

	return geo.NewSize(width, height).
		Add(geo.NewSquareSize(innerPad * 2).Subtract(t.inset).Subtract(t.inset)), true
}

// intrinsicSizeViaRefresh measures the unwrapped extent by driving the widget
// through a temporary wrap-off / truncation-off refresh and reading MinSize, then
// restoring. It is the correct-for-all-content fallback used when naturalSize
// cannot measure the content cheaply. Heavier than a plain MinSize and main-
// goroutine-only (it refreshes), so it is reserved for the content-autosizers
// that call IntrinsicSize, never the layout hot path.
func (t *RichText) intrinsicSizeViaRefresh() geo.Size {
	wrap, trunc := t.Wrapping, t.Truncation
	t.Wrapping, t.Truncation = bellui.TextWrapOff, bellui.TextTruncateOff
	t.Refresh()
	size := t.MinSize()
	t.Wrapping, t.Truncation = wrap, trunc
	t.Refresh()
	return size
}

// SyncState syncs RichText's segments to internal rendering state.
// Called automatically by BaseWidget.Refresh() before the renderer refreshes.
func (t *RichText) SyncState() {
	// Only invalidate row bounds when content has actually changed.
	// Compute a lightweight fingerprint from segment count + total text length.
	// Mutation methods also bump rowBoundsVersion explicitly for immediate detection.
	fp := t.contentFingerprint()
	if fp != t.lastContentFP {
		t.rowBoundsVersion++
		t.lastContentFP = fp
	}
	t.updateRowBounds()

	for _, s := range t.Segments {
		if txt, ok := s.(*TextSegment); ok {
			txt.parent = t
		}
	}
}

// Resize sets a new size for the rich text.
// This should only be called if it is not in a container with a layout manager.
func (t *RichText) Resize(size geo.Size) {
	if size == t.Size() {
		return
	}

	t.size = size

	min := t.MinSize()
	skipResize := !min.IsZero() && size.Width >= min.Width && size.Height >= min.Height && t.Wrapping == bellui.TextWrapOff && t.Truncation == bellui.TextTruncateOff

	if skipResize {
		if len(t.Segments) < 2 { // we can simplify :)
			cache.Renderer(t).Layout(size)
			return
		}
	}

	t.rowBoundsVersion++ // width changed — invalidate cache
	if t.refreshing {
		// Being resized as part of a Refresh — just recompute row bounds
		// for the new width; the ongoing Refresh will handle rendering.
		t.updateRowBounds()
		return
	}
	t.Refresh()
}

// String returns the text widget buffer as string
func (t *RichText) String() string {
	// A single segment (the Entry editing case) is returned verbatim: no builder
	// copy, and the returned string keeps its identity so the lazy edit path's
	// seg.Text == lazyStr guard stays an O(1) pointer compare next keystroke (pre-rebrand tracker #380).
	if len(t.Segments) == 1 {
		return t.Segments[0].Textual()
	}

	// Multi-segment content memoizes the concatenation (pre-rebrand tracker #560): repeated calls
	// (the accessibility walker snapshots AccessibleLabel per tree update, the
	// AX text protocol queries AccessibleText per screen-reader interaction)
	// return the same string object until the content version moves, instead
	// of re-concatenating each time — and identity-keyed consumers (the AX
	// text index, pre-rebrand tracker #555) hit their caches. Staleness detection matches the row
	// model's: the version bumped by the mutation paths plus the length-based
	// fingerprint SyncState uses — a direct same-length Segments edit that
	// bypasses both is already stale for rendering too.
	fp := t.contentFingerprint()
	if t.strMemoOk && t.strMemoVer == t.rowBoundsVersion && t.strMemoFP == fp {
		return t.strMemo
	}
	ret := strings.Builder{}
	for _, seg := range t.Segments {
		ret.WriteString(seg.Textual())
	}
	t.strMemo = ret.String()
	t.strMemoOk = true
	t.strMemoVer = t.rowBoundsVersion
	t.strMemoFP = fp
	return t.strMemo
}

// charSizeKey is everything charMinSize's measurement depends on: the style
// as the font metrics cache keys it (pointer fields resolved to values), and
// the cache generation, which changes whenever fonts or the theme do.
type charSizeKey struct {
	concealed                       bool
	size                            float32
	bold, italic, monospace, symbol bool
	underline, strikethrough        bool
	tabWidth                        int
	weight                          bellui.TextWeight
	lineHeightScale, letterSpacing  float32
	direction                       bellui.TextDirection
	language                        string
	generation                      uint64
}

// charMinSize returns the average char size to use for internal computation
func (t *RichText) charMinSize(concealed bool, style bellui.TextStyle, textSize float32) geo.Size {
	key := charSizeKey{
		concealed: concealed, size: textSize,
		bold: style.Bold, italic: style.Italic, monospace: style.Monospace, symbol: style.Symbol,
		underline: style.Underline, strikethrough: style.Strikethrough,
		tabWidth:        style.TabWidth,
		weight:          style.ResolvedWeight(),
		lineHeightScale: style.ResolvedLineHeightScale(),
		letterSpacing:   style.ResolvedLetterSpacing(),
		direction:       style.Direction,
		language:        style.Language,
		generation:      cache.ThemeCacheGeneration(),
	}
	if t.charMemoOk && t.charMemoKey == key {
		return t.charMemo
	}

	defaultChar := "M"
	if concealed {
		defaultChar = passwordChar
	}
	t.charMemo = bellui.MeasureText(defaultChar, textSize, style)
	t.charMemoKey, t.charMemoOk = key, true
	return t.charMemo
}

// singleSegment returns the sole *TextSegment of this RichText, if the content
// is exactly one text segment (the virtualisation precondition).
func (t *RichText) singleSegment() (*TextSegment, bool) {
	if len(t.Segments) != 1 {
		return nil, false
	}
	seg, ok := t.Segments[0].(*TextSegment)
	return seg, ok
}

// canVirtualize reports whether the uniform-height row-virtualisation fast path
// applies: a parent (Entry) opted in via virtualize, wrapping and truncation are
// off (rows map 1:1 to lines and the row count is cheap to derive), and the
// content is a single text segment so every row shares one style ⇒ uniform
// stride. Mixed/wrapped content falls back to full materialisation.
func (t *RichText) canVirtualize() bool {
	if t.canVirtualizeSingle() {
		return true
	}
	// Multi-segment content has variable per-row heights, so the uniform-stride
	// geometry does not hold; the variable-height model (pre-rebrand tracker #158) handles it via a
	// height prefix sum. varHeight is set by updateRowBounds once the document is
	// large enough to be worth windowing (small docs stay on the simple full path).
	// blockLazy is the windowed-build variant of the same multi-segment model (pre-rebrand tracker #317).
	return t.varHeight || t.blockLazy
}

// canVirtualizeSingle reports whether the single-segment uniform-stride fast path
// (lazy model) applies: virtualisation opted in, truncation off, wrapping in
// {off, word, break}, and exactly one text segment so every row shares one style.
func (t *RichText) canVirtualizeSingle() bool {
	if !t.virtualize || t.Truncation != bellui.TextTruncateOff {
		return false
	}
	// Single-segment content has one uniform row height (stride) whether a row is
	// a whole line or a wrapped fragment, since every row shares the segment's
	// font/size and no line-spacing is inserted between same-segment rows. So the
	// analytic geometry in minVirtual/layoutVirtual holds for word/break wrapping
	// too (pre-rebrand tracker #139): only the row count grows; rowBounds still carries each wrapped
	// fragment's begin/end. Truncation is excluded (it changes per-row content).
	switch t.Wrapping {
	case bellui.TextWrapOff, bellui.TextWrapWord, bellui.TextWrapBreak:
	default:
		return false
	}
	_, ok := t.singleSegment()
	return ok
}

// varHeightEligible reports whether the (already built) dense rowBounds qualify for
// the variable-height multi-segment window: virtualisation opted in, truncation
// off, wrapping in {off, word, break}, more than one segment, and a document large
// enough that windowing pays off (pre-rebrand tracker #158).
func (t *RichText) varHeightEligible() bool {
	if !t.virtualize || t.Truncation != bellui.TextTruncateOff {
		return false
	}
	switch t.Wrapping {
	case bellui.TextWrapOff, bellui.TextWrapWord, bellui.TextWrapBreak:
	default:
		return false
	}
	if _, ok := t.singleSegment(); ok {
		return false
	}
	return len(t.rowBounds) > varHeightMinRows
}

// stride returns the uniform per-row vertical advance for the virtualised path.
// It mirrors the per-row height the non-virtual calculateMin would derive from a
// row's canvas.Text (single-line font height for the segment's style/size), and
// the stride used by Entry.RowMetrics / moveCursor / getRowCol's fallback.
func (t *RichText) stride() float32 {
	if seg, ok := t.singleSegment(); ok {
		return t.charMinSize(seg.Style.concealed, seg.Style.TextStyle, seg.size()).Height
	}
	return t.charMinSize(false, bellui.TextStyle{}, t.Theme().Size(theme.SizeNameText)).Height
}

// contentTopOffset is the Y of the first row's top within the content, matching
// the initial yPos in Layout (innerPadding minus the widget inset).
func (t *RichText) contentTopOffset() float32 {
	return t.Theme().Size(theme.SizeNameInnerPadding) - t.inset.Height
}

// SetVisibleRange informs a virtualised RichText of the viewport (in content
// coordinates) it is being scrolled within, so only the visible rows are
// materialised. A parent scroll container (Entry) calls this on scroll, layout
// and resize. It returns true when the materialised row window changed and the
// caller should trigger a cheap, window-sized re-render of the renderer.
func (t *RichText) SetVisibleRange(top, height float32) bool {
	if !t.canVirtualize() {
		return false
	}
	t.viewTop = top
	t.viewHeight = height
	return !intSliceEqual(t.computeWindowRows(), t.winRows)
}

// computeWindowRows returns the materialised logical rows for the current
// viewport: the rows whose visible-row band (viewport + virtualRowBuffer)
// intersects the screen. Folded rows are skipped, so a collapsed block never
// forces its hidden lines to materialise even when it sits inside the window.
// With no folds the result is a contiguous run, matching the pre-folding
// behaviour exactly.
func (t *RichText) computeWindowRows() []int {
	// Variable-height multi-segment: window via the height prefix sum. Handles
	// folds itself (hidden rows have zero extent in the prefix), so it is used
	// regardless of folding.
	if t.blockLazy {
		return t.computeWindowRowsBlockLazy()
	}
	if t.varHeight {
		return t.computeWindowRowsVar()
	}
	// Lazy, unfolded: measure only the lines spanning the band. With folds active
	// the document is measured eagerly, so the shared path below (which walks the
	// visible-row axis) sees exact counts and is reused as-is.
	if t.lazy && !t.anyHidden() {
		return t.computeWindowRowsLazy()
	}
	n := t.rows()
	s := t.stride()
	if n == 0 || s <= 0 {
		return nil
	}
	h := t.viewHeight
	if h <= 0 {
		h = 40 * s // bootstrap: roughly a screenful before the real viewport is known
	}
	top0 := t.contentTopOffset()
	totalVis := t.visibleRowCount()

	// Band expressed in visible-row indices (folds collapse to zero height, so
	// the viewport maps onto the visible-row axis, not the logical-row axis).
	kStart := int((t.viewTop-top0)/s) - virtualRowBuffer
	kEnd := int((t.viewTop+h-top0)/s) + 1 + virtualRowBuffer
	if kStart < 0 {
		kStart = 0
	}
	if kEnd > totalVis {
		kEnd = totalVis
	}
	if kStart > kEnd {
		kStart = kEnd
	}

	rows := make([]int, 0, kEnd-kStart)
	if !t.anyHidden() {
		for r := kStart; r < kEnd; r++ {
			rows = append(rows, r)
		}
		return rows
	}
	row := t.visibleIndexToRow(kStart)
	for k := kStart; k < kEnd && row < n; k++ {
		rows = append(rows, row)
		row = t.firstVisibleAtOrAfter(row + 1)
	}
	return rows
}

// visibleRowRange reports the contiguous logical extent [start, end) spanned by
// the current materialised window. Selection windowing uses it to bound the
// rows it builds rectangles for; folded rows inside the extent are skipped by
// the caller.
func (t *RichText) visibleRowRange() (start, end int) {
	rows := t.computeWindowRows()
	if len(rows) == 0 {
		return 0, 0
	}
	return rows[0], rows[len(rows)-1] + 1
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// deleteFromTo removes the text between the specified positions
func (t *RichText) deleteFromTo(lowBound int, highBound int) []rune {
	if lowBound >= highBound {
		return []rune{}
	}

	// Single-segment lazy fast path (pre-rebrand tracker #380): splice the range out of the canonical
	// buffer and byte-splice seg.Text, mirroring insertAt.
	if t.lazyCanEdit() {
		removed := t.lazySpliceText(lowBound, highBound, nil)
		t.Refresh()
		return removed
	}

	start := 0
	ret := make([]rune, 0, highBound-lowBound)
	deleting := false
	var segs []RichTextSegment
	for i, seg := range t.Segments {
		if _, ok := seg.(*TextSegment); !ok {
			if !deleting {
				segs = append(segs, seg)
			}
			continue
		}
		end := start + len([]rune(seg.(*TextSegment).Text))
		if end < lowBound {
			segs = append(segs, seg)
			start = end
			continue
		}

		startOff := int(math.Max(float64(lowBound-start), 0))
		endOff := int(math.Min(float64(end), float64(highBound))) - start
		r := ([]rune)(seg.(*TextSegment).Text)
		ret = append(ret, r[startOff:endOff]...)
		r2 := append(r[:startOff], r[endOff:]...)
		seg.(*TextSegment).Text = string(r2)
		segs = append(segs, seg)

		// prepare next iteration
		start = end
		if start >= highBound {
			segs = append(segs, t.Segments[i+1:]...)
			break
		} else if start >= lowBound {
			deleting = true
		}
	}
	t.Segments = segs
	t.rowBoundsVersion++ // invalidate row bounds cache after text change
	t.Refresh()
	return ret
}

// cachedSegmentVisual returns a cached segment visual representation.
// The offset value is > 0 if the segment had been split and so we need multiple objects.
//
// Storage is keyed by offset (not appended) so callers may request offsets
// sparsely and in any order — required by row virtualisation, which only
// materialises the visible window of rows (e.g. offsets {5000, 0, 5000}).
func (t *RichText) cachedSegmentVisual(seg RichTextSegment, offset int) core.CanvasObject {
	t.visualMu.Lock()
	defer t.visualMu.Unlock()

	// Propagate the content-security policy to image segments before their
	// visual is (re)built or updated, so a remote/file image is only ever
	// fetched when the embedding RichText opted in (pre-rebrand tracker #285).
	if img, ok := seg.(*ImageSegment); ok {
		img.allowRemote = t.AllowRemoteContent
	}

	if t.visualCache == nil {
		t.visualCache = make(map[RichTextSegment]map[int]core.CanvasObject)
	}

	bySeg := t.visualCache[seg]
	if bySeg == nil {
		bySeg = make(map[int]core.CanvasObject)
		t.visualCache[seg] = bySeg
	}
	if vis, ok := bySeg[offset]; ok {
		return vis
	}

	vis := seg.Visual()
	bySeg[offset] = vis
	return vis
}

func (t *RichText) cleanVisualCache() {
	t.visualMu.Lock()
	defer t.visualMu.Unlock()

	if len(t.visualCache) == 0 {
		return
	}
	// Evict cached visuals whose segment is no longer present. A set of the
	// current segments makes this O(cache + segments); the previous nested scan
	// was O(cache × segments), which for a large multi-segment document (e.g. an
	// editable RichEntry) cost quadratically per refresh — the pre-rebrand tracker #671 hotspot.
	current := make(map[RichTextSegment]struct{}, len(t.Segments))
	for _, seg := range t.Segments {
		current[seg] = struct{}{}
	}
	for seg := range t.visualCache {
		if _, ok := current[seg]; !ok {
			// The per-object measurement memo (pre-rebrand tracker #923) lives on the canvas.Text
			// itself, so a removed segment's memo is collected with its visual —
			// no side-table pruning needed here.
			delete(t.visualCache, seg)
		}
	}
}

// insertAt inserts the text at the specified position
func (t *RichText) insertAt(pos int, runes []rune) {
	// Single-segment lazy fast path (pre-rebrand tracker #380): splice the canonical rune buffer and
	// byte-splice seg.Text in place instead of decoding then re-encoding the whole
	// document. pos is a global char offset == local offset for a single segment.
	if t.lazyCanEdit() {
		t.lazySpliceText(pos, pos, runes)
		return
	}

	index := 0
	start := 0
	var into *TextSegment
	for i, seg := range t.Segments {
		ts, ok := seg.(*TextSegment)
		if !ok {
			continue
		}
		// Count runes without materialising a []rune (the slice for preceding
		// segments would be discarded immediately); the target segment is converted
		// once below (pre-rebrand tracker #375).
		end := start + utf8.RuneCountInString(ts.Text)
		into = ts
		index = i
		if end >= pos {
			break
		}

		start = end
	}

	if into == nil {
		return
	}
	// Convert global character offset to segment-local offset.
	localPos := pos - start
	r := ([]rune)(into.Text)
	if localPos < 0 {
		localPos = 0
	}
	if localPos > len(r) {
		localPos = len(r)
	}
	r2 := append(r[:localPos], append(runes, r[localPos:]...)...)
	into.Text = string(r2)
	t.Segments[index] = into
	t.rowBoundsVersion++ // invalidate row bounds cache after text change
}

// Len returns the text widget buffer length
func (t *RichText) len() int {
	// In the single-segment lazy path the buffer is the authoritative content,
	// so its length is the exact rune count with no per-call decode. This
	// short-circuits the O(document) utf8.RuneCountInString scan that the emptiness
	// checks in the Entry content renderer's MinSize/Refresh would otherwise run on
	// every keystroke.
	if t.lazy && t.buf != nil {
		return t.buf.Len()
	}
	ret := 0
	for _, seg := range t.Segments {
		ret += utf8.RuneCountInString(seg.Textual())
	}
	return ret
}

// textRange returns the text of the rune range [a, b) of the single editing
// segment. In the lazy path it reads from the buffer — a Substring against the
// clean memo, no full-document decode — and falls back to decoding the segment
// text otherwise (multi-segment content returns ""; callers use the row-based
// reconstruction for that case). Bounds are swapped/clamped as needed (pre-rebrand tracker #401).
func (t *RichText) textRange(a, b int) string {
	seg, ok := t.singleSegment()
	if !ok {
		return ""
	}
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if t.lazy && t.buf != nil && seg.Text == t.lazyStr {
		if n := t.buf.Len(); b > n {
			b = n
		}
		if a > b {
			a = b
		}
		return t.buf.Substring(a, b)
	}
	runes := []rune(seg.Text)
	if b > len(runes) {
		b = len(runes)
	}
	if a > b {
		a = b
	}
	return string(runes[a:b])
}

// lineSizeToColumn returns the rendered size for the line specified by row up to
// the col position. downstream selects the caret affinity used to resolve col on
// a bidi row at a direction boundary (true = forward; the default for callers
// that do not track affinity).
func (t *RichText) lineSizeToColumn(col, row int, textSize, innerPad float32, downstream bool) geo.Size {
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	bound := t.rowBoundary(row)
	total := geo.NewSize(0, 0)
	counted := 0
	last := false
	if bound == nil {
		return t.charMinSize(false, bellui.TextStyle{}, textSize)
	}

	// Bidi caret mapping: in a row containing right-to-left runs the visual x of
	// a logical column is not the width of the logical prefix. Map through the
	// shared visual layout instead. Pure-LTR rows return ok=false and fall
	// through to the exact legacy measurement below (which also expands tabs).
	if x, h, ok := t.rtlCaretX(row, col, downstream); ok {
		return geo.NewSize(x+innerPad-t.inset.Width, h)
	}

	for i, seg := range bound.segments {
		var size geo.Size
		if text, ok := seg.(*TextSegment); ok {
			start := 0
			if i == 0 {
				start = bound.begin
			}
			// Read from the lazy buffer instead of re-converting the whole document
			// for this one row (pre-rebrand tracker #375): fetch only the runes actually measured (up to
			// col), never a full-document slice.
			var measureText []rune
			if t.lazy && t.buf != nil && len(bound.segments) == 1 && text == t.Segments[0] {
				end := t.buf.Len()
				if col < counted+(end-start) {
					end = start + (col - counted)
					last = true
				}
				measureText = t.buf.Slice(start, end)
			} else {
				segRunes := []rune(text.Text)
				measureText = segRunes[start:]
				if col < counted+len(measureText) {
					measureText = measureText[0 : col-counted]
					last = true
				}
			}
			if concealed(seg) {
				measureText = []rune(strings.Repeat(passwordChar, len(measureText)))
			}
			counted += len(measureText)

			label := canvas.NewText(string(measureText), color.Black)
			label.TextStyle = text.Style.TextStyle
			label.TextSize = text.size()

			size = label.MinSize()
		} else {
			size = t.cachedSegmentVisual(seg, 0).MinSize()
		}

		total.Width += size.Width
		total.Height = geo.Max(total.Height, size.Height)
		if last {
			break
		}
	}
	return total.Add(geo.NewSize(innerPad-t.inset.Width, 0))
}

// inlineColumnX returns the x offset of a document-space column within a row that
// contains an inline non-text atom (an inline image, pre-rebrand tracker #665), walking the row's
// segments so an atom advances by its rendered width and each text rune by its
// measured width. The document-space column counts every text rune and every
// inline atom as one position — unlike lineSizeToColumn, whose column is text-rune
// only and which therefore measures *through* an inline image. The returned x uses
// the same baseline convention as lineSizeToColumn (innerPad-inset.Width, gutter
// excluded), so it is a drop-in for the caret and selection geometry.
//
// ok is false when the row has no inline atom, so callers keep the exact
// lineSizeToColumn path for ordinary rows — byte-identical to before (pre-rebrand tracker #893).
func (t *RichText) inlineColumnX(row, col int, textSize, innerPad float32) (float32, bool) {
	b := t.rowBoundary(row)
	if b == nil {
		return 0, false
	}
	hasInline := false
	for _, seg := range b.segments {
		if _, isText := seg.(*TextSegment); !isText && seg.Inline() {
			hasInline = true
			break
		}
	}
	if !hasInline {
		return 0, false
	}

	x := innerPad - t.inset.Width
	consumed := 0
	last := len(b.segments) - 1
	for i, seg := range b.segments {
		if consumed >= col {
			break
		}
		ts, ok := seg.(*TextSegment)
		if !ok {
			x += t.cachedSegmentVisual(seg, 0).MinSize().Width // inline atom
			consumed++
			continue
		}
		runes := []rune(ts.Text)
		lo, hi := 0, len(runes)
		if i == 0 {
			lo = b.begin
		}
		if i == last {
			hi = b.end
		}
		if hi < lo {
			hi = lo
		}
		if want := col - consumed; want <= hi-lo {
			m, _ := bellui.MeasureTextFull(string(runes[lo:lo+want]), ts.size(), ts.Style.TextStyle, ts.Style.FontSource)
			return x + m.Width, true
		}
		m, _ := bellui.MeasureTextFull(string(runes[lo:hi]), ts.size(), ts.Style.TextStyle, ts.Style.FontSource)
		x += m.Width
		consumed += hi - lo
	}
	return x, true
}

// inlineColumnAt is the inverse of inlineColumnX (pre-rebrand tracker #983): it maps a visual x on a
// row containing an inline non-text atom back to a caret position, walking the
// row's segments so an atom occupies its rendered width and each text rune its
// measured width — the atom-aware counterpart of the plain prefix-width scan
// (which measures *through* an inline image, landing every hit after it one
// image-width off). x uses the same baseline as inlineColumnX (innerPad-inset,
// gutter excluded).
//
// It returns the position in both column spaces so either caller model fits:
// textCol counts text runes only (the selectable/RichText model), docCol counts
// every text rune and every atom as one position (RichEntry's document model). A
// click on an atom resolves to the nearer edge — its own document position
// (docCol) before it, docCol+1 after it. ok is false when the row has no inline
// atom, so callers keep the plain scan for ordinary rows.
func (t *RichText) inlineColumnAt(row int, x, innerPad float32) (textCol, docCol int, downstream, ok bool) {
	b := t.rowBoundary(row)
	if b == nil {
		return 0, 0, true, false
	}
	hasInline := false
	for _, seg := range b.segments {
		if _, isText := seg.(*TextSegment); !isText && seg.Inline() {
			hasInline = true
			break
		}
	}
	if !hasInline {
		return 0, 0, true, false
	}

	cur := innerPad - t.inset.Width
	last := len(b.segments) - 1
	for i, seg := range b.segments {
		ts, isText := seg.(*TextSegment)
		if !isText {
			w := t.cachedSegmentVisual(seg, 0).MinSize().Width
			if x < cur+w/2 {
				return textCol, docCol, true, true // before the atom
			}
			if x < cur+w {
				return textCol, docCol + 1, true, true // on the atom, past its midpoint
			}
			cur += w
			docCol++ // atom consumes one document position, no text column
			continue
		}
		runes := []rune(ts.Text)
		lo, hi := 0, len(runes)
		if i == 0 {
			lo = b.begin
		}
		if i == last {
			hi = b.end
		}
		if hi < lo {
			hi = lo
		}
		prev := float32(0) // width of runes[lo:lo+k], k==0
		for k := 0; k < hi-lo; k++ {
			m, _ := bellui.MeasureTextFull(string(runes[lo:lo+k+1]), ts.size(), ts.Style.TextStyle, ts.Style.FontSource)
			mid := cur + (prev+m.Width)/2
			if x < mid {
				return textCol + k, docCol + k, true, true
			}
			prev = m.Width
		}
		cur += prev
		textCol += hi - lo
		docCol += hi - lo
	}
	return textCol, docCol, true, true
}

// rowStyledRuns assembles a row's segments into the per-segment runs that
// rowVisualLayout shapes for bidi layout. Each TextSegment becomes a style run
// holding the slice of its runes that belongs to this (possibly wrapped) row,
// mirroring row(); each non-text segment (link, image) becomes an opaque box run
// of its measured width, occupying a slot in the visual order but no caret column
// (pre-rebrand issue #41). It returns ok=false only if the row holds a concealed (password)
// segment, whose obscured runes can't share the rune-indexed caret model.
func (t *RichText) rowStyledRuns(row int) ([]paint.StyledRun, bool) {
	bound := t.rowBoundary(row)
	if bound == nil {
		return nil, false
	}
	nSegs := len(bound.segments)
	runs := make([]paint.StyledRun, 0, nSegs)
	for i, seg := range bound.segments {
		text, ok := seg.(*TextSegment)
		if !ok {
			runs = append(runs, paint.StyledRun{
				Box:      true,
				BoxWidth: t.cachedSegmentVisual(seg, 0).MinSize().Width,
			})
			continue
		}
		if concealed(seg) {
			return nil, false
		}
		runes := []rune(text.Text)
		start := 0
		end := len(runes)
		if i == 0 {
			start = bound.begin
		}
		if i == nSegs-1 && bound.end <= len(runes) && (bound.end > 0 || bound.begin >= bound.end) {
			end = bound.end
		}
		if start > end {
			start = end
		}
		runs = append(runs, paint.StyledRun{
			Runes:      runes[start:end],
			TextSize:   text.size(),
			Style:      text.Style.TextStyle,
			FontSource: text.Style.FontSource,
		})
	}
	return runs, true
}

// rowVisualLayout returns the bidi visual layout of a row, mapping logical
// columns to visual x positions across the whole row — including rows assembled
// from several differently-styled TextSegments, each shaped in its own style.
// It returns ok=false (so callers keep the legacy logical-prefix path) for rows
// that are pure left-to-right, empty, or contain a non-text or concealed segment.
func (t *RichText) rowVisualLayout(row int) (paint.LineLayout, bool) {
	if !paint.MayContainRTL(t.row(row)) {
		return paint.LineLayout{}, false
	}
	styled, ok := t.rowStyledRuns(row)
	if !ok {
		return paint.LineLayout{}, false
	}

	var layout paint.LineLayout
	if len(styled) == 1 && !styled[0].Box {
		face := paint.CachedFontFace(styled[0].Style, styled[0].FontSource, nil)
		layout = paint.LineVisualLayout(face.Fonts, styled[0].Runes, styled[0].TextSize, styled[0].Style)
	} else {
		layout = paint.StyledLineVisualLayout(styled)
	}
	if !layout.HasRTL() {
		return paint.LineLayout{}, false
	}
	return layout, true
}

// rowVisualHeight returns the row's line height: the tallest of its segments'
// measured heights. Text segments are measured by their own style/size; non-text
// segments (inline image/link box) use the same cached visual height that supplies
// their box width, so a row whose tallest element is a non-text segment reports the
// full row height — and rtlCaretX draws the caret at that height rather than the
// text height (pre-rebrand issue #47). This matches the row height cached in RowGeometry.
func (t *RichText) rowVisualHeight(row int) float32 {
	bound := t.rowBoundary(row)
	if bound == nil {
		return 0
	}
	var h float32
	for _, seg := range bound.segments {
		var m float32
		if text, ok := seg.(*TextSegment); ok {
			m = t.textSegmentHeight(text)
		} else {
			m = t.cachedSegmentVisual(seg, 0).MinSize().Height
		}
		if m > h {
			h = m
		}
	}
	return h
}

// measureText returns the rendered size and baseline of txt. Measurement is
// memoised per visual object by canvas.Text itself (pre-rebrand tracker #923, generalising the
// widget-local memo pre-rebrand tracker #896 introduced here), so an unchanged row reusing its
// cached visual re-measures nothing on a keystroke — only the edited segment's
// object consults the string-keyed global metrics cache.
func (t *RichText) measureText(txt *canvas.Text) (geo.Size, float32) {
	return txt.MinSizeWithBaseline()
}

// rtHeightKey identifies the inputs that determine a text run's line height:
// its size and the style fields that select the face (weight/italic/monospace/
// symbol) plus the line-height scale and any custom font source. Letter spacing
// affects width only, and the language/script only matters for non-ASCII content
// (which is never cached).
type rtHeightKey struct {
	size            float32
	weight          int // -1 when Style.Weight is nil
	lineHeightScale float32
	bold            bool
	italic          bool
	monospace       bool
	symbol          bool
	source          bellui.Resource // nil for the theme font
}

// textSegmentHeight measures a text segment's line height, memoising the result
// by (size, style) for ASCII runs (pre-rebrand tracker #313). The line height of ASCII text is the
// primary face's metric — independent of which characters appear — so one
// measurement serves every same-style ASCII row instead of re-shaping each.
// Non-ASCII runs may shape with a fallback face whose metrics differ, so they are
// always measured fresh (preserving correct variable-height layout, pre-rebrand tracker #158).
func (t *RichText) textSegmentHeight(text *TextSegment) float32 {
	size := text.size()
	style := text.Style.TextStyle
	source := text.Style.FontSource
	if !isASCII(text.Text) {
		h, _ := bellui.MeasureTextFull(text.Text, size, style, source)
		return h.Height
	}
	weight := -1
	if style.Weight != nil {
		weight = int(*style.Weight)
	}
	key := rtHeightKey{
		size:            size,
		weight:          weight,
		lineHeightScale: style.ResolvedLineHeightScale(),
		bold:            style.Bold,
		italic:          style.Italic,
		monospace:       style.Monospace,
		symbol:          style.Symbol,
		source:          source,
	}
	if h, ok := t.styleHeightCache[key]; ok {
		return h
	}
	m, _ := bellui.MeasureTextFull(text.Text, size, style, source)
	h := m.Height
	if t.styleHeightCache == nil {
		t.styleHeightCache = make(map[rtHeightKey]float32)
	}
	t.styleHeightCache[key] = h
	return h
}

// styleNominalHeight returns the primary-face line height for a (size, style),
// measured from a constant ASCII glyph and memoised by the same key as
// textSegmentHeight. It is the shaping-free height estimate used by the block-lazy
// build (pre-rebrand tracker #317): exact heights are deferred to ensureBlockMeasured, so estimating
// from an ASCII proxy avoids shaping a block's (possibly non-ASCII) text up front
// while still seeding the same cache textSegmentHeight reads from.
func (t *RichText) styleNominalHeight(size float32, style bellui.TextStyle, source bellui.Resource) float32 {
	weight := -1
	if style.Weight != nil {
		weight = int(*style.Weight)
	}
	key := rtHeightKey{
		size:            size,
		weight:          weight,
		lineHeightScale: style.ResolvedLineHeightScale(),
		bold:            style.Bold,
		italic:          style.Italic,
		monospace:       style.Monospace,
		symbol:          style.Symbol,
		source:          source,
	}
	if h, ok := t.styleHeightCache[key]; ok {
		return h
	}
	m, _ := bellui.MeasureTextFull("M", size, style, source)
	h := m.Height
	if t.styleHeightCache == nil {
		t.styleHeightCache = make(map[rtHeightKey]float32)
	}
	t.styleHeightCache[key] = h
	return h
}

// isASCII reports whether s is entirely ASCII (every byte < 0x80), i.e. it cannot
// trigger font fallback. Cheaper than utf8 decoding — a single byte scan.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// rowWidth returns the row's natural rendered width, measuring each segment with
// its own style and size, so a multi-segment row whose segments differ in size
// or weight gets an exact width rather than a single-style approximation. Rows
// with RTL content reuse the bidi visual layout's total advance (the same width,
// already computed). This underpins alignment offsets; see pre-rebrand issue #42.
func (t *RichText) rowWidth(row int, textSize float32) float32 {
	if layout, ok := t.rowVisualLayout(row); ok {
		return layout.Width
	}
	bound := t.rowBoundary(row)
	if bound == nil {
		return bellui.MeasureText(string(t.row(row)), textSize, bellui.TextStyle{}).Width
	}
	nSegs := len(bound.segments)
	var width float32
	for i, seg := range bound.segments {
		text, ok := seg.(*TextSegment)
		if !ok {
			width += t.cachedSegmentVisual(seg, 0).MinSize().Width
			continue
		}
		runes := []rune(text.Text)
		start := 0
		end := len(runes)
		if i == 0 {
			start = bound.begin
		}
		if i == nSegs-1 && bound.end <= len(runes) && (bound.end > 0 || bound.begin >= bound.end) {
			end = bound.end
		}
		if start > end {
			start = end
		}
		measure := runes[start:end]
		if concealed(seg) {
			measure = []rune(strings.Repeat(passwordChar, len(measure)))
		}
		m, _ := bellui.MeasureTextFull(string(measure), text.size(), text.Style.TextStyle, text.Style.FontSource)
		width += m.Width
	}
	return width
}

// rtlCaretX returns the visual x and height of logical column col within a row
// that contains right-to-left runs. It returns ok=false for concealed (password)
// text, rows with non-text segments, or pure-LTR content, so callers stay on the
// legacy logical-prefix measurement path in those cases. Both single-segment
// rows (e.g. Entry) and multi-segment RichText rows are handled.
// downstream selects the caret affinity at a direction boundary: true keeps the
// forward (downstream) edge, false the upstream edge of the preceding rune.
func (t *RichText) rtlCaretX(row, col int, downstream bool) (x, height float32, ok bool) {
	layout, ok := t.rowVisualLayout(row)
	if !ok {
		return 0, 0, false
	}
	aff := paint.AffinityDownstream
	if !downstream {
		aff = paint.AffinityUpstream
	}
	return layout.CaretXAt(col, aff), t.rowVisualHeight(row), true
}

// rtlSelectionSpans returns the disjoint visual x-intervals covering the logical
// column range [startCol,endCol) on a row with RTL content, in the same content
// coordinates as lineSizeToColumn's width (i.e. already offset by
// innerPad-inset). It returns ok=false in the same cases as rtlCaretX, so callers
// fall back to a single LTR selection rectangle.
func (t *RichText) rtlSelectionSpans(row, startCol, endCol int, innerPad float32) ([][2]float32, bool) {
	layout, ok := t.rowVisualLayout(row)
	if !ok {
		return nil, false
	}

	spans := layout.SelectionSpans(startCol, endCol)
	off := innerPad - t.inset.Width
	for i := range spans {
		spans[i][0] += off
		spans[i][1] += off
	}
	return spans, true
}

// visualColNeighbor maps a within-row visual arrow step to a logical column for a
// row with RTL content. dir > 0 moves visually right, dir < 0 left. It returns
// moved=true with the neighbour column when a step exists; atEdge=true when the
// cursor is already at the row's visual edge (the caller should cross to an
// adjacent row); and both false for non-RTL rows, where the caller keeps the
// plain logical column step.
func (t *RichText) visualColNeighbor(row, col, dir int) (neighbor int, moved, atEdge bool) {
	layout, ok := t.rowVisualLayout(row)
	if !ok {
		return 0, false, false
	}

	n := layout.VisualNeighbor(col, dir)
	if n < 0 {
		return 0, false, true // at the visual edge of the line
	}
	return n, true, false
}

// Row returns the characters in the row specified.
// The row parameter should be between 0 and t.Rows()-1.
func (t *RichText) row(row int) []rune {
	if row < 0 || row >= t.rows() {
		return nil
	}
	bp := t.rowBoundary(row)
	if bp == nil {
		return nil
	}
	bound := *bp
	var ret []rune
	nSegs := len(bound.segments)
	for i, seg := range bound.segments {
		if text, ok := seg.(*TextSegment); ok {
			// In the lazy single-segment path the buffer holds the whole segment
			// with absolute begin/end offsets, so fetch just this row's runes
			// instead of re-converting the entire document per row (pre-rebrand tracker #375).
			if t.lazy && t.buf != nil && nSegs == 1 && text == t.Segments[0] {
				n := t.buf.Len()
				start := bound.begin
				end := n
				if bound.end <= n && (bound.end > 0 || bound.begin >= bound.end) {
					end = bound.end
				}
				if start > end {
					start = end
				}
				ret = t.buf.AppendRunes(ret, start, end)
				continue
			}
			runes := []rune(text.Text)
			start := 0
			end := len(runes)
			if i == 0 {
				start = bound.begin
			}
			if i == nSegs-1 && bound.end <= len(runes) && (bound.end > 0 || bound.begin >= bound.end) {
				end = bound.end
			}
			if start > end {
				start = end
			}
			ret = append(ret, runes[start:end]...)
		}
	}
	return ret
}

// rowSelectedText returns the logical-order text of row covered by the column
// range [startCol,endCol), where startCol/endCol index the row's text-only runes
// (the same coordinates the selection model uses). Unlike row(), it also emits a
// non-text segment's Textual() content when that segment's opaque box falls in the
// selection — using the same owner-rune test SelectionSpans uses to fill the box's
// rectangle — so the copied string matches the highlighted rectangles for both LTR
// and RTL rows (pre-rebrand issue #46). A box's owner is the text rune it trails (counted-1);
// a box leading the row (owner == -1) is never selected, matching the layout.
func (t *RichText) rowSelectedText(row, startCol, endCol int) string {
	bound := t.rowBoundary(row)
	if bound == nil {
		return ""
	}
	nSegs := len(bound.segments)
	var buf strings.Builder
	counted := 0 // running text-only rune offset within the row
	for i, seg := range bound.segments {
		text, ok := seg.(*TextSegment)
		if !ok {
			if owner := counted - 1; owner >= startCol && owner < endCol {
				buf.WriteString(seg.Textual())
			}
			continue
		}
		runes := []rune(text.Text)
		start := 0
		end := len(runes)
		if i == 0 {
			start = bound.begin
		}
		if i == nSegs-1 && bound.end <= len(runes) && (bound.end > 0 || bound.begin >= bound.end) {
			end = bound.end
		}
		if start > end {
			start = end
		}
		rowRunes := runes[start:end]
		lo := startCol - counted
		hi := endCol - counted
		if lo < 0 {
			lo = 0
		}
		if hi > len(rowRunes) {
			hi = len(rowRunes)
		}
		if lo < hi {
			buf.WriteString(string(rowRunes[lo:hi]))
		}
		counted += len(rowRunes)
	}
	return buf.String()
}

// isBlockRow reports whether row is a standalone block row: it has no text
// columns but does contain a non-text segment (an image/table/separator, or
// leading inline links with no following text). Such a row has no caret column,
// so selection treats it all-or-nothing (pre-rebrand issues #50, pre-rebrand tracker #53). An ordinary blank text
// row (no non-text segment) is not a block row.
func (t *RichText) isBlockRow(row int) bool {
	if t.rowLength(row) != 0 {
		return false
	}
	bound := t.rowBoundary(row)
	if bound == nil {
		return false
	}
	for _, seg := range bound.segments {
		if _, ok := seg.(*TextSegment); !ok {
			return true
		}
	}
	return false
}

// rowBlockText returns the concatenated Textual() of a row's non-text segments,
// used for a standalone block row (a row with zero text columns whose content is
// one or more non-text segments — an image/table/separator, or leading inline
// links with no following text). Such a row has no caret column for the text-based
// selection model, so SelectedText treats it all-or-nothing and splices this text
// when the selection reaches the row (pre-rebrand issues #50, pre-rebrand tracker #53). It returns "" for an
// ordinary empty text row (a blank line), leaving that behaviour unchanged.
func (t *RichText) rowBlockText(row int) string {
	bound := t.rowBoundary(row)
	if bound == nil {
		return ""
	}
	var buf strings.Builder
	for _, seg := range bound.segments {
		if _, ok := seg.(*TextSegment); ok {
			continue
		}
		buf.WriteString(seg.Textual())
	}
	return buf.String()
}

// RowGeometry returns the Y position and height of the given row,
// as computed during the last Layout pass. Returns (0, charMinSize)
// if the row is out of range or layout hasn't run yet.
func (t *RichText) RowGeometry(row int) (yPos, height float32) {
	if t.blockLazy {
		return t.rowGeometryBlockLazy(row)
	}
	if t.varHeight {
		return t.rowGeometryVar(row)
	}
	if t.lazy {
		// Uniform stride: Y is analytic (no dense rowYPos array). Folded rows
		// collapse to zero height and report the fold point's Y.
		s := t.stride()
		y := t.contentTopOffset() + float32(row-t.hiddenBefore(row))*s
		if row >= 0 && row < t.rows() && t.rowHidden(row) {
			return y, 0
		}
		return y, s
	}
	if row >= 0 && row < len(t.rowYPos) && row < len(t.rowHeight) {
		return t.rowYPos[row], t.rowHeight[row]
	}
	// Fallback to uniform row height.
	return 0, t.charMinSize(false, bellui.TextStyle{}, t.Theme().Size(theme.SizeNameText)).Height
}

// rowAtY returns the visual row whose vertical band contains the content-coordinate
// y, clamped to [0, rows()-1], or -1 when there are no rows / no geometry yet.
//
// For the lazy model (pre-rebrand tracker #157) the uniform-stride geometry is invertible, so this is
// O(1) (with folds resolved via visibleIndexToRow) instead of the O(rows) scan a
// click would otherwise cost on a very large document (pre-rebrand tracker #160). For the dense model
// it scans the cached per-row geometry, matching the previous hit-test behaviour.
func (t *RichText) rowAtY(y float32) int {
	n := t.rows()
	if n == 0 {
		return -1
	}
	if t.blockLazy {
		return t.rowAtYBlockLazy(y)
	}
	if t.varHeight {
		return t.rowAtYVar(y)
	}
	if t.lazy {
		s := t.stride()
		if s <= 0 {
			return 0
		}
		// Folded rows collapse to zero height, so y maps onto the visible-row axis.
		k := int((y - t.contentTopOffset()) / s)
		if k < 0 {
			return 0
		}
		if vis := t.visibleRowCount(); k >= vis {
			k = vis - 1
		}
		if k < 0 {
			return 0
		}
		row := t.visibleIndexToRow(k)
		if row >= n {
			row = n - 1
		}
		return row
	}
	for r := 0; r < n; r++ {
		ry, rh := t.RowGeometry(r)
		if ry == 0 && rh == 0 {
			break // no cached geometry yet
		}
		if y < ry+rh {
			return r
		}
	}
	return -1
}

// RowBoundary returns the boundary of the row specified.
// The row parameter should be between 0 and t.Rows()-1.
func (t *RichText) rowBoundary(row int) *rowBoundary {
	// Build the row model first: t.rows() below would otherwise run updateRowBounds
	// mid-call and flip lazy/dense after the branch was chosen.
	if t.rowBounds == nil && t.lineRowCount == nil && t.blocks == nil {
		t.updateRowBounds()
	}
	if t.lazy {
		return t.lazyRowBoundary(row)
	}
	if t.blockLazy {
		return t.blockLazyRowBoundary(row)
	}
	if row < 0 || row >= len(t.rowBounds) {
		return nil
	}
	return &t.rowBounds[row]
}

// RowLength returns the number of visible characters in the row specified.
// The row parameter should be between 0 and t.Rows()-1.
func (t *RichText) rowLength(row int) int {
	return len(t.row(row))
}

// rows returns the number of text rows in this text entry.
// The entry may be longer than required to show this amount of content.
func (t *RichText) rows() int {
	if t.rowBounds == nil && t.lineRowCount == nil && t.blocks == nil { // if the widget API is used before it is shown
		t.updateRowBounds()
	}
	if t.lazy {
		return t.lazyRowCount()
	}
	if t.blockLazy {
		return t.blockLazyRowCount()
	}
	return len(t.rowBounds)
}

// contentFingerprint computes a cheap fingerprint from segment count and total text length.
// Used by SyncState to detect external modifications to segment content.
func (t *RichText) contentFingerprint() uint64 {
	fp := uint64(len(t.Segments))
	for _, seg := range t.Segments {
		if ts, ok := seg.(*TextSegment); ok {
			fp = fp*31 + uint64(len(ts.Text))
		} else {
			fp = fp*31 + 1 // non-text segment contributes a fixed value
		}
	}
	return fp
}

// updateRowBounds updates the row bounds used to render properly the text widget.
// updateRowBounds should be invoked every time a segment Text, widget Wrapping or size changes.
// rowBoundsCacheValid reports whether the current row model (lazy, block-lazy or
// dense) is still valid for the given width, so updateRowBounds can skip the
// rebuild. The cache key is (version, width, truncation, wrapping); a valid key
// still requires the matching model's structures to be populated.
func (t *RichText) rowBoundsCacheValid(width float32, lazy bool) bool {
	if t.rowBoundsComputed != t.rowBoundsVersion ||
		t.rowBoundsTruncation != t.Truncation || t.rowBoundsWrapping != t.Wrapping {
		return false
	}
	// The lazy wrap-off row model is width-independent — one row per logical
	// line, nothing is wrap-measured — so a width-only change must not force
	// an O(lines) rebuild. Without this, typing into the document's widest
	// line grows the scroll content width every keystroke and each one paid a
	// full rebuild (pre-rebrand tracker #410).
	if t.rowBoundsWidth != width && !(lazy && t.Wrapping == bellui.TextWrapOff) {
		return false
	}
	switch {
	case lazy:
		return len(t.lineRowCount) > 0
	case t.blockLazy:
		return len(t.blocks) > 0
	default:
		return len(t.rowBounds) > 0
	}
}

// updateRowBounds (re)builds the active row model when stale, then refreshes the
// content fingerprint so a subsequent SyncState (which bumps rowBoundsVersion when
// the fingerprint changed) does not redundantly rebuild the same model. Without
// this, every keystroke built the lazy model twice: once here via updateText and
// again when SyncState saw the length change against a stale lastContentFP.
func (t *RichText) updateRowBounds() {
	if t.updateRowBoundsInner() {
		// Stamp the fingerprint only after an actual rebuild. A subsequent SyncState
		// compares this fingerprint and rebuilds when it differs, so stamping here
		// stops the redundant second build of the same content each keystroke. We must
		// NOT stamp on a cache-valid early return: a change signalled purely by a
		// direct segment mutation that bumps no rowBoundsVersion (e.g. SetPlaceHolder)
		// leaves the cache "valid" but stale, and stamping would swallow the signal
		// SyncState relies on to trigger the real rebuild.
		t.lastContentFP = t.contentFingerprint()
	}
}

// updateRowBoundsInner (re)builds the active row model when the cache is stale and
// reports whether a rebuild happened (false on a cache-valid early return).
func (t *RichText) updateRowBoundsInner() bool {
	// Skip if content, width, truncation, and wrapping haven't changed since last computation.
	width := t.Size().Width
	// Only single-segment content takes the lazy (uniform-stride) build; multi-
	// segment varHeight content is built densely here and flagged afterwards.
	lazy := t.canVirtualizeSingle()
	if t.rowBoundsCacheValid(width, lazy) {
		return false
	}

	// Lazy/windowed measurement path (pre-rebrand tracker #157): single-segment, scrollable, truncation
	// off — shape only the lines spanning the viewport, not every line up front.
	if lazy {
		t.denseIncr = nil
		t.updateRowBoundsBlockLazyReset()
		t.updateRowBoundsLazy(width)
		return true
	}
	// Windowed multi-segment build (pre-rebrand tracker #317): shape blocks on demand rather than every
	// row up front. Evaluated only on a real rebuild (cache invalid); blockLazyEligible
	// builds t.blocks as a side effect, reused by updateRowBoundsBlockLazy.
	if t.blockLazyEligible(width) {
		t.denseIncr = nil
		t.lazy = false
		t.lineRowCount = nil
		t.updateRowBoundsBlockLazy(width)
		t.buildRowToLine()
		t.applyHiddenRanges()
		return true
	}
	// A single line of plain text, neither wrapped nor truncated (every Label
	// holding one line) has exactly one row: the dense build below would split
	// it at newlines, find none, and return that row unmeasured. Building the
	// row directly skips the build's machinery, which was about a fifth of a
	// Label.SetText (#492).
	if seg, ok := t.singleSegment(); ok && t.Wrapping == bellui.TextWrapOff &&
		t.Truncation == bellui.TextTruncateOff && strings.IndexByte(seg.Text, '\n') < 0 {
		t.lazy = false
		t.lineRowCount = nil
		t.updateRowBoundsBlockLazyReset()
		t.rowBounds = []rowBoundary{{segments: []RichTextSegment{seg}, end: utf8.RuneCountInString(seg.Text)}}
		t.denseIncr = nil
		t.rowHeightCacheVer = -1
		t.rowBoundsCacheStamp(width)
		return true
	}
	// Edit-incremental dense rebuild (pre-rebrand tracker #921): when only a contiguous segment
	// range changed since the last dense build, re-shape just that range and
	// splice, leaving the untouched rows (and their caches) in place.
	if t.denseIncrRebuild(width) {
		return true
	}
	// Dense path: drop any stale lazy/block-lazy model so the accessors take the
	// rowBounds branch. The content buffer (t.buf) is kept: it is a content cache
	// keyed by lazyStr, reusable if the widget returns to the lazy model.
	t.lazy = false
	t.lineRowCount = nil
	t.updateRowBoundsBlockLazyReset()

	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	fitSize := t.Size()
	if t.scr != nil {
		fitSize = t.scr.Content.MinSize()
	}
	fitSize.Height -= (innerPadding + t.inset.Height) * 2

	// The dense iterate lives in denseBuildCtx.iterate (richtext_incremental.go)
	// so the edit-incremental rebuild (pre-rebrand tracker #921) can run the identical logic over a
	// segment subrange; checkpoints are recorded when the document qualifies.
	ctx := &denseBuildCtx{
		t: t, th: th, innerPadding: innerPadding,
		maxWidth: t.Size().Width - 2*innerPadding + 2*t.inset.Width,
		fitSize:  fitSize,
		flat:     true,
	}
	ctx.wrapWidth = ctx.maxWidth
	record := t.denseIncrEligible()
	var cps []denseCheckpoint
	if record {
		cps = ctx.iterate(t.Segments, make([]denseCheckpoint, 0, len(t.Segments)), true)
	} else {
		ctx.iterate(t.Segments, nil, false)
	}
	t.rowBounds = ctx.bounds
	if record {
		t.denseIncrRecord(ctx, cps, width)
	} else {
		t.denseIncr = nil
	}
	// Fresh bounds: force the per-row height cache to rebuild so it never aliases
	// stale heights from a previous layout (pre-rebrand tracker #158 Step 1).
	t.rowHeightCacheVer = -1
	// Decide whether this multi-segment document is large enough for the variable-
	// height virtual window (pre-rebrand tracker #158 Step 2). The dense bounds are kept either way; the
	// flag only changes how geometry/rendering window the rows. The height prefix is
	// marked dirty so it rebuilds against the new bounds. rowBoundsCacheStamp also
	// rebuilds the visual-row -> logical-line map and re-resolves any logical-line
	// folds against it (pre-rebrand tracker #145): when wrapping changes the row layout, a fold
	// requested over logical lines must follow its wrapped rows.
	t.rowBoundsCacheStamp(t.Size().Width)
	return true
}

// RichTextBlock is an extension of a text segment that contains other segments
//
// Since: 1.0
type RichTextBlock interface {
	Segments() []RichTextSegment
}

// Renderer
type textRenderer struct {
	widget.BaseRenderer
	obj      *RichText
	origObjs []core.CanvasObject // original 1-per-row objects (for Layout)
}

// layoutOneRow positions the objects of a single row (bound) starting at object
// index oi and vertical top yTop, returning how many objects it consumed and the
// row's content height (the vertical advance, excluding inter-row line spacing).
// Inline segments share the line horizontally (laid out via layoutRow); a block
// segment (non-text, last in its row) is moved and resized to fill the line width
// at its measured/HeightForWidth height. Shared by the dense Layout and the
// variable-height window layout (pre-rebrand tracker #158) so the two cannot diverge.
func (r *textRenderer) layoutOneRow(bound rowBoundary, objs []core.CanvasObject, oi int,
	yTop, xInset, lineWidth, lineSpacing float32,
) (consumed int, height float32) {
	left := xInset
	yPos := yTop
	var rowItems []core.CanvasObject

	// The row's alignment comes from its leading segment; set it up front so a
	// pre-block inline flush uses the row's own alignment.
	rowAlign := bellui.TextAlignLeading
	if text, ok := bound.segments[0].(*TextSegment); ok {
		rowAlign = text.Style.Alignment
	} else if link, ok := bound.segments[0].(*HyperlinkSegment); ok {
		rowAlign = link.Alignment
	}

	i := oi
	for segI := range bound.segments {
		if i == len(objs) {
			break // Refresh may not have created all objects for all rows yet...
		}
		inline := segI < len(bound.segments)-1
		obj := objs[i]
		obj.Show() // a row may have been hidden by a previous fold, now revealed
		i++
		_, isText := obj.(*canvas.Text)
		if !isText && !inline {
			if len(rowItems) != 0 {
				width, _ := r.layoutRow(rowItems, rowAlign, left, yPos, lineWidth)
				left += width
				rowItems = nil
			}
			h := obj.MinSize().Height
			if hfw, ok := obj.(interface{ HeightForWidth(float32) float32 }); ok {
				h = hfw.HeightForWidth(lineWidth)
			}

			obj.Move(geo.NewPos(left, yPos))
			obj.Resize(geo.NewSize(lineWidth, h))
			yPos += h
			left = xInset
			continue
		}
		rowItems = append(rowItems, obj)
		if inline {
			continue
		}

		leftPad := float32(0)
		if text, ok := bound.segments[0].(*TextSegment); ok && text.Style.isBlockquote() {
			leftPad = lineSpacing * 4
		}
		_, y := r.layoutRow(rowItems, rowAlign, left+leftPad, yPos, lineWidth-leftPad)
		yPos += y
		rowItems = nil
	}
	return i - oi, yPos - yTop
}

func (r *textRenderer) Layout(size geo.Size) {
	if r.obj.canVirtualize() {
		// A scrollable virtualised RichText drives its window from the scroll
		// viewport: resize the scroll container, then (re)compute the window for
		// the current offset/height before positioning it (pre-rebrand tracker #158). On a plain
		// resize this picks up the new viewport without an external driver.
		if r.obj.scr != nil {
			r.obj.scr.Resize(size)
			r.obj.updateVirtualViewport()
		}
		r.layoutVirtual(size)
		return
	}
	if r.obj.isSelectable() && r.obj.selection != nil {
		r.obj.selection.Resize(size)
	}
	th := r.obj.Theme()
	bounds := r.obj.rowBounds
	// Always use origObjs for layout (1 object per row) when highlighting
	// is active. The renderer's Objects may have been replaced with fragments.
	objs := r.origObjs
	if len(objs) == 0 {
		objs = r.Objects()
	}
	if r.obj.scr != nil {
		r.obj.scr.Resize(size)
		if len(r.origObjs) == 0 {
			objs = r.obj.scr.Content.(*bellui.Container).Objects[1].(*bellui.Container).Objects
		}
	}

	// Accessing theme here is slow, so we cache the value
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)

	xInset := innerPadding - r.obj.inset.Width
	// Compute and apply gutter offset for line numbers.
	if r.obj.ShowLineNumbers && r.obj.RowHighlighter != nil {
		textSize := th.Size(theme.SizeNameText)
		digits := len(strconv.Itoa(len(r.obj.rowBounds)))
		if digits < 3 {
			digits = 3
		}
		r.obj.GutterWidth = bellui.MeasureText(strings.Repeat("0", digits)+" ", textSize, bellui.TextStyle{Monospace: true}).Width
		xInset += r.obj.GutterWidth
	} else {
		r.obj.GutterWidth = 0
	}
	left := xInset
	yPos := innerPadding - r.obj.inset.Height
	lineWidth := size.Width - left*2

	// Row geometry caches for selection hit-testing, reused when the row count
	// is unchanged: every entry is written below, and a Label lays out again on
	// every text change (#492).
	if len(r.obj.rowYPos) != len(bounds) {
		r.obj.rowYPos = make([]float32, len(bounds))
	}
	if len(r.obj.rowHeight) != len(bounds) {
		r.obj.rowHeight = make([]float32, len(bounds))
	}

	i := 0
	for row, bound := range bounds {
		rowStartY := yPos
		rowObjStart := i

		// Folded rows (pre-rebrand tracker #141) lay out as absent: consume their objects, hide them,
		// and contribute zero height so following rows shift up by the fold.
		if r.obj.rowHidden(row) {
			for range bound.segments {
				if i == len(objs) {
					break
				}
				objs[i].Hide()
				i++
			}
			r.obj.rowYPos[row] = rowStartY
			r.obj.rowHeight[row] = 0
			continue
		}

		consumed, rh := r.layoutOneRow(bound, objs, i, yPos, xInset, lineWidth, lineSpacing)
		i += consumed
		yPos += rh

		// Cache row geometry for selection coordinate mapping.
		r.obj.rowYPos[row] = rowStartY
		r.obj.rowHeight[row] = yPos - rowStartY

		// Warm the per-row height cache for non-volatile rows so a later
		// MinSize/calculateMin at the same width serves this row without
		// re-measuring (pre-rebrand tracker #158 Step 1). Volatile rows (images/tables) are left
		// unmeasured and always re-measured fresh.
		if consumed := i - rowObjStart; !segmentsVolatile(objs, rowObjStart, consumed) {
			r.obj.ensureRowHeightCache(lineWidth, th)
			if row >= 0 && row < len(r.obj.rowHeightCache) {
				r.obj.rowHeightCache[row] = r.obj.rowHeight[row]
			}
		}

		lastSeg := bound.segments[len(bound.segments)-1]
		if !lastSeg.Inline() && row < len(bounds)-1 && bounds[row+1].segments[0] != lastSeg { // ignore wrapped lines etc
			yPos += lineSpacing
		}
	}
}

// MinSize calculates the minimum size of a rich text widget.
// This is based on the contained text with a standard amount of padding added.
func (r *textRenderer) MinSize() geo.Size {
	if r.obj.canVirtualize() {
		return r.minVirtual()
	}

	th := r.obj.Theme()
	textSize := th.Size(theme.SizeNameText)
	innerPad := th.Size(theme.SizeNameInnerPadding)

	bounds := r.obj.rowBounds
	wrap := r.obj.Wrapping
	trunc := r.obj.Truncation
	scroll := r.obj.Scroll
	// Use origObjs for size calculation (1 per row, no fragments).
	objs := r.origObjs
	if len(objs) == 0 {
		objs = r.Objects()
	}
	if r.obj.scr != nil {
		objs = r.obj.scr.Content.(*bellui.Container).Objects[1].(*bellui.Container).Objects
	}

	charMinSize := r.obj.charMinSize(false, bellui.TextStyle{}, textSize)
	min := r.calculateMin(bounds, wrap, objs, charMinSize, th)
	if r.obj.scr != nil {
		r.obj.prop.SetMinSize(min)
	}

	if trunc != bellui.TextTruncateOff && r.obj.Scroll == widget.ScrollNone {
		minBounds := charMinSize
		if wrap == bellui.TextWrapOff {
			minBounds.Height = min.Height
		} else {
			minBounds = minBounds.Add(geo.NewSquareSize(innerPad * 2).Subtract(r.obj.inset).Subtract(r.obj.inset))
		}
		if trunc == bellui.TextTruncateClip {
			return minBounds
		} else if trunc == bellui.TextTruncateEllipsis {
			ellipsisSize := bellui.MeasureText("…", th.Size(theme.SizeNameText), bellui.TextStyle{})
			return minBounds.AddWidthHeight(ellipsisSize.Width, 0)
		}
	}

	switch scroll {
	case widget.ScrollBoth:
		return geo.NewSize(32, 32)
	case widget.ScrollHorizontalOnly:
		return geo.NewSize(32, min.Height)
	case widget.ScrollVerticalOnly:
		return geo.NewSize(min.Width, 32)
	default:
		return min
	}
}

// rowHeightUnmeasured is the sentinel stored in rowHeightCache for a row that
// has not been measured yet (NaN never equals a real measured height).
var rowHeightUnmeasured = float32(math.NaN())

// segmentsVolatile reports whether any object backing a row's segments has a
// size that can change after first measurement — inline/block images (whose
// MinSize firms up only once the canvas exists) and HeightForWidth widgets
// (tables, code blocks). Such rows are never height-cached so a late resize is
// never served stale; they are measured fresh on every pass (pre-rebrand tracker #158 Step 1).
func segmentsVolatile(objs []core.CanvasObject, start, count int) bool {
	for k := start; k < start+count && k < len(objs); k++ {
		switch objs[k].(type) {
		case *richImage:
			return true
		}
		if _, ok := objs[k].(interface{ HeightForWidth(float32) float32 }); ok {
			return true
		}
	}
	return false
}

// ensureRowHeightCache (re)allocates the per-row height cache, filled with the
// unmeasured sentinel, whenever its validity key changes: the rowBounds version,
// the measurement width, the relevant theme sizes, or the row count. Keying on
// the three theme sizes (rather than the theme pointer) is robust against a theme
// that mutates its sizes in place.
func (t *RichText) ensureRowHeightCache(width float32, th bellui.Theme) {
	innerPad := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)
	textSize := th.Size(theme.SizeNameText)
	if len(t.rowHeightCache) == len(t.rowBounds) &&
		t.rowHeightCacheVer == t.rowBoundsVersion &&
		t.rowHeightCacheWidth == width &&
		t.rowHeightCacheInnerPad == innerPad &&
		t.rowHeightCacheLineSpacing == lineSpacing &&
		t.rowHeightCacheTextSize == textSize {
		return
	}
	if cap(t.rowHeightCache) >= len(t.rowBounds) {
		t.rowHeightCache = t.rowHeightCache[:len(t.rowBounds)]
	} else {
		t.rowHeightCache = make([]float32, len(t.rowBounds))
	}
	for i := range t.rowHeightCache {
		t.rowHeightCache[i] = rowHeightUnmeasured
	}
	t.styleHeightCache = nil // content/theme changed: drop memoised per-style heights (pre-rebrand tracker #313)
	t.rowHeightCacheVer = t.rowBoundsVersion
	t.rowHeightCacheWidth = width
	t.rowHeightCacheInnerPad = innerPad
	t.rowHeightCacheLineSpacing = lineSpacing
	t.rowHeightCacheTextSize = textSize
}

// measuredRowHeight returns the height of visual row `row` — the max over its
// segments' measured heights — serving a cached value on a hit and measuring +
// storing on a miss. It measures exactly as the dense calculateMin does (pre-rebrand tracker #158
// Step 1), so it is behaviour-preserving. Callers must short-circuit folded rows
// (height 0) before calling. `objStart` is the index of the row's first object in
// `objs`; `consumed` is the number of objects the row actually owns, honouring the
// existing "Refresh may not have created all objects yet" partial-row early break.
// Volatile rows (images/tables) are always measured fresh and never stored.
func (r *textRenderer) measuredRowHeight(row int, bound rowBoundary, objs []core.CanvasObject,
	objStart int, width float32, th bellui.Theme,
) (height float32, consumed int) {
	r.obj.ensureRowHeightCache(width, th)

	count := len(bound.segments)
	if objStart+count > len(objs) {
		count = len(objs) - objStart
	}
	if count < 0 {
		count = 0
	}
	volatile := segmentsVolatile(objs, objStart, count)

	if !volatile && row >= 0 && row < len(r.obj.rowHeightCache) && !math.IsNaN(float64(r.obj.rowHeightCache[row])) {
		return r.obj.rowHeightCache[row], count
	}

	var h float32
	for k := objStart; k < objStart+count; k++ {
		obj := objs[k]
		if txt, ok := obj.(*canvas.Text); ok {
			min, _ := r.obj.measureText(txt)
			h = geo.Max(h, min.Height)
			continue
		}
		min := obj.MinSize()
		if hfw, ok := obj.(interface{ HeightForWidth(float32) float32 }); ok && width > 0 {
			h = geo.Max(h, hfw.HeightForWidth(width))
		} else {
			h = geo.Max(h, min.Height)
		}
	}
	if !volatile && row >= 0 && row < len(r.obj.rowHeightCache) {
		r.obj.rowHeightCache[row] = h
	}
	return h, count
}

func (r *textRenderer) calculateMin(bounds []rowBoundary, wrap bellui.TextWrap, objs []core.CanvasObject,
	charMinSize geo.Size, th bellui.Theme,
) geo.Size {
	height := float32(0)
	width := float32(0)
	rowWidth := float32(0)
	trunc := r.obj.Truncation
	innerPad := th.Size(theme.SizeNameInnerPadding)

	// Accessing the theme here is slow, so we cache the value
	lineSpacing := th.Size(theme.SizeNameLineSpacing)

	i := 0
	for row, bound := range bounds {
		// Folded rows (pre-rebrand tracker #141) contribute no height and no width to the content size.
		if r.obj.rowHidden(row) {
			for range bound.segments {
				if i == len(objs) {
					break
				}
				i++
			}
			continue
		}
		// Row height: cache-aware so a re-layout after the first pass does not
		// re-shape unchanged rows (pre-rebrand tracker #158 Step 1). Volatile rows (images, tables)
		// are always measured fresh inside the helper.
		hfwWidth := float32(0)
		if w := r.obj.Size().Width; w > 0 {
			hfwWidth = w - innerPad*2
		}
		rowHeight, consumed := r.measuredRowHeight(row, bound, objs, i, hfwWidth, th)

		// Width (wrap-off + truncation-off only) and async image-resize detection
		// still need a per-segment pass; both are cheap and rare in large documents.
		needWidth := wrap == bellui.TextWrapOff && trunc == bellui.TextTruncateOff
		for k := 0; k < consumed; k++ {
			obj := objs[i+k]
			if needWidth {
				if txt, ok := obj.(*canvas.Text); ok {
					s, _ := r.obj.measureText(txt)
					rowWidth += s.Width
				} else {
					rowWidth += obj.MinSize().Width
				}
			}
			if img, ok := obj.(*richImage); ok {
				if newMin := img.MinSize(); newMin != img.oldMin {
					img.oldMin = newMin

					min := r.calculateMin(bounds, wrap, objs, charMinSize, th)
					if r.obj.scr != nil {
						r.obj.prop.SetMinSize(min)
					}
					r.Refresh() // image size changed — full refresh needed (upstream pre-rebrand tracker #2991)
				}
			}
		}
		i += consumed

		if needWidth {
			width = geo.Max(width, rowWidth)
		}
		height += rowHeight
		rowWidth = 0

		lastSeg := bound.segments[len(bound.segments)-1]
		if !lastSeg.Inline() && row < len(bounds)-1 && bounds[row+1].segments[0] != lastSeg { // ignore wrapped lines etc
			height += lineSpacing
		}
	}

	if height == 0 {
		height = charMinSize.Height
	}
	return geo.NewSize(width, height).
		Add(geo.NewSquareSize(innerPad * 2).Subtract(r.obj.inset).Subtract(r.obj.inset))
}

// minVirtual returns the analytic MinSize for the uniform-height virtualised
// path: height is rows*stride (O(1)) and width comes from a cheap widest-row
// scan, so a huge document never forces shaping of every row. Mirrors the
// non-virtual size branches.
func (r *textRenderer) minVirtual() geo.Size {
	th := r.obj.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	stride := r.obj.stride()

	// Folded rows (pre-rebrand tracker #141) collapse to zero height, so the scroll extent is the
	// visible-row count, not the total row count.
	var height float32
	if r.obj.blockLazy {
		// Block-lazy: total content height is the block Y-prefix total (pre-rebrand tracker #317),
		// estimated for unmeasured blocks and refined as they are measured.
		r.obj.ensureBlockPrefix()
		height = r.obj.blockTopY[len(r.obj.blocks)]
	} else if r.obj.varHeight {
		// Variable-height: total content height is the prefix-sum total (pre-rebrand tracker #158).
		r.obj.ensureHeightPrefix()
		height = r.obj.rowYPrefix[len(r.obj.rowBounds)]
	} else {
		height = float32(r.obj.visibleRowCount()) * stride
	}
	if height == 0 {
		height = stride
	}
	min := geo.NewSize(r.virtualWidth(), height).
		Add(geo.NewSquareSize(innerPad * 2).Subtract(r.obj.inset).Subtract(r.obj.inset))
	if r.obj.scr != nil {
		r.obj.prop.SetMinSize(min)
	}

	switch r.obj.Scroll {
	case widget.ScrollBoth:
		return geo.NewSize(32, 32)
	case widget.ScrollHorizontalOnly:
		return geo.NewSize(32, min.Height)
	case widget.ScrollVerticalOnly:
		return geo.NewSize(min.Width, 32)
	default:
		return min
	}
}

// virtualWidth returns the widest row's width for minVirtual. It scans row
// bounds for the longest row by rune count (cheap integer compare, no shaping)
// and measures only that one row, caching the result per rowBoundsVersion.
func (r *textRenderer) virtualWidth() float32 {
	t := r.obj
	if t.virtWidthOK && t.virtWidthVer == t.rowBoundsVersion {
		return t.virtWidth
	}
	t.virtWidth = 0
	t.virtWidthVer = t.rowBoundsVersion
	t.virtWidthOK = true

	if t.lazy {
		t.virtWidth = t.lazyVirtualWidth()
		return t.virtWidth
	}

	if t.blockLazy {
		t.virtWidth = t.blockLazyVirtualWidth()
		return t.virtWidth
	}

	if t.varHeight {
		// Wrapped rows fit the content width, so (like the dense calculateMin)
		// the natural width is reported as 0; only wrap-off rows can exceed it and
		// need a widest-row scan (pre-rebrand tracker #158).
		if t.Wrapping != bellui.TextWrapOff {
			return 0
		}
		t.virtWidth = t.varHeightVirtualWidth()
		return t.virtWidth
	}

	seg, ok := t.singleSegment()
	if !ok {
		return 0
	}
	runes := []rune(seg.Text)
	maxLen, maxRow := -1, 0
	for i := range t.rowBounds {
		if l := t.rowBounds[i].end - t.rowBounds[i].begin; l > maxLen {
			maxLen = l
			maxRow = i
		}
	}
	if maxLen <= 0 {
		return 0
	}
	b := t.rowBounds[maxRow]
	begin := min(b.begin, len(runes))
	end := min(b.end, len(runes))
	txt := string(runes[begin:end])
	if seg.Style.concealed {
		txt = strings.Repeat(passwordChar, end-begin)
	}
	m, _ := bellui.MeasureTextFull(txt, seg.size(), seg.Style.TextStyle, seg.Style.FontSource)
	t.virtWidth = m.Width
	return t.virtWidth
}

// layoutVirtual positions only the window's row objects, while filling row
// geometry (rowYPos/rowHeight) analytically for every row so selection
// hit-testing and external gutter/decorator consumers see exact positions for
// the whole document without materialising off-screen rows.
func (r *textRenderer) layoutVirtual(size geo.Size) {
	t := r.obj
	if t.blockLazy {
		r.layoutVirtualBlockLazy(size)
		return
	}
	if t.varHeight {
		r.layoutVirtualVar(size)
		return
	}
	if t.lazy {
		r.layoutVirtualLazy(size)
		return
	}
	if t.isSelectable() && t.selection != nil {
		t.selection.Resize(size)
	}
	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	bounds := t.rowBounds
	n := len(bounds)
	stride := t.stride()
	top0 := innerPadding - t.inset.Height

	// Gutter offset depends on the total row count (not the window), so the
	// gutter width stays stable as the user scrolls.
	xInset := innerPadding - t.inset.Width
	if t.ShowLineNumbers && t.RowHighlighter != nil {
		textSize := th.Size(theme.SizeNameText)
		digits := len(strconv.Itoa(n))
		if digits < 3 {
			digits = 3
		}
		t.GutterWidth = bellui.MeasureText(strings.Repeat("0", digits)+" ", textSize, bellui.TextStyle{Monospace: true}).Width
		xInset += t.GutterWidth
	} else {
		t.GutterWidth = 0
	}
	left := xInset
	lineWidth := size.Width - left*2

	// Fill geometry for ALL rows analytically (uniform stride), collapsing folded
	// rows to zero height: a folded row reports the Y of the fold point and a
	// height of 0, so the scroll extent and every visible row shift up by the
	// hidden rows' height. With no folds this is the original top0 + i*stride.
	t.rowYPos = make([]float32, n)
	t.rowHeight = make([]float32, n)
	vis := 0
	for i := 0; i < n; i++ {
		t.rowYPos[i] = top0 + float32(vis)*stride
		if t.rowHidden(i) {
			t.rowHeight[i] = 0
		} else {
			t.rowHeight[i] = stride
			vis++
		}
	}

	// Position only the window's materialised rows at their true Y. winRows holds
	// the exact logical rows (folded rows are never materialised).
	objs := r.origObjs
	rows := t.winRows
	for i := 0; i < len(objs) && i < len(rows); i++ {
		row := rows[i]
		if row < 0 || row >= n {
			continue
		}
		align := bellui.TextAlignLeading
		if seg, ok := bounds[row].segments[0].(*TextSegment); ok {
			align = seg.Style.Alignment
		}
		r.layoutRow([]core.CanvasObject{objs[i]}, align, left, t.rowYPos[row], lineWidth)
	}
}

// layoutVirtualLazy positions the window's row objects for the lazy model. Unlike
// layoutVirtual it allocates no per-row geometry arrays: each row's Y is served
// analytically by RowGeometry, so a huge document is laid out in O(window) time.
func (r *textRenderer) layoutVirtualLazy(size geo.Size) {
	t := r.obj
	if t.isSelectable() && t.selection != nil {
		t.selection.Resize(size)
	}
	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)

	xInset := innerPadding - t.inset.Width
	if t.ShowLineNumbers && t.RowHighlighter != nil {
		textSize := th.Size(theme.SizeNameText)
		// Gutter width from the exact logical-line count (stable; the visual-row
		// total is only an estimate until fully scrolled).
		lines := 0
		if t.buf != nil {
			lines = t.buf.LineCount()
		}
		digits := len(strconv.Itoa(lines))
		if digits < 3 {
			digits = 3
		}
		t.GutterWidth = bellui.MeasureText(strings.Repeat("0", digits)+" ", textSize, bellui.TextStyle{Monospace: true}).Width
		xInset += t.GutterWidth
	} else {
		t.GutterWidth = 0
	}
	left := xInset
	lineWidth := size.Width - left*2

	// No dense geometry arrays in the lazy path — RowGeometry computes Y on demand.
	t.rowYPos = nil
	t.rowHeight = nil

	align := bellui.TextAlignLeading
	if seg, ok := t.singleSegment(); ok {
		align = seg.Style.Alignment
	}
	objs := r.origObjs
	rows := t.winRows
	for i := 0; i < len(objs) && i < len(rows); i++ {
		y, _ := t.RowGeometry(rows[i])
		r.layoutRow([]core.CanvasObject{objs[i]}, align, left, y, lineWidth)
	}
}

// lazyWindowRowText serves a window row's text straight from the lazy buffer —
// a Substring against the clean memo is a zero-copy slice, so the window render
// allocates nothing per row (pre-rebrand tracker #400). Reports false when the row is not the lazy
// single-segment case and the caller must take the generic decode path.
func (r *textRenderer) lazyWindowRowText(txt *canvas.Text, bound rowBoundary, seg *TextSegment) bool {
	buf := r.obj.buf
	if !r.obj.lazy || buf == nil || len(bound.segments) != 1 || seg != r.obj.Segments[0] {
		return false
	}
	// Clamp stale layout bounds to the current text length to avoid panics when
	// text changed between Layout and Refresh.
	runeLen := buf.Len()
	begin := min(bound.begin, runeLen)
	end := min(bound.end, runeLen)
	txt.Text = buf.Substring(begin, end)
	if bound.ellipsis {
		txt.Text = txt.Text + "…"
	}
	if concealed(seg) {
		txt.Text = strings.Repeat(passwordChar, runeLen)
	}
	return true
}

func (r *textRenderer) Refresh() {
	// Edit-incremental render (pre-rebrand tracker #921): when the last row-bounds rebuild spliced
	// a known dirty row range, regenerate objects + layout for those rows only.
	if r.refreshIncremental() {
		return
	}

	bounds := r.obj.rowBounds
	scroll := r.obj.Scroll

	// Row virtualisation: materialise only the rows in the visible window. Row
	// geometry for every row is still derived analytically in layoutVirtual, so
	// selection/cursor/gutter consumers see the full document. winRows lists the
	// exact materialised logical rows so a fold (pre-rebrand tracker #141) inside the window is never
	// materialised.
	winBounds := bounds
	if r.obj.canVirtualize() {
		r.obj.winRows = r.obj.computeWindowRows()
		winBounds = make([]rowBoundary, 0, len(r.obj.winRows))
		for _, row := range r.obj.winRows {
			if r.obj.lazy {
				if b := r.obj.lazyRowBoundary(row); b != nil {
					winBounds = append(winBounds, *b)
				}
				continue
			}
			if r.obj.blockLazy {
				if b := r.obj.blockLazyRowBoundary(row); b != nil {
					winBounds = append(winBounds, *b)
				}
				continue
			}
			winBounds = append(winBounds, bounds[row])
		}
		if len(r.obj.winRows) > 0 {
			r.obj.winStart = r.obj.winRows[0]
			r.obj.winEnd = r.obj.winRows[len(r.obj.winRows)-1] + 1
		} else {
			r.obj.winStart, r.obj.winEnd = 0, 0
		}
		// Grow the wrap-off horizontal extent to cover any newly visible wide row
		// (pre-rebrand tracker #170); monotonic, so the scrollbar never snaps narrower while scrolling.
		r.obj.widenVirtualWidthForWindow()
	}

	var objs []core.CanvasObject
	// Memoise the rune slice of the last text segment seen. In the virtualised
	// single-segment case every window row references the same segment whose Text
	// is the whole document, so converting it per row is O(document × window);
	// segments are contiguous within a window, so a one-entry memo collapses that
	// to a single conversion per distinct segment (pre-rebrand tracker #375). The lazy path skips even
	// that single conversion by reading row substrings from the buffer.
	var lastSeg *TextSegment
	var lastRunes []rune
	for _, bound := range winBounds {
		for i, seg := range bound.segments {
			if _, ok := seg.(*TextSegment); !ok {
				obj := r.obj.cachedSegmentVisual(seg, 0)
				seg.Update(obj)
				objs = append(objs, obj)
				continue
			}

			reuse := 0
			if i == 0 {
				reuse = bound.firstSegmentReuse
			}
			obj := r.obj.cachedSegmentVisual(seg, reuse)
			txt := obj.(*canvas.Text)
			textSeg := seg.(*TextSegment)
			textSeg.apply(txt) // refreshed with the whole RichText below

			if r.lazyWindowRowText(txt, bound, textSeg) {
				objs = append(objs, txt)
				continue
			}

			if showsWholeSegment(bound, textSeg) {
				txt.Text = textSeg.Text
				objs = append(objs, txt)
				continue
			}

			var runes []rune
			if textSeg == lastSeg {
				runes = lastRunes
			} else {
				runes = []rune(textSeg.Text)
				lastSeg, lastRunes = textSeg, runes
			}

			// Clamp stale layout bounds to the current text length to avoid
			// panics when text changed between Layout and Refresh.
			runeLen := len(runes)
			begin := min(bound.begin, runeLen)
			end := min(bound.end, runeLen)

			if i == 0 {
				if len(bound.segments) == 1 {
					txt.Text = string(runes[begin:end])
				} else {
					txt.Text = string(runes[begin:])
				}
			} else if i == len(bound.segments)-1 && len(bound.segments) > 1 {
				txt.Text = string(runes[:end])
			}
			if bound.ellipsis && i == len(bound.segments)-1 {
				txt.Text = txt.Text + "…"
			}

			if concealed(seg) {
				txt.Text = strings.Repeat(passwordChar, len(runes))
			}

			objs = append(objs, txt)
		}
	}

	if r.obj.scr != nil {
		// Reuse the existing content structure ({prop, innerBox}) and update the
		// inner box's objects in place so the scroll offset survives the window
		// re-renders that drive virtualised scrolling (pre-rebrand tracker #158). Replacing scr.Content
		// wholesale resets the offset to the top on every scroll, collapsing the
		// window back to row 0.
		if cont, ok := r.obj.scr.Content.(*bellui.Container); ok && len(cont.Objects) == 2 {
			cont.Objects[0] = r.obj.prop
			if box, ok := cont.Objects[1].(*bellui.Container); ok {
				box.Objects = objs
			} else {
				cont.Objects[1] = &bellui.Container{Objects: objs}
			}
		} else {
			r.obj.scr.Content = &bellui.Container{Layout: layout.NewStackLayout(), Objects: []core.CanvasObject{
				r.obj.prop, &bellui.Container{Objects: objs},
			}}
		}
		r.obj.scr.Direction = scroll
		r.SetObjects([]core.CanvasObject{r.obj.scr})
		r.obj.scr.Refresh()
	} else {
		if r.obj.isSelectable() && r.obj.selection != nil {
			r.obj.selection.provider = r.obj
			r.obj.selection.theme = r.obj.Theme()
			r.SetObjects(append([]core.CanvasObject{r.obj.selection}, objs...))
		} else {
			r.SetObjects(objs)
		}
	}

	r.saveOrigObjs(objs)

	r.Layout(r.obj.Size())

	// Post-layout: replace each row's text with colored fragments.
	if r.obj.RowHighlighter != nil {
		r.splitIntoFragments()
	}

	canvas.Refresh(r.obj.super())

	r.obj.cleanVisualCache()
}

// saveOrigObjs keeps the original 1-per-row objects for Layout (which expects
// this count), in an array of their own as the incremental refresh expects,
// reused when the count is unchanged (#492).
func (r *textRenderer) saveOrigObjs(objs []core.CanvasObject) {
	if len(r.origObjs) != len(objs) {
		r.origObjs = make([]core.CanvasObject, len(objs))
	}
	copy(r.origObjs, objs)
}

// showsWholeSegment reports whether a row holds the whole of a single text
// segment, shown as it is: no ellipsis, not concealed. Its text is then the
// segment's text, and going through runes would give the same string when the
// text is valid UTF-8, at two allocations per refresh (#492).
func showsWholeSegment(bound rowBoundary, seg *TextSegment) bool {
	return len(bound.segments) == 1 && bound.begin == 0 && !bound.ellipsis && !concealed(seg) &&
		bound.end >= utf8.RuneCountInString(seg.Text) && utf8.ValidString(seg.Text)
}

// splitIntoFragments replaces each row's canvas.Text with colored fragments.
// Uses origObjs (which are already positioned by Layout) as the source.
// The new fragments are set on the renderer via SetObjects.
func (r *textRenderer) splitIntoFragments() {
	objs := r.origObjs
	if len(objs) == 0 {
		return
	}

	th := r.obj.Theme()
	textSize := th.Size(theme.SizeNameText)

	// Keep all originals (for scroll sizing), make highlighted ones transparent,
	// and append colored fragments after them.
	transparent := color.NRGBA{A: 0}
	gutterColor := theme.ColorForWidget(theme.ColorNameOnSurfaceVariant, r.obj)
	breakpointColor := color.NRGBA{R: 220, G: 50, B: 50, A: 255}
	highlightColor := color.NRGBA{R: 255, G: 255, B: 100, A: 40}

	// renderGutterExtras adds breakpoint dots and row highlight backgrounds.
	renderGutterExtras := func(row1 int, pos geo.Position, txt *canvas.Text) []core.CanvasObject {
		var extras []core.CanvasObject
		rowH := txt.Size().Height

		// Row highlight background.
		if r.obj.HighlightedRow == row1 {
			bg := canvas.NewRectangle(highlightColor)
			bg.Move(geo.NewPos(0, pos.Y))
			bg.Resize(geo.NewSize(r.obj.Size().Width, rowH))
			extras = append(extras, bg)
		}

		// Breakpoint dot in the gutter.
		if r.obj.BreakpointLines[row1] && r.obj.GutterWidth > 0 {
			dotSize := textSize * 0.6
			dotX := pos.X - r.obj.GutterWidth + (dotSize * 0.3)
			dotY := pos.Y + (rowH-dotSize)/2
			dot := canvas.NewCircle(breakpointColor)
			dot.Move(geo.NewPos(dotX, dotY))
			dot.Resize(geo.NewSize(dotSize, dotSize))
			extras = append(extras, dot)
		}
		return extras
	}

	var fragments []core.CanvasObject
	// Resolve each text object's logical row. The virtualised path materialises an
	// arbitrary set of rows (winRows, with folds skipped); the full path
	// materialises every row in order. Folded rows are laid out as absent, so they
	// carry no highlight, line number or gutter decoration.
	useWin := r.obj.canVirtualize()
	winRows := r.obj.winRows
	ti := 0     // index among materialised text objects
	seqRow := 0 // next logical row for the non-virtual sequential mapping
	for _, obj := range objs {
		txt, ok := obj.(*canvas.Text)
		if !ok {
			continue
		}

		row0 := seqRow
		if useWin && ti < len(winRows) {
			row0 = winRows[ti]
		}
		ti++
		seqRow = row0 + 1

		if r.obj.rowHidden(row0) {
			continue
		}

		runs := r.obj.RowHighlighter(row0)
		row := row0 + 1 // 1-based for line numbers, breakpoints and highlight

		if len(runs) == 0 {
			// Still render line number even for unhighlighted rows.
			if r.obj.ShowLineNumbers {
				pos := txt.Position()
				numStr := strconv.Itoa(row)
				numW := bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}).Width
				numX := pos.X - r.obj.GutterWidth + (r.obj.GutterWidth - numW - bellui.MeasureText(" ", textSize, bellui.TextStyle{Monospace: true}).Width)
				num := canvas.NewText(numStr, gutterColor)
				num.TextSize = textSize
				num.TextStyle = bellui.TextStyle{Monospace: true}
				num.Move(geo.NewPos(numX, pos.Y))
				num.Resize(bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}))
				fragments = append(fragments, num)
				fragments = append(fragments, renderGutterExtras(row, pos, txt)...)
			}
			continue
		}

		// Verify total run runes match text.
		txtRunes := []rune(txt.Text)
		totalRunRunes := 0
		for _, run := range runs {
			totalRunRunes += len([]rune(run.Text))
		}

		pos := txt.Position()

		if totalRunRunes != len(txtRunes) {
			// Mismatch — skip highlighting but still render line number.
			if r.obj.ShowLineNumbers {
				numStr := strconv.Itoa(row)
				numW := bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}).Width
				numX := pos.X - r.obj.GutterWidth + (r.obj.GutterWidth - numW - bellui.MeasureText(" ", textSize, bellui.TextStyle{Monospace: true}).Width)
				num := canvas.NewText(numStr, gutterColor)
				num.TextSize = textSize
				num.TextStyle = bellui.TextStyle{Monospace: true}
				num.Move(geo.NewPos(numX, pos.Y))
				num.Resize(bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}))
				fragments = append(fragments, num)
				fragments = append(fragments, renderGutterExtras(row, pos, txt)...)
			}
			continue
		}

		// Make original transparent — fragments will be visible instead.
		txt.Color = transparent

		// Render line number.
		if r.obj.ShowLineNumbers {
			numStr := strconv.Itoa(row)
			numW := bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}).Width
			numX := pos.X - r.obj.GutterWidth + (r.obj.GutterWidth - numW - bellui.MeasureText(" ", textSize, bellui.TextStyle{Monospace: true}).Width)
			num := canvas.NewText(numStr, gutterColor)
			num.TextSize = textSize
			num.TextStyle = bellui.TextStyle{Monospace: true}
			num.Move(geo.NewPos(numX, pos.Y))
			num.Resize(bellui.MeasureText(numStr, textSize, bellui.TextStyle{Monospace: true}))
			fragments = append(fragments, num)
			fragments = append(fragments, renderGutterExtras(row, pos, txt)...)
		}

		// Create colored fragments using prefix measurement for X positions.
		baseX := pos.X
		style := txt.TextStyle
		source := txt.FontSource
		runeIdx := 0

		for _, run := range runs {
			runLen := len([]rune(run.Text))
			if runeIdx+runLen > len(txtRunes) {
				break
			}
			fragText := string(txtRunes[runeIdx : runeIdx+runLen])

			prefixX := float32(0)
			if runeIdx > 0 {
				m, _ := bellui.MeasureTextFull(string(txtRunes[:runeIdx]), textSize, style, source)
				prefixX = m.Width
			}

			col := theme.ColorForWidget(theme.ColorNameOnSurface, r.obj)
			if run.ForegroundColor != nil {
				col = run.ForegroundColor
			} else if run.ColorName != "" {
				c := theme.ColorForWidget(run.ColorName, r.obj)
				if _, _, _, ca := c.RGBA(); ca > 0 {
					col = c
				}
			}
			fragStyle := style
			if run.Italic {
				fragStyle.Italic = true
			}
			if run.Bold {
				fragStyle.Bold = true
			}
			// Positions stay measured with the base style so fragments remain
			// aligned with the caret/selection geometry (which also use the base
			// style). For the monospace code-editor use case bold/italic share
			// glyph advance, so this is exact; for proportional fonts sub-line
			// weight assumes equal advance.
			fragHSize, _ := bellui.MeasureTextFull(fragText, textSize, style, source)
			fragH := fragHSize.Height
			nextPrefixXSize, _ := bellui.MeasureTextFull(string(txtRunes[:runeIdx+runLen]), textSize, style, source)
			nextPrefixX := nextPrefixXSize.Width
			fragX := baseX + prefixX
			fragW := nextPrefixX - prefixX

			// Background fill sits behind the text fragment, so append it first.
			if run.BackgroundColor != nil {
				bg := canvas.NewRectangle(run.BackgroundColor)
				bg.Move(geo.NewPos(fragX, pos.Y))
				bg.Resize(geo.NewSize(fragW, fragH))
				fragments = append(fragments, bg)
			}

			frag := canvas.NewText(fragText, col)
			frag.TextSize = textSize
			frag.TextStyle = fragStyle
			frag.FontSource = source
			frag.Move(geo.NewPos(fragX, pos.Y))
			frag.Resize(geo.NewSize(fragW, fragH))
			fragments = append(fragments, frag)

			if run.Underline {
				ulColor := run.UnderlineColor
				if ulColor == nil {
					ulColor = col
				}
				fragments = append(fragments,
					newUnderlineDecoration(fragX, fragX+fragW, pos.Y+fragH, textSize, run.UnderlineStyle, ulColor)...)
			}
			if run.StrikeThrough {
				stColor := run.StrikeColor
				if stColor == nil {
					stColor = col
				}
				fragments = append(fragments,
					newStrikeDecoration(fragX, fragX+fragW, pos.Y+fragH/2, textSize, stColor)...)
			}
			runeIdx += runLen
		}
	}

	// Append fragments after the originals. The originals provide scroll
	// sizing (their MinSize is used by the scroll container), while the
	// fragments provide the visible colored text.
	if len(fragments) > 0 {
		r.SetObjects(append(objs, fragments...))
	}
}

func (r *textRenderer) layoutRow(texts []core.CanvasObject, align bellui.TextAlign, xPos, yPos, lineWidth float32) (float32, float32) {
	// Resolve leading/trailing against the active layout direction so the row
	// aligns to the correct edge under RTL (no-op for the default LTR layout).
	align = lang.ResolveAlign(align)
	initialX := xPos
	if len(texts) == 1 {
		if text, ok := texts[0].(*canvas.Text); ok {
			min, _ := r.obj.measureText(text)
			texts[0].Resize(min)
			xPad := float32(0)
			switch lang.ResolveAlign(text.Alignment) {
			case bellui.TextAlignLeading:
			case bellui.TextAlignTrailing:
				xPad = lineWidth - min.Width
			case bellui.TextAlignCenter:
				xPad = (lineWidth - min.Width) / 2
			}
			texts[0].Move(geo.NewPos(xPos+xPad, yPos))
			return min.Width, min.Height
		}
		min := texts[0].MinSize()
		texts[0].Resize(geo.NewSize(lineWidth, min.Height))
		texts[0].Move(geo.NewPos(xPos, yPos))
		return min.Width, min.Height
	}
	height := float32(0)
	tallestBaseline := float32(0)
	realign := false
	baselines := make([]float32, len(texts))

	// Access to theme is slow, so we cache the text size
	textSize := theme.SizeForWidget(theme.SizeNameText, r.obj)

	driver := bellui.CurrentApp().Driver()
	for i, text := range texts {
		var size geo.Size
		if txt, ok := text.(*canvas.Text); ok {
			s, base := r.obj.measureText(txt)
			if base > tallestBaseline {
				if tallestBaseline > 0 {
					realign = true
				}
				tallestBaseline = base
			}
			size = s
			baselines[i] = base
		} else if c, ok := text.(*bellui.Container); ok && len(c.Objects) > 0 {
			wid := c.Objects[0]
			if link, ok := wid.(*Hyperlink); ok {
				s, base := driver.RenderedTextSize(link.Text, textSize, link.TextStyle, nil)
				if base > tallestBaseline {
					if tallestBaseline > 0 {
						realign = true
					}
					tallestBaseline = base
				}
				size = s
				baselines[i] = base
			}
		}
		if size.IsZero() {
			size = text.MinSize()
		}
		text.Resize(size)
		text.Move(geo.NewPos(xPos, yPos))

		xPos += size.Width
		if height == 0 {
			height = size.Height
		} else if height != size.Height {
			height = geo.Max(height, size.Height)
			realign = true
		}
	}

	if realign {
		for i, text := range texts {
			delta := tallestBaseline - baselines[i]
			text.Move(geo.NewPos(text.Position().X, yPos+delta))
		}
	}

	innerPadding := r.obj.Theme().Size(theme.SizeNameInnerPadding)
	spare := lineWidth - xPos
	// The segments grown below are positioned with a physical (already
	// direction-resolved) alignment, but the painter applies lang.ResolveAlign
	// to every canvas.Text. Wrapping the physical value in ResolveAlign here
	// pre-compensates for that single downstream flip — it is a no-op under LTR
	// (ResolveAlign is identity) and cancels the painter's flip under RTL, so
	// the intended physical distribution is preserved either way.
	// Growing a segment to absorb the row's spare width (for trailing/center
	// distribution, or a wider hit box under leading) must keep that segment's OWN
	// height, not the row's max height: a shorter run stretched to the row height
	// (e.g. a normal run beside a taller monospace one) renders its glyphs low in
	// the over-tall box, breaking the baseline the realign above just established.
	switch align {
	case bellui.TextAlignTrailing:
		spare += innerPadding
		first := texts[0]
		first.Resize(geo.NewSize(first.Size().Width+spare, first.Size().Height))
		setAlign(first, lang.ResolveAlign(bellui.TextAlignTrailing))

		for _, text := range texts[1:] {
			text.Move(text.Position().Add(geo.NewPos(spare, 0)))
		}
	case bellui.TextAlignCenter:
		spare += innerPadding
		pad := spare / 2
		first := texts[0]
		first.Resize(geo.NewSize(first.Size().Width+pad, first.Size().Height))
		setAlign(first, lang.ResolveAlign(bellui.TextAlignTrailing))
		last := texts[len(texts)-1]
		last.Resize(geo.NewSize(last.Size().Width+pad, last.Size().Height))
		setAlign(last, lang.ResolveAlign(bellui.TextAlignLeading))

		for _, text := range texts[1:] {
			text.Move(text.Position().Add(geo.NewPos(pad, 0)))
		}
	default:
		last := texts[len(texts)-1]
		last.Resize(geo.NewSize(last.Size().Width+spare, last.Size().Height))
		setAlign(last, lang.ResolveAlign(bellui.TextAlignLeading))
	}

	return xPos - initialX, height
}

// binarySearch accepts a function that checks if the text width less the maximum width and the start and end rune index
// binarySearch returns the index of rune located as close to the maximum line width as possible
func binarySearch(lessMaxWidth func(int, int) bool, low int, maxHigh int) int {
	if low >= maxHigh {
		return low
	}
	if lessMaxWidth(low, maxHigh) {
		return maxHigh
	}
	high := low
	delta := maxHigh - low
	for delta > 0 {
		delta /= 2
		if lessMaxWidth(low, high+delta) {
			high += delta
		}
	}
	for (high < maxHigh) && lessMaxWidth(low, high+1) {
		high++
	}
	return high
}

// concealed returns true if the segment represents a password, meaning the text should be obscured.
func concealed(seg RichTextSegment) bool {
	if text, ok := seg.(*TextSegment); ok {
		return text.Style.concealed
	}

	return false
}

func ellipsisPriorBound(bounds []rowBoundary, trunc bellui.TextTruncation, width float32, measurer func([]rune) geo.Size) []rowBoundary {
	if trunc != bellui.TextTruncateEllipsis || len(bounds) == 0 {
		return bounds
	}

	prior := bounds[len(bounds)-1]
	seg := prior.segments[0].(*TextSegment)
	ellipsisSize, _ := bellui.MeasureTextFull("…", seg.size(), seg.Style.TextStyle, seg.Style.FontSource)

	widthChecker := func(low int, high int) bool {
		return measurer([]rune(seg.Text)[low:high]).Width <= width-ellipsisSize.Width
	}

	limit := binarySearch(widthChecker, prior.begin, prior.end)
	prior.end = limit

	prior.ellipsis = true
	bounds[len(bounds)-1] = prior
	return bounds
}

// findSpaceIndex accepts a slice of runes and a fallback index
// findSpaceIndex returns the index of the last space in the text, or fallback if there are no spaces
func findSpaceIndex(text []rune, fallback int) int {
	curIndex := fallback
	for ; curIndex >= 0; curIndex-- {
		if unicode.IsSpace(text[curIndex]) {
			break
		}
	}
	if curIndex < 0 {
		return fallback
	}
	return curIndex
}

func float32ToFixed266(f float32) fixed.Int26_6 {
	return fixed.Int26_6(float64(f) * (1 << 6))
}

// lineBounds accepts a slice of Segments, a wrapping mode, a maximum size available to display and a function to
// measure text size.
// It will return a slice containing the boundary metadata of each line with the given wrapping applied and the
// total height required to render the boundaries at the given width/height constraints
func lineBounds(seg *TextSegment, wrap bellui.TextWrap, trunc bellui.TextTruncation, firstWidth float32, max geo.Size, measurer func([]rune) geo.Size) ([]rowBoundary, float32) {
	lines := splitLines(seg)

	if wrap == bellui.TextWrap(bellui.TextTruncateClip) {
		if trunc == bellui.TextTruncateOff {
			trunc = bellui.TextTruncateClip
		}
		wrap = bellui.TextWrapOff
	}

	if max.Width < 0 || wrap == bellui.TextWrapOff && trunc == bellui.TextTruncateOff {
		return lines, 0 // don't bother returning a calculated height, our MinSize is going to cover it
	}

	measureWidth := float32(math.Min(float64(firstWidth), float64(max.Width)))
	text := []rune(seg.Text)
	st := lineWrapState{measureWidth: measureWidth}
	// The checker must read the live st.measureWidth: after the first row the
	// wrap width resets from firstWidth (the leftover after preceding inline
	// segments) to the full max.Width, and a checker frozen at the narrow first
	// width would clamp every later row's binary-searched wrap point to the
	// leftover width — one word (or a mid-word fragment) per line (pre-rebrand tracker #1156).
	widthChecker := func(low int, high int) bool {
		return measurer(text[low:high]).Width <= st.measureWidth
	}
	for _, l := range lines {
		if done, retBounds, retY := lineBoundsForRange(seg, text, l.begin, l.end, wrap, trunc, max, measurer, widthChecker, &st); done {
			return retBounds, retY
		}
	}
	return st.bounds, st.yPos
}

// lineBoundsForRange wraps a single logical line [low, high) (delimited from the
// segment by splitLines) into visual rows, appending them to st and carrying the
// reuse counter, measureWidth and yPos across calls. It returns done=true with the
// bounds slice and yPos to return when a height/truncation limit forces lineBounds
// to stop early. Extracted from the per-line body of lineBounds so a single line
// can be wrapped in isolation (the lazy/windowed measurement path, pre-rebrand tracker #157); the dense
// lineBounds loop drives it identically to the previous inline switch.
func lineBoundsForRange(seg *TextSegment, text []rune, low, high int, wrap bellui.TextWrap, trunc bellui.TextTruncation, max geo.Size, measurer func([]rune) geo.Size, widthChecker func(int, int) bool, st *lineWrapState) (bool, []rowBoundary, float32) {
	if low == high {
		st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, high, false})
		st.reuse++
		return false, nil, 0
	}

	switch wrap {
	case bellui.TextWrapBreak:
		return lineBoundsBreak(seg, text, low, high, high, trunc, max, measurer, widthChecker, st)
	case bellui.TextWrapWord:
		return lineBoundsWord(seg, text, low, high, high, trunc, max, measurer, widthChecker, st)
	default:
		lineBoundsDefault(seg, low, high, trunc, widthChecker, st)
		return false, nil, 0
	}
}

// lineWrapState holds the mutable accumulator state shared across the per-line
// wrap-mode handlers in lineBounds.
type lineWrapState struct {
	bounds       []rowBoundary
	reuse        int
	measureWidth float32
	yPos         float32
}

// lineBoundsBreak handles bellui.TextWrapBreak wrapping for a single logical line.
// It mutates st and, if the height limit forces an early return from lineBounds,
// returns done=true with the bounds slice and yPos to return.
func lineBoundsBreak(seg *TextSegment, text []rune, low, high, lineEnd int, trunc bellui.TextTruncation, max geo.Size, measurer func([]rune) geo.Size, widthChecker func(int, int) bool, st *lineWrapState) (bool, []rowBoundary, float32) {
	for low < high {
		measured := measurer(text[low:high])
		if st.yPos+measured.Height > max.Height && trunc != bellui.TextTruncateOff {
			return true, ellipsisPriorBound(st.bounds, trunc, st.measureWidth, measurer), st.yPos
		}

		if measured.Width <= st.measureWidth {
			st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, high, false})
			st.reuse++
			low = high
			high = lineEnd
			st.measureWidth = max.Width

			st.yPos += measured.Height
		} else {
			newHigh := binarySearch(widthChecker, low, high)
			if newHigh <= low {
				if st.measureWidth < max.Width {
					// Wrap to next line at full width instead of 1-char lines.
					st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, low, false})
					st.reuse++
					st.measureWidth = max.Width

					st.yPos += measured.Height
					continue
				}
				st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, low + 1, false})
				st.reuse++
				low++
				st.measureWidth = max.Width

				st.yPos += measured.Height
			} else {
				high = newHigh
			}
		}
	}
	return false, nil, 0
}

// lineBoundsWord handles bellui.TextWrapWord wrapping for a single logical line.
// It mutates st and, if the height limit forces an early return from lineBounds,
// returns done=true with the bounds slice and yPos to return.
func lineBoundsWord(seg *TextSegment, text []rune, low, high, lineEnd int, trunc bellui.TextTruncation, max geo.Size, measurer func([]rune) geo.Size, widthChecker func(int, int) bool, st *lineWrapState) (bool, []rowBoundary, float32) {
	for low < high {
		sub := text[low:high]
		measured := measurer(sub)
		if st.yPos+measured.Height > max.Height && trunc != bellui.TextTruncateOff {
			return true, ellipsisPriorBound(st.bounds, trunc, st.measureWidth, measurer), st.yPos
		}

		subWidth := measured.Width
		if subWidth <= st.measureWidth {
			st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, high, false})
			st.reuse++
			low = high
			high = lineEnd
			if low < high && unicode.IsSpace(text[low]) {
				low++
			}
			st.measureWidth = max.Width

			st.yPos += measured.Height
		} else {
			oldHigh := high
			last := low + len(sub) - 1
			fallback := binarySearch(widthChecker, low, last) - low

			if fallback < 1 { // even a character won't fit
				if st.measureWidth < max.Width {
					// Character doesn't fit in remaining first-line width but
					// would fit at full width. Wrap to next line instead of
					// placing one char per line.
					st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, low, false})
					st.reuse++
					high = oldHigh
					st.measureWidth = max.Width

					st.yPos += measured.Height
					continue
				}
				include := 1
				ellipsis := false
				if trunc == bellui.TextTruncateEllipsis {
					include = 0
					ellipsis = true
				}
				st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, low + include, ellipsis})
				low++
				high = low + 1
				st.reuse++
				st.measureWidth = max.Width

				st.yPos += measured.Height
				if high > lineEnd {
					return true, st.bounds, st.yPos
				}
			} else {
				spaceIndex := findSpaceIndex(sub, fallback)
				if spaceIndex == 0 {
					spaceIndex = 1
				}

				high = low + spaceIndex
			}
			if high == fallback && subWidth <= max.Width { // add a newline as there is more space on next
				st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, low, false})
				st.reuse++
				high = oldHigh
				st.measureWidth = max.Width

				st.yPos += measured.Height
				continue
			}
		}
	}
	return false, nil, 0
}

// lineBoundsDefault handles the non-wrapping (off) case with optional truncation
// for a single logical line, mutating st.
func lineBoundsDefault(seg *TextSegment, low, high int, trunc bellui.TextTruncation, widthChecker func(int, int) bool, st *lineWrapState) {
	if trunc == bellui.TextTruncateEllipsis {
		txt := []rune(seg.Text)[low:high]
		end, full := truncateLimit(string(txt), seg.Visual().(*canvas.Text), st.measureWidth, []rune{'…'})
		high = low + end
		st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, high, !full})
		st.reuse++
	} else if trunc == bellui.TextTruncateClip {
		high = binarySearch(widthChecker, low, high)
		st.bounds = append(st.bounds, rowBoundary{[]RichTextSegment{seg}, st.reuse, low, high, false})
		st.reuse++
	}
}

func setAlign(obj core.CanvasObject, align bellui.TextAlign) {
	if text, ok := obj.(*canvas.Text); ok {
		text.Alignment = align
		return
	}
	if c, ok := obj.(*bellui.Container); ok && len(c.Objects) > 0 {
		wid := c.Objects[0]
		if link, ok := wid.(*Hyperlink); ok {
			link.Alignment = align
			link.Refresh()
		}
	}
}

// splitLines accepts a text segment and returns a slice of boundary metadata denoting the
// start and end indices of each line delimited by the newline character.
func splitLines(seg *TextSegment) []rowBoundary {
	var low, high int
	var lines []rowBoundary
	text := []rune(seg.Text)
	length := len(text)
	for i := 0; i < length; i++ {
		if text[i] == '\n' {
			high = i
			lines = append(lines, rowBoundary{[]RichTextSegment{seg}, len(lines), low, high, false})
			low = i + 1
		}
	}
	return append(lines, rowBoundary{[]RichTextSegment{seg}, len(lines), low, length, false})
}

func truncateLimit(s string, text *canvas.Text, limit float32, ellipsis []rune) (int, bool) {
	face := paint.CachedFontFace(text.TextStyle, text.FontSource, text)

	runes := []rune(s)
	in := shaping.Input{
		Text:      ellipsis,
		RunStart:  0,
		RunEnd:    len(ellipsis),
		Direction: di.DirectionLTR,
		Face:      face.Fonts.ResolveFace(ellipsis[0]),
		Size:      float32ToFixed266(text.TextSize),
	}
	shaper := &shaping.HarfbuzzShaper{}
	segmenter := &shaping.Segmenter{}

	conf := shaping.WrapConfig{}
	conf = conf.WithTruncator(shaper, in)
	conf.BreakPolicy = shaping.WhenNecessary
	conf.TruncateAfterLines = 1
	l := shaping.LineWrapper{}

	in.Text = runes
	in.RunEnd = len(runes)
	ins := segmenter.Split(in, face.Fonts)
	outs := make([]shaping.Output, len(ins))
	for i, in := range ins {
		outs[i] = shaper.Shape(in)
	}

	l.Prepare(conf, runes, shaping.NewSliceIterator(outs))
	// Keep the available width fractional (26.6 fixed point) rather than flooring it
	// to an int, so text sized to exactly its own (fractional) intrinsic width is not
	// spuriously truncated (pre-rebrand tracker #1045). WrapNextLine(int) would floor via fixed.I.
	wrapped, done := l.WrapNextLineF(float32ToFixed266(limit))

	count := len(runes)
	if wrapped.Truncated != 0 {
		count -= wrapped.Truncated
		count += len(ellipsis)
	}

	full := done && count == len(runes)
	if !full && len(ellipsis) > 0 {
		count--
	}
	return count, full
}

type rowBoundary struct {
	segments          []RichTextSegment
	firstSegmentReuse int
	begin, end        int
	ellipsis          bool
}
