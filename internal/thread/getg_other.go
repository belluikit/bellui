//go:build !gc || !(amd64 || arm64)

package thread

import "unsafe"

// haveGetg reports whether getg is implemented for this platform.
const haveGetg = false

func getg() unsafe.Pointer { return nil }
