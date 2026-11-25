//go:build ios

package main

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
#include <stdlib.h>

int micPermission(void);
const char *micStart(void);
void micStop(void);
void micStats(unsigned long long *bufs, unsigned long long *frames, double *sumSq, int *configChanges, int *running);
const char *sessionCategory(void);
const char *sessionOutputs(void);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func micPermission() bool { return C.micPermission() != 0 }

func micStart() error {
	if e := C.micStart(); e != nil {
		defer C.free(unsafe.Pointer(e))
		return errors.New(C.GoString(e))
	}
	return nil
}

func micStop() { C.micStop() }

func readMic() micStats {
	var bufs, frames C.ulonglong
	var sumSq C.double
	var changes, running C.int
	C.micStats(&bufs, &frames, &sumSq, &changes, &running)
	return micStats{bufs: uint64(bufs), frames: uint64(frames), sumSq: float64(sumSq), configChanges: int(changes), running: running != 0}
}

func sessionCategory() string {
	c := C.sessionCategory()
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}

func sessionOutputs() string {
	c := C.sessionOutputs()
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}
