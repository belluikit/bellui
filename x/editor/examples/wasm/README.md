# x/editor — WASM browser harness

A browser harness for exercising the *complete* x/editor widget under
`GOOS=js GOARCH=wasm`. It is the runtime half of
[`docs/roadmap/10-wasm-web-target.md`](../../docs/roadmap/10-wasm-web-target.md).

**Why this exists:** a compiler can prove the editor builds for WASM, but not
whether Bellui's **web layer** carries a real code editor: IME composition,
clipboard, large-file virtualisation, code folding, protected ranges and secret
redaction. Those need a browser. This harness makes each check one click, on
both the **WebGPU** default and the **`-tags gl`** WebGL fallback.

This is a **separate Go module** (its own `go.mod`), so pulling the full Bellui
app stack does not add those dependencies to x/editor's `go.mod`. `go build ./...`
in x/editor does not descend into it. Its `go.mod` replaces
`github.com/belluikit/bellui/x/editor` with `../..`, so the harness always runs
the editor in this checkout. The toolkit itself, `github.com/belluikit/bellui`,
comes from the release that `go.mod` pins, unless you build with `LOCAL=1`.

## Run

```bash
cd x/editor/examples/wasm
make serve            # WebGPU build of editor.wasm + wasm_exec.js, then serve
make serve TAGS=gl    # the same with the WebGL fallback painter
make serve LOCAL=1    # build against the bellui toolkit in this checkout
# open http://localhost:8080 in a browser with WebGPU (or WebGL for TAGS=gl)
```

`make build` alone produces `editor.wasm` and `wasm_exec.js`; both are
gitignored. `PORT=9000 make serve` changes the port.

`LOCAL=1` writes a gitignored `go.local.mod`: `go.mod` plus a `replace` of
`github.com/belluikit/bellui` onto the repository root. This is the same
ephemeral replace that CI's `x_tests.yml` uses. Use it to test toolkit changes
that are not released yet. Never commit a `replace` into `go.mod`.

## Manual test checklist

The editor opens with a Starlark strategy sample: a header comment block, a
`META` dict, a pre-collapsed `PARAMS` dict, and an `evaluate()` function. The
`api_key` line is a protected (read-only) range, and its secret value is masked
by a redaction. The toolbar has **Fold all**, **Unfold all**, **Reveal secret**
(which becomes **Mask secret**), **Load 50000 lines** (which becomes **Load
sample**) and a status label. Deep scrolling uses the editor's own keyboard
paging, described below.

Run each check on **both** backends: `make serve` (WebGPU) and
`make serve TAGS=gl` (WebGL).

- [ ] **AutoFold on load:** `PARAMS` opens pre-collapsed and shows its summary
      placeholder (`... 3 params`). The gutter numbers jump across the hidden
      interior (11 → 16 → 17), and the header line stays visible.
- [ ] **Redaction (caret-opaque masking):** the `api_key` value renders as a
      solid opaque bar. **Reveal secret** shows the real text, and **Mask
      secret** restores the bar. The caret cannot enter a masked span.
- [ ] **Fold via chevron:** click a `∨` chevron in the fold gutter (for example
      on line 5, `META`). The interior collapses to `META = {...`, the chevron
      flips to `>`, and the line-number gutter stays aligned across the fold.
- [ ] **Protected-range veto:** put the caret on the `api_key` line and type or
      paste. The edit is rejected, the text does not change, and the status
      shows `VETOED: line 8 is protected (read-only)`.
- [ ] **Large file and virtualisation:** click **Load 50000 lines**. The
      document renders promptly, the scrollbar shows the full extent, and
      `SetText` does not stall, because the highlighter only works on the
      visible rows. Press **Ctrl+End** to jump to the last line. The gutter
      should read about 49965–50001 with highlighting intact, which shows rows
      are built for the viewport, not for the whole file.
- [ ] **Keyboard paging:** in the 50k document, **PageDown** and **PageUp**
      move the caret by a screenful and keep its column. **Ctrl+Home** and
      **Ctrl+End** (**Cmd+↑/↓** on macOS) jump to the start and end of the
      document. The **Shift+** variants extend the selection.
- [ ] **IME composition** *(interactive only)*: type into a string with a CJK
      IME. The composition preview shows, and the committed text lands at the
      caret. The harness does not import `theme/cjk`, so CJK characters draw
      as boxes; check that the right number of characters lands and that the
      caret moves by that many.

## Notes

- The page is served over plain HTTP. `index.html` falls back from streaming
  to ArrayBuffer instantiation when the server does not send
  `Content-Type: application/wasm`, so `python3 -m http.server` is fine.
- `wasm_exec.js` is copied from your toolchain (`go env GOROOT`) at build time,
  so it always matches the Go version that built the binary. Do not commit it.
- `index.html` is hand-written to match the `bellui` CLI's web template
  (`cmd/bellui/internal/templates/data/index.html`). Two parts of it are
  load-bearing; see the first two troubleshooting items.

## Troubleshooting

- **Black screen, and the console shows `panic: wgpu.(*Queue).TryWriteTexture:
  JavaScript error: wasm is not defined`.** The loader page did not set
  `window.wasm = result` before `go.run(...)`. The WebGPU painter's JavaScript
  glue reads Go's memory through that global, so the first text it draws
  panics. The `index.html` here sets it; a hand-written loader page must too
  (see `docs/webgpu.md`). The WebGL build (`TAGS=gl`) does not need it.
- **IME text never arrives, though plain typing works.** The page is missing
  `<input id="dummyEntry">`. The wasm driver focuses that hidden input and
  listens on it for IME composition. Without it the editor still renders and
  takes ordinary keystrokes, but composed text is dropped. Keep the input from
  the `index.html` here.
- **Black screen on the WebGPU build with bellui v1.2.0, and the console shows
  `Error while parsing WGSL: :85:24 error: 'textureSample' must only be called
  from uniform control flow`.** Chrome's WebGPU rejects that release's
  batched-quad shader, so nothing draws. It is fixed after v1.2.0. Until
  `go.mod` pins a release with the fix, use `make serve LOCAL=1` or
  `make serve TAGS=gl`.
- **`bellui serve` fails here with `open …/wasm/index.html: not a directory`.**
  The CLI names the built binary after the module's last path element, `wasm`,
  which collides with its `wasm/` output folder
  ([#683](https://github.com/belluikit/bellui/issues/683)). It would also use
  its own page template and has no `TAGS=gl` switch. Use `make serve`.
- **`main module (github.com/belluikit/bellui/x/editor) does not contain package
  .../examples/wasm`.** A `go.work` somewhere up the directory tree has put Go
  in workspace mode with x/editor as the main module, which excludes this
  nested module. The Makefile sets `GOWORK=off` to prevent this. If you build
  by hand, use `GOWORK=off go build .`. Check the cause with `go env GOWORK`.
- **`os/user … undefined: current / lookupUser / …`.** `CGO_ENABLED=1` is set
  explicitly (a shell export or `go env -w`), and Go honors it for the
  `js/wasm` cross-compile, but `os/user` has no cgo path for `js`. `js/wasm`
  never uses cgo. The Makefile sets `CGO_ENABLED=0`; when building by hand,
  use `CGO_ENABLED=0 GOOS=js GOARCH=wasm go build .`. Check with
  `go env CGO_ENABLED`.
