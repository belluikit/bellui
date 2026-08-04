package bellui

// BuildType defines different modes that an application can be built using.
type BuildType int

const (
	// BuildStandard is the normal build mode - it is not debug, test or release mode.
	BuildStandard BuildType = iota
	// BuildDebug is used when a developer would like more information and visual output for app debugging.
	BuildDebug
	// BuildRelease is a final production build, it is like [BuildStandard] but will use distribution certificates.
	// A release build is typically going to connect to live services and is not usually used during development.
	BuildRelease
)

// Settings describes the application configuration available.
type Settings interface {
	Theme() Theme
	SetTheme(Theme)
	// ThemeVariant defines which preferred version of a theme should be used (i.e. light or dark)
	//
	// Since: 2.0
	ThemeVariant() ThemeVariant
	// SetThemeVariant sets the preferred theme variant (light or dark) and re-applies the current theme.
	SetThemeVariant(ThemeVariant)
	Scale() float32
	// PrimaryColor indicates a user preference for a named primary color
	//
	// Since: 1.4
	PrimaryColor() string

	// AddListener registers a callback that is invoked whenever the settings change.
	//
	// Since: 2.6
	AddListener(func(Settings))

	BuildType() BuildType

	ShowAnimations() bool
}

// PrimaryColorSettings describes the optional capability of [Settings]
// implementations that can change the user's named primary color at runtime.
// The settings of a running app (and of the test app) implement it.
//
// Since: 1.0
type PrimaryColorSettings interface {
	Settings

	// SetPrimaryColor sets the preference for a named primary color and
	// re-applies the current theme. The change affects the running app only;
	// it is not written to the system-wide Bellui settings.
	SetPrimaryColor(name string)
}
