# Ecosystem Rebrand → Bellui

**Status: core cutover EXECUTED.** This document records what was actually done
for the core toolkit (trendvidia/fyne#730) and what remains for the downstream
repos. The old `trendvidia/*` repos are **frozen, not deleted** — they remain the
history reference and the rollback path.

**Tracking:** epic [trendvidia/fyne#732](https://github.com/trendvidia/fyne/issues/732) ·
core [trendvidia/fyne#730](https://github.com/trendvidia/fyne/issues/730) ·
release-train [trendvidia/fyne#731](https://github.com/trendvidia/fyne/issues/731).

## Identity

| Thing | Value |
|---|---|
| Toolkit (this repo) | **Bellui** — module `github.com/belluikit/bellui`, package `bellui` |
| Domains | `bellui.com`, `bellui.io` (reserved — content not yet published) |
| GitHub org | `belluikit` |
| Flagship IDE (`goed`) | **Bellui Studio** — `studio.bellui.com` — *authoring* |
| Runtime (`appviewer`) | **Bellui Player** — PXF → live UI — *Studio authors, Player runs* |
| Extension libraries | **Bellui X** — `belluikit/bellui/x/{charts,tables,dag,editor,mmedia}` |

> **Naming guard:** the IDE is **Bellui Studio**, *not* `bellui.App`. `bellui.App`
> is the core interface type — reusing it as a product name collides with the
> toolkit's own vocabulary. "Studio" (not "Designer") because it is a UI designer
> **and** a code editor.

## Decisions taken at cutover

These differ from the original plan; they are the decisions of record.

| Question | Decision | Rationale |
|---|---|---|
| Major version | **No suffix** — `github.com/belluikit/bellui` (v1 line) | The plan said "keep `/v2`", but the fork had since moved to `/v3`. A new module path under a new brand is a clean semantic restart; carrying `/v3` into a repo with no v1/v2 history would be misleading. |
| `fyne.io` / `fyne-io` deps | **1 forked, 2 vendored** — `belluikit/systray` stays a fork; `image/ico`, `oksvg` and `srwiley/rasterx` vendored into `internal/` | See "Vendored dependencies" below. Upstream copyright + licenses retained in full; the systray fork keeps an `upstream` git remote for syncing. |
| User-facing surface | **Hard cutover, no back-compat** | CLI `fyne`→`bellui` and `FYNE_*`→`BELLUI_*`. The module path already breaks every consumer, so all disruption is batched into one migration. (App metadata needed no rename: this fork had already replaced `FyneApp.toml` with `manifest.pxf` — see below.) |
| Git history | **Fresh initial commit** | `trendvidia/fyne` is preserved frozen as the history reference. |

## What the core cutover changed

1. **Module path** — `github.com/trendvidia/fyne/v3` → `github.com/belluikit/bellui`
   across both modules (root and `cmd/bellui`).
1a. **The six toolkit-specific trendvidia forks moved to `belluikit`** — see
   "Forked dependencies" below. (The original plan left these alone; that was
   revised.) `trendvidia/protowire-go` deliberately stayed put — see the
   migrate-or-not rule there.
2. **Package** — `package fyne` → `package bellui` (`fyne.App` → `bellui.App`),
   plus every `Fyne*`/`fyne*` identifier in Go, ObjC, C and Java.
3. **Paths** — `cmd/fyne`→`cmd/bellui`, `cmd/fyne_settings`→`cmd/bellui_settings`,
   `fyne_demo`→`bellui_demo`, `fyne.go`→`bellui.go`, `*fyneapppb`→`*belluiapppb`,
   bundled fonts `*-fyne.ttf`→`*-bellui.ttf`.
4. **Protobuf** — proto packages `fyne.*.v1` → `bellui.*.v1`, `go_package`
   updated, and **all four descriptors regenerated** (a text substitution alone
   would leave the compiled `rawDesc` encoding the old names).
5. **Android dex** — regenerated after the Java class rename
   (`FyneFileProvider`→`BelluiFileProvider`).
6. **`x/` umbrella** — `x/{charts,tables,dag,editor,mmedia}` as separate
   modules, each with its own `go.mod`, so the core dep graph stays lean. All
   are **migrated**, not scaffolds — see below.

### Extension libraries (`x/`)

The extension repos were consolidated into the umbrella (epic #732).
32 test packages pass between them.

| Module | From | Kind of move |
|---|---|---|
| `x/tables` | `trendvidia/fytables` | whole repo |
| `x/editor` | `trendvidia/fycoder` | whole repo ("Bellui Editor") |
| `x/dag` | `trendvidia/fydag` | **partial** — `dagwidget` + `dag` only |
| `x/charts` | `trendvidia/fycharts` | whole repo + **API port** |
| `x/mmedia` | `trendvidia/fyplayer` | whole repo + **dependency bump** |

**Three were mechanical, one was not.** `fytables`, `fycoder` and `fydag`
already targeted the fork's modern `geo`/`event`/`core` API, so they needed only
import rewrites. `fycharts` targeted **upstream `fyne.io/fyne` v2.5.4** — the old
monolithic API — and needed a real port:

* `fyne.Position`/`Size`/`NewPos`/`NewSize` → `geo.*`
* `fyne.CanvasObject`/`Widget`/`WidgetRenderer` → `core.*`
* `fyne.DragEvent`/`Key*`/`PointEvent`/`Scrollable`/`Tappable`/… → `event.*`
* `desktop.Hoverable` → `event.Hoverable`, an **interface change** not a rename:
  `MouseIn`/`MouseMoved`/`MouseOut(*desktop.MouseEvent)` became
  `HoverIn`/`HoverMoved`/`HoverOut(*event.HoverEvent)`. Safe here because every
  call site only read `e.Position`, which `HoverEvent` carries via its embedded
  `PointEvent`. `desktop.Mouseable`'s `MouseDown`/`MouseUp` were deliberately
  left alone.
* `w.ShowAndRun()` → `bellui.ShowAndRun(w)` (now a standalone function)
* `theme.ForegroundColor()` → `theme.Color(theme.ColorNameForeground)` (this one
  also hit `x/tables`)

**`x/dag` is a partial extraction whose one gap has since been closed.** The
epic called `dag` a "pure-data leaf", but `dag/proto.go` and `dag/serial.go`
imported fydag's generated gRPC package, because fydag's graph messages live in
one monolithic `.proto` alongside its services. Bringing them would have put
grpc into a widget module, so at consolidation those two files stayed with the
PoC and `MarshalGraph`, `UnmarshalGraph`, `GraphToProto` and `GraphFromProto`
were not part of `x/dag`.

**Restored in `ecb7626` (belluikit/bellui#3), and this is the shipped state.**
The data-model messages were carved out into `x/dag/dag/dagpb` — messages only,
no services — so the generated bindings need `google.golang.org/protobuf`
alone, and `MarshalGraph` rides the same PXF codec the toolkit already uses for
settings, preferences and manifests. Both were existing bellui dependencies, so
the grpc objection was met rather than traded away: `go list -deps ./...` in
that module reports **zero** grpc packages. Field numbers are inherited verbatim
from the fydag schema, so payloads stay wire-compatible with graphs the PoC
wrote; the proto package deliberately differs (`bellui.dag.v1` vs `fydag`),
because an application linking both would otherwise register two messages under
one fully-qualified name and protobuf-go panics at init.

So the extraction is still partial in scope — `x/dag` is `dagwidget` + `dag`,
not all of fydag — but nothing of the graph model is missing from it.
`trendvidia/fydag` remains a working PoC.

**`x/mmedia` was mechanical in its code but not in its dependencies.**
`fyplayer` already targeted the fork's modern API, so the Go rewrite was the
usual path/package/qualifier set — all 32 files reproduce byte-for-byte from
the source under it. What it could not keep was its FFmpeg pin: it required
`trendvidia/goffmpeg v0.2.0`, and `belluikit/goffmpeg` publishes only `v0.4.0`
(the same tree, re-homed), which pulls `goffmpeg-libs` `v0.1.0` → `v0.3.0` and
forces the module to declare `go 1.26.2` rather than the `1.25.0` its siblings
use. The bump is additive on the surface `mmedia` touches; it is recorded
because a version change is not a rename.

The source was `origin/main` @ `57fd6fb`, which **carries no tag** — three
commits past `v0.3.0`. `fyplayer` also has a `chore/fyne-v3` branch, the same
decoy name that shipped `x/tables` and `x/editor` stale, but here it is fully
merged and the pre-flight check said so before anything was copied.

### Forked dependencies (moved to `belluikit`)

The forks that exist *to serve this toolkit* moved org with the rebrand. Code is
unchanged except the module path; each keeps an `upstream` git remote for syncing.

| Module | Ver | Import lines | Role | Upstream |
|---|---|---|---|---|
| `belluikit/webgpu` | v1.1.0 | 31 | **default painter** (`//go:build !gl`) | `oliverbestmann/webgpu` |
| `belluikit/glfw` | v1.9.0 | 27 | desktop windowing — IME/preedit, parent/modal, clipboard MIME | `go-gl/glfw` + vendored GLFW 3.4 |
| `belluikit/typesetting` | v0.3.6 | 24 | shaping (HarfBuzz), font scanning | `go-text/typesetting` |
| `trendvidia/protowire-go` | v1.2.2 | 6 | PXF encoding — manifests, prefs, settings | own |
| `belluikit/gl-js` | v0.2.1 | 5 | wasm GL bindings | `x/mobile/gl` + CL 8793 |
| `belluikit/glfw-js` | v1.0.4 | 4 | wasm windowing | glfw-js lineage |
| `belluikit/render` | v0.2.4 | 2 | text rasterization | `go-text/render` |
| `belluikit/systray` | v1.12.0 | 1 | system tray | `fyne-io/systray` |

Only one inter-fork edge exists: `render` → `typesetting`.

**Migrate-or-not rule.** A dependency moves to `belluikit` only if it exists to
serve this toolkit — i.e. it is a fork we carry patches on (`fork=true`, or a
self-described fork like `render`). A **standalone general-purpose library keeps
its own home**, even when we authored it.

`github.com/trendvidia/protowire-go` is the worked example and stays at
`trendvidia`: it is `fork=false` — not a fork of anything, but the *reference
implementation of the `trendvidia/protowire` spec* (PXF / SBE / Envelope), a
public serialization toolkit with an identity and audience beyond bellui.
Rehoming it under `belluikit` would misrepresent it as a UI component and orphan
it from its spec repo. It is a normal published dependency here — no `replace`,
no fork.

**Base switch — `webgpu` (resolved, #1).** The fork was rebased from
`cogentcore/webgpu` onto `oliverbestmann/webgpu` in v1.1.0. Lineage:
`rajveermalviya/go-webgpu` (archived) → `cogentcore/webgpu` (self-declared
frozen) → `oliverbestmann/webgpu`. The deciding evidence was that
`cogentcore/core` — the project behind our old base — has itself moved there, so
that repo is the live continuation rather than a small unrelated fork.

It brought wgpu-native v22.1.0.5 → v29.0.1.1, generated cgo wrappers (fewer
allocations per call), and all four js/wasm patches this fork carried already
upstream. The static libraries now live in per-OS modules on their own branches,
so a consumer downloads one platform's slice instead of a 126 MB module zip.

Two slices upstream does not ship stay forked, on `libs-ios` and `libs-windows`
branches of our repo: the **arm64 iOS Simulator** archive (Go builds device and
Simulator both as `GOOS=ios/GOARCH=arm64`, so the device slice cannot link a
Simulator binary — gated behind the `iossimulator` tag) and a **windows/arm64
GNU-ABI** rebuild (upstream ships MSVC only, whose C++ EH llvm-mingw cannot
link). Everything else is consumed from upstream unchanged, so upstream fixes
arrive by version bump rather than by merge.

**Retiring srwiley.** `render` is the last holder of `srwiley/{oksvg,rasterx}`.
Note it cannot simply import `internal/{oksvg,rasterx}` — Go's internal rule
scopes those to `github.com/belluikit/bellui/...`, and `belluikit/render` is a
separate module. Retiring them means either vendoring inside `render` itself or
absorbing `render` into `internal/` (it is only 2 import lines and 4 Go files).

### Vendored dependencies

Three former external dependencies now live in `internal/`, each with a
`PROVENANCE.md` recording origin, license and local modifications:

| Package | From | LOC | Why |
|---|---|---|---|
| `internal/ico` | `fyne-io/image/ico` | 391 | Only `ico` was used; the module's other package (`xpm`, 1,269 LOC) was dead weight. Windows `.ico` is a frozen format. **Ships two licenses** — `LICENSE-reader` (Apache-2.0) and `LICENSE-writer` (BSD, Go Authors); do not consolidate. |
| `internal/rasterx` | `srwiley/rasterx` | 3,258 | Upstream dormant since 2022-07-30. Depends only on `golang.org/x/image`, already required — so vendoring adds no modules. |
| `internal/oksvg` | `fyne-io/oksvg` | 2,236 | **Forced by rasterx**: `SvgIcon.Draw` is typed against `*rasterx.Dasher`, so oksvg had to follow rasterx across the module boundary or reference an incompatible type. |
| `internal/render` | `belluikit/render` | 501 | **The last holder of srwiley.** A separate module cannot import `internal/{oksvg,rasterx}` (Go's internal rule), so the choice was duplicating ~5.5k LOC inside `render` or bringing `render` here. It has one consumer using one symbol, so absorbing won. |

**srwiley is fully retired.** `go list -deps` and `go.sum` both contain zero
`srwiley` references, and zero `fyne-io`/`fyne.io` references. Build modules
dropped 33 → 30. The graph is now belluikit forks + `trendvidia/protowire-go` +
ordinary third-party libraries.

`belluikit/render` is consequently unused by bellui and frozen unless another
consumer appears.

**Rejected: `tdewolff/canvas`.** Evaluated as a replacement and declined — its
core package itself depends on `srwiley/rasterx` and `srwiley/scanx`, so it
would not remove srwiley at all, while taking build modules from 2 to 28 and a
minimal binary from 2.5 MB to 6.8 MB (pulling in X11, PDF, freetype and a second
text stack). It also has no tagged releases.

### App metadata: `manifest.pxf`, not a TOML file

Upstream Fyne stores app metadata in `FyneApp.toml`. **This fork already retired
that** before the rebrand: metadata lives in `manifest.pxf` (textual PXF, schema
in `internal/metadata/belluiapppb/belluiapp.proto`), `ManifestFile` in
`internal/metadata/load.go` is the single source of truth, and
`github.com/BurntSushi/toml` survives only as an indirect dependency with no
direct use. So the rebrand required **no** metadata-file rename — any surviving
`FyneApp.toml` mention was stale prose, and those now correctly say
`manifest.pxf`.

### Deliberately NOT renamed

Rewriting these would have falsified attribution or broken real URLs:

* **`LICENSE`** — the original Fyne BSD-3 copyright is retained verbatim, as the
  license requires. A Bellui modifications notice is appended below it.
* **fyne-x attribution comments** (`widget/gif.go`, `widget/filetree.go`,
  `widget/toolbar_twostate.go`, `layout/responsive.go`, `dialog/about.go`) —
  these credit upstream copyright holders.
* **`fyne#NNN` issue references and `github.com/trendvidia/fyne/...` links** —
  they point at the frozen tracker and still resolve.
* **`github.com/fyne-io/...` links** — real upstream resources.
* **`CHANGELOG.md` / `ROADMAP.md`** — historical records of work done under the
  old name; rewriting them would falsify the record.

### Rename hazards found (length-sensitive fixtures)

`fyne` is 4 characters and `bellui` is 6, which broke three fixed-width things.
Anything similar added later should be checked the same way:

* a macOS **FourCharCode** (`hkID.signature = 'fyne'`) — must be exactly 4 chars,
  now `'blui'`;
* a **regex quantifier** fixture (`^\d{2}-\w{4}$` matched `10-fyne`);
* a hardcoded **column index** in an Entry selection test.

## Verification

```sh
env GOWORK=off go build ./...                       # root
env GOWORK=off go test -count=1 -tags ci ./...      # 61 packages
(cd cmd/bellui && env GOWORK=off go test -count=1 ./...)   # 6 packages
env GOWORK=off go test -tags "ci accessibility" ./...
gofumpt -l . && goimports -l .                      # both empty
grep -rl "trendvidia/fyne" . --include='*.go'       # must be empty
```

## Remaining work

Tracked as issues; nothing here lives only in prose.

| Item | Issue |
|---|---|
| `add-to-project.yml` points at pre-rebrand boards | belluikit/bellui#2 |
| Repo visibility decision after the soak | belluikit/bellui#4 |
| `fydag` still on the old toolkit (`fyplayer` is now `x/mmedia`) | belluikit/bellui#5 |
| The `x/` modules have no CI — `x/mmedia` needs a both-painters leg | belluikit/bellui#13 |
| `bellui.io` has no content — dead doc links | belluikit/bellui#6 |
| Gallery: restore DAG Editor + Media Player pages | belluikit/gallery#1 |
| goed → Bellui Studio | trendvidia/goed#95 |
| appviewer → Bellui Player | trendvidia/appviewer#145 |
| Release-train automation | trendvidia/fyne#731 |

## Released

**v1.0.0-rc.1** (2026-07-26) — `github.com/belluikit/bellui`, plus
`x/{tables,editor,dag,charts}` at `x/<name>/v1.0.0-rc.1`. Release candidate
while the rebrand soaks. **Zero `replace` directives** in any module; every one
resolves from published tags.
