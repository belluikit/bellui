# Phase 5 — Code folding & large-file handling

**Size:** XL · **Depends on:** fork F4 (row hiding), fork F5 (virtualization) ·
**Unlocks:** pxfed's dataset auto-fold and responsive editing of large
effective-config / dataset files. **Longest pole on the roadmap — start the
fork work early.**

pxfed folds huge dataset regions automatically (`dataset-folding.ts` subscribes
to a stream and folds any region flagged `huge`) and relies on Monaco's folding
+ virtualized rendering to stay responsive on big files. Neither folding nor
proven large-file behavior exists in fycoder today.

## Deliverables

### 1. Fold-region model
```go
type FoldRegion struct {
    Range    TextRange // the foldable block
    Kind     FoldKind  // Comment | Region | Block | Imports | Custom
    Collapsed bool
    Summary  string    // optional placeholder text shown when collapsed
}

// SetFoldRegions supplies foldable regions (from a language analysis,
// indentation heuristic, or the LSP foldingRange request in P7).
func (e *Editor) SetFoldRegions([]FoldRegion)
func (e *Editor) Fold(r TextRange)
func (e *Editor) Unfold(r TextRange)
func (e *Editor) FoldedRanges() []TextRange
```

### 2. Fold rendering (fork F4)
Collapsing a region hides its interior lines via the fork's row-hiding support
while keeping them in the model, so:
- text content, search, diagnostics, and LSP positions stay correct,
- `RowPositions()`/`RowMetrics()` report visible-row geometry so the gutter and
  span/row overlays (Phases 1, 3, 4) stay aligned across folds,
- a collapsed region shows a placeholder/ellipsis (and optional `Summary`).

### 3. Fold gutter UI
A `FoldGutter` `GutterProvider` (composes via `CompositeGutter`, like
`BreakpointGutter`) drawing fold/unfold chevrons on foldable lines, with
click-to-toggle through the `InteractiveGutter` path that already exists.

### 4. Auto-fold hook
```go
// AutoFold collapses any supplied region immediately — pxfed calls this
// for dataset regions flagged "huge". Folds re-apply when the document is
// re-shown (see Phase 6 view-state).
func (e *Editor) AutoFold([]TextRange)
```

### 5. Large-file validation (fork F5)
Measure and, if needed, fix rendering cost on large documents so that
highlight + decoration + gutter + fold work scales with the **viewport**, not
the document. Establish a benchmark (e.g. a 50k-line file) in the gallery and
guard it with a test. This is where pxfed's "open a giant effective-config"
case lives or dies.

## Design notes
- Folding is the deepest layout change on the roadmap; the risk is almost
  entirely in fork F4 (hiding rows while keeping positions coherent). Prototype
  F4 against a throwaway branch before committing the fycoder API.
- Every prior phase that places things by row (gutter, decorators, spans,
  diagnostics, redaction) must be re-validated against folds — folds change the
  visible-row mapping. Budget integration time, not just feature time.
- Persisting which regions are folded across document switches is a Phase 6
  (view-state) concern; here, just expose `FoldedRanges()`.

## Acceptance criteria
- Folding a region hides its lines, shows a placeholder, and keeps gutter +
  diagnostics aligned on the lines that remain visible.
- Search/diagnostics/positions over a folded document still resolve to correct
  document coordinates.
- `AutoFold` collapses supplied ranges on load.
- Benchmark: editing a 50k-line file stays interactive (define a concrete
  frame/latency budget) — guarded by a test.
