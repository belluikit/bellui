//go:build !ci && !ios && !wasm && !mobile

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework CoreServices

void registerOpenEventHandlers();
*/
import "C"

import (
	"net/url"
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

// platformLaunchOpen registers the macOS Apple Event handlers (odoc/GURL) that
// Launch Services uses to tell a running or launching app to open a document or
// URL. A terminal launch still passes the target in argv, so that is parsed too
// (a Finder launch carries no argv, so this is a harmless no-op there).
func platformLaunchOpen(a *belluiApp) {
	a.dispatchOpenArgs(os.Args[1:])
	C.registerOpenEventHandlers()
}

//export belluiHandleOpenURL
func belluiHandleOpenURL(curl *C.char) {
	a, ok := bellui.CurrentApp().(*belluiApp)
	if !ok {
		return
	}
	u, err := url.Parse(C.GoString(curl))
	if err != nil {
		bellui.LogError("Failed to parse URL from Apple Event", err)
		return
	}
	a.openURL(u)
}

//export belluiHandleOpenFile
func belluiHandleOpenFile(cpath *C.char) {
	a, ok := bellui.CurrentApp().(*belluiApp)
	if !ok {
		return
	}
	a.openFile(storage.NewFileURI(C.GoString(cpath)))
}
