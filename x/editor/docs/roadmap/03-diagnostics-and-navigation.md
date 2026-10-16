# Phase 3 — Diagnostics & navigation

**Size:** M · **Depends on:** Phase 1 (span decorations), Phase 2 (hover popover) ·
**Unlocks:** pxfed's problems panel, inline error squiggles, and Cmd-click
go-to-definition.

> **Status: ✅ complete** ([#42](https://github.com/belluikit/bellui/x/editor/issues/42)),
> all five sub-issues shipped (not fork-blocked — built on Phase 1/2 primitives).
> Diagnostics, the problems-list feed, and go-to-definition are all merged to
> `main`.
> - **Deliverable 1 — diagnostics** (model + three render surfaces) — **✅ shipped:**
>   - **✅** [#43](https://github.com/belluikit/bellui/x/editor/issues/43) /
>     [#50](https://github.com/belluikit/bellui/x/editor/pull/50) — model
>     (`Diagnostic`/`Severity`, `SetDiagnostics`/`Diagnostics`/
>     `OnDiagnosticsChanged`) + severity **squiggles** (internal
>     `diagnosticSpanDecorator` drawing one `SpanUnderline`/`UnderlineWavy` per
>     diagnostic, themed `ColorNameDiagnostic{Error,Warning,Info,Hint}` palette).
>     **Foundation.**
>   - **✅** [#44](https://github.com/belluikit/bellui/x/editor/issues/44) /
>     [#54](https://github.com/belluikit/bellui/x/editor/pull/54) — **gutter markers**
>     (`DiagnosticGutter`, mirroring `BreakpointGutter`, fed automatically from
>     `SetDiagnostics`, composed via `CompositeGutter` /
>     `Editor.DiagnosticGutter()`).
>   - **✅** [#45](https://github.com/belluikit/bellui/x/editor/issues/45) /
>     [#55](https://github.com/belluikit/bellui/x/editor/pull/55) — **hover messages**
>     on a diagnostic range (composes the Phase 2 hover path; active even with no
>     host provider; merges diagnostic + provider content). Bundled
>     [#52](https://github.com/belluikit/bellui/x/editor/issues/52) — point (zero-width)
>     ranges widened to a visible, hoverable cell.
> - **Deliverable 2 — problems-list feed — ✅ shipped:**
>   - **✅** [#46](https://github.com/belluikit/bellui/x/editor/issues/46) /
>     [#53](https://github.com/belluikit/bellui/x/editor/pull/53) — `RevealRange`
>     (scroll + select); the `Diagnostics()`/`OnDiagnosticsChanged` accessors
>     landed with pre-rebrand tracker #43.
> - **Deliverable 3 — navigation — ✅ shipped:**
>   - **✅** [#47](https://github.com/belluikit/bellui/x/editor/issues/47) /
>     [#60](https://github.com/belluikit/bellui/x/editor/pull/60) — `OnRequestNavigation`
>     (Cmd/Ctrl+click → host go-to-def hook) + `SetLinkProvider`/`WordLinkProvider`
>     + **link decorations** (Cmd/Ctrl-hover lights a solid underline + pointer
>     cursor over a navigable symbol). Reuses `RevealRange` (pre-rebrand tracker #46).
>
> Shipped order: pre-rebrand tracker #43 → (pre-rebrand tracker #44, pre-rebrand tracker #45, pre-rebrand tracker #46 in parallel) → pre-rebrand tracker #47, all merged + pre-rebrand tracker #52.
> **Follow-ups — ✅ shipped** (bundled via [#94](https://github.com/belluikit/bellui/x/editor/pull/94)):
> severity-distinct gutter marker shapes
> ([#56](https://github.com/belluikit/bellui/x/editor/issues/56)) — circle/triangle/square/diamond
> per severity plus a `MarkerFunc` override; gutter click-to-reveal
> ([#57](https://github.com/belluikit/bellui/x/editor/issues/57)) — opt-in
> `DiagnosticGutter.RevealOnTap` / `Editor.SetDiagnosticClickToReveal`; hover
> keep-alive over overlapping diagnostics
> ([#58](https://github.com/belluikit/bellui/x/editor/issues/58)) — `hoverShownRange` now
> unions all matched ranges.
> **Follow-up — ✅ shipped** (via [#96](https://github.com/belluikit/bellui/x/editor/pull/96)):
> column-aware `CompositeGutter` tap routing
> ([#95](https://github.com/belluikit/bellui/x/editor/issues/95)) — surfaced by pre-rebrand tracker #57; a tap
> now reaches only the clicked column (`CompositeGutter.TappedAtX`), so
> click-to-reveal and an interactive `BreakpointGutter` no longer both fire on one
> tap.
> **Follow-up — ✅ shipped** (via [#98](https://github.com/belluikit/bellui/x/editor/pull/98)):
> click-to-reveal now targets the most-severe diagnostic on a line
> ([#97](https://github.com/belluikit/bellui/x/editor/issues/97)) — the reveal range tracks
> the marker's severity in lockstep (ties broken by set order), so the tap reveals
> the diagnostic the glyph represents.
> **Follow-up — ✅ shipped** (via [#100](https://github.com/belluikit/bellui/x/editor/pull/100)):
> cycle through a line's diagnostics on repeated gutter taps
> ([#99](https://github.com/belluikit/bellui/x/editor/issues/99)) — opt-in
> `DiagnosticGutter.CycleOnTap`; the first tap reveals the most severe, repeats walk
> the rest (severity order, wrapping), and a different line restarts the walk.
>
> The diagnostic-gutter/hover follow-up chain (pre-rebrand tracker #56 → pre-rebrand tracker #57 → pre-rebrand tracker #58 → pre-rebrand tracker #95 → pre-rebrand tracker #97 → pre-rebrand tracker #99) is
> now **fully closed**.
> **Released:** fycoder **[v0.2.0](https://github.com/belluikit/bellui/x/editor/releases/tag/v0.2.0)**
> (Phase 2 + Phase 3) is tagged, and the gallery Code Editor page now demos the
> Phase 2/3 features ([#22](https://github.com/belluikit/bellui/x/editor/issues/22) via
> pre-rebrand gallery tracker #4) — completion, hover,
> signature help (with overload control), diagnostics (squiggle + severity-shaped
> gutter markers + hover + click-to-reveal), and Cmd/Ctrl-click navigation. pre-rebrand tracker #22
> stays open only for a manual live-GL pass (link cursor/focus path, off-screen
> `RevealRange` scroll, overload-arrow focus preservation).

This phase composes the Phase 1/2 primitives into the two features that make an
editor feel "language-aware": **diagnostics** and **definition navigation**.
Both stay host-driven — fycoder renders and routes; the host (or P7) supplies
data and performs navigation.

## Deliverables

### 1. Diagnostics model + rendering
```go
type Severity int // Error | Warning | Info | Hint

type Diagnostic struct {
    Range    TextRange
    Severity Severity
    Message  string
    Source   string // e.g. "protolsp", "protocompile"
}

// SetDiagnostics replaces the editor's diagnostic set. Idempotent and
// cheap to call on every publishDiagnostics notification.
func (e *Editor) SetDiagnostics([]Diagnostic)
```
Rendering combines three existing surfaces:
- **Squiggles** — a built-in `SpanDecorator` (Phase 1) drawing a wavy underline
  per diagnostic, colored by severity.
- **Gutter markers** — a new `DiagnosticGutter` `GutterProvider` (mirrors
  `BreakpointGutter` in `gutter_breakpoint.go`) showing a severity dot/icon on
  lines that have diagnostics; composes via `CompositeGutter`.
- **Hover messages** — the Phase 2 hover path, extended so that hovering a
  diagnostic range shows its message(s). (pxfed renders these inline in
  Monaco's marker hovers today.)

### 2. Problems-list feed
pxfed has a `DiagnosticsList` panel. fycoder doesn't own panels, but should make
the data trivially consumable:
```go
func (e *Editor) Diagnostics() []Diagnostic
func (e *Editor) OnDiagnosticsChanged(func([]Diagnostic))
func (e *Editor) RevealRange(r TextRange) // scroll + select, for "jump to problem"
```
`RevealRange` is the host's hook for "click a problem → jump to it" and is
reused by go-to-definition below.

### 3. Go-to-definition + link decorations
fycoder cannot open files (the host owns tabs/files), so it exposes a hook:
```go
// OnRequestNavigation fires on Cmd/Ctrl+click or the "Go to Definition"
// action. The host resolves the target (possibly cross-file) and calls
// RevealRange, or opens another document and reveals there.
func (e *Editor) OnRequestNavigation(func(row, col int))
```
Plus **link decorations**: when a `DefinitionProvider`-style hook reports that
the symbol under the cursor/mouse is navigable, draw a solid underline (Phase 1
span, `UnderlineStyle=Solid`) and switch the cursor to a pointer — matching
Monaco's Cmd-hover link affordance. pxfed's `provideDefinition` middleware and
`registry://` model pre-creation map directly onto this hook.

## Design notes
- `SetDiagnostics` / providers are LSP-shaped but LSP-free, same as Phase 2.
- Severity → color/icon goes through the theme (`theme.go`) so it's themable
  and consistent with the syntax palette.
- Multi-file navigation (pxfed's go-to-def into `.proto` / `registry://`
  sources) is the host's job; fycoder's contract ends at `OnRequestNavigation`
  + `RevealRange`.

## Acceptance criteria
- `SetDiagnostics` with a mix of severities renders aligned squiggles + gutter
  markers; hovering a squiggle shows the message.
- `RevealRange` scrolls to and selects an off-screen range.
- Cmd/Ctrl+click over a symbol fires `OnRequestNavigation` with the right
  position; a navigable symbol shows a link underline on hover.
- Clearing diagnostics removes all three surfaces.
