# fycoder — WASM runtime / F1–F5 acceptance harness

A browser harness for exercising the *complete* fycoder editor under
`GOOS=js GOARCH=wasm`. It backs the runtime half of
[`docs/roadmap/10-wasm-web-target.md`](../../docs/roadmap/10-wasm-web-target.md)
and is the acceptance harness for the F1–F5 browser-verify
([trendvidia/fyne#376](https://github.com/trendvidia/fyne/issues/376),
[#150](https://github.com/trendvidia/fyne/issues/150)).

**Why this exists:** compilation to WASM is already proven. What a compiler
*cannot* tell us is whether Bellui's **web layer** carries a real code editor —
IME composition, clipboard, large-file virtualisation, code folding, protected
ranges, and secret redaction. Those need a browser. This harness makes each
check one click, on both the **WebGPU** default and the **`-tags gl`** WebGL
fallback.

This is a **separate Go module** (its own `go.mod`, with `replace` pointing at
the repo root) so pulling the full Bellui app stack does not add those deps to the
fycoder library's `go.mod`. `go build ./...` at the repo root does not descend
into it.

## Run

```bash
cd examples/wasm
make serve          # builds editor.wasm + copies wasm_exec.js, then serves
# open http://localhost:8080
```

`make build` alone produces `editor.wasm` + `wasm_exec.js` (both gitignored).
`PORT=9000 make serve` changes the port.

## Manual test checklist

The editor opens with a Starlark strategy sample: a header comment block, a
`META` dict, a pre-collapsed `PARAMS` dict, and an `evaluate()` function. The
`api_key` line is a protected (read-only) range and its secret value is a masked
redaction. Toolbar: **Fold all · Unfold all | Reveal/Mask secret ·
Load 50000 lines** and a status label. (Deep scroll is driven by the editor's
own keyboard paging — see below — not a button; the old "Jump to end"
`RevealRange` workaround was retired once glfw-js v1.0.2 let paging keys reach
the browser, #120.)

Run each on **both** backends — `make serve` (WebGPU default) and
`make serve TAGS=gl` (WebGL fallback):

- [ ] **AutoFold on load** — `PARAMS` opens pre-collapsed, showing its summary
      placeholder (`... 3 params`); the gutter numbers jump across the hidden
      interior (11 → 16 → 17) with the header line still visible.
- [ ] **Redaction (caret-opaque masking)** — the `api_key` value renders as a
      solid opaque bar. **Reveal secret** shows the real text in the clear;
      **Mask secret** restores the bar. The caret cannot enter a masked span.
- [ ] **Fold via chevron** — click a `∨` chevron in the fold gutter (e.g. line 5
      `META`); the interior collapses to `META = {...`, the chevron flips to
      `>`, and the external line-number gutter stays aligned across the fold.
- [ ] **Protected-range veto** — put the caret on the `api_key` line and type /
      paste; the edit is rejected and the status shows
      `VETOED: line 8 is protected (read-only)`. The text does not change.
- [ ] **Large file + virtualisation** — click **Load 50000 lines**; the document
      renders promptly (scrollbar shows the full 50k extent) and `SetText` does
      not stall — the highlighter is windowed to the visible rows (#106). Press
      **Ctrl+End** to jump to the last line — the gutter reads ~49965–50001 with
      highlighting intact, proving rows are windowed to the viewport, not
      materialised whole.
- [ ] **Keyboard paging** (#120, needs glfw-js ≥ v1.0.2) — in the 50k doc:
      **PageDown**/**PageUp** move the caret by a viewport screenful (preserving
      the column, not one line); **Ctrl+Home**/**Ctrl+End** (macOS **Cmd+↑/↓**)
      jump to document start/end; **Shift+** variants extend the selection. Each
      key must actually reach the editor (pre-v1.0.2 they were silently dropped).
- [ ] **IME composition** *(interactive only)* — type into a string with a
      CJK IME; composition preview shows and the commit lands correctly.

Record results back in the Phase 10 doc / on #376.

## Notes

- Served over plain HTTP; `index.html` falls back from streaming to
  ArrayBuffer instantiation if the server doesn't send
  `Content-Type: application/wasm`, so `python3 -m http.server` is fine.
- `wasm_exec.js` is copied from your toolchain (`go env GOROOT`) at build time
  so it always matches the Go version that built the binary — do not commit it.
- `index.html` is hand-written to match the **trendvidia/fyne** web template,
  including the `#dummyEntry` input the wasm driver looks up by id for
  keyboard/IME focus. Do not strip it.

## Troubleshooting

- **Black screen / no UI, but `editor.wasm` returned 200.** The page is
  missing the structure the Bellui wasm driver expects — most importantly the
  `<input id="dummyEntry">` element. Use the `index.html` shipped here; a bare
  loader page will run the binary but render nothing usable.
- **Renderer fix is wired in via `replace`.** Upstream `fyne-io/gl-js@v0.2.0`
  panics on first paint (`index out of range [0] with length 0` in
  `gl-js.TexImage2D` during `EnsureCanvasFBO`): the canvas FBO path passes a
  typed-nil pixel slice that gl-js converts to a zero-length array instead of
  WebGL `null`. This `go.mod` `replace`s gl-js with the fixed `trendvidia/gl-js`
  fork (trendvidia/gl-js#1). If you see that panic, your build isn't picking up
  the replace. Root cause + fix detail:
  [`docs/roadmap/10-wasm-web-target.md`](../../docs/roadmap/10-wasm-web-target.md).
- **Don't use the upstream `fyne serve` / `fyne package` CLI here.** This module
  builds against the `github.com/trendvidia/fyne` fork, and the upstream `fyne`
  CLI fails with `fyne version not found` because it can't resolve the fork.
  `make serve` (plain `go build` + `python3 -m http.server`) is the supported
  path.
- **`main module (github.com/belluikit/bellui/x/editor) does not contain package
  .../examples/wasm`.** A `go.work` somewhere up the directory tree (a
  multi-module dev workspace) has put Go in workspace mode and pinned the main
  module to the fycoder *root*, which excludes this nested module. The Makefile
  already sets `GOWORK=off` to prevent this; if you build by hand, use
  `GOWORK=off go build .`, or run `go work use ./examples/wasm` from the
  workspace root. Confirm the cause with `go env GOWORK`.
- **`os/user … undefined: current / lookupUser / …`.** You have
  `CGO_ENABLED=1` set explicitly (shell export or `go env -w`), and Go honors
  it for the `js/wasm` cross-compile — but `os/user` has no cgo path for `js`,
  so it fails to build. `js/wasm` never uses cgo. The Makefile already sets
  `CGO_ENABLED=0`; building by hand, use `CGO_ENABLED=0 go build .`. Check with
  `go env CGO_ENABLED`.
