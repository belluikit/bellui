# Phase 6 — Document model & view-state

**Size:** M · **Depends on:** Phases 1–5 (everything that has per-document
state) · **Unlocks:** pxfed's multi-tab host, and correct incremental sync to a
language server (Phase 7).

pxfed manages many open documents in a tab bar, swapping the editor's model and
restoring each tab's cursor/scroll/fold state (Monaco's `saveViewState` /
`restoreViewState`). It also feeds the LSP **incremental** `didChange` events
with document versions. fycoder is currently single-buffer (`Text`/`SetText`,
no version, no view-state). This phase makes fycoder a clean editor core for a
tab-managing host.

## Deliverables

### 1. View-state snapshot/restore
```go
// ViewState captures everything a host must restore to make switching
// back to a document seamless: caret, selection, scroll, and folds.
type ViewState struct {
    Caret       Position
    Selection   *TextRange
    ScrollY     float32
    FoldedRanges []TextRange
}

func (e *Editor) SaveViewState() ViewState
func (e *Editor) RestoreViewState(ViewState)
```
This lets the host keep one `Editor` and swap documents (cheap), or keep one
`Editor` per tab — either way state survives the switch. pxfed does the former
today; both must work.

### 2. Versioned document + incremental edits
```go
// Document is an optional richer model the host can attach instead of a
// raw string. It carries a monotonically increasing version and emits
// granular change events suitable for LSP didChange.
type Document struct { /* text, version, language id, uri */ }

func (e *Editor) SetDocument(*Document)
func (e *Editor) Document() *Document

// OnDocumentChange reports granular edits (range replaced + new text +
// new version) — the shape an LSP textDocument/didChange needs. The
// existing OnChanged(string) remains for simple hosts.
func (e *Editor) OnDocumentChange(func(DocumentChange))
```
`DocumentChange` carries `{Range TextRange, NewText string, Version int}`.
Today `OnChanged` only hands back the full string; Phase 7's LSP sync needs the
delta and version, which is why this lives here.

### 3. Read-only / virtual documents
Formalize the read-only document case pxfed uses for `registry://` schema
sources and the effective-config view: a `Document` can be marked read-only at
the model level (not just `SetReadOnly` on the widget), and the host can label
its origin (uri/language) so highlighting and LSP routing pick the right
behavior.

## Design notes
- Keep the simple path simple: `Text`/`SetText`/`OnChanged` stay and keep
  working for hosts that don't need versioning. `Document` is additive.
- View-state must round-trip folds (Phase 5) and selection precisely, or tab
  switching feels broken — this is the acceptance bar pxfed will judge against.
- The document/version model is the seam the LSP layer (Phase 7) plugs into;
  design the `DocumentChange` shape to map 1:1 onto LSP incremental
  `contentChanges` to avoid an adapter.

## Acceptance criteria
- Save/restore view-state round-trips caret, selection, scroll, and folds
  across a simulated tab switch.
- `OnDocumentChange` reports correct ranges, replacement text, and increasing
  versions for typed, pasted, and find-replaced edits.
- A read-only `Document` cannot be edited but highlights and supports
  hover/diagnostics/navigation.
