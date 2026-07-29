# Phase 8 — General-editor extras *(stretch — beyond pxfed-parity)*

**Size:** XL (a basket of features; size per area below) · **Depends on:**
Phases 1–7 · **Status:** stretch. **pxfed does not require any of this.** These
are the Monaco features a *general-purpose* code editor is expected to have but
which pxfed's config/secret-editing workflow doesn't exercise. Pursue only if
fycoder is being positioned as a standalone VS Code-class editor for the wider
Bellui ecosystem.

This phase exists mainly to **document the Monaco gap explicitly** rather than
leave it implicit. Treat each area as an independently schedulable mini-project;
there is no need to do them in order or do all of them.

## Areas

### 8a — Multi-cursor & column selection · **L** · (fork)
Multiple simultaneous carets, add-cursor-above/below, add-cursor-at-next-match,
and column (block) selection. This is deep `Entry` work in the
[fork](00-fyne-fork-dependencies.md) — the current `Entry` models a single
caret + single selection. Highest-value general-editor feature; also the most
invasive.

### 8b — Code actions / quick-fixes · **M** · (LSP: `codeAction`)
The lightbulb affordance: surface server-provided actions at a range, apply the
resulting `WorkspaceEdit`. Needs a small anchored menu (reuse Phase 2 popover)
and a workspace-edit applier (multi-range, possibly multi-document via the host).

### 8c — Rename symbol · **M** · (LSP: `rename` / `prepareRename`)
Inline rename box at the symbol, apply the `WorkspaceEdit` across the document
(and other documents via the host). Optional preview list. Reuses 8b's edit
applier.

### 8d — Formatting · **S–M** · (LSP: `formatting` / `rangeFormatting` / `onTypeFormatting`)
Format document, format selection, and optional format-on-type / format-on-save.
Apply server text-edits. pxfed advertises this capability today but never
triggers it; here it becomes a real action + keybinding.

### 8e — Outline, breadcrumbs & symbols · **M** · (LSP: `documentSymbol`)
A symbol tree the host can render as an outline panel or breadcrumb bar, plus
"go to symbol in file." fycoder exposes the symbol model + a reveal hook; the
host owns the panel chrome (same division as the Phase 3 problems feed).

### 8f — References & peek · **L** · (LSP: `references`)
Find-all-references results, and **peek** — an inline, in-editor view showing a
definition/reference without leaving the current file. Peek is a "view zone"
(content inserted between lines), which the fork doesn't support today, so this
carries a fork dependency similar to folding's row machinery.

### 8g — Document highlights · **S** · (LSP: `documentHighlight`)
Highlight other occurrences of the symbol under the caret. Pure Phase 1 span
decorations driven by an LSP request — cheap once Phases 1/7 exist.

### 8h — Editor niceties · **M** (collectively)
The smaller Monaco touches, roughly in value order:
- **Snippets** with tab-stops/placeholders (insertion + tab navigation)
- **Inlay hints** (inline type/param annotations) — (LSP `inlayHint`)
- **Inline / ghost completions** (Copilot-style suggestion overlay)
- **Code lens** (actionable annotations above lines) — (LSP `codeLens`)
- **Bracket-pair colorization**, **indentation guides**, **sticky scroll**
- **Linked editing** (rename matching tags/identifiers in tandem) — (LSP `linkedEditingRange`)
- **Color decorators** + inline color picker — (LSP `documentColor`)

Each is small-to-medium individually; cherry-pick by demand.

## Cross-cutting deep gaps (call out, schedule separately)

These are not single features but qualitative parity gaps that surfaced in the
Monaco comparison; they belong here as explicit, separately-funded tracks:

- **TextMate grammar tokenization.** fycoder's `Highlighter` is per-line and
  stateful but simpler than Monaco's full TextMate engine (pxfed even has a TODO
  to migrate Monarch → `tmLanguage`). True highlighting parity means a TextMate
  grammar runner feeding `StyleRun`s. **Size: L.** Independent of LSP.
- **Accessibility.** Monaco has a mature screen-reader / ARIA mode and a deep
  command + keybinding system. Matching it depends substantially on the Bellui
  fork's accessibility baseline. **Size: L+, partly upstream.**

## Acceptance criteria
Each sub-area is independently "done" against its own demo in the gallery; there
is no single acceptance bar for the phase. The phase's purpose is documented
scope, not a monolithic deliverable.
