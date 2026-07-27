# Bellui X — editor (Bellui Editor)

A composable code editor widget for [Fyne](https://bellui.io) applications.

fycoder provides pluggable syntax highlighting, a customizable gutter (line numbers, breakpoints, annotations), row-level decorations, and a built-in find bar. The gutter renders as a separate widget outside the text scroll viewport, so it stays pinned when scrolling horizontally — matching the behavior of production code editors.

## Quick Start

```go
import (
    "github.com/belluikit/bellui/x/editor"
    "github.com/belluikit/bellui/x/editor/highlight/starlark"
)

editor := editor.NewEditor()
editor.Highlighter = starlark.New()
editor.Gutter = editor.NewCompositeGutter(
    editor.NewLineNumberGutter(),
    editor.NewBreakpointGutter(func(lines map[int]bool) {
        fmt.Println("breakpoints:", lines)
    }),
)
editor.Decorators = []editor.RowDecorator{
    editor.NewCurrentLineDecorator(0),
}

editor.OnChanged = func(text string) { /* user edited */ }
editor.OnSave = func() { /* Cmd+S pressed */ }

window.SetContent(editor)
editor.Focus() // enable keyboard shortcuts immediately
```

## Architecture

Three independent, composable interfaces:

| Interface | Purpose | Built-in implementations |
|-----------|---------|------------------------|
| `Highlighter` | Per-line syntax highlighting | `HighlighterFunc`, `highlight/starlark` |
| `GutterProvider` | Gutter area left of text | `LineNumberGutter`, `BreakpointGutter`, `DiagnosticGutter`, `FoldGutter`, `CompositeGutter` |
| `RowDecorator` | Row background/overlay effects | `CurrentLineDecorator`, `SearchHighlightDecorator` |

These are fully decoupled — you can have line numbers without highlighting, or highlighting without a gutter.

### Gutter columns

`NewCompositeGutter` arranges its providers in the conventional editor order, whatever order you pass them in:

```
[ markers ][ line numbers ][ fold ]  code…
```

Markers (breakpoints, diagnostics, and any provider that does not declare a column) take the leading edge, the line numbers sit next to the code, and only the fold control comes between them. A provider declares its column by implementing `GutterColumnProvider`; use `NewCompositeGutterInOrder` for a bespoke layout the conventional order does not express.

## Features

- **Syntax highlighting** with multi-line state tracking (triple-quoted strings, block comments)
- **Line number gutter** with configurable minimum digit width
- **Breakpoint markers** with click-to-toggle and callback
- **Find bar** — Cmd+F to open, Cmd+G / Shift+Cmd+G for next/prev, case-sensitive toggle
- **Row decorators** — current line highlight, search match highlight, or custom
- **Theme integration** — semantic color names with built-in Monokai-inspired fallbacks
- **Rich StyleRun** — supports ForegroundColor, BackgroundColor, Bold, Italic, Underline

## Adding a Language Highlighter

Implement the `Highlighter` interface:

```go
type Highlighter interface {
    Highlight(line string, state int) ([]StyleRun, int)
}
```

`state` is 0 for the first line and the previous line's returned state thereafter, enabling multi-line constructs. See `highlight/starlark` for a complete example.

Use the semantic color names (`ColorNameSyntaxKeyword`, `ColorNameSyntaxString`, etc.) so themes can provide consistent syntax coloring across languages.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Cmd+F | Open find bar |
| Cmd+G | Next match |
| Shift+Cmd+G | Previous match |
| Escape | Close find bar |
| Cmd+S | Save (fires `OnSave`) |

Shortcuts require the editor to have keyboard focus. Call `editor.Focus()` after adding to the window, or the user can click in the editor.

## Demo

```bash
# Demo lives in the gallery app (../gallery):
go run ../gallery
```

## Dependencies

Requires the [trendvidia/fyne](https://github.com/trendvidia/fyne) fork, which adds `OnScrolled`, `SuppressGutter`, and `RowMetrics()` to `widget.Entry` for external gutter synchronization.
