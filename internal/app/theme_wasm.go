//go:build wasm

package app

import (
	"syscall/js"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/theme"
)

// DefaultVariant returns the systems default bellui.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() bellui.ThemeVariant {
	matches := js.Global().Call("matchMedia", "(prefers-color-scheme: dark)")
	if matches.Truthy() {
		if matches.Get("matches").Bool() {
			return theme.VariantDark
		}
		return theme.VariantLight
	}
	return theme.VariantDark
}
