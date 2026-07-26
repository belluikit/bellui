//go:build !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd)

package app

import (
	"sync/atomic"

	"github.com/belluikit/bellui"
)

// CurrentVariant contains the system’s theme variant.
// It is intended for internal use, only!
var CurrentVariant atomic.Uint64

// DefaultVariant returns the systems default bellui.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() bellui.ThemeVariant {
	return bellui.ThemeVariant(CurrentVariant.Load())
}
