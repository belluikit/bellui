# fycoder Roadmap

## Vision — the end goal

fycoder today is a **composable syntax-highlighting code editor widget** for
Bellui. The goal of this roadmap is to grow it into a **complete, LSP-capable
editor core** — capable enough to be the editor at the heart of a production
IDE such as [**pxfed**](https://github.com/trendvidia/pxfed), the PXF desktop
editor, so pxfed can replace its current **Electron + React + Monaco** renderer
with a native **Bellui / Go** stack built on fycoder (and rendered through the
[appviewer](https://github.com/trendvidia/appviewer) engine).

"Monaco-parity for pxfed" is the concrete bar. pxfed leans on
Monaco + `monaco-languageclient` for a specific, enumerable set of features.
fycoder must surface each of them natively in Bellui:

| pxfed needs (today via Monaco) | fycoder phase |
|---|---|
| Diagnostics — squiggles, gutter markers, hover messages | [Phase 3](docs/roadmap/03-diagnostics-and-navigation.md) |
| Completion popups, hover tooltips, signature help | [Phase 2](docs/roadmap/02-floating-ui-primitives.md) |
| Go-to-definition + link decorations | [Phase 3](docs/roadmap/03-diagnostics-and-navigation.md) |
| Code folding, incl. auto-fold of huge dataset regions | [Phase 5](docs/roadmap/05-folding-and-large-files.md) |
| Inline / character-range decorations (secret redaction bars, semantic overlays) | [Phase 1](docs/roadmap/01-inline-and-span-decorations.md) |
| Protected (read-only) ranges with full edit-guard (keystroke / paste / IME / drag / find-replace / undo) | [Phase 4](docs/roadmap/04-protected-ranges-and-redaction.md) |
| Multi-document host: per-document view-state, incremental edits, doc versioning | [Phase 6](docs/roadmap/06-document-model-and-viewstate.md) |
| The wiring that turns all of the above into one language-server connection | [Phase 7](docs/roadmap/07-lsp-integration-layer.md) |

## Design principles

1. **The core stays protocol-agnostic.** fycoder never imports an LSP client.
   It exposes rendering primitives (decorations, popups, gutter markers) and
   provider/callback hooks. A real language server is bound to those hooks by
   the optional `fycoder/lsp` package (Phase 7) or by the host app. fycoder
   must remain pleasant to use as a *plain* editor with zero LSP.
2. **Extend the pluggable-interface pattern, don't bake features in.** The
   existing `Highlighter` / `GutterProvider` / `RowDecorator` interfaces are the
   model. New capabilities arrive as new interfaces + provider hooks, not as
   hard-coded behavior.
3. **Layout-internal features land in the fork first.** Anything that needs
   text-layout internals — sub-line styling, character-range geometry,
   column→pixel mapping, row hiding for folds, protected ranges — requires
   changes to the [`trendvidia/fyne`](https://github.com/trendvidia/fyne) fork's
   `widget.Entry`. fycoder consumes those. This dependency track is real and is
   tracked separately in
   [00-fyne-fork-dependencies.md](docs/roadmap/00-fyne-fork-dependencies.md).
4. **Every phase ships value standalone.** A general-purpose code editor (Zed /
   VS Code-like for any Bellui app) benefits from each phase independently of
   pxfed. pxfed is the forcing function, not the only consumer.

## Current baseline (Milestone 0 — shipped)

What fycoder already provides, grounded in the current source:

- **`Editor`** widget (`editor.go`) wrapping a forked `widget.Entry`
  (`editorEntry`). The fork already exposes `OnScrolled`, `SuppressGutter`,
  `RowMetrics()`, `RowPositions()`, `LineHighlighter`, `KeepSelectionVisible`,
  and `ReadOnly`.
- **Pluggable `Highlighter`** (`style.go`), per-line and stateful across lines.
  Highlighters exist for `go`, `pxf`, and `starlark`. `StyleRun` *already
  declares* `BackgroundColor`, `Bold`, and `Underline` — but the fork's
  `widget.StyleRun` currently consumes only `ColorName` + `Italic`, so those
  fields are inert today (see `applyHighlighter`).
- **`GutterProvider` / `InteractiveGutter` / `CompositeGutter`** (`gutter.go`)
  — pinned (non-horizontally-scrolling) gutter with line numbers and
  click-to-toggle breakpoint markers.
- **`RowDecorator` overlay** (`decorator.go`, `decorator_overlay.go`) —
  **row-level**, full-width background/overlay effects (current-line,
  search-row highlight). Purely visual: the overlay takes no input.
- **Find bar** (`findbar.go`) — Cmd+F / Cmd+G / Shift+Cmd+G, case-sensitive
  toggle.
- **Editor ergonomics** — `OnChanged`, `OnSave`, `OnRun`, `OnCursorChanged`,
  `CursorPos`/`SetCursorPos`, `SelectedText`, `SetReadOnly`, `AutoGrow`,
  `MinRows`.

The three biggest structural gaps between this and an IDE core:

- Decorations are **row-level only** — there is no character-range ("span")
  decoration, which diagnostics squiggles, redaction bars, and semantic
  overlays all need.
- There are **no anchored floating widgets** (completion / hover / signature),
  and no **column→pixel** mapping to anchor them.
- There is **no folding, no protected ranges, and no document/version model.**

## Milestones

| Phase | Title | Headline capability | Depends on | Size | Status |
|------|-------|---------------------|------------|------|--------|
| **1** | [Inline & span decorations](docs/roadmap/01-inline-and-span-decorations.md) | Character-range styling: squiggles, opaque overlays, sub-line backgrounds; `PositionAt(row,col)` | Fork: render `StyleRun` BG/underline; expose column geometry | **M** | ✅ **released [v0.1.0](https://github.com/belluikit/bellui/x/editor/releases/tag/v0.1.0)** ([#4](https://github.com/belluikit/bellui/x/editor/pull/4)) — incl. follow-ups: span-level find ([#8](https://github.com/belluikit/bellui/x/editor/pull/8)), gallery demo ([gallery#1](https://github.com/trendvidia/gallery/pull/1)) |
| **2** | [Floating UI primitives](docs/roadmap/02-floating-ui-primitives.md) | Anchored popovers → completion, hover, signature help | Phase 1 (`PositionAt`) | **L** | ✅ **complete** ([#10](https://github.com/belluikit/bellui/x/editor/issues/10)) — popover infra ✅ ([#13](https://github.com/belluikit/bellui/x/editor/issues/13) / [#17](https://github.com/belluikit/bellui/x/editor/pull/17)), completion ✅ ([#14](https://github.com/belluikit/bellui/x/editor/issues/14) / [#21](https://github.com/belluikit/bellui/x/editor/pull/21)), hover ✅ ([#15](https://github.com/belluikit/bellui/x/editor/issues/15) / [#28](https://github.com/belluikit/bellui/x/editor/pull/28)), signature help ✅ ([#16](https://github.com/belluikit/bellui/x/editor/issues/16) / [#31](https://github.com/belluikit/bellui/x/editor/pull/31)); follow-ups ✅ popover canvas-overflow + shadow ([#69](https://github.com/belluikit/bellui/x/editor/pull/69)), interactive/keyboard hover ([#70](https://github.com/belluikit/bellui/x/editor/pull/70)); **released [v0.2.0](https://github.com/belluikit/bellui/x/editor/releases/tag/v0.2.0)** + gallery demo ([#22](https://github.com/belluikit/bellui/x/editor/issues/22) / [gallery#4](https://github.com/trendvidia/gallery/pull/4)) |
| **3** | [Diagnostics & navigation](docs/roadmap/03-diagnostics-and-navigation.md) | Diagnostics model → squiggles + gutter markers + hover; go-to-def hook + link decorations | Phases 1, 2 | **M** | ✅ **complete** ([#42](https://github.com/belluikit/bellui/x/editor/issues/42)) — squiggles ✅ ([#43](https://github.com/belluikit/bellui/x/editor/issues/43) / [#50](https://github.com/belluikit/bellui/x/editor/pull/50)), gutter markers ✅ ([#44](https://github.com/belluikit/bellui/x/editor/issues/44) / [#54](https://github.com/belluikit/bellui/x/editor/pull/54)), hover ✅ ([#45](https://github.com/belluikit/bellui/x/editor/issues/45) / [#55](https://github.com/belluikit/bellui/x/editor/pull/55), incl. point ranges [#52](https://github.com/belluikit/bellui/x/editor/issues/52)), RevealRange ✅ ([#46](https://github.com/belluikit/bellui/x/editor/issues/46) / [#53](https://github.com/belluikit/bellui/x/editor/pull/53)), go-to-def + link decorations ✅ ([#47](https://github.com/belluikit/bellui/x/editor/issues/47) / [#60](https://github.com/belluikit/bellui/x/editor/pull/60)); diagnostics-polish follow-ups ✅ ([#56](https://github.com/belluikit/bellui/x/editor/issues/56)/[#57](https://github.com/belluikit/bellui/x/editor/issues/57)/[#58](https://github.com/belluikit/bellui/x/editor/issues/58) via [#94](https://github.com/belluikit/bellui/x/editor/pull/94), [#95](https://github.com/belluikit/bellui/x/editor/issues/95)/[#96](https://github.com/belluikit/bellui/x/editor/pull/96), [#97](https://github.com/belluikit/bellui/x/editor/issues/97)/[#98](https://github.com/belluikit/bellui/x/editor/pull/98), [#99](https://github.com/belluikit/bellui/x/editor/issues/99)/[#100](https://github.com/belluikit/bellui/x/editor/pull/100)); **released [v0.2.0](https://github.com/belluikit/bellui/x/editor/releases/tag/v0.2.0)** + gallery demo ([#22](https://github.com/belluikit/bellui/x/editor/issues/22) / [gallery#4](https://github.com/trendvidia/gallery/pull/4)) |
| **4** | [Protected ranges & redaction](docs/roadmap/04-protected-ranges-and-redaction.md) | Read-only ranges with full edit-guard; reveal/re-mask redaction overlays | Phase 1; Fork: range protection in `Entry` | **L** | ✅ **complete** ([#62](https://github.com/belluikit/bellui/x/editor/issues/62)) — protected ranges ([#63](https://github.com/belluikit/bellui/x/editor/issues/63) / [#68](https://github.com/belluikit/bellui/x/editor/pull/68)), redaction + reveal/mask ([#64](https://github.com/belluikit/bellui/x/editor/issues/64) / [#68](https://github.com/belluikit/bellui/x/editor/pull/68)), `ClassifySpanChanges` ([#65](https://github.com/belluikit/bellui/x/editor/issues/65) / [#67](https://github.com/belluikit/bellui/x/editor/pull/67)). Consumes fork F3; the fyne fork has now cut **v2.10.0** and the go.mod pin moves from the pre-release pseudo-version to the clean tag ([#101](https://github.com/belluikit/bellui/x/editor/pull/101)) |
| **5** | [Folding & large files](docs/roadmap/05-folding-and-large-files.md) | Fold regions + gutter UI + auto-fold; windowed rendering for big docs | Fork: row hiding in `Entry` | **XL** | ✅ **complete** ([fyne#150](https://github.com/trendvidia/fyne/issues/150)) — `FoldRegion` model + `SetFoldRegions`/`Fold`/`Unfold`/`FoldedRanges`/`AutoFold`, `FoldGutter` chevrons via `CompositeGutter`, collapsed-region placeholder overlay, all on fork F4 `Entry.HiddenRanges`. Gutter + decorator overlay now viewport-windowed (`Editor.rowWindow`, O(viewport) not O(document)) so the external render cycle on a 50k-line file drops from ~15ms → ~0.05ms/frame; guarded by a large-file benchmark + budget test. (The Entry's own per-keystroke relayout stays O(document) in the fork — a separate F5 follow-up, unaffected by folding.) Pins fyne **v2.11.0** |
| **6** | [Document model & view-state](docs/roadmap/06-document-model-and-viewstate.md) | Versioned document, incremental edits, per-document view-state for tab hosts | Phases 1–5 | **M** | — |
| **7** | [LSP integration layer](docs/roadmap/07-lsp-integration-layer.md) | Optional `fycoder/lsp` bridging a language server to all hooks above | Phases 1–6 | **L** | — |
| **0** | [Bellui fork dependencies](docs/roadmap/00-fyne-fork-dependencies.md) | Cross-cutting `trendvidia/fyne` `Entry` work that phases 1/4/5 require | — | runs in parallel | F1+F2 ✅ (fyne v2.9.0); F3+F4 ✅ (fyne **v2.10.0**, consumed via [#101](https://github.com/belluikit/bellui/x/editor/pull/101)) |

Size key: **S** ≈ days · **M** ≈ 1–2 wk · **L** ≈ 3–5 wk · **XL** ≈ 6 wk+
(single-engineer, including tests + gallery demo).

### Stretch phases (beyond pxfed-parity)

Phases 1–7 reach parity with the slice of Monaco that **pxfed** uses. Monaco the
*product* is far larger; the following phases close the remaining gap toward a
general-purpose, VS Code-class editor. **pxfed needs none of them** — they are
listed so the Monaco gap is documented explicitly rather than implied, and
should be pursued only if fycoder is positioned as a standalone editor for the
wider Bellui ecosystem.

| Phase | Title | Headline capability | Depends on | Size |
|------|-------|---------------------|------------|------|
| **8** | [General-editor extras](docs/roadmap/08-general-editor-extras.md) | Multi-cursor, code actions, rename, formatting, outline/symbols, references/peek, + niceties; plus the TextMate-grammar and accessibility deep gaps | Phases 1–7; fork (multi-cursor, view zones) | **XL** (basket) |
| **9** | [Diff editor](docs/roadmap/09-diff-editor.md) | Side-by-side & inline diff with intra-line highlighting | Phases 1, 6; fork (view zones for inline) | **L–XL** |

Both are independently schedulable mini-projects, not a continuation of the
critical path. Phase 8 is a basket — cherry-pick areas by demand. Phase 9 is
self-contained and largely independent of the LSP work.

### Cross-cutting target

- **[Phase 10 — WASM / web target](docs/roadmap/10-wasm-web-target.md)** — a
  feasibility analysis of running a fycoder-based, pxfed-class app in the
  browser via `GOOS=js`. Verdict: possible (the native-Go stack is what makes it
  possible), gated chiefly on Bellui-WASM's text-input layer. fycoder's own
  obligation is narrow — the editor must be usable under `js/wasm` — and is best
  de-risked with the one-day spike described there before committing.

## Critical path & sequencing

```
        ┌─ Phase 1 (span decorations) ─┬─ Phase 2 (popups) ─┬─ Phase 3 (diagnostics/nav) ─┐
fork ───┤                              │                                                   ├─ Phase 6 ─ Phase 7
track   └─ Phase 4 (protected ranges) ─┘            Phase 5 (folding) ──────────────────────┘
```

- **Phase 1 is the keystone — ✅ shipped ([#4](https://github.com/belluikit/bellui/x/editor/pull/4)).**
  Span decorations + `PositionAt`/`RangeRects` are in, on fyne v2.9.0 (fork F1
  rich `StyleRun` + F2 `CaretRect`). This unblocks diagnostics squiggles (3),
  anchored popups (2), and redaction overlays (4).
- **Phase 2 is complete.** The anchored-popover foundation shipped
  ([#13](https://github.com/belluikit/bellui/x/editor/issues/13) / [#17](https://github.com/belluikit/bellui/x/editor/pull/17)):
  popovers render inside the editor's own Stack (not the canvas overlay stack),
  keeping the Entry focused so typing keeps working while a popover is open. All
  three surfaces build on it — completion ([#14](https://github.com/belluikit/bellui/x/editor/issues/14) / [#21](https://github.com/belluikit/bellui/x/editor/pull/21)),
  hover ([#15](https://github.com/belluikit/bellui/x/editor/issues/15) / [#28](https://github.com/belluikit/bellui/x/editor/pull/28)), signature help
  ([#16](https://github.com/belluikit/bellui/x/editor/issues/16) / [#31](https://github.com/belluikit/bellui/x/editor/pull/31)). Gallery demos landed
  with the v0.2.0 release ([#22](https://github.com/belluikit/bellui/x/editor/issues/22) / [gallery#4](https://github.com/trendvidia/gallery/pull/4)).
- **Phases 2 and 4 can proceed in parallel** once Phase 1 lands — different
  surfaces (popups vs. input-guarding).
- **Phase 5 (folding) is the riskiest and most fork-heavy.** It can run in
  parallel with the 2/3/4 cluster because it touches different machinery, but
  budget it as the long pole.
- **The fork track ([Phase 0](docs/roadmap/00-fyne-fork-dependencies.md)) gates
  1, 4, and 5.** Schedule the fork changes ahead of the fycoder work that
  consumes them, or the fycoder phases stall.
- **pxfed can begin its migration after Phase 3** with a known, scoped feature
  gap (no redaction-guard, no folding yet), then close the gap as 4 and 5 land.

## Overall effort

Reaching the end goal — fycoder as a drop-in IDE editor core with pxfed parity —
is roughly **4–6 engineer-months** for fycoder + its fork dependencies,
sequenced as above. That is the editor-core portion of the larger
"rebuild pxfed on Bellui" effort; the protolsp server, sops, age keygen, and
Connect-RPC services are reused as-is and are out of scope for this repo.

The single largest line items are **Phase 5 (folding + large-file
virtualization)** and the **fork-side `Entry` work** it and Phases 1/4 depend
on. De-risk both early.

The **stretch phases (8–9) are additive and excluded from that estimate.** They
move fycoder from "pxfed-parity" toward "general VS Code-class editor"; matching
Monaco *fully* (multi-cursor, code actions, diff editor, TextMate grammars,
accessibility) is a materially larger program than reaching the end goal above,
and most of it pxfed never exercises.

See each linked document for API sketches, fork dependencies, and acceptance
criteria.
