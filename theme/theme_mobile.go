//go:build android || ios || mobile

package theme

import "github.com/belluikit/bellui"

func setupSystemTheme(fallback bellui.Theme) bellui.Theme {
	return fallback
}
