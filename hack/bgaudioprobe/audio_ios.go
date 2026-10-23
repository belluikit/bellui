//go:build ios

package main

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework UIKit -framework Foundation
#include <stdint.h>
#include <stdlib.h>

const char *bgaStart(void);
uint64_t bgaFrames(void);
int bgaRunning(void);
int bgaState(void);
int bgaInterruptions(void);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func startAudio() error {
	if msg := C.bgaStart(); msg != nil {
		defer C.free(unsafe.Pointer(msg))
		return errors.New(C.GoString(msg))
	}
	return nil
}

func audioStats() stats {
	states := map[C.int]string{0: "active", 1: "inactive", 2: "background"}
	return stats{
		state:         states[C.bgaState()],
		frames:        uint64(C.bgaFrames()),
		running:       C.bgaRunning() != 0,
		interruptions: int(C.bgaInterruptions()),
	}
}
