//go:build !ci && !ios && !wasm && !mobile

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#include <AppKit/AppKit.h>

bool isBundled();
void watchTheme();
*/
import "C"

import (
	"net/url"
	"os"
	"os/exec"

	"github.com/belluikit/bellui"
)

func (a *belluiApp) OpenURL(url *url.URL) error {
	cmd := exec.Command("open", url.String())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

//export themeChanged
func themeChanged() {
	bellui.CurrentApp().Settings().(*settings).setupTheme()
}

func watchTheme(_ *settings) {
	C.watchTheme()
}

func (a *belluiApp) registerRepositories() {
	// no-op
}
