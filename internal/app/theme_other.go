//go:build !linux && !darwin && !windows && !freebsd && !openbsd && !netbsd && !wasm

package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/theme"
)

// DefaultVariant returns the systems default bellui.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() bellui.ThemeVariant {
	return theme.VariantDark
}
