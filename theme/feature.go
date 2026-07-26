package theme

import internaltheme "github.com/belluikit/bellui/internal/theme"

// RenderStyle identifies a visual rendering style that widget renderers can switch on.
type RenderStyle = internaltheme.RenderStyle

const (
	// RenderStyleClassic is the original Bellui look. This is the default when the
	// theme does not implement RenderStyleTheme.
	RenderStyleClassic = internaltheme.RenderStyleClassic

	// RenderStyleModern provides enhanced visuals such as gradient buttons,
	// card background fills and deeper shadows.
	RenderStyleModern = internaltheme.RenderStyleModern
)

// RenderStyleTheme is implemented by themes that provide a render style selection.
// Custom themes wrapping enhanced or dense themes should implement this interface
// to propagate the render style through the wrapper.
type RenderStyleTheme = internaltheme.RenderStyleTheme

// MobileTheme is implemented by themes that override the device-is-mobile flag.
// ThemeOverride uses this to force mobile layout in containers like AppTabs.
type MobileTheme = internaltheme.MobileTheme
