package desktop

// ScreenArea describes a rectangle on the virtual desktop in the same loose
// native-system coordinates as [Window.RequestPosition] — positions inside it
// can be passed to RequestPosition verbatim.
//
// Since: 1.0
type ScreenArea struct {
	X, Y          int
	Width, Height int
}

// ScreenInfo describes a single display attached to the system, providing
// enough information for window placement logic — restoring a saved position
// onto a screen that still exists, or placing a window on a specific display.
// It is deliberately not a display-configuration API.
//
// Since: 1.0
type ScreenInfo struct {
	// Name is the system-reported name of the display.
	Name string

	// Bounds is the full extent of the display on the virtual desktop.
	Bounds ScreenArea

	// WorkArea is the part of Bounds not occupied by task bars, docks or
	// menu bars. On systems that do not report a work area it equals Bounds.
	WorkArea ScreenArea

	// Scale is the output scale Bellui detected for this display — the factor
	// between Bellui's device-independent units and the display's native
	// coordinates, before any user scale preference is applied.
	Scale float32

	// Primary reports whether this is the system's primary display.
	Primary bool
}

// ScreensDriver describes drivers that can enumerate the displays attached
// to the system. The GLFW desktop driver implements it; mobile and web
// drivers do not. Call on the main goroutine.
//
// On Linux Wayland global coordinates are compositor-private, so the reported
// bounds are best-effort and may not relate to actual window positions.
//
// Since: 1.0
type ScreensDriver interface {
	// Screens returns information about every display attached to the
	// system, in system order. The primary display has Primary set.
	Screens() []ScreenInfo
}
