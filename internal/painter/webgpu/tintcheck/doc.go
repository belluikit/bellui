// Package tintcheck holds a test that compiles every WGSL shader of the
// WebGPU painter (internal/painter/webgpu/shaders/*.wgsl) with a browser's
// WebGPU shader compiler — Chrome's Tint — in headless Chrome (#685).
//
// Native wgpu (naga) and Tint disagree at the edges. v1.2.0 shipped a shader
// native accepted and Tint rejected ("'textureSample' must only be called from
// uniform control flow"), and every wasm WebGPU app drew a black page. The
// painter's own tests run on native devices only, so they cannot see this.
//
// The test lives in its own package so that it does not link the painter:
// it reads the same files the painter embeds and needs no cgo and no GPU.
// Chrome compiles on its SwiftShader software adapter.
//
// Without a Chrome or Chromium binary, or without a WebGPU adapter, the test
// skips. With BELLUI_REQUIRE_TINT=1 (set in CI) both are failures. Point it
// at a specific browser with BELLUI_CHROME=/path/to/chrome.
package tintcheck
