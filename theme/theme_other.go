//go:build !hints

package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
)

var (
	fallbackColor = color.Transparent
	fallbackIcon  = &bellui.StaticResource{}
)
