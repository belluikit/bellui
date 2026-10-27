// Separate module so x/mmedia's own go.mod stays free of the app/GUI
// dependency tree this probe pulls in, like hack/playprobe.
module github.com/belluikit/bellui/x/mmedia/hack/audiorateprobe

go 1.26.2

require (
	github.com/belluikit/bellui v1.3.0-rc.1
	github.com/belluikit/bellui/x/mmedia v0.0.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/belluikit/fsnotify v1.10.2 // indirect
	github.com/belluikit/gl-js v0.2.1 // indirect
	github.com/belluikit/glfw v1.10.0 // indirect
	github.com/belluikit/glfw-js v1.0.4 // indirect
	github.com/belluikit/goffmpeg v0.5.0 // indirect
	github.com/belluikit/goffmpeg-libs/android_amd64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/android_arm v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/android_arm64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/darwin_amd64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/darwin_arm64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/ios_arm64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/iossimulator_arm64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/linux_amd64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/linux_arm64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/windows_amd64 v0.3.1 // indirect
	github.com/belluikit/goffmpeg-libs/windows_arm64 v0.3.1 // indirect
	github.com/belluikit/systray v1.12.2 // indirect
	github.com/belluikit/typesetting v0.3.6 // indirect
	github.com/belluikit/webgpu v1.2.2 // indirect
	github.com/belluikit/webgpu/libs-android v0.0.0-20260731101013-82e3bdc8353b // indirect
	github.com/belluikit/webgpu/libs-darwin v0.0.0-20260731101014-ce4f86d4438c // indirect
	github.com/belluikit/webgpu/libs-ios v0.0.0-20260727093643-fcde44a448a6 // indirect
	github.com/belluikit/webgpu/libs-linux v0.0.0-20260731101015-884076933a5d // indirect
	github.com/belluikit/webgpu/libs-windows v0.0.0-20260727093645-6b8e55d25f27 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/fredbi/uri v1.1.1 // indirect
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71 // indirect
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1.0.20260628091122-0bd588dc30cf // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/hack-pad/go-indexeddb v0.3.2 // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.5.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rymdport/portal v0.4.2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/trendvidia/protowire-go v1.2.2 // indirect
	github.com/yuin/goldmark v1.8.5 // indirect
	golang.org/x/image v0.44.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/belluikit/bellui/x/mmedia => ../..
