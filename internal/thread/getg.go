//go:build gc && (amd64 || arm64)

package thread

import "unsafe"

// haveGetg reports whether getg is implemented for this platform.
const haveGetg = true

// getg returns the current goroutine's runtime record, in assembly.
func getg() unsafe.Pointer
