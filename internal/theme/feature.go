package theme

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// RenderStyle identifies a visual rendering style that widget renderers can switch on.
type RenderStyle string

const (
	// RenderStyleClassic is the original Bellui look. This is the default when the
	// theme does not implement RenderStyleTheme.
	RenderStyleClassic RenderStyle = ""

	// RenderStyleModern provides enhanced visuals: gradient buttons, deeper shadows,
	// larger corner radii, and stronger hover/focus feedback.
	RenderStyleModern RenderStyle = "modern"
)

// RenderStyleTheme is implemented by themes that provide a render style selection.
// Widget renderers type-assert the current theme to this interface to decide
// between classic and modern visuals.
type RenderStyleTheme interface {
	RenderStyle() RenderStyle
}

// MobileTheme is implemented by themes that override the device-is-mobile flag.
// ThemeOverride uses this to force mobile layout in containers like AppTabs.
type MobileTheme interface {
	DeviceIsMobile() bool
}

// RenderStyleForWidget returns the RenderStyle for the given widget's theme.
// Returns RenderStyleClassic if the theme does not provide a render style.
func RenderStyleForWidget(w core.Widget) RenderStyle {
	if custom := cache.WidgetTheme(w); custom != nil {
		if rs, ok := custom.(RenderStyleTheme); ok {
			return rs.RenderStyle()
		}
	}

	app := bellui.CurrentApp()
	if app == nil {
		return RenderStyleClassic
	}
	th := app.Settings().Theme()
	if th == nil {
		return RenderStyleClassic
	}
	th = CurrentlyRenderingWithFallback(th)
	if rs, ok := th.(RenderStyleTheme); ok {
		return rs.RenderStyle()
	}

	return RenderStyleClassic
}

// DeviceIsMobileForWidget checks whether the widget's theme overrides the
// device-is-mobile flag. Returns the override value and true if set, or
// false and false if no override exists.
func DeviceIsMobileForWidget(w core.Widget) (mobile bool, ok bool) {
	if custom := cache.WidgetTheme(w); custom != nil {
		if mt, is := custom.(MobileTheme); is {
			return mt.DeviceIsMobile(), true
		}
	}

	app := bellui.CurrentApp()
	if app == nil {
		return false, false
	}
	th := app.Settings().Theme()
	if th == nil {
		return false, false
	}
	th = CurrentlyRenderingWithFallback(th)
	if mt, is := th.(MobileTheme); is {
		return mt.DeviceIsMobile(), true
	}

	return false, false
}
