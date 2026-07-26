// Package thread provides low-level goroutine identification primitives
// with zero dependencies on the bellui package, allowing both the root bellui
// package and internal packages to import it without circular dependencies.
package thread

import (
	"runtime"
	"sync/atomic"
)

// mainGoroutineID stores the main goroutine ID.
var mainGoroutineID atomic.Uint64

// SetMainGoroutine records the current goroutine as the main goroutine.
// This must be called during app initialization from the main goroutine.
func SetMainGoroutine() {
	mainGoroutineID.Store(GoroutineID())
}

// GoroutineID returns the current goroutine's ID.
func GoroutineID() (id uint64) {
	var buf [30]byte
	runtime.Stack(buf[:], false)
	for i := 10; buf[i] != ' '; i++ {
		id = id*10 + uint64(buf[i]&15)
	}
	return id
}
