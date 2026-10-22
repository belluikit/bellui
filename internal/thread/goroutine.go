// Package thread provides low-level goroutine identification primitives
// with zero dependencies on the bellui package, allowing both the root bellui
// package and internal packages to import it without circular dependencies.
package thread

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

// mainGoroutineID stores the main goroutine ID.
var mainGoroutineID atomic.Uint64

// SetMainGoroutine records the current goroutine as the main goroutine.
// This must be called during app initialization from the main goroutine.
func SetMainGoroutine() {
	mainGoroutineID.Store(GoroutineID())
}

// MainGoroutineRegistered reports whether SetMainGoroutine has been called.
// IsMainGoroutine cannot answer this on mobile, where it reports true for every
// goroutine until one is registered.
func MainGoroutineRegistered() bool {
	return mainGoroutineID.Load() != 0
}

// GoroutineID returns the current goroutine's ID.
//
// Where the runtime's goroutine record can be read (gc on amd64 and arm64), the
// ID is read from it in a few nanoseconds. Elsewhere it is parsed out of a stack
// trace, which takes microseconds. Every widget setter checks its goroutine, so
// the stack trace cost most of a busy frame's CPU (#457).
func GoroutineID() uint64 {
	if goidOffset >= 0 {
		return *(*uint64)(unsafe.Add(getg(), goidOffset))
	}
	return stackGoroutineID()
}

// stackGoroutineID parses the ID from a stack trace's first line,
// "goroutine 123 [running]:".
func stackGoroutineID() (id uint64) {
	var buf [30]byte
	runtime.Stack(buf[:], false)
	for i := 10; buf[i] != ' '; i++ {
		id = id*10 + uint64(buf[i]&15)
	}
	return id
}

// goidOffset is where the goroutine ID sits in the runtime's goroutine record,
// or -1 when GoroutineID must parse a stack trace instead.
var goidOffset = findGoidOffset()

// recordScan is how many bytes of the goroutine record findGoidOffset searches.
// The ID has sat about 150 bytes in, and the record is larger than 400 bytes
// in every Go release that builds Bellui.
const recordScan = 256

// findGoidOffset finds the goroutine ID in the runtime's goroutine record. The
// record's layout is private to the runtime and changes between Go releases,
// so the offset is not hard-coded. It is the one field that holds the ID the
// stack trace reports, in this goroutine and in four others. When no single
// field does, GoroutineID keeps parsing stack traces.
func findGoidOffset() int {
	if !haveGetg {
		return -1
	}
	offsets := matchingOffsets(nil)
	for i := 0; i < 4 && len(offsets) > 0; i++ {
		found := make(chan []uintptr)
		go func() { found <- matchingOffsets(offsets) }()
		offsets = <-found
	}
	if len(offsets) != 1 {
		return -1
	}
	return int(offsets[0])
}

// matchingOffsets returns the offsets, among candidates or across the whole
// searched record when candidates is nil, where the current goroutine's record
// holds its ID.
func matchingOffsets(candidates []uintptr) []uintptr {
	if candidates == nil {
		for off := uintptr(0); off+8 <= recordScan; off += 8 {
			candidates = append(candidates, off)
		}
	}
	id, g := stackGoroutineID(), getg()
	var matches []uintptr
	for _, off := range candidates {
		if *(*uint64)(unsafe.Add(g, off)) == id {
			matches = append(matches, off)
		}
	}
	return matches
}
