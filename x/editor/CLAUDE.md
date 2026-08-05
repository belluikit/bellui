# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./...                          # build all packages
go test -race -count=1 ./...            # run all tests with race detector
go test -race -run TestName ./...       # run a single test
go vet ./...                            # static analysis
go run ../gallery                        # demo lives in the gallery app ("Code Editor" page)
```

Tests that call Bellui APIs (MeasureText, widget constructors) require `test.NewTempApp(t)` at the start of the test function.

## Architecture

Composable code editor widget for Bellui. Three independent, pluggable axes:

- **`Highlighter`** (`style.go`) — per-line syntax highlighting via `StyleRun` tokens. Stateful across lines (state flows line-to-line for multi-line strings/comments).
- **`GutterProvider`** (`gutter.go`) — renders the area left of text. `InteractiveGutter` adds click handling. `CompositeGutter` stacks multiple providers side by side, arranged by the column each declares via `GutterColumnProvider` — markers (breakpoints, diagnostics, undeclared host providers) on the leading edge, then the line numbers, then the fold control beside the text — regardless of the order they were passed. `NewCompositeGutterInOrder` opts out and renders them verbatim. Column hit-testing (`TappedAtX`) walks the same arranged order.
- **`RowDecorator`** (`decorator.go`) — row-level background/overlay effects (current line highlight, search matches).

### Editor composition

```
Editor
 └─ root (Border: findBar on top, hbox in center)
     └─ hbox (Border: gutterWidget on left, center content)
         └─ center (Stack: entry + decoratorOverlay on top)
```

Key design: the **gutter is a separate widget** outside the Entry's scroll viewport. It syncs vertically via `Entry.OnScrolled` but does not scroll horizontally — this is why line numbers stay pinned.

### Sync mechanism

`syncExternalWidgets()` pushes Entry's text and `RowMetrics()` (topOffset, stride) to the gutter widget and decorator overlay on every text change and scroll event. `RowMetrics()` returns cached row geometry from the Entry's internal RichText after layout, with a theme-derived fallback before the first layout pass.

### Bellui fork dependency

Depends on `github.com/belluikit/bellui` (pinned release in go.mod — no `replace`; CI injects an ephemeral one against the checkout, never committed). The fork adds three fields to `widget.Entry`:
- `OnScrolled` — fires on scroll offset changes
- `SuppressGutter` — disables built-in gutter when external gutter is active
- `RowMetrics()` — exposes exact row Y positions for alignment

### StyleRun conversion

`Editor.applyHighlighter()` converts fycoder's rich `StyleRun` (supports Bold, Underline, BackgroundColor) to the fork's simpler `widget.StyleRun` (only ColorName + Italic). Extra fields activate as the fork adds support.

## Sub-packages

- **`highlight/starlark`** — Starlark/Python syntax highlighter. `New()` creates with default builtins; `AddBuiltins(...)` registers domain-specific names. Designed as a template for adding other language highlighters.
- The reference demo lives in the gallery app (`../gallery`, "Code Editor" page).

## Key patterns

- The `Gutter`, `Decorators`/`SpanDecorators` and `Highlighter` fields are applied at first
  render (`CreateRenderer`). To change them on a live editor call the re-apply methods —
  `ApplyGutter`, `ApplyDecorators`, `RefreshHighlighter` (#281); `RefreshGutter`/
  `RefreshDecorators` only repaint the installed objects (use those after mutating a
  retained provider/decorator's state).
- `SetText()` suppresses `OnChanged` via a `suppressChange` flag — only user edits fire the callback.
- Gutter and decorator renderers call `Refresh()` from `Layout()` when size changes, because the initial `Refresh()` runs before `Layout()` gives the widget its real dimensions.
- `themeColor()` asks the app theme first — defining the `editor.*` names in an app theme restyles the editor (#150). A nil or `color.Transparent` answer (the toolkit's unknown-name sentinel) falls back to the built-in Monokai-inspired defaults.
