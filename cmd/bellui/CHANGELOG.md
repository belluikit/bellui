# Changelog

> **Note on naming.** This project was renamed to **Bellui**
> (`github.com/belluikit/bellui`) in July 2026. Entries below predate the rename
> and are left verbatim — they refer to the toolkit by its former names (`fyne`,
> `github.com/trendvidia/fyne`, `fyne.io/fyne`) and to files and flags by the
> identifiers actually in use at the time (e.g. `FyneApp.toml`, `fyneDo`).
> Rewriting them would falsify the record.

This file lists the main changes with each version of the Fyne tools project.
More detailed release notes can be found on the [releases page](https://github.com/fyne-io/tools/releases).

## Unreleased

### Fixed

* Packages now credit the gopher artwork they ship
  ([#362](https://github.com/belluikit/bellui/issues/362)). The default icon
  (used when an app has no `Icon.png`) and the web splash spinner are based on
  the Go gopher, which is CC BY 4.0 and must be credited wherever it is
  shared, but no package said so. The web splash screen now shows "Gopher
  artwork based on the Go gopher by Renée French, CC BY 4.0" under the spinner. A package that uses the default
  icon carries the credit in its macOS or iOS Info.plist (shown in About), its
  Windows version resource (shown in Properties), its Android manifest, or a
  `share/doc/<app>/CREDITS` file in Linux and BSD archives. `bellui package`
  also says so when it falls back to the default icon, and warns when an
  Android app's own `AndroidManifest.xml` leaves the credit out.

* The web page `bellui package -os web` and `bellui serve` write now has a
  viewport meta tag. Without it, phone browsers laid the app out 980 CSS px
  wide, so it showed as a shrunken desktop page with a canvas of about six
  times the pixels it needed (2940×6393 instead of 1206×2622 on a 402×874
  @3x iPhone).

## v1.0.1 — 2026-10-16

### Fixed

* The page `bellui package -os web` and `bellui serve` write now reports a
  failed download of the app's `.wasm` (a 404, a network error) as
  "Could not load the application: …" instead of showing
  "Downloading web assembly file" forever.

## v1.0.0 — 2026-10-16

Version numbers restart at 1.0.0 with the first public release of the
`github.com/belluikit/bellui/cmd/bellui` module; the 3.x entries below are
the history of the tools project it grew from. Built against bellui v1.1.0.

### Fixed

* `bellui build` passed `go build` the original `GOFLAGS` even after moving
  its `-ldflags` onto the command line, because it copied the environment
  before cleaning `GOFLAGS`
  ([#371](https://github.com/belluikit/bellui/issues/371)).
* `bellui build` and `bellui package` embedded no app metadata for any bellui
  release. Injection was gated on Fyne's version numbers (`>=2.2`, and
  `>=2.3` / `>=2.6` for Release, Custom and Migrations), which no bellui
  version matches, so an app relying on `manifest.pxf` for its ID ran with an
  empty ID and its preferences were not saved. The file is now written
  whenever the app requires bellui (#337).
* `bellui init` wrote `w.ShowAndRun()`, which does not compile against
  bellui; it now writes `bellui.ShowAndRun(w)` (#337).
* `bellui release -os windows` failed to find `makeappx.exe` and
  `signtool.exe` when the Windows SDK's bin directory was on `PATH` (#337).
* On Linux and BSD, the generated Makefile installed `$(AppID).desktop` even
  when the app has no ID and the `.desktop` file is named after the app (#337).

## 3.4.2 - 18 Jul 2026

### Changed

* Regenerated the embedded Android dex for hardware-keyboard support in the
  toolkit: `GoNativeActivity`'s `FyneInputConnection` forwards Ctrl/Alt/Meta
  shortcut combinations (Ctrl+A/C/V/X/Z/Y) into fyne via the new
  `keyboardShortcut(keyCode, metaState)` native, so they reach the editor
  instead of the inert host EditText (pre-rebrand tracker #1077); and
  `keyboardArrow` now takes a `metaState` argument so Shift+Arrow extends an
  editor's selection from a hardware keyboard (pre-rebrand tracker #1083). Also
  consolidates the trendvidia dex line's IME session-restart at field
  boundaries (pre-rebrand tracker #966) and the bounded per-frame text-layout
  sidecar that feeds TalkBack character/line geometry on the focused editor
  (pre-rebrand tracker #995/#1009). Requires a fyne at or after pre-rebrand tracker #1083 (the
  `keyboardArrow` native signature changed).


## 3.4.1 - 16 Jul 2026

### Fixed

* Regenerated the embedded Android dex with two composition-safety fixes to
  the 3.4.0 IME transport: `setSelection` commits an active fyne preedit
  before forwarding, so Select All during composition marks the whole
  document instead of coming up short by the preedit length
  (pre-rebrand tracker #1032); and the fyne→IME state push now distinguishes
  per-keystroke composing echoes from genuine out-of-band document changes
  (undo/redo from the accessibility actions menu), converging via
  restartInput instead of silently skipping — fixing the stale display and
  the orphaned preedit landing at the start of the document after an undo
  mid-composition (pre-rebrand tracker #1034).


## 3.4.0 - 16 Jul 2026

### Changed

* Regenerated the embedded Android dex with the custom-accessibility-action
  and IME-transport work from the toolkit (pre-rebrand tools tracker #31):
  `GoNativeActivity` gains `addAccessibilityNodeV4` carrying declared widget
  actions with no standard Android semantic — Entry/RichEntry Undo/Redo —
  registered as custom `AccessibilityAction`s in TalkBack's actions menu and
  dispatched through the existing action up-call (pre-rebrand tracker #1025);
  the `keyboardArrow` native, so the soft keyboard's editing-panel arrow keys
  move the fyne caret instead of being swallowed by the hidden IME-host
  EditText (pre-rebrand tracker #1029); and the `keyboardSetSelection` native plus
  the `updateSelection` acknowledgement, so an IME-driven Select All is
  visible, offers Copy/Cut to assistive technology, and typing commits over
  the selection (pre-rebrand tracker #1030). Also drops the pre-rebrand tracker #974 epic's debug
  instrumentation from the shipped dex (pre-rebrand tracker #1010). Additive
  protocol: the native library's V4 lookup falls back to V3 against an older
  dex, and apps built against an older fyne simply don't use the new node
  variant.


## 3.3.1 - 14 Jul 2026

### Changed

* Removed the `google.golang.org/protobuf => github.com/trendvidia/protobuf-go`
  replace directive. Since protowire-go v1.2.2 the forked protobuf runtime is
  no longer required, so the module now resolves upstream
  `google.golang.org/protobuf` (v1.36.11) directly.


## 3.3.0 - 14 Jul 2026

### Changed

* Regenerated the embedded Android dex with the editable-text accessibility
  protocol (pre-rebrand tracker #974): `GoNativeActivity` gains
  `addAccessibilityNodeV3` carrying the document text + selection,
  granularity/selection/clipboard accessibility-action handling with event
  synthesis, and the `accessibilityTextAction` native up-call — the Java half
  of TalkBack cursor movement, selection and clipboard actions on text
  fields (pre-rebrand tracker #965). Also includes the shadow-EditText
  accessibility-event swallowing + touch-exploration re-query stopgaps
  (pre-rebrand tracker #968) and the UI-thread `hideKeyboard` discipline that
  fixes the second-text-field deadlock under TalkBack (pre-rebrand tracker #973).
  Additive protocol: apps built against an older fyne simply don't use the
  new node variant (pre-rebrand tools tracker #29).
* Bumped `github.com/trendvidia/protowire-go` to v1.2.2, matching the
  toolkit (pre-rebrand tracker #979).


## 3.2.1 - 14 Jul 2026

### Fixed

* **Regression in 3.2.0:** the embedded Android dex was regenerated from a
  fyne branch that predated the pre-rebrand tracker #817 virtualized-grid accessibility work, so
  3.2.0 silently dropped `accessibilityActionScrollTo` and
  `addAccessibilityNodeV2` (TalkBack data-table support shipped in 3.1.0).
  Regenerated from the correct combined `GoNativeActivity.java` (the pre-rebrand tracker #817
  grid support **and** the 3.2.0 CJK/announce fixes). Upgrade from 3.2.0.


## 3.2.0 - 14 Jul 2026

### Fixed

* `fyne package` injected a `fyne_metadata_init.go` (and the `hello_world`
  template) that still imported `github.com/trendvidia/fyne/v2` after the v3
  migration, so packaging a fyne/v3 app failed to build. Both templates now
  import /v3 (pre-rebrand tools tracker #27, pre-rebrand tracker #28).

### Changed

* Regenerated the embedded Android dex from fyne's updated
  `GoNativeActivity.java` for two on-device fixes (pre-rebrand tracker #945): the
  shadow `EditText` inputType now advertises a real text class
  (`TYPE_CLASS_TEXT`) so input methods engage inline composition — a
  class-less inputType previously read as `TYPE_NULL` and disabled CJK
  composition; and `setKeyboardState` is now a strict no-op on unchanged
  state, ending the TalkBack announce-loop caused by redundant selection
  events (pre-rebrand tracker #28).


## 3.1.0 - 13 Jul 2026

### Changed

* Regenerated the embedded Android dex from fyne's updated
  `GoNativeActivity.java` to carry the TalkBack virtualized-collection
  support (pre-rebrand tracker #948, pre-rebrand tracker #817): `addAccessibilityNodeV2` with grid
  association, a ViewGroup grid proxy exposing `CollectionInfo` and nested
  `CollectionItemInfo` cells, and `ACTION_SCROLL_TO_POSITION` routed to the
  new `accessibilityActionScrollTo` native method (pre-rebrand tracker #26). Backward compatible
  both ways: the V1 `addAccessibilityNode` wrapper remains for older native
  libraries, and fyne's C shim falls back to V1 when packaged with an older
  dex.


## 3.0.0 - 13 Jul 2026

### Changed

* **Breaking:** the tools now build apps against Fyne v3
  (`github.com/trendvidia/fyne/v3`, currently v3.0.0-rc.2), following the
  v2 → v3 import-path move and the removal of all deprecated public API (pre-rebrand tracker #24).
  Projects packaged with these tools must themselves be on fyne/v3.

### Security

* Bumped `golang.org/x/image` to v0.44.0 to clear the high-severity
  dependabot alert for excessive resource consumption in the tiff package's
  PackBits decoding (pre-rebrand tracker #25). The moderate `golang.org/x/net` HTML-parser alert
  is also cleared: fyne/v3 no longer pulls x/net into the module graph at all.


## 2.9.0 - 13 Jul 2026

### Added

* The web package now serves a notification service worker (`fyne-sw.js`) so
  rich notifications can render action buttons on the web target
  (pre-rebrand tracker #724). The page-context browser `Notification` cannot render
  actions — only a service worker's `registration.showNotification` can, and
  action clicks are delivered to the worker rather than the page. Fyne's wasm
  runtime registers the worker lazily and only for notifications carrying
  actions; apps whose bundle predates this file degrade to the basic
  notification path (body click only).


## 2.8.0 - 12 Jul 2026

### Added

* `fyne translate` now recognizes the grammatical-select localization seams
  `lang.LSelect` and `lang.LSelectPlural` (pre-rebrand tracker #505). Because the
  selector is usually a runtime variable, the scanner seeds the mandatory
  `<prefix>.other` key (and any literal selector variant), never the bare prefix,
  so an existing key is never clobbered; `LSelectPlural` seeds a `{"other": …}`
  plural object like `N`/`XN`.


## 2.7.0 - 10 Jul 2026

### Changed

* Regenerated the embedded Android dex from `GoNativeActivity.java` to carry the
  Android IME / TalkBack fixes: correct caret advance during IME composition,
  word-backspace via `setComposingRegion`, and the TalkBack text mirror
  (`setKeyboardState`, replacing the one-space sentinel so the screen reader
  announces real content and backspace deletes real characters;
  pre-rebrand tracker #760), plus the explore-by-touch tap gate that gives
  single-vs-double-tap semantics under TalkBack (`setTouchExploration`;
  pre-rebrand tracker #761). Both were verified end-to-end on a physical device
  (OnePlus Pad 2, Adreno 750, Android 15).


## 2.6.0 - 10 Jul 2026

### Added

* `fyne package -os darwin` now injects the app's `NS*UsageDescription` strings
  from `manifest.pxf` `details.ios_usage_descriptions` into the macOS bundle's
  `Info.plist` (sorted for deterministic output). macOS only delivers
  CoreLocation (and camera/microphone) access to a bundled `.app` whose
  `Info.plist` declares the matching usage string — the iOS path already sourced
  these, the darwin desktop path did not, so a packaged app using desktop GPS
  never prompted and never got a fix. The same `NS*` keys work for both platforms
  (pre-rebrand appviewer tracker #157). Enables desktop CoreLocation GPS on a packaged
  macOS app (pre-rebrand appviewer tracker #159).


## 2.5.0 - 10 Jul 2026

### Added

* The generated Android manifest now declares `android.permission.POST_NOTIFICATIONS`.
  Without it, on Android 13+ (targetSdk 33+) the OS silently drops every
  notification an app posts (`POST_NOTIFICATION` app-op stays `ignore`), so the
  Fyne notification API — plain and rich — never appeared. Found while verifying
  rich-notification callbacks on a real device (pre-rebrand tracker #723). Runtime
  requesting of the permission is a separate Fyne-side follow-up
  (pre-rebrand tracker #756).

### Changed

* Regenerated the embedded Android dex from `GoNativeActivity.java` to carry the
  new native bridges for two Fyne features: rich-notification click/action
  callbacks (`notificationActivated`; pre-rebrand tracker #723) and image clipboard
  (`doSetClipboardImage`/`doGetClipboardImage`; pre-rebrand tracker #580). Both were
  verified end-to-end on a physical device (OnePlus Pad 2, Adreno 750, Android 15).


## 2.4.0 - 10 Jul 2026

### Added

* `fyne package -os ios` now emits an **Associated Domains** entitlement from
  `manifest.pxf` `details.associated_domains` (each entry becomes
  `applinks:<domain>`), wired in via `CODE_SIGN_ENTITLEMENTS`. This is the
  packaging half of iOS Universal Links — `xcodebuild -allowProvisioningUpdates`
  registers the capability with the App ID under automatic signing
  (pre-rebrand tracker #622). Verified on device: the app installs with the
  entitlement and Apple's AASA CDN validates a matching
  `apple-app-site-association`.

### Fixed

* `fyne package -os ios` now emits UTI `LSItemContentTypes` (derived from the
  declared file-association MIME types) alongside the legacy
  `CFBundleTypeMIMETypes`. iOS 14+ routes "Open in…" / share-sheet document
  targets by UTI, so without this the app never surfaced as a file handler
  (pre-rebrand tracker #622). Verified on device: a shared `.txt`/`.md` now opens the
  app.


## 2.3.1 - 10 Jul 2026

### Fixed

* `fyne package` no longer rewrites the whole `manifest.pxf` when it bumps
  `details.build`. The build number is now updated with a format-preserving edit
  (via the pxf Rewriter), so the author's comments, key ordering and spacing
  round-trip byte-for-byte instead of being stripped/normalized on every
  package. `fyne init` still writes fresh manifests through the full marshal.


## 2.3.0 - 10 Jul 2026

### Fixed

* `fyne package -os ios` against the **Xcode 26 / iOS 26 SDK** (Go 1.26). Three
  changes let the produced app link, install, and launch on a real device:
  * Link `-framework CoreGraphics` — the iOS 26 SDK no longer re-exports the
    CGGeometry symbols (e.g. `CGRectIsEmpty`, used by the mobile driver's
    `GoInputView`) through UIKit.
  * Compile and link a no-op `pthread_jit_write_protect_np` stub into **device**
    builds. The Go darwin runtime / GoVM jit references this macOS-only symbol
    (`__API_UNAVAILABLE(ios)`); left undefined the app crashed at launch with
    dyld "symbol not found in flat namespace". Shipped as a static archive so
    on-demand extraction avoids a duplicate symbol, and only for the device
    (the simulator's genuine macOS symbol must not be shadowed).
  * Default `-iosversion` 7.0 → 14.0 (and the generated pbxproj
    `IPHONEOS_DEPLOYMENT_TARGET` 9.0 → 14.0), which predated current Go iOS
    support and the prebuilt arm64 frameworks.


## 2.2.0 - 9 Jul 2026

### Added

* Mobile file-share and open-with packaging — the OS side of `fyne.SharerApp` and
  `fyne.OpenHandlerApp` on mobile (fyne pre-rebrand tracker #711 / pre-rebrand tracker #622). `fyne package`:
  * **Android** — declares a `<provider>` for the bundled `FyneFileProvider`
    (authority `<appId>.fileprovider`), so `ShareFile` can hand a file to the system
    chooser as a `content://` URI. The regenerated dex ships that provider plus the
    `GoNativeActivity` launch / `onNewIntent` open-with hooks. The `ACTION_VIEW`
    intent-filters were already emitted from `manifest.pxf`.
  * **iOS** — emits `CFBundleURLTypes` (from `Details.URLSchemes`) and
    `CFBundleDocumentTypes` (from `[[Details.FileAssociations]]` MIME types) into the
    generated `Info.plist`, so iOS routes custom-scheme deep links and document
    "Open in…" to the app.


## 2.1.0 - 9 Jul 2026

### Added

* iOS `Info.plist` usage strings via `Details.IOSUsageDescriptions` in the application
  manifest (`manifest.pxf`). `fyne package -os ios` injects each entry
  (`NSCameraUsageDescription`, `NSLocationWhenInUseUsageDescription`, …) into the
  generated `Info.plist` — iOS refuses to prompt for, and App Review rejects, any
  permission whose usage string is missing. A raw-key `map<string, string>` keeps the
  schema future-proof as Apple adds keys; entries render sorted for reproducible builds.

### Changed

* Bumped `protowire-go` to v1.2.0 for better comment and inline-update preservation in
  the PXF codec, so editing a `manifest.pxf` keeps its comments.


## 2.0.0 - 7 Jul 2026

### Added

* File-type and URL-scheme handler metadata via `[[Details.FileAssociations]]` and
  `Details.URLSchemes` in the application manifest. `fyne package`/`install` now emit the
  per-platform handler declarations (macOS `CFBundleDocumentTypes`/`CFBundleURLTypes`,
  Android `ACTION_VIEW` intent-filters, Linux desktop `MimeType`/`x-scheme-handler`,
  Windows registry keys) that route matching files and URLs to the app — the OS side
  of `fyne.OpenHandlerApp` (fyne pre-rebrand tracker #510).

### Changed

* Application metadata is now stored in `manifest.pxf` (textual PXF format) instead
  of `FyneApp.toml`. `fyne init` writes `manifest.pxf`, and `package`/`install`/`build`
  read it. This is a hard replacement: `FyneApp.toml` is no longer recognized — rename
  and convert existing projects. The schema is defined in
  `cmd/fyne/internal/metadata/fyneapppb/fyneapp.proto` (regenerate with `go generate`).


## 1.7.2 - 29 May 2026

### Changed

* Compatibility mode for fyne-cross-images (fyne-io/tools#130)

### Fixed

* Pass flag to compiler instead of preprocessor directly (fyne-io/tools#129)
* Build with compilers missing support flow-control protection (fyne-io/tools#133)


## 1.7.1 - 12 Apr 2026

### Added

* Start support for open with in metadata (fyne-io/tools#104)
* Support for adaptive and themed android icons (fyne-io/tools#115)
* Parsing of concatenated strings with fyne translate (fyne-io/tools#117)

### Changed

* Apply hardening C/LD-FLAGS and optimize (fyne-io/tools#100)
* Improve help to show that at least one file is needed (fyne-io/tools#102)
* Set StartupWMClass in Unix/Linux .desktop files (fyne-io/tools#112)
* gendex: Sort directory names in findLast (fyne-io/tools#123)

### Fixed

* Update Fyne version (fyne-io/tools#97)
* Don't generate Migration code for pre-2.6 library usage (fyne-io/tools#98)
* Support remote install with icons that are simply Icon.png (fyne-io/tools#99)
* Fix release flags not being set for WASM (fyne-io/tools#101)
* Don't make tarbombs (fyne-io/tools#105)
* Fallback to app name if AppID is not present for package_unix (fyne-io/tools#114)
* Update Android to API 21 in to keep the builds for Play Store happy (fyne-io/tools#119)
* Make sure that CGO is not enabled when building in WASM mode (fyne-io/tools#121)
* fyne serve: fix blank wasm loading screen while app is starting (fyne-io/tools#122)

### New Contributors

Code in v1.7.1 contains work from the following first time contributors:

* @dweymouth
* @gezb
* @MaxGyver83
* @ncw
* @typenoob


## 1.7.0 - 16 Oct 2025

### Added

* Support Description metadata
* Allow more than one bundle argument
* Full support for semver 2.0 spec
* Translation setup for newly generated apps

### Changed

* Use embed instead of generated []byte for bundle command
* Make library version output deterministic and more forgiving
* Better error messages when trying to install remote apps without FyneApp.toml
* Replace dashes with underscores in generated app ids
* Renamed release flag --key-store and --key-store-pass to --keystore and --keystore-pass
* Create 16k aligned files for Google Play

### Fixed

* Remove legacy usage and files
* toml file should not be indented
* Correct lookup path for tools repo alongside fyne
* Fix up some keyboard issues on Android (fyne-io/fyne#5806)
* Entry with mobile.NumberKeyboard does not Type comma and separators (fyne-io/fyne#5101)
* Support d8 instead of dx for generating dex
* Make sure translation files end with a newline to prevent warnings from git and other tools
* Fix missing Migrations in compiled metadata
* Support installing Fyne apps in subdirectories

### New Contributors

Code in v1.7.0 contains work from the following first time contributors:

* @ErikKalkoken


## 1.6.2 - 22 August 2025

### Fixed

* Resolve compile issue with Go 1.25.0 caused by golang.org/x/tools conflict


## 1.6.1 - 15 April 2025

### Changed

 * New apps from "fyne init" will have the fyneDo migration turned on by default


## 1.6.0 - 11 April 2025

This is the beginning of the Fyne tools releases having migrated the `fyne` command line
tool from the [fyne-io/fyne](/fyne-io/fyne) repository.

### Added

 * Added new "fyne init" command to set up new apps
 * Update to support all new features of Fyne v2.6.0
 * Show the Fyne library version number next to the tool info in "fyne version"

