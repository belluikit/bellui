package templates

import "text/template"

var (
	// MakefileUNIX is the template for the makefile on UNIX systems like Linux and BSD
	MakefileUNIX = template.Must(template.New("Makefile").Parse(makefile))

	// DesktopFileUNIX is the template the desktop file on UNIX systems like Linux and BSD
	DesktopFileUNIX = template.Must(template.New("DesktopFile").Parse(desktopEntry))

	// EntitlementsDarwin is a plist file that lists build entitlements for darwin releases
	EntitlementsDarwin = template.Must(template.New("Entitlements").Parse(entitlementsDarwinPlist))

	// EntitlementsDarwinMobile is a plist file that lists build entitlements for iOS releases
	EntitlementsDarwinMobile = template.Must(template.New("EntitlementsMobile").Parse(entitlementsIosPlist))

	// BelluiMetadataInit is the metadata injecting file for bellui metadata
	BelluiMetadataInit = template.Must(template.New("bellui_metadata_init.got").Parse(belluiMetadataInit))

	// BelluiPprofInit is the file injected to support pprof
	BelluiPprofInit = template.Must(template.New("bellui_pprof_init.got").Parse(belluiPprof))

	// HelloWorld is a simple hello word app used to initialize new projects
	HelloWorld = template.Must(template.New("hello_world.got").Parse(helloWorld))

	// ManifestAndroid is the default manifest for building an Android application
	ManifestAndroid = template.Must(template.New("AndroidManifest").Parse(androidManifestXML))

	// ManifestWindows is the manifest file for windows packaging
	ManifestWindows = template.Must(template.New("Manifest").Parse(appManifest))

	// AppxManifestWindows is the manifest file for windows packaging
	AppxManifestWindows = template.Must(template.New("ReleaseManifest").Parse(appxManifestXML))

	// InfoPlistDarwin is the manifest file for darwin packaging
	InfoPlistDarwin = template.Must(template.New("InfoPlist").Parse(infoPlist))

	// XCAssetsDarwin is the Contents.json file for darwin xcassets bundle
	XCAssetsDarwin = template.Must(template.New("XCAssets").Parse(xcassetsJSON))

	// IndexHTML is the index.html used to serve web package
	IndexHTML = template.Must(template.New("index.html").Parse(indexHTML))

	// CSSDark is a CSS that define color for element on the web splash screen following the dark theme
	CSSDark = darkCSS

	// CSSLight is a CSS that define color for element on the web splash screen following the light theme
	CSSLight = lightCSS

	// DefaultIcon is the bundled default application icon, used when a
	// package has no Icon.png of its own (1024x1024 PNG). Its origin and the
	// gopher design credit are in THIRD_PARTY_NOTICES.md.
	DefaultIcon = defaultIconPNG

	// SpinnerDark is the animated gopher shown on the web splash screen under
	// the dark theme. It is currently the same transparent GIF as SpinnerLight.
	SpinnerDark = spinnerDarkGIF

	// SpinnerLight is the animated gopher shown on the web splash screen under
	// the light theme.
	SpinnerLight = spinnerLightGIF

	// WebGLDebugJs is the content of https://raw.githubusercontent.com/KhronosGroup/WebGLDeveloperTools/b42e702487d02d5278814e0fe2e2888d234893e6/src/debug/webgl-debug.js
	WebGLDebugJs = webGLDebug

	// WebServiceWorker is the notification service worker served alongside the web
	// package; it renders rich-notification action buttons and forwards clicks to
	// the app (pre-rebrand tracker #724).
	WebServiceWorker = belluiServiceWorker
)
