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
| Extension libraries | **Bellui X** — `belluikit/bellui/x/{charts,tables,dag,editor}` |

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
6. **`x/` umbrella** — `x/{charts,tables,dag,editor}` as four separate modules,
   each with its own `go.mod`, so the core dep graph stays lean. All four are
   **migrated**, not scaffolds — see below.

### Extension libraries (`x/`)

The four extension repos were consolidated into the umbrella (epic #732).
27 test packages pass between them.

| Module | From | Kind of move |
|---|---|---|
| `x/tables` | `trendvidia/fytables` | whole repo |
| `x/editor` | `trendvidia/fycoder` | whole repo ("Bellui Editor") |
| `x/dag` | `trendvidia/fydag` | **partial** — `dagwidget` + `dag` only |
| `x/charts` | `trendvidia/fycharts` | whole repo + **API port** |

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

**`x/dag` is a deliberate partial extraction.** The epic called `dag` a
"pure-data leaf", but `dag/proto.go` and `dag/serial.go` imported fydag's
generated gRPC package. Those two files (and their tests) stayed with the PoC,
which keeps grpc/protobuf out of a widget module — `dagwidget` used only the
graph types, never the serialization. The trade-off is that `MarshalGraph`,
`UnmarshalGraph`, `GraphToProto` and `GraphFromProto` are **not** part of
`x/dag`; consumers needing persistence encode the exported `dag` types
themselves. `trendvidia/fydag` remains a working PoC.

### Forked dependencies (moved to `belluikit`)

The forks that exist *to serve this toolkit* moved org with the rebrand. Code is
unchanged except the module path; each keeps an `upstream` git remote for syncing.

| Module | Ver | Import lines | Role | Upstream |
|---|---|---|---|---|
| `belluikit/webgpu` | v1.0.8 | 31 | **default painter** (`//go:build !gl`) | `cogentcore/webgpu` |
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

**Upstream health note — `webgpu`.** Its README carries an inherited notice that
the upstream (`cogentcore/webgpu`) is frozen and points at
`oliverbestmann/webgpu` as the maintained fork. Lineage:
`rajveermalviya/go-webgpu` (archived) → `cogentcore/webgpu` (self-declared
frozen; still pushed 2026-06) → `oliverbestmann/webgpu` (active, small). This
sits under the **default** rendering path, so it is worth tracking — no action
taken.

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

**What this did and did not achieve.** Direct requires dropped 33 → 31, two fork
repos are no longer needed, and the code we call is now ours to patch. The build
module graph is **unchanged at 33**, because `belluikit/render` — which
`internal/painter` imports for text — still requires `srwiley/oksvg` and
`srwiley/rasterx`:

```
$ go mod why github.com/srwiley/rasterx
github.com/belluikit/bellui/internal/painter
github.com/belluikit/render
github.com/srwiley/rasterx
```

Fully retiring srwiley therefore needs `belluikit/render` (a fork we own)
repointed at these vendored packages — or its SVG/raster use removed. Until
then srwiley remains as an `// indirect` entry.

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

* ~~Publish the dependency repos~~ — **done.** All seven are published under
  `belluikit` with `main` as the rebranded line and a single version tag each
  (`systray v1.12.0`, `glfw v1.9.0`, `webgpu v1.0.8`, `typesetting v0.3.6`,
  `render v0.2.4`, `gl-js v0.2.1`, `glfw-js v1.0.4`). Every `replace` directive
  is gone from the root module and resolution is verified against the published
  modules. Note the tags were **recreated on the rebrand commits** — the tags
  inherited from upstream point at pre-rename code whose `go.mod` still declares
  the old module path, so only the retagged refs were pushed.
* **`cmd/bellui` still replaces the root module** (`=> ../../`) because bellui
  itself is untagged. Drop it at the first bellui tag; CI already adds it
  dynamically (`cli_tests.yml`).
* **DECIDE: repo visibility.** All eight `belluikit/*` repos are **private**,
  whereas every `trendvidia/*` fork they replaced was **public**. Consequences:
  `go get github.com/belluikit/bellui` fails for anyone without org access; every
  consumer and CI job needs `GOPRIVATE=github.com/belluikit/*` plus git
  credentials (SSH, or a token with `insteadOf`); and the README's claim of
  "public; no special build flags" is no longer true. Either make them public to
  match the old setup, or accept the private posture and document the required
  `GOPRIVATE` + auth for consumers.
* **Retire srwiley entirely** — `belluikit/render` is all that still holds
  `srwiley/{oksvg,rasterx}`. It cannot import `internal/{oksvg,rasterx}` (Go's
  internal rule scopes those to this module), so this means vendoring inside
  `render` or absorbing `render` here — it is only 4 Go files and 2 import
  lines.
* **Downstream repos** — fycharts#2, fytables#75, fycoder#170, fydag#5, goed#95,
  appviewer#145, gallery#20.
* **`.github/workflows/add-to-project.yml`** — still points at the pre-rebrand
  project boards and needs the `ADD_TO_PROJECT_PAT` secret provisioned.
* **Release-train automation** (trendvidia/fyne#731) — auto-bump PRs into
  consuming repos on each Bellui tag.
