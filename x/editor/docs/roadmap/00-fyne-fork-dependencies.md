# Phase 0 — `trendvidia/fyne` fork dependencies (cross-cutting)

fycoder already depends on a forked `widget.Entry` for `OnScrolled`,
`SuppressGutter`, `RowMetrics()`, and `RowPositions()`. Several roadmap phases
need *more* from the fork, because they touch text-layout internals that a
wrapper widget cannot reach. This document tracks that work as a single
dependency track so it can be scheduled ahead of the fycoder phases that
consume it.

> **Rule of thumb:** if a feature needs to know *where a character is*, *style
> part of a line*, *hide a line*, or *reject an edit*, it lives in the fork.
> If it composes existing primitives, it lives in editor.

## Required fork capabilities

### F1 — Render the rich `StyleRun` fields *(gates Phase 1)*
`editor.StyleRun` already declares `BackgroundColor`, `Bold`, and `Underline`,
but `applyHighlighter()` drops them because the fork's `widget.StyleRun`
consumes only `ColorName` + `Italic`. Extend `widget.StyleRun` (and the
RichText render path) to honor:
- `BackgroundColor` — sub-line background fill.
- `Bold` — font weight.
- `Underline` — straight underline, **and** a `UnderlineStyle` (solid / wavy /
  dotted) + `UnderlineColor` so diagnostics can draw colored squiggles.

Acceptance: a highlighter returning a wavy red underline run renders one.

### F2 — Column geometry / `PositionAt` *(gates Phases 1, 2, 3)*
The fork exposes row Y geometry via `RowPositions()`, but nothing maps a
`(row, col)` to an **X** pixel offset. Add an `Entry` method to return the
caret rectangle for a logical position, e.g.:
```go
// CaretRect returns the pixel rectangle of the caret at (row, col),
// in the Entry's content coordinate space (pre-scroll).
func (e *Entry) CaretRect(row, col int) geo.Rectangle
```
Monospace makes the math simple, but it must account for tabs, theme text size,
and padding — so it belongs in the fork next to the real layout, not
re-derived in editor. fycoder wraps it as `Editor.PositionAt(row,col)`.

### F3 — Range-level edit interception *(gates Phase 4)*
Protected/redacted ranges need the `Entry` to **reject mutations** that fall
inside a protected range, across every mutation path: typed keys, paste,
drag-drop, IME composition commit, and find-replace. Today an external widget
can only observe `OnChanged` after the fact. Add either:
- a `BeforeChange func(old, new string, sel TextRange) bool` veto hook, or
- a first-class `ProtectedRanges []TextRange` field the `Entry` enforces.

Either must also block the *caret* from entering a fully-redacted span when in
"masked" mode, and survive undo/redo. See Phase 4 for the fycoder-side API.

### F4 — Row hiding for folding *(gates Phase 5)*
Folding requires the `Entry` to **not render** a contiguous block of lines
while keeping them in the model (so text, search, and LSP positions stay
correct). Add a `HiddenRanges []LineRange` (or `FoldedRegions`) concept that
the layout skips, with `RowPositions()`/`RowMetrics()` reporting visible-row
geometry so the gutter and decorator overlay stay aligned. This is the
largest and riskiest fork change.

### F5 — Windowed / virtualized rendering *(supports Phase 5)*
pxfed opens large effective-config and dataset files. Confirm (or add) that the
`Entry` only lays out and paints visible rows, so highlight + decoration + gutter
cost scales with viewport, not document size.

> **✅ Resolved** by pre-rebrand tracker #119:
> `widget.Entry` previously materialised *every* row (large documents froze,
> fatally on single-threaded wasm) and now virtualizes. F4 (row hiding) must
> compose with this — a hidden row inside a virtualized window is simply never
> materialised.

## Coordination notes

- The fork is shared with appviewer and other trendvidia Bellui apps; additive,
  backward-compatible changes only. New fields default to today's behavior.
- Mirror the existing fycoder convention: "extra fields activate as the fork
  adds support". fycoder can land the *consuming* API behind
  the inert fields first, then light up when the fork ships — F1 in particular
  can be developed this way.
- Each fork capability above should ship with a fycoder gallery demo that
  exercises it, so regressions in the fork are caught downstream.

## Scheduling

| Fork item | Gates | When | Status |
|---|---|---|---|
| F1 (rich StyleRun) | Phase 1 | before/with Phase 1 | ✅ shipped — fyne v2.9.0 (pre-rebrand tracker #114, pre-rebrand tracker #116) |
| F2 (`CaretRect`) | Phases 1, 2, 3 | before/with Phase 1 | ✅ shipped — fyne v2.9.0 (pre-rebrand tracker #115, pre-rebrand tracker #116) |
| F3 (edit interception) | Phase 4 | before Phase 4 | ✅ merged to fork `main` — pre-rebrand tracker #140 — **unreleased**, consumed now via a pre-release go.mod pin (pseudo-version of the fork commit); swap to the fyne v2.10.0 tag when cut |
| F4 (row hiding) | Phase 5 | before Phase 5 — **start early, long pole** | ✅ merged to fork `main` — pre-rebrand tracker #141 — **unreleased**, consumable via the same pre-release pin as F3 |
| F5 (virtualization) | Phase 5 | measure during Phase 5 | ✅ resolved — pre-rebrand tracker #119 (`Entry` no longer materialises every row) |

> **Pre-release pin (2026-06-26):** F3 + F4 are merged on `trendvidia/fyne` `main`
> but **not in any release tag** — the latest, v2.9.0, was cut before they
> landed. Rather than wait, fycoder's `go.mod` pins a **pseudo-version of the fork
> commit** (`github.com/trendvidia/fyne/v2 v2.9.1-0.…`), so Phase 4 (and, when
> built, Phase 5) consume F3/F4 today. The project has **no CI**, so changes are
> validated locally (`GOWORK=off go build/vet/test ./...` resolves the pin via the
> Go proxy). **Swap the pin for the real tag (e.g. v2.10.0) with a one-line go.mod
> bump once fyne cuts it.** A gitignored `go.work` remains a local convenience
> pointing at a fork checkout. The critical path (Phases 2→3) is unaffected.
