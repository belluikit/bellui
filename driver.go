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
	// It is the position from layout and ignores transforms ([core.Transformable]); to find where a point of
	// an object is drawn, use [AbsolutePositionForPoint].
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
	// Since: 1.0
	DoFromGoroutine(fn func(), wait bool)
}

// DoubleTapDriver is an optional interface that a [Driver] can implement to report
// the maximum duration for double-tap recognition.
type DoubleTapDriver interface {
	DoubleTapDelay() time.Duration
}

// OnScreenDriver is an optional interface that a [Driver] can implement to
// report whether an object is on screen. See [IsOnScreen].
type OnScreenDriver interface {
	IsOnScreen(core.CanvasObject) bool
}

// PositionMappingDriver is an optional interface that a [Driver] can implement
// to map positions between an object's own coordinates and its canvas through
// the object's world transform, the one it is drawn and hit with
// ([core.Transformable]). Positions on the canvas are measured from the same
// origin as [Driver.AbsolutePositionForObject]. Use [AbsolutePositionForPoint]
// and [PositionForAbsolutePoint], which fall back when the driver does not
// implement it.
type PositionMappingDriver interface {
	// AbsolutePositionForPoint returns where pos, in obj's own coordinates, is
	// drawn on obj's canvas.
	AbsolutePositionForPoint(obj core.CanvasObject, pos geo.Position) geo.Position
	// PositionForAbsolutePoint maps abs, a position on obj's canvas, into obj's
	// own coordinates. ok is false when obj is on no canvas or its transform
	// cannot be inverted.
	PositionForAbsolutePoint(obj core.CanvasObject, abs geo.Position) (pos geo.Position, ok bool)
}

// AbsolutePositionForPoint returns where pos, in obj's own coordinates, is
// drawn on obj's canvas: under a translation, a rotation or a scale on obj or
// an ancestor, the point moves with the drawing. Popups anchored to a widget
// use it, for example the point just below its bottom-left corner. Without a
// transform it is [Driver.AbsolutePositionForObject] plus pos, which is also
// the fallback when the driver does not implement [PositionMappingDriver].
// Call it on the main goroutine.
func AbsolutePositionForPoint(obj core.CanvasObject, pos geo.Position) geo.Position {
	d := CurrentApp().Driver()
	if m, ok := d.(PositionMappingDriver); ok {
		return m.AbsolutePositionForPoint(obj, pos)
	}
	return d.AbsolutePositionForObject(obj).Add(pos)
}

// PositionForAbsolutePoint maps abs, a position on obj's canvas, into obj's own
// coordinates, the inverse of [AbsolutePositionForPoint]. When the driver does
// not implement [PositionMappingDriver], or obj's transform cannot be inverted,
// it returns abs minus [Driver.AbsolutePositionForObject].
// Call it on the main goroutine.
func PositionForAbsolutePoint(obj core.CanvasObject, abs geo.Position) geo.Position {
	d := CurrentApp().Driver()
	if m, ok := d.(PositionMappingDriver); ok {
		if pos, ok := m.PositionForAbsolutePoint(obj, abs); ok {
			return pos
		}
	}
	return abs.Subtract(d.AbsolutePositionForObject(obj))
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

// IsOnScreen reports whether obj is drawn: the last frame of a window that is
// shown placed it, and it and everything above it are still visible. So it is
// false on a tab that is not selected, under a hidden container, in a window
// that is not shown, and before the first frame that draws it; and it turns
// false as soon as an ancestor is hidden, without waiting for the next frame.
// An object scrolled out of a scroll's view is still on screen.
//
// It reports visibility, not what to do about it: an animation can skip its
// values, a video keep its sound but decode no frames, a chart keep its data
// but not redraw. Code that runs on its own clock asks once per tick. Call it
// on the main goroutine. A refresh of an object out of sight already draws no
// frame; this is for the work done before refreshing (#1061).
//
// Falls back to obj.Visible() if the driver doesn't implement [OnScreenDriver].
func IsOnScreen(obj core.CanvasObject) bool {
	if a := CurrentApp(); a != nil {
		if osd, ok := a.Driver().(OnScreenDriver); ok {
			return osd.IsOnScreen(obj)
		}
	}
	return obj.Visible()
}
