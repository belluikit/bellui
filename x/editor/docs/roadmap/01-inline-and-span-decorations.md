# Phase 1 — Inline & span decorations

**Size:** M · **Depends on:** fork F1 (rich `StyleRun`), F2 (`CaretRect`) ·
**Unlocks:** diagnostics squiggles (P3), anchored popups (P2), redaction
overlays (P4), semantic-token overlays.

> **Status: ✅ shipped** in [#4](https://github.com/belluikit/bellui/x/editor/pull/4)
> on fyne v2.9.0. Delivered: `TextRange`, `Editor.PositionAt` / `RangeRects`,
> the `SpanDecorator` / `DecoratedSpan` overlay path (background / box / opaque
> redaction / strike-through / solid+wavy+dotted underline), `StaticSpanDecorator`
> + `SearchMatchSpanDecorator`, and rich-`StyleRun` lightup in `applyHighlighter`.
> **Follow-ups — all merged:** span-level find — the find bar highlights matches
> as `SearchMatchSpanDecorator` spans with a distinct active-match color
> ([#7](https://github.com/belluikit/bellui/x/editor/issues/7) →
> [#8](https://github.com/belluikit/bellui/x/editor/pull/8)); and the gallery
> squiggle+redaction demo on the Code Editor page
> ([#6](https://github.com/belluikit/bellui/x/editor/issues/6) →
> [gallery#1](https://github.com/trendvidia/gallery/pull/1)).
> **Released as [v0.1.0](https://github.com/belluikit/bellui/x/editor/releases/tag/v0.1.0)**
> ([#9](https://github.com/belluikit/bellui/x/editor/issues/9)); the gallery now consumes
> the tag with no local `replace` ([gallery#3](https://github.com/trendvidia/gallery/pull/3)).
> Phase 1 is complete — next is [Phase 2](02-floating-ui-primitives.md)
> ([#10](https://github.com/belluikit/bellui/x/editor/issues/10)).

## Why this is the keystone

Today `RowDecorator` decorates **whole rows** (`decorator.go` — full editor
width). Almost every IDE feature pxfed needs from Monaco is actually a
**character-range** effect:

- diagnostics → squiggle under columns 12–18 of line 40
- secret redaction → opaque bar over the value span of a `password = "…"` line
- semantic tokens / link decorations → color or underline a single identifier
- find-match → highlight the matched substring, not the row

None of these are expressible with row decorators. Phase 1 adds a
**span decoration** primitive and the geometry to place it.

## Deliverables

### 1. `PositionAt` — logical → pixel mapping
Wrap fork F2:
```go
// PositionAt returns the on-screen rectangle of the character cell at
// (row, col), accounting for the current scroll offset. The zero
// Rectangle means the position is scrolled out of view.
func (e *Editor) PositionAt(row, col int) geo.Rectangle
func (e *Editor) RangeRects(r TextRange) []geo.Rectangle // one rect per visual row spanned
```
This is the foundation for both span decorations and Phase 2 popovers.

### 2. `TextRange` type
```go
// TextRange is a half-open [Start,End) range over the document,
// in 0-based (row, col). Shared by spans, protected ranges (P4),
// folds (P5), diagnostics (P3) and the LSP layer (P7).
type TextRange struct{ StartRow, StartCol, EndRow, EndCol int }
```

### 3. `SpanDecorator` interface + overlay support
Generalize the decorator overlay to render per-range objects, not just
per-row. Keep `RowDecorator` (it's still the right tool for current-line).
```go
type SpanDecorator interface {
    // Spans returns the ranges this decorator wants to draw, each with
    // a render style. Called on change/scroll; cheap to recompute.
    Spans() []DecoratedSpan
}

type DecoratedSpan struct {
    Range TextRange
    Kind  SpanKind   // Underline | Background | Box | StrikeThrough
    Color color.Color
    // Underline sub-style for squiggles (wavy) vs links (solid).
    UnderlineStyle UnderlineStyle
    // Opaque, when true, fully covers the text (redaction). The overlay
    // paints an opaque rectangle instead of a translucent wash.
    Opaque bool
}
```
The overlay (`decorator_overlay.go`) gains a span path that, for each
`DecoratedSpan`, calls `RangeRects` and emits canvas objects (rectangle for
background/box/opaque, wavy line for squiggle underline, line for strike).

Built-in implementations to ship:
- `SearchMatchSpanDecorator` — upgrade find-bar highlighting from row to span.
- `StaticSpanDecorator` — host supplies a fixed `[]DecoratedSpan` (the building
  block P3 diagnostics and P4 redaction sit on).

### 4. Rich-run rendering via the highlighter (fork F1)
With F1 landed, light up `StyleRun.BackgroundColor` / `Bold` / `Underline` in
`applyHighlighter()`. This gives highlighters (and the future semantic-token
provider) sub-line backgrounds and weighted/underlined text for free, without
a decorator.

## Out of scope / deferred
- Hover *interaction* over a span (that's P2/P3 — here spans are visual only,
  consistent with the existing input-transparent overlay).
- Animated decorations.

## Acceptance criteria
- A `StaticSpanDecorator` can draw a red wavy underline under an arbitrary
  `(row,col)–(row,col)` range and an opaque gray box over another, both
  staying aligned through vertical scroll and window resize.
- `PositionAt`/`RangeRects` return correct rects for multi-row ranges and the
  zero value when off-screen.
- Gallery "Code Editor" page demonstrates squiggle + redaction-box on sample
  text.
- Existing `RowDecorator` behavior is unchanged.
