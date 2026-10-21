# cefprobe — the #60 Linux web-view spike, with CEF

> **Outcome: the architecture works, the CPU path is shippable, and — as of
> 2026-07-30 — the dma-buf path answers YES on this machine.** Web content
> composited *inside* bellui's canvas across **96.8%** of the surface with bellui
> drawing over it, at **1.9 ms per frame extrapolated to 1080p** — 11–12% of a
> frame budget, against the 17–59% `hack/wpeprobe` measured for WPE.
> `on_accelerated_paint` delivers a **single-plane BGRA_8888 dma-buf** with
> `DRM_FORMAT_MOD_LINEAR` and a per-frame fd; details and the two conditions it
> needs are below. Note this is **not zero-copy** — CEF's contract requires
> copying the frame into a texture you own, inside the callback (see below) — so
> the prize is a GPU-side blit instead of a CPU roundtrip, not the elimination of
> a copy. The import half (belluikit/bellui#113) is still open.

Direct successor to `hack/wpeprobe`. That probe established the architecture and
then rejected WPE as the *engine* — 112 unbundled dependencies, absent from
Ubuntu since 22.04, zero-copy conditional on the user's session type. CEF
replaces the engine, not the approach, so this probe reuses wpeprobe's
assertions verbatim and prints them in the same shape; the two reports are meant
to be read side by side.

Behind the `cef` build tag, so `go build ./...` on a machine with no CEF payload
— which is every CI runner — does not try to compile it.

## Setup

Needs Linux. The CPU path is verified on Ubuntu 22.04.5, kernel 6.8, x86_64,
with an NVIDIA RTX 2070 SUPER, over SSH with no desktop session. The
accelerated path additionally needs a live X11 or Wayland session on the box
(see the [accel] section below).

```sh
./hack/cefprobe/setup.sh              # fetch, verify, strip, lay out (~10 min first time)
go run -tags cef ./hack/cefprobe -dump out                                              # CPU phase
go run -tags cef ./hack/cefprobe -accel-only \
    -switches="--enable-features=Vulkan --use-vulkan=native"                            # accel phase
```

`cef_initialize` is once-per-process, so the two phases have to run in separate
invocations. Without `-accel-only` the default flow runs the CPU phase and
prints a note pointing at the second command.

`setup.sh` pins CEF **154.0.34 / Chromium 154.0.8037.98** and verifies the
archive's SHA-1 before using it. It strips `libcef.so` from about **1.4 GB to
260 MiB** — that 81% is what makes the payload shippable at all, and it is the
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

**The GPU frame path works, and the machine turns out to be up to it.** With a
live X session on the box (a `startx` gnome-shell on tty1 is enough — see the
recorded prerequisite below), CEF's `on_accelerated_paint` delivers a
**single-plane BGRA_8888 dma-buf** with `DRM_FORMAT_MOD_LINEAR` and a per-frame
file descriptor. Byte-identical output across six consecutive runs, no
retry-flake, independently reproduced 3/3 afterwards. The import half is still
open, but the CEF half — "can this backend hand us a dma-buf at all" — is now
yes.

**It is not zero-copy, and calling it that misprices the work.** #60, this
README and the plan all used that word before the contract was read closely.
`cef_accelerated_paint_info_t` says the resource "cannot be cached and cannot be
accessed outside of this callback. It should be reopened each time this callback
is executed and **the contents should be copied to a texture owned by the client
application**". So the shippable shape is: import the dma-buf and blit it into
our own texture, synchronously, inside the callback, every frame. That is still
worth having — it trades a CPU copy plus a PCIe upload for a GPU-side blit — but
the copy does not go away, which matters when the CPU path already fits in 11%
of a frame budget.

That result is **conditional on two things**, either of which absent gives the
"never fires" outcome the earlier spike reported:

1. **A live display session.** Chromium's compositor initializes ANGLE before
   Vulkan takes over, and ANGLE's EGL backend on Linux requires an X (or
   Wayland) display — even under `--ozone-platform=headless`. Without a session
   the GPU process still exits with `Could not open the default X display` and
   nothing paints. Not solvable from switches on this stack; `xvfb-run` gives a
   display but no GPU.
2. **`--enable-features=Vulkan --use-vulkan=native`** — Chromium's *native*
   Vulkan renderer, distinct from `--use-angle=vulkan` (still ruled out; goes
   through ANGLE). With the default ANGLE-on-GL backend the NVIDIA proprietary
   driver hits `GL_FRAMEBUFFER_UNSUPPORTED` (`0x8CDD`) in ANGLE's
   `FramebufferGL.cpp:1340 (checkStatus)` and Skia reports
   `Unable to initialize SkSurface` — no frame lands on either callback.
   ANGLE-on-GLES is not an option: it errors out with "Intel or NVIDIA OpenGL
   ES drivers are not supported". `--use-cmd-decoder=validating` is silently
   ignored. Native Vulkan is what bypasses the ANGLE FBO path and lets the
   shared image be backed by a native pixmap (dma-buf).

This is **not** the structural dead end WPE hit. `WPEDisplayHeadless` exposes no
DRM device, so it could only *ever* hand over SHM, by construction. CEF's
`cef_accelerated_paint_info_t` carries dma-buf fds per plane and the code path
delivers them here. Concrete numbers from the probe:

```
[accel] on_accelerated_paint (shared_texture_enabled = 1, single-init)
  PASS: dma-buf frame delivered
    planes    1
    fd        125
    stride    3328 bytes                   # = 832 pixels * 4bpp, 32-pixel alignment for an 800px view
    modifier  0x0 (DRM_FORMAT_MOD_LINEAR)
    format    1 (CEF_COLOR_TYPE_BGRA_8888)
```

The import half is belluikit/bellui#113, and is explicitly not a blocker for
shipping (#60). The binding constraint is the fd lifetime quoted above: it is
valid only inside the callback, so an importer has to do its work synchronously
there rather than handing the frame to another goroutine or deferring it to the
next paint.

### Prerequisite for the accel path on this box

The box hosts CI runners and is dual-boot; the display session that made the
result reproducible is:

- User `bene` logged into tty1
- `startx`-style Xorg + `gnome-session-binary` + `gnome-shell` running there
- `DISPLAY=:0`, `XAUTHORITY=/home/bene/.Xauthority`, X access via
  `SI:localuser:bene`

Any SSH shell that inherits `DISPLAY` (or that `export DISPLAY=:0` and can read
the same Xauthority) can reach it. No systemwide change was made; the session
is `bene`'s tty1 login and can be undone with `pkill gnome-session-binary` +
`pkill Xorg`.

## Six traps, all of which cost real time

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

- **Every CEF call must come from the thread that called `cef_initialize`, and
  Go does not give you that for free.** The main goroutine can be rescheduled
  onto another OS thread at any preemption point, and a pump loop that sleeps
  between `cef_do_message_loop_work()` calls is nothing but preemption points.
  Without `runtime.LockOSThread()` this probe delivered a first frame on roughly
  one run in three, with **byte-identical logs whether it worked or not** — it
  reads like a GPU or CEF fault and is neither. bellui's main loop is already
  locked to the main thread, so the backend inherits this, but only for calls
  that actually go through `bellui.Do`.

- **`cef_initialize` is once-per-process, and there is no way around it.** The
  first version of the accel probe called `cef_shutdown` then re-initialized
  with `shared_texture_enabled=1` in the same process. That path dumps core on
  the second `cef_initialize` — not on `cef_shutdown`, which succeeds — and
  gives no diagnostic. The probe now runs the two phases as separate processes
  (`./cefprobe` for CPU, `./cefprobe -accel-only …` for accel) and this trap
  goes away. Any real backend has to pick one browser lifetime per process; the
  original CEF documentation says as much but its C API surface makes it look
  like a retry is possible.
- **`--use-angle=<X>` doesn't cover NVIDIA the way it looks like it does.**
  ANGLE's Linux backends are `gl`, `gl-egl`, `gles`, `vulkan`, `swiftshader`.
  On NVIDIA proprietary: `gles` errors out with "Intel or NVIDIA OpenGL ES
  drivers are not supported" (hardcoded rejection); `gl`/`gl-egl` build a
  context but their shared-image FBOs are `GL_FRAMEBUFFER_UNSUPPORTED`;
  `vulkan` goes through ANGLE-over-Vulkan and hits the same SkSurface init
  failure. Only Chromium's *native* Vulkan renderer
  (`--enable-features=Vulkan --use-vulkan=native`) works, and that is easy to
  miss because the switch shape looks redundant with `--use-angle=vulkan`.

A smaller one: `helper.c` lives in its own directory because cgo compiles every
`.c` file in a package directory, so a second `main()` beside `main.go` fails
the link.

### A false lead, recorded so it is not chased again

While the thread bug was still unfound, `--disable-gpu --disable-gpu-compositing`
appeared to fix frame delivery — it took the success rate from about one run in
three to two in three, which is exactly the kind of evidence that convinces. It
does nothing. With the thread locked, frames arrive on every run with **no GPU
switches at all** (6/6, then repeatedly since). Neither is `was_hidden(false)` +
`set_focus(true)` + `invalidate(PET_VIEW)` load-bearing here, though those calls
are kept because they are what a real backend must drive from Show/Hide, from
scrolling out of a clipping ancestor, and from the #59 focus bridge.

## Layout

| | |
|---|---|
| `main.go` | the probe: composite proof, frame cost, popup count, accelerated-paint attempt (`-accel-only`) |
| `cef.c` / `cef.h` | the C shim — dlopen'd CEF C API, handler structs, the version handshake |
| `helper/helper.c` | the subprocess helper (`cef_execute_process`), in its own directory |
| `setup.sh` | fetch, verify, strip and lay out the pinned payload |

The Go binary is deliberately **not** the subprocess executable: the Go runtime
starts threads before `main` and Chromium's zygote forks, so every working
Go/CEF integration uses a separate helper. It finds `libcef.so` beside itself
via `/proc/self/exe`, so it needs no rpath and no environment.
