# cefprobe — the #60 Linux web-view spike, with CEF

> **Outcome: the architecture works, and the CPU path is shippable.** Web content
> composited *inside* bellui's canvas across **96.8%** of the surface with bellui
> drawing over it, at **1.9 ms per frame extrapolated to 1080p** — 11–12% of a
> frame budget, against the 17–59% `hack/wpeprobe` measured for WPE. The
> zero-copy question is **unanswered**, for a reason that is about this machine
> rather than about CEF: see below.

Direct successor to `hack/wpeprobe`. That probe established the architecture and
then rejected WPE as the *engine* — 112 unbundled dependencies, absent from
Ubuntu since 22.04, zero-copy conditional on the user's session type. CEF
replaces the engine, not the approach, so this probe reuses wpeprobe's
assertions verbatim and prints them in the same shape; the two reports are meant
to be read side by side.

Behind the `cef` build tag, so `go build ./...` on a machine with no CEF payload
— which is every CI runner — does not try to compile it.

## Setup

Needs Linux. Verified on Ubuntu 22.04.5, kernel 6.8, x86_64, with an NVIDIA
RTX 2070 SUPER, over SSH with no desktop session.

```sh
./hack/cefprobe/setup.sh              # fetch, verify, strip, lay out (~10 min first time)
go run -tags cef ./hack/cefprobe -dump out
```

`setup.sh` pins CEF **151.3.12 / Chromium 151.0.7922.47** and verifies the
archive's SHA-1 before using it. It strips `libcef.so` from **1.36 GB to
256 MiB** — that 81% is what makes the payload shippable at all, and it is the
same step `belluikit/gocef-libs`' mirror workflow performs (belluikit/bellui#104).

## What it found

**Architecture (B) works with CEF, and beats the hole punch.** Frames from
`on_paint` pushed through `canvas.Raster.Streaming` + `SetPixels` land *inside*
bellui's canvas: `software.RenderCanvas` sees the page across 96.8% of the
surface, with a bellui rectangle drawn *over* it. Both are impossible under the
hole punch every other backend uses, and this is what dissolves that model's
whole limitation list — overlays, `Canvas.Capture`, clipping, `RepaintBoundary`.

**The CPU path is cheap enough to ship.** The copy plus the BGRA→RGBA swizzle
costs ~435 µs at 800×600, extrapolating to **1.9 ms at 1080p**, or 11–12% of a
16.6 ms budget. wpeprobe measured 3–9 ms (17–59%) for WPE's copy alone. Two
caveats kept honest: the swizzle belongs in the shader rather than in Go, and
CEF hands over dirty rects that this path throws away — a full-frame upload
where a caret blink should cost a few thousand pixels (belluikit/bellui#99).

**Zero-copy is unanswered here, and the reason is the machine.** The GPU process
exits during initialization — `Could not open the default X display`, then
`Exiting GPU process due to errors during initialization` — so
`on_accelerated_paint` never fires and CEF correctly falls back to `on_paint`.
Tried and did not help: `--use-angle=vulkan`, `--disable-gpu-sandbox`,
`--in-process-gpu`, and `xvfb-run` (a virtual display has no GPU).

This is **not** the structural dead end WPE hit. `WPEDisplayHeadless` exposes no
DRM device, so it could only *ever* hand over SHM, by construction. CEF's
`cef_accelerated_paint_info_t` carries dma-buf fds per plane and the code path
exists; what is missing is a display session with GPU access on this box.
Answering it needs a real X11 or Wayland session on hardware with a working EGL
display — and then the separate question of whether wgpu can import a dma-buf at
all, which is risk R1 on #60 and explicitly not a blocker for shipping.

## Three traps, all of which cost real time

Recorded because `belluikit/gocef` (belluikit/bellui#103) will hit each one
again, and none of them fails in a way that names the actual problem.

- **The API-version handshake is mandatory, and `cef_api_hash` is what performs
  it.** It reads like a query — it returns a hash string — but its doc says it
  "configures the CEF API version ... any changes to this value will be ignored
  after the first call". Skip it and `cef_api_version()` reports `-1`, and the
  C→C++ layer fatally rejects the first struct you hand it, naming whatever type
  it happened to be validating: `CefClient_0_CToCpp called with invalid version
  -1`. Both the browser process *and* the subprocess helper must call it, before
  any other CEF function. This is also why the binding pins `CEF_API_VERSION`
  explicitly: leaving it undefined selects `CEF_API_VERSION_EXPERIMENTAL`, which
  carries no compatibility guarantee across CEF builds.
- **Resources go flat, beside `libcef.so`.** The distribution ships them under
  `Resources/`, and its own CMake copies them next to the binary. Point
  `resources_dir_path` at a nested `Resources/` and it fails inside ICU —
  `Invalid file descriptor to ICU data received` — followed by a core dump.
- **The V8 snapshot blobs are in `Release/`, not `Resources/`.** They are the one
  part of the runtime that is not where the rest of the resources are. Omitting
  them gives `Error loading V8 startup snapshot file`, twice, then a core dump.

A fourth, smaller one: `helper.c` lives in its own directory because cgo
compiles every `.c` file in a package directory, so a second `main()` beside
`main.go` fails the link.

## Layout

| | |
|---|---|
| `main.go` | the probe: composite proof, frame cost, popup count, accelerated-paint attempt |
| `cef.c` / `cef.h` | the C shim — dlopen'd CEF C API, handler structs, the version handshake |
| `helper/helper.c` | the subprocess helper (`cef_execute_process`), in its own directory |
| `setup.sh` | fetch, verify, strip and lay out the pinned payload |

The Go binary is deliberately **not** the subprocess executable: the Go runtime
starts threads before `main` and Chromium's zygote forks, so every working
Go/CEF integration uses a separate helper. It finds `libcef.so` beside itself
via `/proc/self/exe`, so it needs no rpath and no environment.
