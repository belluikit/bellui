# Phase 4 — Protected ranges & secret redaction

**Size:** L · **Depends on:** Phase 1 (opaque span overlay), fork F3 (range-level
edit interception) · **Unlocks:** pxfed's `.sops.pxf` secret redaction with
edit-guard — one of pxfed's defining features.

> **Status: ✅ complete** ([#62](https://github.com/belluikit/bellui/x/editor/issues/62)),
> all three sub-issues shipped:
> - [#63](https://github.com/belluikit/bellui/x/editor/issues/63) — **protected ranges**
>   (`SetProtectedRanges`/`ProtectedRanges`/`OnProtectedEditAttempt`) via the fork's
>   `entry.BeforeChange` veto. ✅ ([#68](https://github.com/belluikit/bellui/x/editor/pull/68))
> - [#64](https://github.com/belluikit/bellui/x/editor/issues/64) — **redaction overlay +
>   reveal/re-mask** (`SetRedactions`/`SetRedactionMode`), opaque bars (Phase 1
>   `DecoratedSpan{Opaque:true}`) + caret-barring via `ProtectedRanges{CaretOpaque}`.
>   ✅ ([#68](https://github.com/belluikit/bellui/x/editor/pull/68))
> - [#65](https://github.com/belluikit/bellui/x/editor/issues/65) — **`ClassifySpanChanges`**
>   (port of pxfed `secret-changes.ts`). Pure helper. ✅ ([#67](https://github.com/belluikit/bellui/x/editor/pull/67))
>
> **F3 consumed via a pre-release pin:** F3 (pre-rebrand tracker #140)
> is merged to the `trendvidia/fyne` fork but **not yet in a tagged release**.
> Rather than wait, fycoder's go.mod pins a **pseudo-version of the fork commit**
> (`v2.9.1-0.…`); pre-rebrand tracker #63/#64 are validated locally (the project has no CI —
> `GOWORK=off go build/vet/test ./...` green against the pin via the Go proxy).
> Swap to the real tag (e.g. v2.10.0) with a one-line go.mod bump once fyne cuts
> it. A gitignored `go.work` remains as a local convenience pointing at a fork
> checkout. pre-rebrand tracker #65 was fork-independent and landed earlier with no pin.
>
> **Addressing is all rune-based** (verified): (row,col) columns are rune indices
> and F3 uses flat **rune offsets** into Text — the only conversion is a structural
> `(row,col) → offset` flatten computed in editor.
>
> **Enforcement split** (fork `editAllowed` checks `ProtectedRanges` before
> `BeforeChange`): plain protected ranges enforce via `BeforeChange` so
> `OnProtectedEditAttempt` can fire (caret still allowed); masked redactions use
> `ProtectedRanges{CaretOpaque}` (caret barred, silent veto — the bar covers them).
>
> Shipped order: pre-rebrand tracker #65 → pre-release pin → pre-rebrand tracker #63 → pre-rebrand tracker #64. Remaining: swap the pin for
> the fyne v2.10.0 tag when released; gallery demo (deferred, post-release).

pxfed protects decrypted secret values with a **three-layer edit guard**
(`secret-edit-guard.ts`): a keydown blocker, a capture-phase IME-composition
blocker, and a post-edit `onDidChangeContent` undo of any mutation that slipped
through (paste, drag-drop, find-replace). It also renders **opaque redaction
bars** over secret spans with a reveal/re-mask toggle (`secret-decorations.ts`).

In Monaco those layers are bolted on from outside the editor. In Bellui the only
robust place to *reject* an edit is inside the `Entry` itself (fork F3) — an
external observer can only undo after the fact, which flickers and races. So
Phase 4 is split: the fork enforces, fycoder models and renders.

## Deliverables

### 1. Protected ranges API
```go
// SetProtectedRanges marks document ranges as read-only. Edits whose
// effect would fall inside (or partially overlap) a protected range are
// rejected before they apply. Ranges move with surrounding edits.
func (e *Editor) SetProtectedRanges([]TextRange)

// OnProtectedEditAttempt fires when an edit was rejected, so the host
// can surface a hint ("this value is encrypted — unlock to edit").
func (e *Editor) OnProtectedEditAttempt(func(r TextRange))
```
Backed by fork F3's `BeforeChange` veto / `ProtectedRanges` enforcement. This
single mechanism replaces all three of pxfed's JS guard layers, because the
veto sits on the one code path every mutation (key, paste, IME commit, drag,
find-replace) funnels through. Undo/redo respect it because rejected edits
never enter the history.

### 2. Redaction overlay + reveal/re-mask
```go
type RedactionMode int // Masked | Revealed

// SetRedactions marks ranges to visually mask. In Masked mode the overlay
// paints an opaque bar (Phase 1 DecoratedSpan{Opaque:true}) over each
// range and the caret is barred from entering it. In Revealed mode the
// text shows normally (still protected from edits unless unprotected).
func (e *Editor) SetRedactions(ranges []TextRange)
func (e *Editor) SetRedactionMode(RedactionMode)
```
- Masked: opaque rectangle (theme-colored) over the span; caret skips it
  (fork F3 caret-barring).
- Revealed: bar removed, text visible; toggling is a pure re-render, no
  document mutation — mirroring pxfed's mode sync "without re-attach".
- Redacted ranges are typically *also* protected; the two APIs compose but are
  independent (a host may reveal-but-keep-protected).

### 3. Change classification helper (port of `secret-changes.ts`)
pxfed diffs the open-time secret snapshot against the current buffer to
classify each secret as modified/added/removed and drive a save-confirm modal.
This is pure range/text logic with no Bellui dependency — port it as a small
helper so any host gets it:
```go
func ClassifySpanChanges(original, current string, spans []TextRange) []SpanChange
```

## Design notes
- The hard correctness bar is **IME**: the fork veto must reject a composition
  *commit* that targets a protected range, not just printable keystrokes.
  pxfed needed a dedicated capture-phase listener for exactly this; F3 must
  cover it. Call it out as the key fork-test case.
- Keep redaction (visual) and protection (editability) as separate flags — they
  coincide for secrets but are conceptually orthogonal and other hosts will
  want one without the other.

## Acceptance criteria
- Typing, pasting, dragging, find-replacing, and IME-composing into a protected
  range are all rejected; `OnProtectedEditAttempt` fires; undo history is clean.
- Masked redactions show opaque bars and bar the caret; `SetRedactionMode`
  toggles visibility with no document change and no `OnChanged`.
- `ClassifySpanChanges` correctly reports modified/added/removed against a
  snapshot (unit-tested, no app needed).
