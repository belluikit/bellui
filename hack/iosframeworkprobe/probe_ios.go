//go:build ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#include <Probe/Probe.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>

static char *backgroundModes(void) {
	NSArray *modes = [[NSBundle mainBundle] objectForInfoDictionaryKey:@"UIBackgroundModes"];
	NSString *s = modes ? [modes componentsJoinedByString:@","] : @"";
	return strdup([s UTF8String]);
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// probeAnswer calls into the embedded Probe.framework.
func probeAnswer() int { return int(C.probe_answer()) }

// backgroundModes reads UIBackgroundModes back from the running app's
// Info.plist.
func backgroundModes() []string {
	c := C.backgroundModes()
	defer C.free(unsafe.Pointer(c))
	s := C.GoString(c)
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
