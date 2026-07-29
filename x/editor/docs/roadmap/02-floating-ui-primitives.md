# Phase 2 — Floating UI primitives (completion, hover, signature help)

**Size:** L · **Depends on:** Phase 1 (`PositionAt`/`RangeRects`) ·
**Unlocks:** completion, hover, signature help in pxfed and any fycoder host.

> **Status: ✅ complete** ([#10](https://github.com/belluikit/bellui/x/editor/issues/10)),
> split into four sub-issues, all shipped:
> - **Deliverable 1 — anchored popover infrastructure: ✅ shipped**
>   ([#13](https://github.com/belluikit/bellui/x/editor/issues/13) /
>   [#17](https://github.com/belluikit/bellui/x/editor/pull/17)). `popover.go` is the
>   primitive (`popoverPosition` flips above/below and clamps left/right;
>   `popoverLayer` re-anchors on resize); the editor floats it in its own center
>   Stack via `showPopoverAt` / `dismissPopover` / `repositionPopover`, routing
>   keys (opt-in `keyHandler`; Escape dismisses), outside clicks, focus loss, and
>   scroll to the active popover. **Key decision:** popovers render in-tree, not
>   on `canvas.Overlays()` — a `widget.PopUp` pushes its own focus manager and
>   stops keystrokes reaching the Entry, but completion must keep the Entry
>   typable. Follow-ups shipped ([#69](https://github.com/belluikit/bellui/x/editor/pull/69)):
>   popovers now clamp to the **host canvas content area** instead of the editor
>   viewport (`popoverBounds` translates the canvas size by the editor's offset),
>   so completion/hover lists overflow a compact/embedded editor into the window
>   while staying in-tree; nothing in the editor's own tree clips so the overflow
>   renders and stays interactive (a host wrapping the editor in a `container.Scroll`
>   clips to that scroll's viewport), with `Editor.MaxCompletionRows` bounding a
>   long list ([#19](https://github.com/belluikit/bellui/x/editor/issues/19)); and the
>   surface gained a soft elevation shadow
>   ([#20](https://github.com/belluikit/bellui/x/editor/issues/20)). Both later upgraded
>   ([#74](https://github.com/belluikit/bellui/x/editor/pull/74)) to use fork helpers:
>   a true GPU-blurred `canvas.BoxShadow` elevation
>   ([#72](https://github.com/belluikit/bellui/x/editor/issues/72)) and
>   `driver.ClipBoundsForObject` to clamp overflow to the nearest clipping
>   ancestor for hosts that wrap the editor in a scroll
>   ([#73](https://github.com/belluikit/bellui/x/editor/issues/73), fork
>   [trendvidia/fyne#171](https://github.com/trendvidia/fyne/pull/171)).
> - **Deliverable 2 — completion: ✅ shipped**
>   ([#14](https://github.com/belluikit/bellui/x/editor/issues/14) /
>   [#21](https://github.com/belluikit/bellui/x/editor/pull/21)). `completion.go` adds
>   `CompletionProvider` / `CompletionItem` / `Editor.SetCompletionProvider` and a
>   `completionController` that triggers on identifier typing and Ctrl+Space,
>   queries the provider once per word and **live fuzzy-filters** client-side as
>   the user types; Arrow navigates, Tab/Enter accept (applying `Replace`,
>   defaulting to the typed prefix, then firing `OnChanged`), Esc dismisses. The
>   popup is a `widget.List` inside the popover; the Entry keeps focus.
>   `WordListCompletionProvider` is the demo provider. Follow-ups shipped
>   ([#75](https://github.com/belluikit/bellui/x/editor/pull/75)): **trigger characters**
>   ([#23](https://github.com/belluikit/bellui/x/editor/issues/23)) —
>   `Editor.SetCompletionTriggerCharacters(chars ...rune)` opens completion on a
>   language character (e.g. `.`) with `CompletionTriggerCharacter` at the cursor,
>   empty prefix anchored there; the typed-identifier and Ctrl+Space paths are
>   unchanged. A provider may also declare its own triggers via the optional
>   `CompletionTriggerProvider` interface (unioned with the editor-level set,
>   consulted on demand so an async LSP bridge can report them after its
>   handshake — [#76](https://github.com/belluikit/bellui/x/editor/issues/76),
>   [#80](https://github.com/belluikit/bellui/x/editor/issues/80); via
>   [#78](https://github.com/belluikit/bellui/x/editor/pull/78),
>   [#81](https://github.com/belluikit/bellui/x/editor/pull/81)). The completion list and
>   the signature card now show **simultaneously** at a call site — the float
>   layer holds one interactive popover (completion/hover) plus a non-interactive
>   companion (the signature card, placed above the line while completion sits
>   below); when there is too little room for both, the companion yields and
>   completion stays primary ([#77](https://github.com/belluikit/bellui/x/editor/issues/77),
>   [#79](https://github.com/belluikit/bellui/x/editor/issues/79); via
>   [#81](https://github.com/belluikit/bellui/x/editor/pull/81)). A **documentation side
>   panel** ([#24](https://github.com/belluikit/bellui/x/editor/issues/24); via
>   [#82](https://github.com/belluikit/bellui/x/editor/pull/82)) renders
>   `CompletionItem.Documentation` (markdown) to the right of the list, updating as
>   the selection moves; it is reserved only when some candidate carries docs, so a
>   docless list keeps its size. The panel grows to a minimum height so a long doc
>   beside a short list stays readable
>   ([#83](https://github.com/belluikit/bellui/x/editor/issues/83)), and its content is
>   plain text by default — `CompletionItem.DocumentationMarkdown` opts into
>   markdown rendering ([#84](https://github.com/belluikit/bellui/x/editor/issues/84)) —
>   both via [#85](https://github.com/belluikit/bellui/x/editor/pull/85). PageUp/PageDown
>   and Ctrl/Cmd+Up/Down scroll the doc panel while the list is open
>   ([#86](https://github.com/belluikit/bellui/x/editor/issues/86),
>   [#89](https://github.com/belluikit/bellui/x/editor/issues/89)), and a shared
>   `MarkupKind` (`markup.go`) backs plaintext-vs-markdown rendering across all
>   three surfaces — the hover card (`HoverContent.PlainText`,
>   [#90](https://github.com/belluikit/bellui/x/editor/issues/90)), the doc panel, and
>   signature docs (a `DocumentationMarkdown` opt-in matching completion,
>   [#87](https://github.com/belluikit/bellui/x/editor/issues/87)); via
>   [#88](https://github.com/belluikit/bellui/x/editor/pull/88) and
>   [#91](https://github.com/belluikit/bellui/x/editor/pull/91). The hover card body is
>   assembled as rich-text segments (not a composed markdown string), so diagnostic
>   messages — plain text per LSP — render verbatim while their dimmed `(source)`
>   stays styled and the provider's markdown is still parsed
>   ([#92](https://github.com/belluikit/bellui/x/editor/issues/92); via
>   [#93](https://github.com/belluikit/bellui/x/editor/pull/93)). Deferred: gallery demo
>   ([#22](https://github.com/belluikit/bellui/x/editor/issues/22), post-release).
>   `Complete` is synchronous for now — async providers arrive with the Phase 7
>   LSP layer.
> - **Deliverable 3 — hover: ✅ shipped**
>   ([#15](https://github.com/belluikit/bellui/x/editor/issues/15) /
>   [#28](https://github.com/belluikit/bellui/x/editor/pull/28)). `hover.go` adds
>   `HoverProvider` / `HoverContent` / `Editor.SetHoverProvider`. A transparent
>   `hoverCapture` layer (`event.Hoverable` + `Cursorable`) detects pointer dwell;
>   `Editor.positionAtPoint` (inverse of `PositionAt`, via `RowPositions`+`CaretRect`
>   — **no new fork dep**) resolves the position; a short dwell timer shows a
>   `widget.RichText` markdown card through the popover (flips near edges), outlines
>   the hovered range (`SpanBox`), and dismisses on move-away / pointer-out.
>   `WordHoverProvider` is the demo provider. Follow-ups shipped
>   ([#70](https://github.com/belluikit/bellui/x/editor/pull/70)): the card is now
>   interactive/scrollable — wrapped in a `Hoverable` card over a vertical scroll
>   with a grace timer bridging the token→card gap so it stays alive to be scrolled
>   ([#27](https://github.com/belluikit/bellui/x/editor/issues/27)); and
>   `Editor.ShowHoverAtCursor()` adds a keyboard trigger bound to the VS Code chord
>   Ctrl+K Ctrl+I ([#29](https://github.com/belluikit/bellui/x/editor/issues/29)).
>   Deferred: gallery demo ([#22](https://github.com/belluikit/bellui/x/editor/issues/22)).
> - **Deliverable 4 — signature help: ✅ shipped**
>   ([#16](https://github.com/belluikit/bellui/x/editor/issues/16) /
>   [#31](https://github.com/belluikit/bellui/x/editor/pull/31)). `signature.go` adds
>   `SignatureHelp` / `SignatureInformation` / `ParameterInformation`,
>   `SignatureHelpProvider`, and `Editor.SetSignatureHelpProvider`. A
>   `signatureController` shows a `widget.RichText` card in the popover, refreshed
>   when the character before the cursor is `(` or `,` (and on every keystroke
>   while visible, so the active parameter tracks typing); it dismisses when the
>   provider returns nil (cursor left the call) and **yields the popover to
>   completion** while a list is open. The active parameter is rendered inline-bold
>   in the primary color, with optional docs beneath. `callContext` (a backward,
>   paren-depth-aware scan resolving the enclosing call name + active-argument
>   index, across nesting and lines) and `splitSignature` (locating the active
>   parameter's span) back the `FuncSignatureProvider` demo. Typing/arrows fall
>   through to the Entry and Escape dismisses via the editor.
>   Follow-ups shipped ([#36](https://github.com/belluikit/bellui/x/editor/pull/36)):
>   keyboard invoke `Editor.ShowSignatureHelpAtCursor()` on Ctrl+Shift+Space
>   ([#34](https://github.com/belluikit/bellui/x/editor/issues/34)); overload cycling
>   ([#33](https://github.com/belluikit/bellui/x/editor/issues/33)); and exact
>   active-parameter spans via optional `ParameterInformation` rune offsets,
>   falling back to substring matching ([#35](https://github.com/belluikit/bellui/x/editor/issues/35)).
>   Overload UX ([#40](https://github.com/belluikit/bellui/x/editor/pull/40)): a cycled
>   overload now persists while the same call stays in view
>   ([#38](https://github.com/belluikit/bellui/x/editor/issues/38)), and the counter is a
>   clickable **▲ N/M ▾** control (non-focusable tappables that cycle without
>   blurring the Entry) ([#39](https://github.com/belluikit/bellui/x/editor/issues/39)).
>   Deferred: gallery demo ([#22](https://github.com/belluikit/bellui/x/editor/issues/22)).
>   `Signatures` is synchronous for now — async providers arrive with the Phase 7
>   LSP layer.

Monaco gives pxfed completion lists, hover cards, and signature help for free
from the language server. fycoder has **no floating UI** today. This phase
builds the anchored-popover infrastructure and the three concrete surfaces on
top, each driven by a host-supplied provider so the core stays LSP-agnostic.

## Deliverables

### 1. Anchored popover infrastructure
A reusable overlay anchored to a document position (via `PositionAt`) or the
mouse, that:
- positions itself adjacent to the anchor, flipping above/below and
  left/right to stay on-screen,
- tracks the anchor on scroll/resize (reposition or dismiss),
- dismisses on Escape, focus loss, or anchor scroll-off,
- renders over the editor without stealing text focus unless it needs keys.

This is internal plumbing reused by all three surfaces below. Likely backed by
Bellui's `widget.PopUp`/canvas overlay, wrapped for position tracking.

### 2. Completion
```go
// CompletionProvider is asked for completions at a position. Returning
// fast/async is fine — the editor shows results when the future resolves.
type CompletionProvider interface {
    Complete(ctx context.Context, row, col int, trigger CompletionTrigger) ([]CompletionItem, error)
}

type CompletionItem struct {
    Label, Detail, Documentation string
    InsertText  string
    Replace     TextRange   // range to replace on accept
    Kind        CompletionKind
    SortText    string
}

func (e *Editor) SetCompletionProvider(p CompletionProvider)
```
Behavior: trigger on typing / Ctrl+Space / trigger chars; live fuzzy filter as
the user keeps typing; arrow/Tab/Enter to navigate and accept; Esc to dismiss;
optional detail/documentation side panel; accept applies `Replace` then fires
`OnChanged`.

### 3. Hover
```go
type HoverProvider interface {
    Hover(ctx context.Context, row, col int) (*HoverContent, error)
}
type HoverContent struct {
    Markdown string
    Range    TextRange // optional: range to outline while hovering
}
func (e *Editor) SetHoverProvider(p HoverProvider)
```
Behavior: on mouse dwell over a position (and on keyboard "show hover"), query
the provider, render markdown in a popover; dismiss on move-away. Reuses a
markdown renderer (Bellui `widget.RichText` from markdown, already in the dep
tree via goldmark).

### 4. Signature help *(stretch within this phase)*
```go
type SignatureHelpProvider interface {
    Signatures(ctx context.Context, row, col int) (*SignatureHelp, error)
}
```
Small popover showing the active signature + parameter, refreshed on `(` and
`,`. Lowest priority of the three; ship if budget allows, else defer to P7.

## Design notes
- All providers are **optional and nil-safe** — a plain editor sets none.
- Providers are intentionally LSP-shaped (positions, replace ranges, markdown)
  so the Phase 7 `fycoder/lsp` bridge is a thin adapter, but they carry **no
  LSP types** — a host can implement them from a static dictionary, ctags, or
  anything.
- Keyboard ownership is the subtle part: completion must intercept
  Arrow/Tab/Enter/Esc while open without breaking normal typing. Centralize
  this in the popover infra.

## Acceptance criteria
- A demo `CompletionProvider` backed by a word list shows a filtering popup
  that inserts on Enter and replaces the typed prefix.
- A demo `HoverProvider` renders a markdown card on dwell, correctly anchored
  and flipping near screen edges.
- Popovers track/dismiss correctly across scroll, resize, and focus changes.
- With no providers set, editor behavior is identical to today.
