package theme

// DarkTheme and LightTheme expose the built-in variant themes to tests only.
// They were removed from the public API (applications must follow the user's
// variant preference via DefaultTheme, not hard-code one); tests that need a
// deterministic variant use these, so masters stay stable.
var (
	DarkTheme  = darkTheme
	LightTheme = lightTheme
)
