// Separate module so the runtime-spike harness can pull the full Fyne app
// stack without adding those deps to the fycoder library's go.mod.
// The replace points at the repo root, so it always builds against the
// working-tree fycoder, not a published version.
module github.com/belluikit/bellui/x/editor/examples/wasm

go 1.25.0

require (
	github.com/belluikit/bellui/x/editor v0.0.0
	github.com/belluikit/bellui v2.15.3
)

require (
	github.com/belluikit/systray v1.12.0 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fredbi/uri v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fyne-io/image v0.1.1 // indirect
	github.com/fyne-io/oksvg v0.2.0 // indirect
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71 // indirect
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/hack-pad/go-indexeddb v0.3.2 // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.5.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rymdport/portal v0.4.2 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/belluikit/gl-js v0.2.1 // indirect
	github.com/belluikit/glfw v1.0.1 // indirect
	github.com/belluikit/glfw-js v1.0.3 // indirect
	github.com/belluikit/render v0.2.3 // indirect
	github.com/belluikit/typesetting v0.3.5 // indirect
	github.com/belluikit/webgpu v1.0.6 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	golang.org/x/image v0.41.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/belluikit/bellui/x/editor => ../..

// WASM renderer fix: upstream fyne-io/gl-js v0.2.0 panics on first paint
// (EnsureCanvasFBO -> TexImage2D with a typed-nil pixel slice). Point at the
// trendvidia/gl-js fork (renamed module path) that returns js.Null() for
// empty slices — fix merged in trendvidia/gl-js#1, released as v0.2.1.
// Remove this once trendvidia/fyne itself replaces gl-js with the fixed fork.
replace github.com/belluikit/gl-js => github.com/belluikit/gl-js v0.2.1
