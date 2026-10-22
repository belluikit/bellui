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
  `com.apple.developer.associated-domains` in both is an error). Xcode
  expands `$(AppIdentifierPrefix)`. With automatic signing,
  `-allowProvisioningUpdates` registers the capabilities an entitlement needs
  (HealthKit, …) on an explicit App ID for the bundle ID. `bellui release`
  signs the store build with the same entitlements. A static file cannot vary
  between development and release builds, so `aps-environment` is not
  expressible yet.
- **`ios/Frameworks/*.xcframework`, `*.framework`** — for each build
  architecture the matching slice is linked into the app: the packager adds
  `-F<slice> -framework <Name>` (or the static `.a` and its headers) to that
  architecture's cgo flags, so the app's cgo code only needs the `#include`.
  Dynamic frameworks are embedded in `Frameworks/` and signed by the Xcode
  project, with `@executable_path/Frameworks` on the binary's rpath; static
  ones are linked only. Swift is not compiled by the packager: build Swift
  code into an xcframework in Xcode, exposing a C or `@objc` API.

`hack/iosframeworkprobe` exercises all three on a device.

## History

Migrated from `github.com/trendvidia/tools` (which mirrored `fyne.io/tools`);
that repo is archived. The move eliminated the cross-repo coupling where a dex
regen could ship ahead of the toolkit's native signatures.
