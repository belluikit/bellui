package core

// ToolTipable is implemented by a CanvasObject that offers hover tooltip text.
// widget.BaseWidget satisfies it through SetToolTipText, so any widget can carry
// a tooltip with widget.Button{...}.SetToolTipText("New file"). A non-empty
// ToolTip() tells the driver to show a hover bubble after a short delay; an empty
// string means the object has no tooltip.
//
// This is the driver-level replacement for the overlay-based widget.Tooltip,
// which never fired on the glfw desktop driver: the driver delivers hover to the
// widget itself reliably, so tooltips ride that instead of an invisible
// hit-test overlay (#800).
type ToolTipable interface {
	ToolTip() string
}

// ShowToolTip and HideToolTip are an optional hook installed by the widget
// package to render hover tooltips; both are nil until then, so a driver that
// runs without the widget package (or before it initializes) pays a single nil
// check per pointer move. The seam mirrors the OnActivation /
// SetFontFallbackRegister hook pattern, keeping core widget-free.
//
// A driver calls [NotifyToolTip] when the tooltip target under the pointer
// changes: with the new [ToolTipable] target (which starts the show delay), or
// with nil (which hides any visible tooltip). Both run on the main goroutine
// from the driver's hover dispatch. The installed controller owns the delay
// timer, the bubble overlay and its lifecycle.
var (
	ShowToolTip func(obj CanvasObject, c Canvas)
	HideToolTip func()
)

// NotifyToolTip forwards a tooltip target change to the installed controller. A
// nil obj hides any visible tooltip; a non-empty [ToolTipable] obj starts the
// show delay. It is a single nil check when no controller is registered.
func NotifyToolTip(obj CanvasObject, c Canvas) {
	if obj == nil {
		if HideToolTip != nil {
			HideToolTip()
		}
		return
	}
	if ShowToolTip != nil {
		ShowToolTip(obj, c)
	}
}
