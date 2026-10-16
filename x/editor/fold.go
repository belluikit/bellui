package editor

import (
	"sort"
	"strings"

	"github.com/belluikit/bellui/widget"
)

// FoldKind classifies a foldable region so a host (or a gutter styling hook) can
// tell an import block from a comment block from an arbitrary user fold. It is
// advisory only — folding behaves identically regardless of kind.
type FoldKind int

const (
	// FoldKindComment marks a foldable comment block.
	FoldKindComment FoldKind = iota
	// FoldKindRegion marks an explicit region marker (e.g. //region … //endregion).
	FoldKindRegion
	// FoldKindBlock marks a language block (a function body, a braced block).
	FoldKindBlock
	// FoldKindImports marks an import/using block.
	FoldKindImports
	// FoldKindCustom marks a host-defined fold with no language meaning — the
	// kind [Editor.Fold] and [Editor.AutoFold] register ad-hoc ranges under.
	FoldKindCustom
)

// FoldRegion describes one foldable block of the document.
//
// Folding is line-granular: the region's first line ([TextRange.StartRow]) stays
// visible as the header — carrying the fold chevron and, when collapsed, the
// placeholder — and the interior lines StartRow+1 .. EndRow are hidden. A region
// whose range spans a single line ([TextRange.StartRow] == [TextRange.EndRow]) is
// not foldable (there is nothing to hide). Columns are ignored: a fold always
// hides whole lines.
type FoldRegion struct {
	// Range is the foldable block, in document (row, col) coordinates. Only the
	// start and end rows matter for folding.
	Range TextRange
	// Kind classifies the region (advisory).
	Kind FoldKind
	// Collapsed reports whether the region is currently folded.
	Collapsed bool
	// Summary is optional placeholder text shown in place of the hidden interior
	// when the region is collapsed. Empty shows the default ellipsis.
	Summary string
}

// defaultFoldPlaceholder is shown after a collapsed region's header line when the
// region has no Summary. It is deliberately ASCII ("...") so it renders with the
// bundled (Latin-only) fonts on every backend — including wasm, which has no
// system-font scanning for fallbacks. A non-Latin ellipsis glyph would render as
// tofu there (and can nil-deref the software painter's fallback path). A host that
// wants a fancier marker can supply a Summary of its own.
const defaultFoldPlaceholder = "..."

// SetFoldRegions replaces the editor's set of foldable regions. Each region's
// Collapsed flag is honored as given, so a caller can supply a pre-collapsed set
// (e.g. restored view state). Collapsing is applied immediately: interior lines
// of every collapsed region are hidden via the fork's row-hiding support, so the
// text model, search, diagnostics and LSP positions stay in document
// coordinates while only the rendered geometry collapses.
//
// Regions are expressed in (row, col) and anchor-tracked through edits (pre-rebrand tracker #130):
// an insert or delete above a region shifts it (collapsed state and hidden rows
// included), an edit inside resizes it, and a deletion that swallows a region
// removes it — no host re-publish needed. A programmatic [Editor.SetText] is a
// document swap, not an edit: regions are left as published and the host
// supplies regions for the new document.
//
// Passing nil or an empty slice clears all folds.
func (e *Editor) SetFoldRegions(regions []FoldRegion) {
	e.fold.setRegions(regions)
}

// FoldRegions returns a copy of the current foldable regions, in the order they
// were last set (folds added via [Editor.Fold]/[Editor.AutoFold] are appended).
// It returns nil when none are set.
func (e *Editor) FoldRegions() []FoldRegion {
	return e.fold.regionsCopy()
}

// Fold collapses the region identified by r (matched by range). If no registered
// region has that range, an ad-hoc [FoldKindCustom] region is registered and
// collapsed, so Fold works standalone without a prior [Editor.SetFoldRegions].
// Folding a single-line range (nothing to hide) is a no-op.
func (e *Editor) Fold(r TextRange) {
	e.fold.setCollapsed(r, true, true)
}

// Unfold expands the region identified by r (matched by range), revealing its
// hidden interior. It is a no-op when no registered region has that range.
func (e *Editor) Unfold(r TextRange) {
	e.fold.setCollapsed(r, false, false)
}

// FoldAll collapses every registered foldable region. Regions that hide nothing
// (single-line) or are already collapsed are left untouched. Like the other fold
// operations it never mutates the document text and fires no [Editor.OnChanged].
func (e *Editor) FoldAll() {
	e.fold.foldAll()
}

// UnfoldAll expands every collapsed region, revealing all hidden interiors. It
// never mutates the document text and fires no [Editor.OnChanged].
func (e *Editor) UnfoldAll() {
	e.fold.unfoldAll()
}

// FoldedRanges returns the ranges of every currently collapsed region, in
// registration order. It is the Phase 6 view-state hook for persisting which
// regions are folded across document switches.
func (e *Editor) FoldedRanges() []TextRange {
	return e.fold.foldedRanges()
}

// FoldGutter returns the editor's internal [FoldGutter], wired to its fold state
// so tapping a chevron toggles the region. Compose it into the gutter to show
// fold affordances, e.g.:
//
//	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
func (e *Editor) FoldGutter() *FoldGutter {
	return e.foldGutter
}

// AutoFold collapses each supplied range immediately, registering it as a
// [FoldKindCustom] foldable region if it is not already known. It is pxfed's
// entry point for auto-collapsing dataset regions flagged "huge" on load.
// Single-line ranges (nothing to hide) are ignored.
func (e *Editor) AutoFold(ranges []TextRange) {
	e.fold.autoFold(ranges)
}

// foldController owns the foldable regions and drives the Entry's HiddenRanges
// from the collapsed subset. It also supplies the fold gutter's foldable/collapsed
// queries and the placeholder overlay's data, so all three views read one source
// of truth.
type foldController struct {
	e       *Editor
	regions []FoldRegion
}

func newFoldController(e *Editor) *foldController {
	return &foldController{e: e}
}

// foldable reports whether r hides at least one line when collapsed.
func foldable(r TextRange) bool {
	return r.EndRow > r.StartRow
}

func (c *foldController) setRegions(regions []FoldRegion) {
	if len(regions) == 0 {
		c.regions = nil
	} else {
		c.regions = append([]FoldRegion{}, regions...)
	}
	c.apply()
}

func (c *foldController) regionsCopy() []FoldRegion {
	if len(c.regions) == 0 {
		return nil
	}
	return append([]FoldRegion{}, c.regions...)
}

// setCollapsed sets the collapsed state of the region matching r. When the range
// is unknown and register is true it is added as a collapsed custom region (the
// Fold/AutoFold path); when register is false an unknown range is a no-op (the
// Unfold path). It applies the change and reports whether anything changed.
func (c *foldController) setCollapsed(r TextRange, collapsed, register bool) bool {
	for i := range c.regions {
		if c.regions[i].Range == r {
			if c.regions[i].Collapsed == collapsed {
				return false
			}
			c.regions[i].Collapsed = collapsed
			c.apply()
			return true
		}
	}
	if !register || !collapsed || !foldable(r) {
		return false
	}
	c.regions = append(c.regions, FoldRegion{Range: r, Kind: FoldKindCustom, Collapsed: true})
	c.apply()
	return true
}

func (c *foldController) autoFold(ranges []TextRange) {
	changed := false
	for _, r := range ranges {
		if !foldable(r) {
			continue
		}
		found := false
		for i := range c.regions {
			if c.regions[i].Range == r {
				found = true
				if !c.regions[i].Collapsed {
					c.regions[i].Collapsed = true
					changed = true
				}
				break
			}
		}
		if !found {
			c.regions = append(c.regions, FoldRegion{Range: r, Kind: FoldKindCustom, Collapsed: true})
			changed = true
		}
	}
	if changed {
		c.apply()
	}
}

func (c *foldController) foldedRanges() []TextRange {
	var out []TextRange
	for _, rg := range c.regions {
		if rg.Collapsed && foldable(rg.Range) {
			out = append(out, rg.Range)
		}
	}
	return out
}

// toggle flips the collapsed state of the foldable region whose header line is
// row (0-based). It is the fold gutter's click handler. A no-op when no foldable
// region starts on that row.
func (c *foldController) toggle(row int) {
	for i := range c.regions {
		if foldable(c.regions[i].Range) && c.regions[i].Range.StartRow == row {
			c.regions[i].Collapsed = !c.regions[i].Collapsed
			c.apply()
			return
		}
	}
}

// foldAll collapses every foldable region and reports whether anything changed.
func (c *foldController) foldAll() bool {
	changed := false
	for i := range c.regions {
		if foldable(c.regions[i].Range) && !c.regions[i].Collapsed {
			c.regions[i].Collapsed = true
			changed = true
		}
	}
	if changed {
		c.apply()
	}
	return changed
}

// unfoldAll expands every collapsed region and reports whether anything changed.
func (c *foldController) unfoldAll() bool {
	changed := false
	for i := range c.regions {
		if c.regions[i].Collapsed {
			c.regions[i].Collapsed = false
			changed = true
		}
	}
	if changed {
		c.apply()
	}
	return changed
}

// revealRows expands every collapsed region that hides any row in the inclusive
// range [startRow, endRow], so a jump into a folded block (a search match,
// diagnostic, or go-to-definition target) lands the caret on a visible line.
// Nested folds are handled: a row hidden by several collapsed regions unfolds all
// of them. It applies once and reports whether anything changed.
func (c *foldController) revealRows(startRow, endRow int) bool {
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	changed := false
	for i := range c.regions {
		rg := &c.regions[i]
		if !rg.Collapsed || !foldable(rg.Range) {
			continue
		}
		// A collapsed region hides its interior rows [StartRow+1, EndRow]; unfold
		// it when that interior overlaps the target range.
		if rg.Range.StartRow+1 <= endRow && rg.Range.EndRow >= startRow {
			rg.Collapsed = false
			changed = true
		}
	}
	if changed {
		c.apply()
	}
	return changed
}

// foldableAtHeader reports whether a foldable region's header line is row (0-based).
func (c *foldController) foldableAtHeader(row int) bool {
	for _, rg := range c.regions {
		if foldable(rg.Range) && rg.Range.StartRow == row {
			return true
		}
	}
	return false
}

// collapsedAtHeader reports whether a foldable region headed at row (0-based) is
// currently collapsed.
func (c *foldController) collapsedAtHeader(row int) bool {
	for _, rg := range c.regions {
		if foldable(rg.Range) && rg.Range.StartRow == row {
			return rg.Collapsed
		}
	}
	return false
}

// hiddenRanges resolves the collapsed regions to the logical-line ranges the
// Entry hides. Each collapsed region hides its interior lines StartRow+1 .. EndRow
// inclusive, i.e. LineRange{StartRow+1, EndRow+1}; the fork normalises and merges
// overlaps.
func (c *foldController) hiddenRanges() []widget.LineRange {
	var out []widget.LineRange
	for _, rg := range c.regions {
		if !rg.Collapsed || !foldable(rg.Range) {
			continue
		}
		out = append(out, widget.LineRange{Start: rg.Range.StartRow + 1, End: rg.Range.EndRow + 1})
	}
	return out
}

// mergedHidden returns the collapsed regions' hidden line ranges normalized:
// clamped to >= 0, empties dropped, sorted by start, overlaps/adjacent merged.
// It mirrors the fork's own normalisation so fycoder's analytic visible-row
// mapping (used for large-file windowing) matches the Entry's geometry exactly.
func (c *foldController) mergedHidden() []widget.LineRange {
	return normalizeFoldRanges(c.hiddenRanges())
}

// normalizeFoldRanges sorts, clamps, drops empties, and merges overlapping or
// adjacent ranges — a minimal, ordered, disjoint set (nil when nothing survives).
func normalizeFoldRanges(rs []widget.LineRange) []widget.LineRange {
	cleaned := make([]widget.LineRange, 0, len(rs))
	for _, r := range rs {
		s, e := r.Start, r.End
		if s < 0 {
			s = 0
		}
		if e > s {
			cleaned = append(cleaned, widget.LineRange{Start: s, End: e})
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	sort.Slice(cleaned, func(i, j int) bool { return cleaned[i].Start < cleaned[j].Start })
	merged := []widget.LineRange{cleaned[0]}
	for _, r := range cleaned[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End { // overlapping or adjacent
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// hiddenBefore returns the number of hidden rows in [0, row), given a normalized
// hidden set. Range ends are clamped to total so a stale range past the document
// does not over-count.
func hiddenBefore(row, total int, hidden []widget.LineRange) int {
	if row > total {
		row = total
	}
	count := 0
	for _, r := range hidden {
		if r.Start >= row {
			break
		}
		e := r.End
		if e > total {
			e = total
		}
		if e > row {
			e = row
		}
		if e > r.Start {
			count += e - r.Start
		}
	}
	return count
}

// visibleRowCount returns how many of total rows are not folded away.
func visibleRowCount(total int, hidden []widget.LineRange) int {
	return total - hiddenBefore(total, total, hidden)
}

// visibleIndexToRow maps a 0-based visible-row index (its position among the
// non-folded rows) to its logical row, given a normalized hidden set.
func visibleIndexToRow(k, total int, hidden []widget.LineRange) int {
	row := 0
	for _, r := range hidden {
		s, e := r.Start, r.End
		if s > total {
			s = total
		}
		if e > total {
			e = total
		}
		if s < row {
			s = row
		}
		avail := s - row
		if k < avail {
			return row + k
		}
		k -= avail
		row = e
	}
	res := row + k
	if res > total {
		res = total
	}
	return res
}

// foldPlaceholder is one collapsed-region marker to draw inline: the ellipsis or
// Summary shown just after the header line's text.
type foldPlaceholder struct {
	row, col int    // header line and the column just past its last character
	text     string // Summary, or the default ellipsis
}

// placeholders returns one entry per collapsed region, positioned at the end of
// its header line. The column is the header line's rune length so the marker sits
// immediately after the visible text.
func (c *foldController) placeholders() []foldPlaceholder {
	if len(c.regions) == 0 {
		return nil
	}
	// Raw Text read (pre-rebrand tracker #129-safe): placeholders render only while regions are
	// collapsed, and active folds disable the fork's deferred edit path, so
	// Text is always current here.
	lines := strings.Split(c.e.entry.Text, "\n")
	var out []foldPlaceholder
	for _, rg := range c.regions {
		if !rg.Collapsed || !foldable(rg.Range) {
			continue
		}
		row := rg.Range.StartRow
		if row < 0 || row >= len(lines) {
			continue
		}
		text := rg.Summary
		if text == "" {
			text = defaultFoldPlaceholder
		}
		out = append(out, foldPlaceholder{row: row, col: len([]rune(lines[row])), text: text})
	}
	return out
}

// apply pushes the recomputed hidden ranges to the Entry, relays out so the
// folded geometry takes effect, and repaints the external gutter, decorators and
// placeholders. It performs no document mutation and fires no OnChanged.
func (c *foldController) apply() {
	c.e.entry.HiddenRanges = c.hiddenRanges()
	// A relayout is needed for the fork to re-resolve HiddenRanges into visual-row
	// geometry (syncSegments → setHiddenRanges) and reflow the collapsed height.
	c.e.entry.Refresh()
	c.e.syncExternalWidgets()
	c.e.RefreshGutter()
	c.e.RefreshDecorators()
}
