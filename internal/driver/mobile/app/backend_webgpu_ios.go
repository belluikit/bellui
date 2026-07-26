//go:build ios && !gl

package app

// BELLUI_WEBGPU is defined for the Objective-C sources (darwin_ios.m) by default
// (and absent under the -tags gl fallback), so they can #ifdef out the GLKView/GLES draw path
// (which is the broken path on the iOS Simulator, #195). cgo merges the CFLAGS
// of every cgo file in the package into the C/.m compile.
//
// The blank line below is required: it separates this prose from the cgo
// preamble so the preamble is just the #cgo directive, not this comment.

/*
#cgo CFLAGS: -DBELLUI_WEBGPU
*/
import "C"

// webgpuBackend is true by default (false under the -tags gl fallback): the GLES present (swapBuffers) and
// context-current calls are skipped so wgpu-native owns the view via a
// CAMetalLayer (Metal). gomobile still drives lifecycle, input and the publish
// handshake; the bellui painter presents.
const webgpuBackend = true
