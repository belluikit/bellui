package core

// OnActivation is an optional hook invoked once each time an interactive object
// is activated: a pointer tap, or a screen-reader "press"/"toggle"/"select"
// action (screen-reader activations route through the same Tapped dispatch, so
// they are covered for free). It is nil unless a consumer installs it, so an app
// that never imports a consumer pays a single nil check per activation.
//
// The seam mirrors the SetFontFallbackRegister/RegisterTextMeasure hook
// pattern: a package var wired by an optional peripheral package, keeping core
// dependency-free and widget-free. The analytics package's EnableAutoCapture is
// the reference consumer. viaKeyboard is true when the activation came from a
// keyboard Enter/Space on the focused object rather than a pointer tap.
//
// It is always called on the main goroutine, from the driver's input dispatch,
// right where the object's Tapped is fired. Keep the callback cheap and
// non-blocking; hand heavy work to a queue (as the analytics tracker does).
var OnActivation func(obj CanvasObject, viaKeyboard bool)

// NotifyActivation invokes [OnActivation] when a consumer is installed. Drivers
// call it at their tap/activate dispatch sites, once per activation, right where
// the object's Tapped is fired. It is a single nil check when no consumer is
// registered.
func NotifyActivation(obj CanvasObject, viaKeyboard bool) {
	if OnActivation != nil && obj != nil {
		OnActivation(obj, viaKeyboard)
	}
}
