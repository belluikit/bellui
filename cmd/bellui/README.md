# bellui command-line tool

The `bellui` CLI packages, bundles, installs and manages Bellui applications.

This is a **nested Go module** (`github.com/belluikit/bellui/cmd/bellui`) inside
the toolkit repo. It keeps its packaging-only dependencies (icns, goversioninfo,
rsrc, image codecs, …) out of the `github.com/belluikit/bellui` library module,
while living in the same repo as the toolkit so the embedded Android dex is
regenerated from the toolkit's own Java sources — no cross-repo drift.

## Install

```sh
go install github.com/belluikit/bellui/cmd/bellui@latest
```

Releases are tagged `cmd/bellui/vX.Y.Z` on this repo (nested-module tag prefix).

## Android dex

`internal/mobile/dex.go` embeds a compiled dex extending `NativeActivity`
(`internal/driver/mobile/app/GoNativeActivity.java` in the toolkit). It is
checked in and regenerated with the Android SDK on the `PATH`:

```sh
cd internal/mobile && go generate    # go run gendex/gendex.go -o dex.go
```

Because the CLI now lives beside the toolkit, `gendex` reads the toolkit's Java
directly (`../../../../internal/driver/mobile/app/*.java`). **Any change to that
Java must land with a matching dex regen in the same commit** — CI verifies the
committed dex matches a fresh regen.

## iOS: the `ios/` directory

`bellui package -os ios` reads three optional things from an `ios/` directory
next to the app's main package, for what `manifest.pxf` has no field for
(#672):

- **`ios/Info.plist`** — a plist `<dict>` merged into the generated
  Info.plist. Any value type works, so arrays such as
  `UIBackgroundModes = [audio]` can be declared. Keys the packager owns
  (`CFBundleIdentifier`, `CFBundleExecutable`, `CFBundleName`,
  `CFBundleShortVersionString`, `CFBundleVersion`) are an error, as is a key
  the manifest also produces (a usage string from `IOSUsageDescriptions`,
  `CFBundleURLTypes` from `URLSchemes`, `CFBundleDocumentTypes` from file
  associations, the default icon's `NSHumanReadableCopyright`). Any other
  generated key, such as `UISupportedInterfaceOrientations`, is replaced by
  the fragment's value.
- **`ios/Entitlements.plist`** — a plist `<dict>` signed in alongside the
  Associated Domains from the manifest (setting
  `com.apple.developer.associated-domains` in both is an error), as is
  `application-identifier`, which signing sets. Xcode expands
  `$(AppIdentifierPrefix)`; `bellui release` expands the same build settings
  (`AppIdentifierPrefix`, `TeamIdentifierPrefix`, `DEVELOPMENT_TEAM`,
  `PRODUCT_BUNDLE_IDENTIFIER`) itself and fails on any other `$(…)`. With
  automatic signing,
  `-allowProvisioningUpdates` registers the capabilities an entitlement needs
  (HealthKit, …) on an explicit App ID for the bundle ID. `bellui release`
  signs the store build with the same entitlements. A static file cannot vary
  between development and release builds, so `aps-environment` is not
  expressible yet.
- **`ios/PrivacyInfo.xcprivacy`** — a plist `<dict>` merged into the privacy
  manifest every iOS app now ships. The toolkit declares the required-reason
  APIs every Go program calls: file timestamps for files in the app's container
  (`C617.1`) and system boot time for measuring elapsed time (`35F9.1`). Your
  `NSPrivacyAccessedAPITypes` are merged with those per category, so declare
  what your own code and libraries call, for example `3B52.1` when you read the
  dates of files the user picked, or `CA92.1` for `NSUserDefaults`. Tracking
  (`NSPrivacyTracking`, `NSPrivacyTrackingDomains`) and collected data
  (`NSPrivacyCollectedDataTypes`) are yours alone to declare; the toolkit never
  sets them. Without the manifest App Store Connect refuses the upload
  (ITMS-91053).
- **`ios/Frameworks/*.xcframework`, `*.framework`** — for each build
  architecture the matching slice is linked into the app: the packager adds
  `-F<slice> -framework <Name>` (or the static `.a` and its headers) to that
  architecture's cgo flags, so the app's cgo code only needs the `#include`.
  Dynamic frameworks are embedded in `Frameworks/` and signed by the Xcode
  project, with `@executable_path/Frameworks` on the binary's rpath; static
  ones are linked only. Swift is not compiled by the packager: build Swift
  code into an xcframework in Xcode, exposing a C or `@objc` API.

`hack/iosframeworkprobe` exercises all three on a device.

## Android: app Java and library dependencies

`bellui package -os android` picks up native extras from the app's main
package directory. All of them need `ANDROID_HOME` (javac, `android.jar`,
build-tools).

- `android/*.java` is compiled against `android.jar` and dexed into the APK,
  for small JNI shims without a Gradle project.
- `android/libs/*.jar` is on the classpath for that Java and is dexed into
  the APK.
- `android/libs/*.aar` adds an Android library: its `classes.jar` (and any
  `libs/*.jar` inside), its `jni/<abi>/*.so` for each ABI being built, its
  `assets/`, and its `res/`. Resources are linked with aapt2 and every
  library package gets its `R` class. The library manifest's
  `uses-permission`, `uses-permission-sdk-23`, `uses-feature` and `queries`
  elements, and the children of its `<application>`, are merged into the app
  manifest (the generated one or the app's own `AndroidManifest.xml`), with
  `${applicationId}` resolved.
- `android/res/` holds the app's own resources, laid out like a Gradle
  module's `res/` (for example `android/res/xml/rules.xml`, for
  `android:dataExtractionRules="@xml/rules"` in your own
  `AndroidManifest.xml`). They are linked after the libraries' resources and
  the generated icon, so the app's resource wins a name clash.

Limits:

- There is no Maven resolution. `android/libs` must hold the library's whole
  dependency closure, read from its POM. `hack/androidlibprobe/fetch.sh` is a
  worked example for `androidx.core`.
- The manifest merge is simplified. Relative class names (`.Foo`, `Foo`) are
  qualified with the library's package. An entry whose `android:name` the app
  already declares is left out (the app wins). One that two libraries declare
  (such as androidx.startup's `InitializationProvider`) is kept once, with the
  children of both. One merge directive is read: an element in your own
  `AndroidManifest.xml` marked `tools:node="remove"` keeps every library's
  element of that name and `android:name` out, and is itself left out of the
  APK. For example, an app that bundles the ads SDK but does not ask for
  tracking removes the advertising ID permission:
  `<uses-permission android:name="com.google.android.gms.permission.AD_ID" tools:node="remove" />`
  (with `xmlns:tools="http://schemas.android.com/tools"` on `<manifest>`).
  Other `tools:*` attributes and directives are dropped, and a library's
  `<application>` attributes (such as `appComponentFactory`) are not merged.
  Put anything else in your own `AndroidManifest.xml`.
- An AAR with native libraries for some ABIs but not for one being built
  fails the build. Build only the targets it supports (`-target
  android/arm64`).
- An AAR that ships `libopenal.so` fails the build of an app using
  `exp/audio/al`, which bundles its own.
- An app asset wins over a library asset at the same path, with a warning.
- The generated manifest sets `android:extractNativeLibs="false"`, so every
  `.so`, a library's included, is loaded straight from the APK and none is
  unpacked to disk (#889). A library loaded with `System.loadLibrary` works.
  One that needs its `.so` as a file, for example one that reads
  `nativeLibraryDir` or runs a bundled executable, does not. For such a
  library, use your own `AndroidManifest.xml` with the attribute set to
  `"true"`.
- Without an AAR, resources are packaged exactly as before; jars only change
  the dex.

## History

Migrated from `github.com/trendvidia/tools` (which mirrored `fyne.io/tools`);
that repo is archived. The move eliminated the cross-repo coupling where a dex
regen could ship ahead of the toolkit's native signatures.
