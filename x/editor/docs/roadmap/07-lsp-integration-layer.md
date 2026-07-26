# Phase 7 — LSP integration layer (`fycoder/lsp`)

**Size:** L · **Depends on:** Phases 1–6 · **Unlocks:** the "batteries-included"
path — a host points fycoder at a language server and gets completion, hover,
diagnostics, definition, folding, and incremental sync wired automatically.
This is what lets pxfed connect to **protolsp** with a few lines instead of
re-implementing every provider.

## Principle: opt-in, separate package

The fycoder **core stays LSP-free** (Design Principle 1). All LSP code lives in
a new sub-package, `github.com/belluikit/bellui/x/editor/lsp`, that depends on the core
but not vice versa. A plain editor never imports it; an IDE does. This mirrors
how pxfed keeps Monaco and `monaco-languageclient` as separable layers today.

## Deliverables

### 1. Transport-agnostic client binding
```go
package lsp

// Bind connects a language-server connection to an Editor, wiring every
// provider/hook the editor exposes. The conn is any JSON-RPC transport —
// in-process pipe, stdio, or socket — so the host chooses how to reach
// the server.
func Bind(e *editor.Editor, conn jsonrpc.Conn, opts Options) (*Session, error)
```
`Options` carries the document URI, language id, workspace folder, and an
`InitializationOptions`/`configuration` callback — the same surface pxfed feeds
protolsp via `workspace/configuration` middleware today (registry
address/namespace/token/cacheDir).

### 2. Feature wiring
`Bind` installs adapters that translate between LSP and the Phase 1–6 hooks:

| LSP | fycoder hook |
|---|---|
| `publishDiagnostics` | `Editor.SetDiagnostics` (P3) |
| `completion` (+ `resolve`) | `CompletionProvider` (P2) |
| `hover` | `HoverProvider` (P2) |
| `signatureHelp` | `SignatureHelpProvider` (P2) |
| `definition` / `declaration` | `OnRequestNavigation` → `Location` → host (P3) |
| `documentLink` / definition-on-hover | link decorations (P3) |
| `foldingRange` | `SetFoldRegions` (P5) |
| `didOpen`/`didChange`/`didClose` | `OnDocumentChange` + `Document` version (P6) |
| `window/showMessage` | `Options.OnMessage` callback → host toasts |
| `semanticTokens` *(optional)* | span decorations / rich `StyleRun` (P1) |

### 3. Multi-document & navigation routing
Definition results often target *another* document (pxfed's go-to-def into
`.proto` or `registry://` schema sources). The LSP layer resolves the
`Location` and hands the host a `(uri, range)` to open + reveal — it does **not**
open files itself (the host owns tabs/files). This is the exact contract pxfed's
`openEditorFallback` implements against Monaco.

### 4. Reference adapters
- A small example binding `fycoder/lsp` to a stdio language server in the
  gallery, proving the loop end-to-end against a real server.
- Document the **protolsp** wiring as the canonical example (in-process or piped
  Connect/JSON-RPC), since that's the driving consumer.

## Design notes
- Keep adapters thin: Phases 2/3/5/6 deliberately shaped their provider types
  (positions, replace ranges, markdown, folding ranges, incremental changes) to
  match LSP, so this layer should be translation, not logic.
- Position encoding: LSP uses UTF-16 code units for columns. The adapter owns
  the UTF-16 ↔ rune/byte conversion so the core can stay in simple
  `(row, col)` rune coordinates. Call this out — it's a classic source of
  off-by-one bugs.
- Configuration is dynamic: support `workspace/didChangeConfiguration` and the
  pull-based `workspace/configuration` so settings changes reach the server
  without a restart — pxfed depends on this to push registry settings live.

## Acceptance criteria
- `Bind` against a real language server yields working completion, hover,
  diagnostics, and go-to-definition in the gallery demo with no per-feature
  host code.
- Editing produces correct incremental `didChange` with UTF-16 positions and
  increasing versions.
- A definition into a second document surfaces a `(uri, range)` to the host.
- The core `fycoder` package still builds and tests with no dependency on
  `fycoder/lsp`.
