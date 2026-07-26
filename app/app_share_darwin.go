//go:build !ci && !ios && !wasm && !mobile && !tinygo

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#include <stdlib.h>

int shareItems(const char **items, int count, int isFile);
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/bellui"
)

// ShareText presents the macOS share sheet (NSSharingServicePicker) for the
// given text, anchored on the key window. Part of [bellui.SharerApp].
func (a *belluiApp) ShareText(text string) error {
	return shareNative([]string{text}, false)
}

// ShareFile presents the macOS share sheet for the given file URI (which must
// use the "file" scheme). Part of [bellui.SharerApp].
func (a *belluiApp) ShareFile(uri bellui.URI) error {
	if uri == nil || uri.Scheme() != "file" {
		return bellui.ErrShareNotSupported
	}
	return shareNative([]string{uri.Path()}, true)
}

func shareNative(items []string, isFile bool) error {
	if len(items) == 0 {
		return bellui.ErrShareNotSupported
	}

	cItems := make([]*C.char, len(items))
	for i, s := range items {
		cItems[i] = C.CString(s)
		defer C.free(unsafe.Pointer(cItems[i]))
	}
	flag := C.int(0)
	if isFile {
		flag = 1
	}
	if C.shareItems(&cItems[0], C.int(len(items)), flag) != 0 {
		return bellui.ErrShareNotSupported
	}
	return nil
}

var _ bellui.SharerApp = (*belluiApp)(nil)
