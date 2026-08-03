module github.com/belluikit/bellui

go 1.25.0

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/belluikit/fsnotify v1.9.1
	github.com/belluikit/gl-js v0.2.1
	github.com/belluikit/glfw v1.9.0
	github.com/belluikit/systray v1.12.0
	github.com/belluikit/typesetting v0.3.6
	// WebGPU rendering backend (#198), now the default painter: the impl in
	// internal/painter/webgpu builds under `//go:build !gl`, so plain `go mod tidy`
	// keeps it and GL is the opt-in fallback via `-tags gl`. belluikit/webgpu
	// forks oliverbestmann/webgpu (the maintained continuation of the
	// cogentcore/webgpu lineage) to add two platform slices upstream does not
	// ship — the arm64 iOS Simulator archive and a windows/arm64 GNU-ABI
	// rebuild (#1) — and, as of v1.2.0, to source all five platform libs
	// modules from branches that carry the wgpu-native MIT-OR-Apache-2.0
	// notice, which upstream's binary-asset modules lack (#21).
	github.com/belluikit/webgpu v1.2.0
	github.com/fredbi/uri v1.1.1
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71
	github.com/godbus/dbus/v5 v5.2.2
	github.com/hack-pad/go-indexeddb v0.3.2
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/nicksnyder/go-i18n/v2 v2.5.1
	github.com/rymdport/portal v0.4.2
	github.com/stretchr/testify v1.11.1
	github.com/yuin/goldmark v1.8.5
	golang.org/x/image v0.44.0
	golang.org/x/net v0.57.0
	golang.org/x/sys v0.47.0
	golang.org/x/text v0.40.0
)

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/belluikit/glfw-js v1.0.4
	github.com/belluikit/gocef v0.5.0
	github.com/go-text/typesetting-utils v0.0.0-20260419141703-4ffe8874dabc
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25
	github.com/trendvidia/protowire-go v1.2.2
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/belluikit/webgpu/libs-android v0.0.0-20260731101013-82e3bdc8353b // indirect
	github.com/belluikit/webgpu/libs-darwin v0.0.0-20260731101014-ce4f86d4438c // indirect
	github.com/belluikit/webgpu/libs-ios v0.0.0-20260727093643-fcde44a448a6 // indirect
	github.com/belluikit/webgpu/libs-linux v0.0.0-20260731101015-884076933a5d // indirect
	github.com/belluikit/webgpu/libs-windows v0.0.0-20260727093645-6b8e55d25f27 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1.0.20260628091122-0bd588dc30cf // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
