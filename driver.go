package bellui

import (
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Driver defines an abstract concept of a Bellui render driver.
// Any implementation must provide at least these methods.
type Driver interface {
	// CreateWindow creates a new UI Window for a certain implementation.
	// Developers should use [App.NewWindow].
	CreateWindow(string) Window
	// AllWindows returns a slice containing all app windows.
	AllWindows() []Window

	// RenderedTextSize returns the size required to render the given string of specified
	// font size and style. It also returns the height to text baseline, measured from the top.
	// If the source is specified it will be used, otherwise the current theme will be asked for the font.
	RenderedTextSize(text string, fontSize float32, style TextStyle, source Resource) (size geo.Size, baseline float32)

	// CanvasForObject returns the canvas that is associated with a given [core.CanvasObject].
	CanvasForObject(core.CanvasObject) core.Canvas
	// AbsolutePositionForObject returns the position of a given [core.CanvasObject] relative to the top/left of a canvas.
	AbsolutePositionForObject(core.CanvasObject) geo.Position

	// Device returns the device that the application is currently running on.
	Device() Device
	// Run starts the main event loop of the driver.
	Run()
	// Quit closes the driver and open windows, then exit the application.
	// On some operating systems this does nothing, for example iOS and Android.
	Quit()

	// StartAnimation registers a new animation with this driver and requests it be started.
	// Developers should use the [Animation.Start] function.
	StartAnimation(*Animation)
	// StopAnimation stops an animation and unregisters from this driver.
	// Developers should use the [Animation.Stop] function.
	StopAnimation(*Animation)

	// DoFromGoroutine provides a way to queue a function `fn` that is running on a goroutine back to
	// the central thread for Bellui updates, waiting for it to return if `wait` is true.
	// The driver provides the implementation normally accessed through [bellui.Do].
	// This is required when background tasks want to execute code safely in the graphical context.
	//
	// Since: 2.6
	DoFromGoroutine(fn func(), wait bool)
}

// DoubleTapDriver is an optional interface that a [Driver] can implement to report
// the maximum duration for double-tap recognition.
type DoubleTapDriver interface {
	DoubleTapDelay() time.Duration
}

// ScreenBlankingDriver is an optional interface that a [Driver] can implement
// to allow an app to ask the device not to sleep/lock/blank displays.
type ScreenBlankingDriver interface {
	SetDisableScreenBlanking(bool)
}

// AccessibilityAnnouncer is an optional interface that a [Driver] can
// implement to push a transient status message to a running screen reader (a
// "live region" announcement). Widgets whose text appears without the user
// navigating to it — toasts, snackbars, status updates — use it so the
// message is spoken before it auto-dismisses; applications can call it via
// [AnnounceToScreenReader] for their own transient status.
//
// assertive requests that the message interrupt current speech; polite
// (false) queues it after. The call is a no-op when the toolkit was built
// without the `accessibility` tag or no screen-reader backend is active.
type AccessibilityAnnouncer interface {
	AnnounceToScreenReader(text string, assertive bool)
}

// AnnounceToScreenReader asks the current driver to speak a transient status
// message through a running screen reader, if the driver supports it. See
// [AccessibilityAnnouncer]. Call it on the main goroutine.
func AnnounceToScreenReader(text string, assertive bool) {
	if a := CurrentApp(); a != nil {
		if an, ok := a.Driver().(AccessibilityAnnouncer); ok {
			an.AnnounceToScreenReader(text, assertive)
		}
	}
}

const defaultDoubleTapDelay = 300 * time.Millisecond

// DoubleTapDelay returns the maximum duration where a second tap after a first one
// will be considered a double-tap. Falls back to 300ms if the driver doesn't implement
// [DoubleTapDriver].
func DoubleTapDelay() time.Duration {
	if a := CurrentApp(); a != nil {
		if dtd, ok := a.Driver().(DoubleTapDriver); ok {
			return dtd.DoubleTapDelay()
		}
	}
	return defaultDoubleTapDelay
}
