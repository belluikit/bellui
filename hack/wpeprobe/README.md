# wpeprobe — the #60 Linux web-view spike

Measures whether Linux can host web content as a **texture bellui composites**
rather than as the hole punch every other platform uses.

Every other backend (macOS, iOS, Android, wasm) puts an OS view *over* bellui's
surface, which costs three things documented on `widget.NativeViewHost`: nothing
bellui draws can appear above the web content, `Canvas.Capture` sees a hole, and
the render pipeline does not reach it. WPE WebKit is the WebKit port built for
embedding into someone else's renderer, so on paper Linux could avoid all three.

## Setup

Needs a Linux box with a GPU. Verified on Ubuntu 22.04.5 with an NVIDIA RTX 2070
SUPER, **headless** — no X11, no Wayland compositor, an SSH session is enough.

```sh
sudo apt-get install -y \
  libwpe-1.0-dev libwpebackend-fdo-1.0-dev libwpewebkit-1.0-dev \
  libegl1-mesa-dev libgl1-mesa-dev libwayland-dev pkg-config build-essential

go run ./hack/wpeprobe            # report
go run ./hack/wpeprobe -dump out  # also write the captured PNG
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
