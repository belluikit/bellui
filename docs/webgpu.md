# WebGPU rendering backend

As of the Phase 5 migration (#204), **WebGPU is the default GPU rendering backend**
on every platform this fork targets. The legacy OpenGL / OpenGL&nbsp;ES / WebGL
backend is still fully supported and is selected with the **`gl`** build tag.

WebGPU is implemented on top of [`oliverbestmann/webgpu`](https://github.com/oliverbestmann/webgpu)
(forked as `github.com/belluikit/webgpu`), which binds the native
[`wgpu-native`](https://github.com/gfx-rs/wgpu-native) on desktop/mobile and the
browser's WebGPU via `syscall/js` on the web. The fork was rebased onto that base
in v1.1.0, from the frozen `cogentcore/webgpu` it started on; our fork now carries
only two platform slices upstream does not ship — see `docs/REBRAND.md`.

## Why

- Apple has deprecated OpenGL/GLES, and the **iOS Simulator's GLES path is broken
  on Apple&nbsp;Silicon** (renders magenta and crashes). WebGPU is Metal-backed on
  Apple platforms, which restores rendering — and visual verification — in the
  simulator.
- WebGL is being superseded by WebGPU in browsers.
- A single backend now drives Metal, Vulkan, D3D12 and browser WebGPU from one
  set of WGSL shaders and one painter.

## Backend selection

| Build | Backend | How |
| --- | --- | --- |
| default (no tag) | **WebGPU** | `go build ./...` |
| fallback | OpenGL / GLES / WebGL | `go build -tags gl ./...` |

The selection is purely a compile-time build tag; there is no runtime switch.

## Platform support matrix

| Platform | WebGPU API | Status |
| --- | --- | --- |
| macOS (desktop) | Metal | Verified live |
| Linux (X11 + Wayland) | Vulkan | Verified live (Wayland), compile + headless readback (X11) |
| Windows | D3D12 | Verified live |
| Web (WebAssembly) | Browser WebGPU | Verified live (headless Chromium/Dawn) |
| Android | Vulkan (wgpu-native via NDK) | Verified live (emulator) |
| iOS Simulator | Metal | Verified live |
| iOS device | Metal | Links; not yet verified on hardware (#242) |

The GL fallback (`-tags gl`) supports the same desktop/mobile/web platforms as
before the migration.

## Build requirements

### Desktop (macOS, Linux, Windows)

The default build links the native `wgpu-native` library through cgo. **The
prebuilt libraries ship inside the `github.com/belluikit/webgpu` Go module** —
there is no system package to install; a normal `go build` downloads them with the
module.

Requirements:

- **cgo enabled** with a working C toolchain (clang or gcc; MinGW on Windows).
- A **clean cgo environment** for the link: build with empty `CGO_LDFLAGS` and
  `CGO_CFLAGS` (a host `-fuse-ld=lld` or stray flags can break the wgpu-native
  link). For example:

  ```sh
  CGO_LDFLAGS='' CGO_CFLAGS='' go build ./...
  ```

- A working GPU / driver at **run time** (Metal, a Vulkan ICD such as a real GPU
  or lavapipe, or D3D12). Building does not require a GPU.

To build the pure-Go-link GL fallback instead (no wgpu-native), add `-tags gl`:

```sh
go build -tags gl ./...
```

### Web (WebAssembly)

The browser provides WebGPU directly, so there is **no native library and no cgo**.
Build with `GOOS=js GOARCH=wasm`. The default produces the browser-WebGPU backend;
`-tags gl` produces the WebGL (`gl-js`) backend.

```sh
GOOS=js GOARCH=wasm go build -tags osusergo ./...   # WebGPU (default)
GOOS=js GOARCH=wasm go build -tags gl ./...          # WebGL fallback
```

Notes:

- `osusergo` avoids the wasm `os/user` cgo dependency on the default build.
- The page must serve over a secure context and the browser must support WebGPU.
- The wasm loader HTML must set `window.wasm = result` **before** `go.run(...)`
  (the zero-copy JS bridge reads `globalThis.wasm.instance.exports.mem.buffer`;
  the stock `wasm_exec.js` does not set it).

### Android

`wgpu-native` renders via **Vulkan** on the `ANativeWindow`; gomobile keeps
driving lifecycle and input but its EGL/GLES path is bypassed. Package with the
Bellui CLI:

```sh
bellui package -os android/arm64 --appID com.example.app --icon Icon.png
```

(The default is now WebGPU; pass `--tags gl` to package the GLES build.)

Gotchas:

- Set `ANDROID_HOME` / `ANDROID_NDK_HOME`, and use a clean `CGO_LDFLAGS` /
  `CGO_CFLAGS`.
- API&nbsp;30+ requires `resources.arsc` stored **and** 4-byte aligned: re-pack with
  `zipalign -f -p 4` then re-sign with `apksigner`.

### iOS / iOS Simulator

`wgpu-native` renders via **Metal** on a `CAMetalLayer`; the gomobile GLKView/GLES
draw path is compiled out (it is the broken-on-simulator path, #195). The
simulator needs a simulator slice of `wgpu-native`, selected with the
**`iossimulator`** tag:

```sh
bellui package -os iossimulator --tags iossimulator
```

For a device build, omit `iossimulator`. (Add `gl` to either to build the legacy
GLES app.)

## Testing

The WebGPU painter has headless, GPU-backed readback tests under
`internal/painter/webgpu/` (gated by the default build; they skip automatically
when no WebGPU adapter is available):

```sh
CGO_LDFLAGS='' CGO_CFLAGS='' go test ./internal/painter/webgpu/...
```

CI runs **both** backends (see `.github/workflows/`):

- a **GL leg** (`-tags gl`) — pure-Go link, no GPU, holds the coverage gate and
  proves the fallback still builds, on every runner;
- a **WebGPU leg** — builds the default `./...` and runs the painter readback
  tests on Metal (macOS), Vulkan/lavapipe (Linux) and D3D12 (Windows).

## Visual baselines

`internal/painter/webgpu/baseline_test.go` renders a fixed multi-primitive scene
offscreen and compares it pixel-for-pixel (within a small tolerance for GPU
anti-aliasing differences) against a committed golden image in `testdata/`. It
runs on the WebGPU CI legs and guards against shader/pipeline regressions across
Metal, Vulkan and D3D12. Regenerate the golden with:

```sh
CGO_LDFLAGS='' CGO_CFLAGS='' BELLUI_WEBGPU_UPDATE_BASELINE=1 \
  go test ./internal/painter/webgpu/ -run TestVisualBaseline
```
