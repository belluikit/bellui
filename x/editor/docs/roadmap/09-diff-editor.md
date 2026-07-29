# Phase 9 — Diff editor *(stretch — beyond pxfed-parity)*

**Size:** L–XL · **Depends on:** Phases 1 (decorations) & 6 (document model);
optionally fork view-zone support for inline mode · **Status:** stretch.
**pxfed does not require this.** Monaco ships a full diff editor as effectively
a second product; fycoder has nothing equivalent. This phase is a self-contained
mini-project, largely independent of the LSP work, worth doing only if a host
(e.g. a VCS/review tool, or pxfed gaining a "compare layers" view) needs it.

It is listed because the Monaco comparison flagged the diff editor as the single
largest capability fycoder lacks, and the roadmap should name it rather than
leave it implicit.

## Deliverables

### 1. `DiffEditor` widget
A new top-level widget composing the diff of two documents (original vs.
modified). Reuses the core `Editor`'s machinery — `Highlighter`, gutter,
span/row decorations — rather than reimplementing rendering.
```go
type DiffEditor struct { /* original, modified *Document */ }
func NewDiffEditor() *DiffEditor
func (d *DiffEditor) SetOriginal(*Document)
func (d *DiffEditor) SetModified(*Document)
func (d *DiffEditor) SetMode(DiffMode)   // SideBySide | Inline
func (d *DiffEditor) NextChange() / PrevChange()
```

### 2. Diff computation
A line-level diff (Myers or histogram) with **intra-line (character-level)**
refinement so changed regions within a line are highlighted precisely. Pure Go,
no Bellui dependency, independently unit-testable — keep it in a `diff` sub-package
so it's reusable and fast to test.

### 3. Rendering
- **Side-by-side**: two synchronized `Editor` panes (scroll-locked), with
  added/removed/changed lines as **row decorators** (Phase 1) and intra-line
  changes as **span decorations** (Phase 1). Gutter shows +/−/~ markers via a
  diff `GutterProvider` (mirrors `BreakpointGutter`).
- **Inline (unified)**: a single pane interleaving removed and added lines.
  Showing removed lines that aren't in the buffer needs **view zones**
  (content rendered between real lines) — a fork capability fycoder doesn't have
  yet, so inline mode carries a fork dependency (shared with Phase 8f peek).
- Change-bar navigation (`NextChange`/`PrevChange`) reusing `RevealRange`.

### 4. Optional: editable diff / hunk staging *(stretch within stretch)*
Accept/reject individual hunks (the modified side stays editable). High effort,
only if a review/merge use-case demands it.

## Design notes
- Build on the core widget, do not fork it: a `DiffEditor` is two `Editor`s plus
  a diff model plus decorations. If that composition proves awkward, that's a
  signal the Phase 1 decoration API is too weak — fix it there, not here.
- Side-by-side has **no fork dependency** and delivers most of the value; inline
  mode (view zones) is the expensive part. Ship side-by-side first.
- Keep the diff algorithm separate and well-tested; rendering bugs and diff
  bugs should never be entangled.

## Acceptance criteria
- Side-by-side mode shows two scroll-locked panes with correctly marked
  added/removed/changed lines and intra-line highlights, with working
  next/prev-change navigation.
- The diff algorithm is unit-tested against known fixtures independent of any
  UI.
- (If pursued) inline mode renders removed lines via view zones with correct
  alignment.
