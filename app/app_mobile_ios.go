//go:build !ci && ios && !mobile

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UIKit -framework CoreGraphics -framework UserNotifications

#include <stdlib.h>

void openURL(char *urlStr);
void sendNotification(char *title, char *content);
void shareItem(char *value, int isFile);
*/
import "C"

import (
	"net/url"
	"unsafe"

	"github.com/belluikit/bellui"
)

var _ bellui.SharerApp = (*belluiApp)(nil)

func (a *belluiApp) OpenURL(url *url.URL) error {
	urlStr := C.CString(url.String())
	C.openURL(urlStr)
	C.free(unsafe.Pointer(urlStr))

	return nil
}

// ShareText presents the iOS share sheet (UIActivityViewController) for the
// given text, implementing bellui.SharerApp (fyne#711).
func (a *belluiApp) ShareText(text string) error {
	return shareNative(text, false)
}

// ShareFile presents the iOS share sheet for the given file URI (which must use
// the "file" scheme), implementing bellui.SharerApp (fyne#711).
func (a *belluiApp) ShareFile(uri bellui.URI) error {
	if uri == nil || uri.Scheme() != "file" {
		return bellui.ErrShareNotSupported
	}
	return shareNative(uri.Path(), true)
}

func shareNative(value string, isFile bool) error {
	valueStr := C.CString(value)
	defer C.free(unsafe.Pointer(valueStr))

	flag := C.int(0)
	if isFile {
		flag = 1
	}
	C.shareItem(valueStr, flag)
	return nil
}
