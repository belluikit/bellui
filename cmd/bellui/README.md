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

Limits:

- There is no Maven resolution. `android/libs` must hold the library's whole
  dependency closure, read from its POM. `hack/androidlibprobe/fetch.sh` is a
  worked example for `androidx.core`.
- The manifest merge is simplified. An entry whose `android:name` the app
  already declares is left out (the app wins), `tools:*` attributes and merge
  directives are dropped, and a library's `<application>` attributes (such as
  `appComponentFactory`) are not merged. Put anything else in your own
  `AndroidManifest.xml`.
- An AAR with native libraries for some ABIs but not for one being built
  fails the build. Build only the targets it supports (`-target
  android/arm64`).
- An app asset wins over a library asset at the same path, with a warning.
- Without an AAR, resources are packaged exactly as before; jars only change
  the dex.

## History

Migrated from `github.com/trendvidia/tools` (which mirrored `fyne.io/tools`);
that repo is archived. The move eliminated the cross-repo coupling where a dex
regen could ship ahead of the toolkit's native signatures.
