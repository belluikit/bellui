//go:build !ios && !wasm && !mobile

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#include <AppKit/AppKit.h>

bool isDarkMode();
*/
import "C"

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/theme"
)

// DefaultVariant returns the systems default bellui.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() bellui.ThemeVariant {
	if C.isDarkMode() {
		return theme.VariantDark
	}
	return theme.VariantLight
}
