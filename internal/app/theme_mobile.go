//go:build android || ios || mobile

package app

import (
	"github.com/belluikit/bellui"
)

// SystemTheme contains the system’s theme variant.
// It is intended for internal use, only!
var SystemTheme bellui.ThemeVariant

// DefaultVariant returns the systems default bellui.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() bellui.ThemeVariant {
	return SystemTheme
}
