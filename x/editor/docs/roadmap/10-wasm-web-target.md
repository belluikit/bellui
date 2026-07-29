# Phase 10 — WASM / web target *(cross-cutting feasibility target)*

**Size:** see effort table · **Depends on:** Phases 1–7 (the editor core), and a
host that links its backend in-process or over a socket · **Status:**
exploratory target, not on the pxfed-parity critical path.

**Tracking:** [trendvidia/fyne#95](https://github.com/trendvidia/fyne/issues/95)
(umbrella — "make the WebGL backend render a real UI") ·
[trendvidia/gl-js#1](https://github.com/trendvidia/gl-js/pull/1)
(fix for blocker #1) · harness: [`examples/wasm`](../../examples/wasm).

This document records whether — and how — a pxfed-class app built on fycoder
could run in a **browser via `GOOS=js GOARCH=wasm`**, and what fycoder
specifically owes that goal. It is the durable form of a feasibility analysis;
treat the numbers as estimates pending the spike below.

## Verdict

**Possible — and the native-Go stack is what makes it possible.** An
Electron/Monaco app cannot run in a sandboxed browser tab (it relies on Node
subprocesses + local filesystem). A pure-Go app can, because every piece that is
a native subprocess today — the protolsp language server, sops, age — is itself
Go and can either be **linked into the WASM binary** or **served behind a
socket**. The subprocess model that makes the desktop app browser-hostile
disappears.

## Evidence (from the trendvidia tree, at time of writing)

- **Bellui's fork already targets WASM** — `internal/driver/glfw/device_wasm.go`,
  `app/app_wasm.go`, `app/preferences_wasm.go`; deps include `fyne-io/gl-js`
  (WebGL canvas), `hack-pad/go-indexeddb`, `hack-pad/safejs`. Preferences
  already persist to `localStorage` under WASM.
- **fycoder is pure Go + Bellui** — no cgo, no syscalls; it compiles for `js/wasm`
  as-is (modulo the input-layer caveat below).
- **protolsp has no cgo and no sqlite**, and `internal/server/` is a
  library-shaped handler set (callable in-process, not only as a binary).
- **protolsp already ships a WebSocket transport** (`cmd/protolsp-ws`,
  `internal/transport/websocket.go`) — browser clients were anticipated.
- **sops has a pure-Go age path** (`filippo.io/age`); pxfed's age keygen is
  already pure.

## Empirical findings (spike, 2026-06-23)

A `GOOS=js` spike against the current tree (fyne fork v2.8.3) produced concrete
results, recorded here so they aren't lost:

- ✅ **fycoder compiles to WASM unmodified.** `GOOS=js GOARCH=wasm go build`
  on the package tree is clean — no cgo, no wasm-hostile syscalls.
- ✅ **A minimal editor app links to a runnable `.wasm`** — **38 MB raw /
  11.2 MB gzipped** (harness in [`examples/wasm/`](../../examples/wasm)).
- ⚠️ **The gallery app fails the WASM build** — but on `go.etcd.io/bbolt` +
  `blevesearch/mmap-go` (`syscall.Flock`, `mmap`) pulled in by *non-fycoder*
  pages, not the editor. Confirms the blocker pattern: the editor is web-ready;
  the work is curating out host-side fs/syscall deps via a dedicated web
  entrypoint.
- ⛔ **BLOCKER — the fyne fork's WASM renderer panics on first paint.** Once
  served in a browser, the page loads, the IndexedDB repository registers, and
  the window is created — then it crashes:
  ```
  panic: runtime error: index out of range [0] with length 0
    github.com/fyne-io/gl-js.sliceToBytes  (gl_webgl_convertions_wasm.go:17)
    github.com/fyne-io/gl-js.TexImage2D    (gl_webgl_wasm.go:553)
    .../internal/painter/gl.(*painter).EnsureCanvasFBO  (canvas_fbo.go:58)
  ```
  **Root cause (typed-nil-in-interface):** `EnsureCanvasFBO` allocates the
  canvas framebuffer texture with `TexImage2D(..., nil)` — a nil `[]uint8`,
  which is valid on desktop GL (allocates uninitialized storage). That nil
  slice is boxed into gl-js's `TexImage2D(data any)`; a *typed* nil slice in an
  `any` is **not** `== nil`, so gl-js's `SliceToTypedArray` nil-guard misses it,
  falls into `case []uint8:`, and `sliceToBytes` indexes `&s[0]` on a
  zero-length slice → panic. `fyne-io/gl-js` publishes only v0.1.0 and v0.2.0,
  so there is no version bump that fixes it.

  **FIXED** in the `trendvidia/gl-js` fork (PR trendvidia/gl-js#1):
  `SliceToTypedArray` now detects an empty/typed-nil slice (via `reflect`) and
  returns `js.Null()`, so `texImage2D(..., null)` allocates the FBO storage as
  on desktop GL. The [`examples/wasm`](../../examples/wasm) harness wires the fix
  in via a `replace` and builds clean.

  **Remaining step (separate repo):** `trendvidia/fyne` v2.8.3 currently
  requires *upstream* `fyne-io/gl-js v0.2.0` with **no `replace`**, so stock
  Bellui apps don't pick up the fork. Once gl-js#1 merges + tags, the fyne fork
  needs `replace github.com/fyne-io/gl-js => github.com/trendvidia/gl-js vX`
  for any Bellui app to render under wasm without each consumer adding the replace
  itself. Until then, each app's main module carries the `replace` (as the
  harness does).

  **Note on verification:** the fix is confirmed at the compile/link level and
  by root-cause analysis (the empty-slice path now yields `null`, the value
  `texImage2D` requires). In-browser, it does resolve the panic — the renderer
  now proceeds to paint, surfacing the next defect below.

- ⛔ **BLOCKER #2 — the fork's WebGL backend is not WebGL-conformant.** With
  the FBO fix in place, the browser fills with:
  ```
  WebGL: INVALID_OPERATION: drawArrays: no buffer is bound to enabled attribute
    .../internal/painter/gl.(*painter).drawText -> drawTextureWithDetails -> DrawArrays
  ```
  and **no text / textured geometry renders.** Root cause is structural, not a
  one-line bug: the painter (`internal/painter/gl`) enables each program's
  vertex attributes **once at program creation** (`enableAttribArrays` in
  `gl_wasm.go`), **never calls `DisableVertexAttribArray`, and uses no VAOs.**
  Desktop GL tolerates leftover enabled attributes; **WebGL does not** —
  vertex-attribute-array enable state is global, so after one program enables
  attributes at locations 0–1, a later draw with a different attribute layout
  leaves an enabled attribute with no bound buffer → `INVALID_OPERATION` on
  every `drawArrays`.

  **Fix scope:** per-draw, enable exactly the current program's attributes and
  disable the rest (or adopt WebGL2 VAOs per program). This touches every draw
  path in the fork's painter and must be verified to not regress desktop GL —
  i.e. real fyne-fork WebGL-backend work, not a dependency patch.

### Revised verdict for the web target

The earlier verdict ("possible; gated on IME polish") was too optimistic about
the rendering layer. Corrected picture from the spike:

- The **application stack is web-ready**: fycoder and a full editor app compile
  and link to `js/wasm`, IndexedDB storage and window creation work.
- The **fork's WebGL renderer is not** — it has at least two distinct defects
  (FBO nil-slice [fixed in gl-js], and non-conformant vertex-attribute
  management [open]), and the fact that they surfaced one after another implies
  the wasm GL path has never rendered a real UI. **Expect more.**
- Therefore the web target is **gated on finishing/hardening the
  `trendvidia/fyne` fork's WebGL backend** — a meaningful, open-ended effort
  *upstream of any fycoder work*. IME/clipboard/large-file (the runtime
  checklist) can only be assessed once the UI actually paints.

This is the spike's most important result: **the editor is ready for the web;
the Bellui fork's WebGL backend is the real, larger blocker.**

## Two viable architectures

### Path A — fat client (everything in the WASM binary)
Bellui UI + fycoder + protolsp (handlers called in-process) + sops-age + keygen,
all compiled into one `js/wasm` binary. Serverless; works offline.

### Path B — thin client (UI in WASM, protolsp hosted over WebSocket)
Browser runs only the Bellui/fycoder UI and talks to a hosted protolsp over the
**already-existing** WebSocket/Connect transport. Keeps the messy parts
(buf, KMS, filesystem) server-side. Needs a backend; not offline.

## Blockers and mitigations

| Blocker | Detail | Mitigation |
|---|---|---|
| **No subprocesses** | `protolsp/internal/buftool` execs the `buf` CLI for format/lint; user-configured Bitwarden/1Password/Vault key-fetch are shell commands | Replace `buf` with in-process `protocompile` (already a pure-Go dep); web-ify or disable CLI key-fetch — or push both server-side (Path B). |
| **No local filesystem** | host reads/writes `.pxf` files | Virtual FS via IndexedDB (dep already present), File System Access API (Chromium), or remote. pxfed already abstracts fs behind a `Services` interface — swap the impl. |
| **sops KMS** | AWS/GCP KMS SDKs won't run under WASM | Age-only in the browser (covers pxfed's actual use), or KMS server-side (Path B). |
| **`fsnotify` watching** | no-op in browser | disable or poll. |
| **Networking / CORS** | registry + any fetch must be CORS-permitted | Go's `net/http` transparently uses `fetch()` under WASM; needs CORS headers or a proxy. Ops, not code. |
| **Binary size** | Bellui + protolsp + protocompile → ~30–60 MB WASM; TinyGo is **not** viable (Bellui needs full reflection/runtime) | Brotli/gzip, lazy-load, loading UX. |

## What fycoder specifically owes this target

Most blockers above are the **host's** problem (fs, subprocesses, transport).
fycoder's own responsibility is narrow but critical: **the editor widget must be
usable under `js/wasm`.** That is entirely about Bellui's web input layer, and it
is the decisive risk:

- **IME composition** — CJK and dead-key input via Bellui-WASM is less mature than
  desktop. This directly intersects [Phase 4](04-protected-ranges-and-redaction.md):
  the protected-range edit-guard must reject composition commits, and the WASM
  IME path must deliver the events for that to work.
- **Clipboard** — paste/copy semantics differ in the browser sandbox; verify
  against the find/replace and redaction paths.
- **Large-document rendering** — [Phase 5](05-folding-and-large-files.md)'s
  virtualization must hold under the WebGL canvas, not just desktep GL.
- **Focus & keyboard shortcuts** — the Cmd/Ctrl shortcut routing in `editor.go`
  must survive browser key-event quirks.

These should become explicit acceptance criteria for the relevant phases, gated
on the spike below — there is no point hardening the desktop editor for features
the web input layer can't deliver.

## Effort

Assuming the desktop fycoder host already links protolsp in-process:

| Path | Scope | Estimate |
|---|---|---|
| **B (thin / WebSocket)** | virtual-or-remote FS, CORS, client-side feature gating, IME polish | **~3–5 engineer-weeks** |
| **A (fat / all-in-WASM)** | + subprocess elimination, virtual FS, binary-size/load UX, age-vs-KMS decision, editor polish | **~6–10 engineer-weeks** |

## Recommended first step — a one-day spike

Before committing to either path, **compile today's bare fycoder `Editor` to
`js/wasm` and try, in a browser:**

1. editing CJK text via an IME,
2. paste from the system clipboard,
3. scrolling and editing a 10k-line file,
4. the Cmd+F / Cmd+S shortcuts.

This single experiment de-risks the web target more than any further analysis:
if Bellui-WASM's input layer clears that bar, both paths are mostly host
plumbing; if it doesn't, the web target waits on upstream Bellui work regardless
of how complete the editor core becomes.

## Spike results — F1–F5 browser-verify (2026-07-01) ✅

Done via the `examples/wasm` harness in headed Chrome (DPR2, dark) on **both**
web backends — WebGPU (default) and the `-tags gl` WebGL fallback — rendering
**pixel-identically**, no wasm-specific rendering issues. Tracking:
[trendvidia/fyne#376](https://github.com/trendvidia/fyne/issues/376) (closed) ·
[#150](https://github.com/trendvidia/fyne/issues/150) (closed).

All five Phase 4/5 acceptance scenarios pass:

1. **Large-file + virtualisation** — a 50k-line document renders; "Jump to end"
   windows the gutter to rows ~49965–50001 with highlighting intact (rows are
   viewport-windowed, not materialised whole).
2. **Folding via `FoldGutter` chevron** — collapses the block; the external
   line-number gutter stays aligned across the fold.
3. **AutoFold** — a pre-collapsed region renders folded on load with its summary.
4. **Protected-range veto** — typing into a protected range is rejected, text
   unchanged, and the attempt is reported.
5. **Caret-opaque redaction** — a masked secret shows an opaque bar and bars the
   caret; reveal/mask toggles it.

**Caveats / follow-ups:**

- **Scroll input** ([fyne#377](https://github.com/trendvidia/fyne/issues/377)):
  neither mouse-wheel (Chrome treats the canvas wheel handler as passive) nor
  `PageDown`/`PageUp` scrolled the viewport deep into the doc — the harness uses
  a programmatic `Editor.RevealRange` ("Jump to end") to drive deep scroll.
  Rendering/virtualisation itself is unaffected.
- **Initial `SetText` cost** ([fycoder#106](https://github.com/belluikit/bellui/x/editor/issues/106)):
  previously loading the 50k doc blocked the main thread ~9 s because the
  highlighter ran over the whole document synchronously on `SetText`. **Fixed by
  bumping to `trendvidia/fyne v2.13.0`**, which windows the highlight cache
  ([fyne#401](https://github.com/trendvidia/fyne/issues/401)) — `HighlightRunsForRow`
  drives an on-demand, forward-chained build for just the rendered rows, with a
  per-frame background prefill of the state chain
  ([fyne#417](https://github.com/trendvidia/fyne/issues/417)) and trusted-stretch
  deep jumps ([fyne#412](https://github.com/trendvidia/fyne/issues/412)). fycoder
  itself never looped the document (it only wires `LineHighlighter`), so no app
  code changed; `TestLargeFile_HighlighterIsWindowed` guards that a `SetText` of
  50k lines highlights ~one screenful (43 calls), not every line. Sibling of
  [fyne#375](https://github.com/trendvidia/fyne/issues/375) (Entry per-keystroke
  relayout O(document)). **Live browser verification done (2026-07-02,
  fycoder#133, against fyne v2.14.0, BOTH backends — WebGPU and `TAGS=gl`
  WebGL):** "Load 50000 lines" renders promptly with no multi-second
  main-thread stall, and keyboard paging reaches the app —
  `PageDown`/`PageUp` move by a viewport screenful preserving the column,
  Ctrl+Home/Ctrl+End (macOS Cmd+↑/↓) jump to document start/end, Shift
  variants extend the selection (fyne#377 + glfw-js v1.0.2 confirmed live).
  Feedback from the session: on a 50k-line document the scrollbar thumb is
  too small to grab and drag (tracked as a fork scroller follow-up).
- **IME composition on wasm** stays an interactive-only check (the automated
  harness can't drive a native IME); the desktop path is verified (fyne#130) and
  the wasm composition seam is wired (fyne#128).
