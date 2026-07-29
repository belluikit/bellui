# wpeprobe — the #60 Linux web-view spike

Measures whether Linux can host web content as a **texture bellui composites**
rather than as the hole punch every other platform uses.

Every other backend (macOS, iOS, Android, wasm) puts an OS view *over* bellui's
surface, which costs three things documented on `widget.NativeViewHost`: nothing
bellui draws can appear above the web content, `Canvas.Capture` sees a hole, and
the render pipeline does not reach it. WPE WebKit is the WebKit port built for
embedding into someone else's renderer, so on paper Linux could avoid all three.

Behind the `wpe` build tag, so `go build ./...` on a machine without the WPE
development packages — which is every CI runner — does not try to compile it.

## Setup

Needs a Linux box with a GPU. Verified on Ubuntu 22.04.5 with an NVIDIA RTX 2070
SUPER, **headless** — no X11, no Wayland compositor, an SSH session is enough.

```sh
sudo apt-get install -y \
  libwpe-1.0-dev libwpebackend-fdo-1.0-dev libwpewebkit-1.0-dev \
  libegl1-mesa-dev libgl1-mesa-dev libwayland-dev pkg-config build-essential

go run -tags wpe ./hack/wpeprobe            # report
go run -tags wpe ./hack/wpeprobe -dump out  # also write the captured PNG
```

Two things that are easy to lose an hour to:

- **`libWPEBackend-default.so: cannot open shared object file`, then a core
  dump.** libwpe loads a backend by name and Ubuntu ships no default one. The
  probe sets `WPE_BACKEND_LIBRARY=libWPEBackend-fdo-1.0.so.1` itself; anything
  else driving libwpe has to do the same.
- **Headless needs `EGL_EXT_platform_device`.** `eglGetDisplay(EGL_DEFAULT_DISPLAY)`
  has no window system to attach to on a tty session. The probe enumerates EGL
  devices and takes one directly, which is what makes an SSH session sufficient.

## What it found

**WPE renders offscreen headless.** EGLDisplay via `EGL_PLATFORM_DEVICE_EXT`,
vendor NVIDIA, frames produced with no display server present.

**Architecture (B) works, and is visibly better than the hole punch.** Frames
pushed through `canvas.Raster.Streaming` + `SetPixels` land *inside* bellui's
canvas: `software.RenderCanvas` sees the web content across ~95% of the surface
and a bellui rectangle drawn *over* it. Both are impossible under the hole
punch — that is the whole argument for this approach.

**Zero-copy is not reachable on this stack.** Frames export as **SHM (CPU
buffers), never EGLImages** — on both the NVIDIA and the Mesa EGL device, with
`EGL_WL_bind_wayland_display`, `EGL_KHR_image_base` and
`EGL_EXT_image_dma_buf_import` all present, and with
`WEBKIT_FORCE_COMPOSITING_MODE=1`. `EGL_LOG_LEVEL=debug` shows why: the
WebProcess's own EGL falls back to Mesa `swrast_dri.so`, so WebKit renders in
software and has nothing but SHM to hand over. The wgpu interop question the
spike was meant to answer therefore cannot even be posed here — there are no
EGLImages to import.

**The copy is the cost.** 0.4–1.5 ms per frame at 640×480 (it varies: the copy
contends with the engine thread), which extrapolates to roughly 3–9 ms at 1080p
— 17–59% of a 16.6 ms frame budget. Fine for a small embedded view, not for
full-window web content.

`belluiWpeExportKind` is the measurement that matters: rerun on a newer WPE or a
Mesa-only GPU and it reports `2` (EGLImage) if the zero-copy path has become
reachable, at which point stage 2 is worth building.

## What the SHM result does and does not establish

It was measured on a machine whose **only** GPU is NVIDIA, running **WPE WebKit
2.36 (2022)**. Both are confounds:

- The WebProcess's client-side EGL is Mesa, and Mesa has no hardware driver for
  that card, so a Mesa EGL client there has no hardware path at all and swrast
  is the only possible outcome. The EGL device reporting vendor `Mesa Project`
  on that box is llvmpipe — software — not a Mesa-driven GPU.
- 2.36 is roughly sixteen releases behind current (2.52.5), and the fdo export
  path was reworked in between.

A follow-up attempt on genuinely Mesa-accelerated hardware (virtio-gpu/virgl)
with **WPE 2.48.3** from Debian trixie, in a container with the render node
passed through, got as far as: all dma-buf EGL extensions present, WebProcess
confirmed to `execve`, view given
`wpe_view_activity_state_visible|focused|in_window` (which newer WebKit needs or
it will not paint at all) — and then **produced no frames whatsoever** over 90
seconds, via either export callback. The legacy `webkit_web_view_backend_new` +
`wpe_view_backend_exportable_fdo` path is still declared and undeprecated in
2.48.3, so this is unexplained rather than expected.

That port was then done — `wpe_platform_probe.c`, against the WPE Platform API
(`WPEDisplay`/`WPEView`/`WPEBuffer`) that 2.44+ development moved to. It works
where the legacy path did not: frames arrive, delivered through the view class's
`render_buffer` vfunc, which the probe chains in front of.

**It answers the export question structurally, and the answer does not depend on
the GPU.** `WPEDisplayHeadless` reports **no DRM device**
(`wpe_display_get_drm_device` is NULL), and a display with no DRM device cannot
allocate dma-bufs — so headless can only ever hand over SHM, by construction.
Confirmed on WPE WebKit **2.52.5**, the current release. The earlier SHM results
were therefore never evidence about GPU capability at all.

Zero-copy needs one of the other display backends. WPE ships four —
`wpe-platform-2.0`, `wpe-platform-drm-2.0`, `wpe-platform-headless-2.0`,
`wpe-platform-wayland-2.0` — and only the DRM and Wayland ones can carry
dma-bufs. DRM means taking DRM master (exclusive display control, so not
something an app shares with a running desktop); Wayland means negotiating
`linux-dmabuf` with an existing compositor.

**The consequence for bellui is the useful part**: a WPE integration would get
zero-copy only while running under a dma-buf-capable Wayland compositor, and
would silently fall back to CPU copies on X11 or anywhere the negotiation fails
— performance depending on the user's session type at runtime. That is the same
silent-degradation shape that ruled out X11-only reparenting in #542.

## Distro availability, which bears on the licensing argument

The case for WPE rested on WPE WebKit being a *system shared library the user
supplies* rather than something bellui vendors. That holds unevenly:

| distro | WPE WebKit |
|---|---|
| Ubuntu | **22.04 only** (2.36) — dropped from 24.04 onward |
| Debian | bookworm 2.38.6, trixie 2.48.3, sid 2.52.5 |
| Arch | 2.52.5, **and the only one of the three that enables the Platform API** |

Ubuntu, the most common Linux desktop, has shipped no WPE since 22.04. Debian
packages WPE at 2.48/2.52 but builds it **without** `ENABLE_WPE_PLATFORM`, so
the API that current WPE development targets is absent there too — verified by
searching the full include tree of `libwpewebkit-2.0-dev` on both trixie and
sid. Of the distros checked, only Arch ships it.
