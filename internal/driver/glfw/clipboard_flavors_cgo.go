//go:build (darwin || windows) && !wasm

package glfw

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// cPathArray copies paths into a C array of C strings for the native
// clipboard bridges. The returned free func releases the array and every
// string; it is safe to call for an empty list (nil array).
func cPathArray(paths []string) (**C.char, func()) {
	if len(paths) == 0 {
		return nil, func() {}
	}
	cPaths := (**C.char)(C.malloc(C.size_t(len(paths)) * C.size_t(unsafe.Sizeof(uintptr(0)))))
	cSlice := unsafe.Slice(cPaths, len(paths))
	for i, path := range paths {
		cSlice[i] = C.CString(path)
	}
	return cPaths, func() {
		for _, p := range cSlice {
			C.free(unsafe.Pointer(p))
		}
		C.free(unsafe.Pointer(cPaths))
	}
}
