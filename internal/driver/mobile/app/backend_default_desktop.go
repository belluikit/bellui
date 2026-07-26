//go:build (darwin && !ios) || (linux && !android) || freebsd || openbsd || windows

package app

// webgpuBackend is false on the desktop/testing backends (darwin_desktop.go,
// x11.go), which host a mobile app in a desktop window through the GLES present
// path — there is no WebGPU/CAMetalLayer present here, so app.Publish keeps its
// real GLES flush + swap. Defined so app.go (shared across mobile and these
// desktop hosts) can gate on it; the mobile backends set it per android/ios ×
// gl/!gl.
const webgpuBackend = false
