package bellui

import (
	"net/url"
	"sync/atomic"
)

// An App is the definition of a graphical application.
// Apps can have multiple windows, by default they will exit when all windows
// have been closed. This can be modified using SetMaster or SetCloseIntercept.
// To start an application you need to call Run somewhere in your main function.
// Alternatively use the [ShowAndRun] function for your main window.
type App interface {
	// Create a new window for the application.
	// The first window to open is considered the "master" and when closed
	// the application will exit.
	//
	// How additional windows are presented depends on the platform:
	//   - Desktop: each window is a real, independent OS window; any number
	//     can be shown concurrently.
	//   - Web (wasm): the first window owns the single browser canvas;
	//     additional windows are shown as draggable in-canvas windows overlaid
	//     on it (there is one HTML canvas, so they cannot be separate OS
	//     windows).
	//   - Mobile: windows form a stack and only the topmost visible one is
	//     shown at a time. Opening a window covers the previous one; closing it
	//     reveals the previous again.
	NewWindow(title string) Window

	// Icon returns the application icon, this is used in various ways
	// depending on operating system.
	// This is also the default icon for new windows.
	Icon() Resource

	// SetIcon sets the icon resource used for this application instance.
	SetIcon(Resource)

	// Run the application - this starts the event loop and waits until [App.Quit]
	// is called or the last window closes.
	// This should be called near the end of a main() function as it will block.
	Run()

	// Calling Quit on the application will cause the application to exit
	// cleanly, closing all open windows.
	// This function does no thing on a mobile device as the application lifecycle is
	// managed by the operating system.
	Quit()

	// Driver returns the driver that is rendering this application.
	// Typically not needed for day to day work, mostly internal functionality.
	Driver() Driver

	// UniqueID returns the application unique identifier, if set.
	// This must be set for use of the [App.Preferences]. see [NewWithID].
	UniqueID() string

	// Settings return the globally set settings, determining theme and so on.
	Settings() Settings

	// Preferences returns the application preferences, used for storing configuration and state
	Preferences() Preferences

	// Storage returns a storage handler specific to this application.
	Storage() Storage

	// Lifecycle returns a type that allows apps to hook in to lifecycle events.
	//
	// Since: 1.0
	Lifecycle() Lifecycle

	// Metadata returns the application metadata that was set at compile time.
	// The items of metadata are available after "bellui package" or when running "go run"
	// Building with "go build" may cause this to be unavailable.
	//
	// Since: 1.0
	Metadata() AppMetadata

	// Clipboard returns the system clipboard.
	//
	// Since: 1.0
	Clipboard() Clipboard
}

// CloudApp is an optional interface that an [App] can implement to support cloud services.
type CloudApp interface {
	// CloudProvider returns the current app cloud provider,
	// if one has been registered by the developer or chosen by the user.
	CloudProvider() CloudProvider
	// SetCloudProvider allows developers to specify how this application should integrate with cloud services.
	SetCloudProvider(CloudProvider)
}

// NotifierApp is an optional interface that an [App] can implement to support system notifications.
type NotifierApp interface {
	// SendNotification sends a system notification that will be displayed in the
	// operating system's notification area.
	SendNotification(*Notification)
}

// NotifierWithOptionsApp is an optional interface that an [App] can implement to
// support rich system notifications with click activation, inline action buttons
// and a per-notification icon (see [NotificationOptions]).
//
// It is asserted from the running app like the other optional interfaces
// ([CloudApp], [BadgeApp], …). Where a capability is not available the
// implementation degrades to a plain notification rather than dropping it, and
// callbacks that the platform cannot deliver simply never fire — so a caller can
// always send the rich form and rely on at least the title and content showing.
//
// Callback delivery is on the main goroutine and session-scoped; see
// [NotificationOptions].
//
// Since: 1.0
type NotifierWithOptionsApp interface {
	// SendNotificationWithOptions sends a rich system notification.
	SendNotificationWithOptions(*NotificationOptions)
}

// BadgeApp is an optional interface that an [App] can implement to display a
// count badge on its icon in the OS launcher: the dock icon on macOS, the
// taskbar button on Windows, and the launcher entry on Linux desktops that
// support the Unity LauncherEntry protocol (KDE and others).
//
// It is best-effort — desktops without a badge affordance silently ignore it —
// and only implemented on desktop platforms; mobile and web apps do not satisfy
// this interface.
//
// Since: 1.0
type BadgeApp interface {
	// SetBadge sets the numeric badge on the app icon. A count of 0 (or less)
	// clears the badge. Call on the main goroutine.
	SetBadge(count int)
}

// LaunchAtLoginApp is an optional interface that an [App] can implement to
// control whether it starts automatically when the user logs in.
//
// This is a desktop capability (macOS, Windows, Linux); mobile and web apps do
// not satisfy this interface. It applies to installed applications — an
// unpackaged binary run from a build directory (e.g. "go run") registers a
// path that changes between builds, so enabling it there is of limited use, and
// on macOS the underlying service requires a bundled app (SetLaunchAtLogin
// returns an error when that is not the case).
//
// Since: 1.0
type LaunchAtLoginApp interface {
	// SetLaunchAtLogin enables or disables starting this app at user login,
	// returning an error if the change could not be applied (for example an
	// unbundled macOS binary, or insufficient permissions).
	SetLaunchAtLogin(enabled bool) error

	// LaunchAtLogin reports whether the app is currently registered to launch at
	// login. It returns false when the state cannot be determined.
	LaunchAtLogin() bool
}

// SingleInstanceApp is an optional interface that an [App] can implement to support
// single-instance mode with argument forwarding.
//
// When enabled, at most one process per application unique ID holds the "primary"
// role. A subsequent launch of the same app detects the running primary, forwards
// its command-line arguments to it, and reports that it is not the primary so the
// caller can exit without opening a second copy. This is the building block behind
// OS file-association and URL-scheme handling on Windows and Linux, where "open
// with" relaunches the binary with the target in the arguments.
//
// Since: 1.0
type SingleInstanceApp interface {
	// SetSingleInstance enables single-instance mode, keyed on the app's UniqueID.
	//
	// It reports whether this process became the primary instance. When primary is
	// false the arguments of this launch (os.Args[1:]) have already been forwarded to
	// the running primary, and the caller should exit without calling [App.Run] - no
	// driver or window has been created at this point, so there is no window flash.
	//
	// The onOpen callback is invoked on the main goroutine, with the arguments
	// forwarded from each subsequent launch that was redirected to this primary. It
	// may be nil to enable single-instance mode without receiving forwarded arguments.
	//
	// SetSingleInstance must be called before [App.Run]. It is idempotent: repeated
	// calls report the same role without rebinding. It returns an error if the app has
	// no unique ID (see [NewWithID]). On platforms that are inherently single-instance
	// (mobile, web) it is a no-op that reports primary true.
	//
	// Since: 1.0
	SetSingleInstance(onOpen func(args []string)) (primary bool, err error)
}

// OpenHandlerApp is an optional interface that an [App] can implement to be
// notified that it was launched — or, while already running, asked — to open a
// file or a URL. This is the receive half of OS "open with" / file-association
// and custom URL-scheme handling.
//
// A payload can arrive two ways: as the reason the app was launched (buffered
// and delivered once the app is running), or while the app is already running.
// The launch case works out of the box on every desktop platform and the web.
// The already-running case is automatic on macOS (Launch Services re-activates
// the instance); on Windows and Linux it is delivered through single-instance
// argument forwarding, so an app that also opts into [SingleInstanceApp] (which
// it does anyway to avoid opening a second copy) receives already-running
// "open with" events with no extra wiring.
//
// Both callbacks are invoked on the main goroutine, matching the model of other
// driver callbacks. Register them before [App.Run]. A nil callback clears the
// handler. On platforms without a delivery mechanism the setters are accepted
// but never fire (for example [SetOnFileOpened] on the web).
//
// Since: 1.0
type OpenHandlerApp interface {
	// SetOnFileOpened registers a callback fired with the URI of a file the app
	// was asked to open.
	SetOnFileOpened(func(URI))

	// SetOnURLOpened registers a callback fired with a URL the app was asked to
	// open (a custom scheme registered to the app, or a deep link).
	SetOnURLOpened(func(*url.URL))
}

// urlOpener is an unexported interface for apps that can open URLs.
type urlOpener interface {
	OpenURL(url *url.URL) error
}

// OpenURL opens a URL in the default browser application.
func OpenURL(url *url.URL) error {
	if a := CurrentApp(); a != nil {
		if opener, ok := a.(urlOpener); ok {
			return opener.OpenURL(url)
		}
	}
	return nil
}

// SendNotification sends a system notification via the current app, if it supports notifications.
func SendNotification(n *Notification) {
	if a := CurrentApp(); a != nil {
		if notifier, ok := a.(NotifierApp); ok {
			notifier.SendNotification(n)
		}
	}
}

// SendNotificationWithOptions sends a rich system notification (click activation,
// action buttons, custom icon) via the current app if it implements
// [NotifierWithOptionsApp]. If the app supports only plain notifications the call
// falls back to a basic title-and-content notification so it is never silently
// dropped; the callbacks and actions are lost in that case. It reports whether
// the rich form was delivered.
//
// Since: 1.0
func SendNotificationWithOptions(n *NotificationOptions) bool {
	a := CurrentApp()
	if a == nil || n == nil {
		return false
	}
	if rich, ok := a.(NotifierWithOptionsApp); ok {
		rich.SendNotificationWithOptions(n)
		return true
	}
	if notifier, ok := a.(NotifierApp); ok {
		notifier.SendNotification(&Notification{Title: n.Title, Content: n.Content})
	}
	return false
}

var app atomic.Pointer[App]

// SetCurrentApp is an internal function to set the app instance currently running.
func SetCurrentApp(current App) {
	app.Store(&current)
}

// CurrentApp returns the current application, for which there is only 1 per process.
func CurrentApp() App {
	val := app.Load()
	if val == nil {
		LogError("Attempt to access current Bellui app when none is started", nil)
		return nil
	}
	return *val
}

// AppMetadata captures the build metadata for an application.
//
// Since: 1.0
type AppMetadata struct {
	// ID is the unique ID of this application, used by many distribution platforms.
	ID string
	// Name is the human friendly name of this app.
	Name string
	// Version represents the version of this application, normally following semantic versioning.
	Version string
	// Build is the build number of this app, some times appended to the version number.
	Build int
	// Icon contains, if present, a resource of the icon that was bundled at build time.
	Icon Resource
	// Release if true this binary was build in release mode
	// Since: 1.0
	Release bool
	// Custom contain the custom metadata defined either in manifest.pxf or on the compile command line
	// Since: 1.0
	Custom map[string]string
	// Migrations allows an app to opt into features before they are standard
	// Since: 1.0
	Migrations map[string]bool
}

// Lifecycle represents the various phases that an app can transition through.
//
// Since: 1.0
type Lifecycle interface {
	// SetOnEnteredForeground hooks into the app becoming foreground and gaining focus.
	SetOnEnteredForeground(func())
	// SetOnExitedForeground hooks into the app losing input focus and going into the background.
	SetOnExitedForeground(func())
	// SetOnStarted hooks into an event that says the app is now running.
	SetOnStarted(func())
	// SetOnStopped hooks into an event that says the app is no longer running.
	SetOnStopped(func())
}
