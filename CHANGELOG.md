# Changelog

> **Note on naming.** This project was renamed to **Bellui**
> (`github.com/belluikit/bellui`) in July 2026. Entries below predate the rename
> and are left verbatim — they refer to the toolkit by its former names (`fyne`,
> `github.com/trendvidia/fyne`, `fyne.io/fyne`) and to files and flags by the
> identifiers actually in use at the time (e.g. `FyneApp.toml`, `fyneDo`).
> Rewriting them would falsify the record.

This file lists the main changes with each version of Bellui. Entries up to
2.7.3 are inherited from upstream Fyne; 2.8.0, 2.9.0 and the Unreleased section
after them describe the trendvidia fork (github.com/trendvidia/fyne) that
became Bellui; v1.0.0-rc.1 onward describe Bellui itself.

## Unreleased

### Added

* **Two-finger pan scrolls**
  ([#398](https://github.com/belluikit/bellui/issues/398)). Two fingers moving
  together scroll what is under them, in both directions, on the web and on
  desktop touchscreens. A page of charts or 3D views, which take one-finger drags
  for themselves, can now be scrolled past them. Charts pass two-finger
  scrolling on to the page; the x/dag canvas pans with it. Two fingers moving
  apart or together over a Pinchable are still a pinch.
* **Sign-in tokens per issuer**
  ([#356](https://github.com/belluikit/bellui/issues/356)). New
  `auth.SaveTokenFor`, `LoadTokenFor` and `ClearTokenFor` take an issuer as
  well as a provider, so an app that signs in to two issuers under one provider
  name (two Keycloak realms, two Okta tenants) keeps a token for each; with
  one token per provider name, the second sign-in replaced the first.
  `widget.AuthButton` saves this way with its `Issuer`; an empty `Issuer`, or
  the provider preset's own, is the same as no issuer.
  **Behaviour change:** a button whose `Issuer` is not the preset's (every
  "okta", "keycloak" and "custom" button, and a "microsoft" button set to one
  tenant) saves a token that is read back with `auth.LoadTokenFor(provider,
  issuer)` and removed with `auth.ClearTokenFor(provider, issuer)`.
  `auth.LoadToken(provider)` no longer finds it and `auth.ClearToken(provider)`
  no longer removes it, so an app that signs out with `ClearToken` leaves the
  refresh token on disk until it switches. A token saved before upgrading
  stays in the provider's old file, where `LoadToken` still reads it; an app
  that switches to `LoadTokenFor` can remove it with one `ClearToken` call.
  Buttons on a preset issuer (Google, Apple, GitHub, Microsoft's common
  endpoint) are unchanged.

* **Pinch-to-zoom** ([#379](https://github.com/belluikit/bellui/issues/379)).
  New `event.Pinchable` (`Pinched(*PinchEvent)`, `PinchEnded()`): two fingers
  on a Pinchable widget report the change in their distance and the point
  between them, on phones, tablets and desktop touchscreens. The web follows
  once the browser driver handles touch input
  ([#378](https://github.com/belluikit/bellui/issues/378)). The x/dag canvas
  zooms on pinch and pans with the fingers; x/charts (data window, 3D camera
  and maps) zooms on pinch.
  
### Fixed

* **Cards cost less memory and CPU on WebGPU**
  ([#466](https://github.com/belluikit/bellui/issues/466)). Each card clips its
  content to its rounded corners, and every clip cleared the whole window's
  stencil buffer first, once per card per frame, including cards scrolled out
  of view. Clips now erase only what they drew, and skip cards outside the
  visible area. On an M3 Max, a grid of 500 cards uses about 33 MB less memory
  and 5–10% less CPU per frame.

* **Screens that update many widgets are up to 4× faster**
  ([#457](https://github.com/belluikit/bellui/issues/457)). Every widget
  setter checks that it runs on the main goroutine, and the check took a stack
  trace, which cost most of a busy frame's CPU. It now reads the goroutine's
  ID directly on amd64 and arm64, in about 2 ns instead of 1.4 µs. On an M3
  Max, 2000 labels re-texted every frame went from 10 to 47 fps, and 500
  updating cards from 16 to 50–54 fps.
  
* **Less CPU per frame on screens with many widgets**
  ([#458](https://github.com/belluikit/bellui/issues/458)). Every frame read
  the system clock once for each widget, to keep its cached renderer alive. It
  now reads it once per frame. A static screen of 500 cards uses about 15% less
  CPU per frame on an M3 Max.

* **Mobile: touches are no longer lost or renumbered mid-gesture**
  ([#392](https://github.com/belluikit/bellui/issues/392)). On iOS, lifting one
  finger gave a finger still down a new id, so after a pinch the remaining
  finger could pan or tap. On Android, a touch the system cancelled (another
  app opened on top while the finger was down) was reported as a move and never
  ended. iOS now keeps each touch's id until it ends, Android and iOS report a
  cancelled touch as a cancel (no tap, no long press), and a touch keeps the
  window it began in if another window opens while the finger is down. If its
  window closes instead, the touch is cancelled there, and it no longer taps
  the closed window's content.
  
* **Web: keyboard users can leave a multi-line field**
  ([#357](https://github.com/belluikit/bellui/issues/357)). A multi-line
  `Entry` and the x/dag canvas keep Tab for themselves, and the way out,
  Ctrl+Tab, never reaches a web page because browsers use it to switch tabs, so
  a keyboard-only user was trapped (WCAG 2.1.2). Escape followed by Tab or
  Shift+Tab now moves focus out of such a widget, on every platform. Escape
  still closes a pop-up or dialog first; the Escape after that frees the next
  Tab.
  
* **x/mmedia: video playback uses far less memory at high resolution**
  ([#110](https://github.com/belluikit/bellui/issues/110)). The decode-ahead
  queue held 120 frames whatever their size: about 1 GB at 1080p and 4 GB at
  4K. It is now sized to a 128 MB budget, between 8 and 120 frames, so small
  videos are unchanged. Playing a 1080p clip peaked at 2.1 GB of memory and
  now peaks at about 470 MB on a Mac, with no more frames dropped than
  before. A shallower queue tolerates a shorter decode stall (about half a
  second at 1080p30); that has not yet been measured on a phone.
  
* **Sign-in tokens from before v1.1.0 are deleted**
  ([#355](https://github.com/belluikit/bellui/issues/355)). Since tokens are
  stored per app, the old shared `bellui-auth/<provider>.enc` was no longer
  read, but nothing removed it, and signing out did not either, so a refresh
  token that still decrypts stayed on disk. It is now deleted the first time
  any app saves or clears a token for that provider, even when that save or
  clear fails, as it does for an app without an ID.

* **x/charts: charts no longer rebuild on every frame**
  ([#396](https://github.com/belluikit/bellui/issues/396)). Every layout rebuilt
  the chart, and the rebuild triggered another layout, so idle charts were
  rebuilt about 60 times a second. For 3D charts each rebuild allocated a new
  GPU texture, which took a phone browser's GPU memory past 4 GB in seconds and
  crashed it. A chart now rebuilds only when its size or content changes.
  
* **x/tables: the data grid scrolls by finger**
  ([#394](https://github.com/belluikit/bellui/issues/394)). A touch drag on the
  grid reached its mouse drag handler, which ignored it, so the grid could not
  be scrolled on phones and tablets. Drags without a mouse button down now
  scroll the body; mouse drags still don't.
  
* **x/charts: a 3D chart with several series rotates as a whole**
  ([#399](https://github.com/belluikit/bellui/issues/399)). Each 3D series drew
  its own scene, so orbiting or zooming moved only one of them and the series
  could not hide each other in depth. All 3D series of a chart now share one
  scene and one camera.

* **Web: a page wider than the screen no longer squeezes the whole UI**
  ([#387](https://github.com/belluikit/bellui/issues/387)). When content needed
  more room than the browser viewport, the canvas grew past it and the frame
  was drawn squeezed sideways until the next window resize. In the browser the
  canvas now stays the size of the viewport and wider content is clipped, as
  on mobile.
  
* **Web: touch works on phones and tablets**
  ([#378](https://github.com/belluikit/bellui/issues/378)). Only the click a
  browser synthesises after a tap reached the app, so nothing could be scrolled
  or dragged by finger. Finger contacts now go through the same gesture
  handler as the mobile driver — drags, flick scrolling, long-press and
  double-tap — and the page no longer pans or zooms under the app.
  
* **Web: GPU memory was never freed, so Android killed the tab**
  ([#385](https://github.com/belluikit/bellui/issues/385)). In the browser,
  releasing a WebGPU texture or buffer did nothing; its memory waited for
  JavaScript's garbage collector, which ignores GPU memory pressure. The
  shadow, off-screen and per-frame resources bellui creates piled up — on a
  Pixel 9a Chrome's GPU memory reached 2.1 GB and the tab crashed within 40
  seconds. They are now destroyed when bellui is done with them; the same
  test ran 3 minutes with GPU memory bounded at 600–750 MB.

## v1.1.2 — 2026-10-16

Fixes a memory leak that every app had: content removed from a window was
never freed. Also makes the slider's value indicator keep up while dragging.
The `x/*` modules are re-released at v1.1.2 unchanged, and the `bellui`
command is released alongside as `cmd/bellui` v1.0.1 (see
`cmd/bellui/CHANGELOG.md`).

### Fixed

* **Memory leak: content removed from a window stayed in memory for the
  window's lifetime** ([#380](https://github.com/belluikit/bellui/issues/380)).
  The canvas kept a lookup from every object it had ever drawn to its render
  node and never removed entries, so a replaced page — with all its data —
  could not be collected. In the gallery, every page visit kept about 8 MB;
  in the browser this grew past 1 GB in minutes and Android closed the tab.
  Entries for objects the latest layout pass no longer reaches are now
  dropped.
* **Slider: the value indicator keeps up while dragging.** It was rebuilt
  for every drag event, replacing the popup and restarting its fade-in each
  time, so it lagged behind the thumb. It is now built once per drag and
  updated in place.

## v1.1.1 — 2026-10-16

A patch release for x/dag. The toolkit itself is unchanged from v1.1.0; the
other `x/*` modules are re-released at v1.1.1 to keep one version across the
repository. The `bellui` command is released alongside as `cmd/bellui` v1.0.0,
its first tag (see `cmd/bellui/CHANGELOG.md`).

### Fixed

* **x/dag: note text no longer overflows its box when zoomed.** The box was
  sized from the text measured at the base size and scaled by the zoom, but a
  fractional text size draws like the next whole pixel size
  ([#374](https://github.com/belluikit/bellui/issues/374)), so at zoom 0.67 (the
  gallery's default view) a one-line note ran 28 px past its box. Notes now
  measure at the size they are drawn.
* **x/dag: palette entries show their whole port summary.** The second line
  sat in a fixed 14 px box, and its arrow comes from a fallback font whose line
  is about 22 px at 11 pt, so it was cut off. Entries now size to their text.

## v1.1.0 — 2026-10-16

Android's Storage Access Framework becomes usable, two long-standing cgo
memory bugs in the Android bridge are fixed, and the test helpers stop
shipping in production binaries. The minor bump is for the new
`storage.CreateChild` API. Apps that use `auth` should read *Changed*:
sign-in tokens are now kept per app, so users sign in once more after
upgrading. Everything else is additive or a fix. This is also the first
release made with the repository public.

### Added

* **`storage.CreateChild`, for stores that name their own children**
  ([#303](https://github.com/belluikit/bellui/issues/303)). `storage.Child`
  does path arithmetic — it names a resource whether or not anything is
  there — and that does not work where the store assigns identity. Under
  Android's SAF a document ID is opaque and `DocumentsContract` hands the URI
  back only on creation, so a folder the user picked could not be scaffolded
  into at all. `CreateChild(parent, name, listable)` asks the store to make
  the resource and returns what it called it, and
  `repository.ChildCreatingRepository` is the optional capability a repository
  implements to answer it. Repositories that can name a child by path
  arithmetic implement nothing: for them `CreateChild` is `Child` plus
  `CreateListable` or an empty `Writer`, and it verifies the resource exists
  before returning rather than handing back a URI pointing at nothing.
* `driver/software.WindowlessCanvas`, so the software driver's constructors
  return a nameable type without reaching into `bellui/test`
  ([#304](https://github.com/belluikit/bellui/issues/304)).

### Changed

* **Sign-in tokens are stored per app**
  ([#335](https://github.com/belluikit/bellui/issues/335)). Every bellui app
  on a machine shared one `bellui-auth/<provider>.enc`, so one app could read
  another's sign-in. Tokens now live under `bellui-auth/<app ID>/`, and the
  app ID is part of the encryption key. **Behaviour change:** tokens saved by
  an earlier version are not read, so users sign in once more after
  upgrading; and `SaveToken`, `LoadToken` and `ClearToken` now need an app ID,
  returning the new `auth.ErrNoAppID` without one (Preferences already need
  it).

### Fixed

* **Ctrl+Tab moves focus out of a widget that keeps Tab**
  ([#340](https://github.com/belluikit/bellui/issues/340)). A multi-line
  `Entry` and the x/dag canvas keep Tab and Shift+Tab for themselves, and every
  Tab with a modifier went to the shortcut path, so a keyboard-only user who
  tabbed into one could not leave it (WCAG 2.1.2). Ctrl+Tab and Ctrl+Shift+Tab
  now move focus to the next and previous widget from any such widget, on the
  desktop, embedded and mobile drivers. **Behaviour change:** while a widget
  that keeps Tab is focused, Ctrl+Tab no longer reaches the app's main-menu or
  widget shortcuts; on every other widget it still does. The web build is not
  covered yet: browsers keep Ctrl+Tab for themselves
  ([#357](https://github.com/belluikit/bellui/issues/357)).
* **GitHub sign-in asks for GitHub's scopes**
  ([#335](https://github.com/belluikit/bellui/issues/335)). The OpenID
  default (`openid email profile`) was applied before the GitHub branch, so
  GitHub never got its own default of `user:email read:user`.
* **`desktop.App` is implemented on every desktop platform**
  ([#338](https://github.com/belluikit/bellui/issues/338)). It gained
  `SetSystemTrayTooltip` and `SetSystemTrayOnTapped`, but the app did not
  forward them on macOS, Windows or Linux/BSD, so the documented
  `if desk, ok := a.(desktop.App); ok { … }` check was always false and an
  app following it silently got no system tray.
* **The test helpers no longer ship in production binaries**
  ([#304](https://github.com/belluikit/bellui/issues/304)). `driver/software`
  imported `bellui/test` for one interface and two constructors, and
  `bellui/test` imports testify — so every desktop and web binary linked
  testify, `gopkg.in/yaml.v3`, spew, difflib and `testing`. In the js/wasm
  build of `cmd/bellui_settings` that was **988,505 bytes**, 2.09% of the
  artifact, and 208 yaml symbols. The canvas moved to `internal/memcanvas`;
  the public API is unchanged, since `test.WindowlessCanvas` and
  `test.SoftwarePainter` are now aliases of the moved types. A CI gate keeps
  it that way.
* **Android: every write to a file was a double free**
  ([#315](https://github.com/belluikit/bellui/issues/315)). `writeStream`
  freed the buffer the Go caller had allocated with `C.CBytes` and frees
  itself, which Scudo catches and turns into an abort — *"invalid chunk state
  when deallocating"*. Pre-existing; found by the first complete
  `hack/safprobe` run.
* **Android: `contentURIGetFileName` corrupted the heap and answered the wrong
  name** ([#315](https://github.com/belluikit/bellui/issues/315)). It returned
  a string literal on its error path, which the Go caller frees — `free()` on
  static storage. It also queried the raw URI, but `_display_name` is a column
  of the *document*, so a tree URI had no row to find and `Name()` fell back to
  the percent-encoded document ID: `primary%3ADocuments` where a user expects
  `Documents`. The same literal-return shape was in `listContentURI` and
  `listURI`.
* **Android: a picked SAF folder can be scaffolded into**
  ([#303](https://github.com/belluikit/bellui/issues/303)). `CreateChild` for
  `content://` goes through `DocumentsContract.createDocument`, and refuses a
  duplicate name rather than letting the provider silently invent
  `"name (1)"`. `Child` resolves an existing child by display name, and
  reports `storage.ErrNotExists` for one that is not there instead of a URI
  that looks real. `Exists` and `Delete` work for `content://`. `Parent`
  remains unsupported, and now says why: SAF exposes no upward link.
* **Android: listing a subdirectory listed the picked folder instead**
  ([#303](https://github.com/belluikit/bellui/issues/303)). `List` and
  `CanList` took their document ID from `getTreeDocumentId`, which returns the
  *root's* ID for any URI under the tree — so a scaffolded tree looked flat.
* **Android: folder grants survive a restart**
  ([#303](https://github.com/belluikit/bellui/issues/303)). Nothing called
  `takePersistableUriPermission`, so every pick was single-use and an app
  could not store a folder URI and reopen it later.
* **Android: `listContentURI` leaked a JNI local reference per row**
  ([#303](https://github.com/belluikit/bellui/issues/303)) — a directory of
  any size overruns the 512-entry table, which aborts the process.
* **A pending double-tap no longer outlives its widget**
  ([#310](https://github.com/belluikit/bellui/issues/310)). A single tap on a
  `DoubleTappable` leaves a timer waiting `TapDoubleDelay` for a second one,
  and nothing cancelled it when the canvas went away — only a second tap did.
  It then dispatched into widgets whose renderers had been destroyed. Tap a
  widget, close the dialog holding it inside the double-tap window, and that
  was a nil dereference.
* `URI.Path()` now says what it is: the RFC3986 path component, and a
  filesystem path only under `file://`. On Android a `content://` URI gives
  `/tree/primary%3ADocuments`, which looks like a path, is not one, and
  resolves under `/` if handed to `os.MkdirAll`
  ([#303](https://github.com/belluikit/bellui/issues/303)).

### Note

The SAF behaviour above is verified on a device — 21 assertions against the
external-storage provider on an API 34 emulator, including the duplicate-name
refusal and a grant surviving a force-stop — using the new `hack/safprobe`
harness. The remaining gap is that the run was driven by hand rather than by
the harness's own script
([#314](https://github.com/belluikit/bellui/issues/314)).

### Dependencies

* `github.com/belluikit/systray` v1.12.1: an app's exit callback could run
  twice when `Quit` raced with the platform's own close handling; it is now
  guarded so it runs once.
* `github.com/belluikit/fsnotify` v1.10.2: synced with upstream fsnotify
  (v1.10.1 plus later fixes), keeping the fork's Windows fixes.
* `x/mmedia` requires `github.com/belluikit/goffmpeg` v0.4.1, whose prebuilt
  purego bridges were rebuilt and now expose the device-modes API
  ([#330](https://github.com/belluikit/bellui/pull/330)).

### Licensing

* The EmojiOne Color font ships with its license file again, and
  `THIRD_PARTY_NOTICES.md` now covers the fyne-x code and test assets, the
  vendored packages, every bundled font, the Material Design icons and the
  sign-in logos ([#325](https://github.com/belluikit/bellui/pull/325)).
  Attributions the rebrand had rewritten to "bellui-x" name fyne-x again.

### CI

* CI runs on GitHub-hosted runners with stock Go instead of self-hosted
  machines and the go-plus toolchain, and installs the system packages those
  images lack ([#325](https://github.com/belluikit/bellui/pull/325),
  [#329](https://github.com/belluikit/bellui/pull/329)).
* `apt-get install` needs `-y`: five Linux legs died the moment apt had an
  upgrade to do ([#307](https://github.com/belluikit/bellui/issues/307)).
* chromium is launched without `snap-confine`, which had refused to start it
  from the runner service and took every wasm package down with it
  ([#311](https://github.com/belluikit/bellui/issues/311)).
* `TestRender` asserts the rendered image rather than an exact PNG byte
  string, which go1.27's `compress/flate` changed
  ([#318](https://github.com/belluikit/bellui/issues/318)).
* The PowerShell go-plus assertions accept `(plus <version>)`, which go1.27's
  fork build reports ([#320](https://github.com/belluikit/bellui/issues/320)).

## v1.0.0 — 2026-08-05

The first **bellui stable**, closing out the v1.0.0-rc line
([#136](https://github.com/belluikit/bellui/issues/136)). The v1 line in one
paragraph: the MD3 foundation (tonal palettes, type scale, 36 components),
the text/IME/editor stack, the observable data-model layer, opt-in
screen-reader accessibility on five platform backends, deep platform
integration (native dialogs, runtime Wayland/X11 selection, clipboard
flavors, notifications, single-instance/open-with), the portable webview —
including the Linux CEF backend with its documented bundling costs
(docs/CEF-MAINTENANCE.md) — and the opt-in devtools inspector and
consent-first analytics. Everything below is new since v1.0.0-rc.5; the
rc.6 tag (2026-07-31) cut mid-section and shares this entry.

### Added

* **Desktop touchscreen gesture dispatch**
  ([#266](https://github.com/belluikit/bellui/issues/266)). The GLFW driver
  now receives real touch input on Windows (`WM_POINTER`) and Wayland
  (`wl_touch`) via the glfw fork's new touch callback (belluikit/glfw
  v1.10.0) and feeds it through the same gesture state machine the mobile
  driver runs — so a desktop touchscreen gets slop-cancelled taps, drags,
  double-tap and the hold-fire long-press, making the #259 file-dialog
  selection mode (and `event.LongPressable` generally) reachable from real
  desktop touch. Touch dispatches the mobile vocabulary (no hover, no
  `desktop.Mouseable`); on Windows this replaces the system's legacy
  touch-to-mouse emulation for bellui windows. X11 and macOS are unchanged
  (touch continues to arrive as an emulated mouse, which the press-and-hold
  entry below covers). Verify with `hack/touchprobe`.

* **Desktop touch flick-scrolls content**
  ([#291](https://github.com/belluikit/bellui/issues/291)). A touchscreen
  drag over scrollable content now scrolls it mobile-style: the drag rides
  the wheel's scroll dispatch as touch-sourced `event.ScrollEvent`s
  (`ScrollSourceTouch`), so nested-scroll propagation applies, and a flick
  keeps scrolling with the mobile decay curve until it fades, hits the
  boundary, or the next contact stops it. Mouse drags deliberately still do
  not scroll content, and Draggables under the finger (scrollbar thumbs,
  sliders) keep winning the gesture. Touch-sourced scrolls are
  direction-absolute in `widget.Scroll`: no RTL flip, no vertical→horizontal
  redirect — the content tracks the finger, as on mobile.

* **Desktop press-and-hold dispatches `LongPressed`**

  ([#266](https://github.com/belluikit/bellui/issues/266)). The glfw driver
  hold-fires the gesture the touch drivers already had (pre-rebrand tracker #758): a lone
  primary-button press held stationary for 300 ms dispatches `LongPressed`
  to the `LongPressable` under the pointer while the button is still down,
  and the release of a hold-fired press is not a tap. This applies to every
  primary press — mouse included, matching how GTK, Flutter and Qt
  long-press recognizers treat mice, and on a desktop touchscreen the
  synthesized events are indistinguishable anyway — so `Entry` word-selects
  on a slow click and the canvas file dialog's selection mode (#259) is now
  reachable on desktop. Unlike the touch drivers, the release of a
  hold-fired press dispatches nothing further: right-click keeps sole
  ownership of `TappedSecondary` on desktop. Windows and Wayland
  touchscreens now deliver real touch contacts past this path entirely
  (the touchscreen gesture dispatch entry above, belluikit/glfw#1).
  
* **Diagonal resize cursors, and a pointer affordance on the `InnerWindow`
  resize corner** ([#269](https://github.com/belluikit/bellui/issues/269)).
  `desktop.StandardCursor` gains `NWSEResizeCursor` and `NESWResizeCursor`
  (glfw 3.4 shapes; the driver falls back to the horizontal-resize shape
  wherever the platform cannot create a diagonal — an X11/Wayland cursor theme
  without it, or a Cocoa build without the private NSCursor selector — and
  wasm maps them to CSS `nwse-resize`/`nesw-resize`). `container.InnerWindow`'s
  resize corner now returns the NWSE shape, so on desktop the pointer signals
  the corner resizes before it is dragged.

* **Touch multi-select for the canvas file dialog — long-press selection
  mode** ([#259](https://github.com/belluikit/bellui/issues/259)). In
  open-multiple mode, long-pressing a file item enters selection mode: plain
  taps toggle set membership — the modifier-less counterpart to
  Ctrl/Cmd/Shift-click, for touchscreens and for mobile embeddings whose
  platform app provides no multi picker (the canvas-dialog fallback from
  #255). Escape or Android's Back leaves the mode before it cancels the
  dialog, closing the dialog by any path ends it too (as does deselecting the
  last file), the double-tap open shortcut is suspended while the mode is
  active, and the pointer modifiers keep their usual precedence throughout.
  As part of this, the mobile driver now gives Android's Back button the
  Escape meaning generally — a behavior change for existing apps, recorded
  under Changed below. The GLFW driver's press-and-hold dispatch (#266,
  above) makes the gesture reachable on desktop too.

* **Escape closes the innermost pop-up, menu or dialog**
  ([#229](https://github.com/belluikit/bellui/issues/229)). The drivers give
  Escape a default meaning ahead of key delivery: the focused widget gets
  first refusal through the new `event.EscapeHandler` (`Entry` while composing
  or with `OnEscape` set, the in-place cell editor, `TimePicker` holding a
  half-typed value), then overlays are asked innermost-first through
  `event.EscapeDismissible`, implemented by `widget.PopUp` — covering every
  canvas dialog, sheet and drawer. Modality does not block it: Escape is how a
  user cancels a modal dialog. Snackbars and toasts decline and pass the key
  beneath; an overlay that implements neither interface (a menu, a completion
  list) ends the search and keeps handling Escape through the focus path. The
  new `widget.PopUp.OnDismissed` hook lets a host route dismissal through its
  own close path, which is how a dialog cancelled with Escape still reports
  its result.

* **`x/mmedia` — media playback widgets, consolidated from
  `trendvidia/fyplayer`.** `MediaPlayer`, `VideoPlayer`, `AudioPlayer`,
  `SubtitleOverlay` and `TransportControls`, decoding through
  `belluikit/goffmpeg`. Both the `io.Reader` and the URL constructor forms are
  present, so HLS, RTSP and RTMP work — a playlist read as a byte stream is
  only text. The fifth `x/` module; not yet tagged.

  Two things to know before depending on it. It moves the FFmpeg binding from
  `goffmpeg v0.2.0` to `v0.4.0` (and `goffmpeg-libs` `v0.1.0` → `v0.3.0`),
  additive on the API it uses but a real version change; and it declares
  `go 1.26.2` rather than the `1.25.0` its siblings use, because
  `goffmpeg-libs` requires it. Source ref, rewrite set and verification are in
  `x/mmedia/PROVENANCE.md`.

* **`x/editor`: `Editor.ApplyGutter` and `Editor.ApplyDecorators`**
  ([#281](https://github.com/belluikit/bellui/issues/281)). The `Gutter`,
  `Decorators` and `SpanDecorators` fields were read exactly once, at first
  render — `RefreshGutter`/`RefreshDecorators` repaint the *installed* objects
  and `RefreshHighlighter` re-applies the `Highlighter` field, but nothing
  re-applied a swapped gutter or decorator set, so hosts had to seal their
  gutter composition at construction and mutate retained provider instances
  for anything dynamic. The new methods mirror `RefreshHighlighter`: they
  re-read the field(s), rebuild the gutter widget / decorator overlay (the
  editor's internal decorators — search matches, diagnostics, hover, links,
  redaction — are re-appended) and refresh the layout, so a gutter can be
  installed, swapped or removed on a live editor. Downstream, player's
  `CodeEditor` can drop its build-time-only `gutters` sealing
  (belluikit/player#45).

### Fixed

* **Android: a per-app language change applies without restarting the app**
  ([#288](https://github.com/belluikit/bellui/issues/288)). With `locale` and
  `layoutDirection` declared in the packaged manifest (#283, #287), an
  Android 13+ per-app language change reaches a foregrounded app as an
  in-place `onConfigurationChanged` — no activity recreation, and therefore
  no lifecycle crossing for the #282 foregrounding re-resolve to catch, so
  the app kept running in the old language until it was next backgrounded
  and brought forward. The native callback now forwards a
  configuration-change event through the app event queue (the send is
  unbounded, so the platform main thread never blocks) and the mobile driver
  answers it on its own goroutine with the same `lang.ResolveSystemLocale`
  seam: translations, CLDR formatting, collation and text direction
  re-resolve and every window re-lays-out in place. An explicit
  `lang.SetLocale` override still wins, and an unchanged locale skips the
  relayout, so the other declared configuration changes that land on the same
  path (rotation, dark mode, keyboard availability) cost only the locale
  lookup. Verified on a pixel143 AVD with `hack/deviceinfoprobe` and
  `hack/recreateprobe`; vendor builds whose relaunch mask still covers the
  change (ColorOS 16) keep recreating the activity and pick the new locale up
  on the foregrounding re-cross, as before.

* **Android: the in-place language change reaches ColorOS 16 devices too**
  ([#299](https://github.com/belluikit/bellui/issues/299)). ColorOS 16
  (OPPO/realme/OnePlus) adds `CONFIG_SCREEN_LAYOUT` to every per-app locale
  change mask, so the #288 in-place path never engaged there — each per-app
  language change relaunched the activity. The packaged manifest now declares
  `screenLayout` as well, which defeats the vendor hook; measured on an
  OPD2403, changes carrying the still-undeclared screen-size bits
  (fold/unfold, split-screen divider resizes) keep recreating exactly as
  before, while screen-layout-only transitions (ColorOS split-screen entry)
  now resize in place. Declaring the flag also fixed the `bellui package`
  resource table: the table-stripping generator blanked any attribute-flag
  name whose key-pool slot it shared with a dropped private entry —
  `screenLayout` was the casualty — making the name a hard manifest-compile
  error; every `configChanges` flag of the pinned SDK platform is now
  declarable and locked by tests.

* **Android: in-process activity recreation no longer hangs the app (ANR)**
  ([#283](https://github.com/belluikit/bellui/issues/283)). A configuration
  change the manifest does not declare — an Android 13+ per-app language
  override was the guaranteed trigger until `locale` was declared (#287,
  below), a display density/size change another — recreates the activity
  inside the surviving process, and the WebGPU painter kept presenting
  frames into the just-destroyed native window during the
  destroy→create gap; the poisoned surface then made the swap to the new
  window block forever, parking the platform main thread in the redraw
  handshake (on the emulator it also wedged the host GPU channel so *every*
  later app launch hung until the emulator process was restarted). The driver
  now tracks native-window liveness (`app.NativeWindowLive`): frames are
  skipped while the window is dead, and a frame already being recorded when
  the window dies is discarded instead of presented. Verified with
  `hack/recreateprobe` on a pixel143 AVD and an OPD2403 (ColorOS 16) through
  repeated locale-override and density recreations, rotation and
  background/resume cycles. The packaging template additionally declares
  `locale` in `configChanges` — completed by `layoutDirection` once the
  binres SDK table learned that flag
  ([#287](https://github.com/belluikit/bellui/issues/287), next entry) — and
  the recreation that still happens is now survivable.

* **`bellui package` Android manifests: `configChanges` flags newer than the
  gomobile-era SDK table now encode, and an unknown name fails loudly**
  ([#287](https://github.com/belluikit/bellui/issues/287)). The CLI's binres
  manifest encoder resolves each `|`-separated `configChanges` name against
  an embedded, stripped copy of the platform resource table that dated from
  the upstream gomobile era and silently dropped any name it did not know:
  `orientation|keyboardHidden|uiMode|locale|layoutDirection` encoded as
  0x2a4 — `layoutDirection` (API 17, bit 0x2000) simply vanished from the
  packaged APK. The table is now regenerated from a pinned platform
  (`binres.SDKPlatform` = android-35, recorded in the generated arsc.go
  header so regeneration is reproducible), which required teaching the
  binres decoder the modern aapt2 table encodings (sparse and offset16 entry
  indices, compact entries, staged-alias chunks, multi-package tables). The
  same mask now encodes as 0x22a4, verified against `aapt dump xmltree`, and
  an unresolvable flag name is a hard manifest-compile error naming the flag
  instead of silent data loss. With the table current, the packaging
  template declares `layoutDirection` alongside `locale` (#283's pair), so
  an Android 13+ per-app language change reaches the app as
  `onConfigurationChanged` instead of recreating the activity.

* **`bellui package` Android manifests: an unknown enum attribute value is a
  hard error instead of silently encoding 0**
  ([#295](https://github.com/belluikit/bellui/issues/295)). #287 made
  unresolvable `configChanges` *flag* names fatal, but the sibling enum
  branch kept the silent miss: a single-valued enum attribute matching no
  table value left the typed value zeroed, and 0 is a valid enum value for
  most attributes — `android:screenOrientation="typo"` compiled to a
  manifest that meant `landscape`. An unresolvable enum value now fails
  manifest compilation naming the value, the attribute and the pinned
  platform.

* **The CLI's aapt2-link step picks the newest installed SDK platform
  numerically** ([#296](https://github.com/belluikit/bellui/issues/296)).
  `binres.LatestAPIResourcesPath` sorted platform directories lexically, so
  a stray ancient `android-9` beat `android-36`. It now compares the API
  level as a number; preview and extension platform directories
  (`android-VanillaIceCream`, `android-36-ext4`) deliberately rank behind
  every numbered release and are only picked when nothing numbered is
  installed.

* **The binres aapt-golden tests run again**
  ([#297](https://github.com/belluikit/bellui/issues/297)). `TestEncode`,
  `TestTableRefByName` and `TestTableMarshal` were gated by a skip on any
  SDK newer than android-15, so on every real machine they never ran — and
  the checked-in golden had drifted from the encoder's synthesized
  build-version attributes years ago without anything noticing. The goldens
  are regenerated from the pinned android-35 platform, the encoder
  synthesizes the four build-platform attributes modern aapt records
  (`compileSdkVersion`, `compileSdkVersionCodename`, both
  `platformBuildVersion*` — derived from the `binres.SDKPlatform` pin),
  `TestEncode` runs unconditionally (both sides ride the repo),
  `TestTableMarshal` round-trips the real installed platform table when the
  pinned platform is present, and #287's throwaway `aapt dump xmltree`
  verification of the compiled template manifest is now a test, opt-in on
  ANDROID_HOME + build-tools.

* **The mobile pickers honour `UseNativeFileDialogs(false)`**
  ([#267](https://github.com/belluikit/bellui/issues/267)). The doc promised
  the canvas dialog "everywhere", but the iOS/Android overrides consulted
  only picker availability, so the opt-out was silently inert on mobile —
  and the canvas dialog was unreachable on a packaged device build. The
  toggle now gates both mobile overrides; wasm remains the documented
  exception (a canvas dialog has no real filesystem to browse in a
  browser). Note the cost, now recorded in the doc: the platform picker is
  the only surface that reaches outside the app sandbox on modern mobile
  systems, so an app opting out for desktop-consistency reasons should gate
  the call on the platform.
  
* **A malformed system locale entry no longer degrades the whole locale**
  ([#271](https://github.com/belluikit/bellui/issues/271)). On Android a
  failed locale lookup came back from go-locale as a list holding one empty
  string, which failed BCP-47 parsing at every start: the region-carrying
  tag behind CLDR date/number formatting, collation and text direction was
  left at the matcher's language-only value, `SystemLocale()` reported the
  undetermined locale `und`, and the log named no locale at all. Locale
  strings are now normalized before parsing — Java `Locale.toString()`
  underscores, and the trailing `#...` marker whose script is folded back
  into the tag (`zh_CN_#Hans` → `zh-Hans-CN`) rather than dropped. An entry
  that is still not well-formed is logged with the raw string quoted and
  keeps whatever part of the tag parsed, so it cannot pass the locale to the
  entry behind it; only an entry carrying nothing at all is dropped. A
  failed lookup no longer has its result parsed, and the formatting tag
  comes from the first entry that parsed rather than blindly the first entry
  reported. The underlying Android lookup failure (`no current JVM` — the
  locale resolves during library init, before JNI_OnLoad) is fixed below
  ([#282](https://github.com/belluikit/bellui/issues/282)).

* **Android apps no longer run in English on a non-English device**
  ([#282](https://github.com/belluikit/bellui/issues/282)). The lang package
  resolves the system locale during package init, which on Android runs in the
  shared library's global constructor — before the JVM is attached — so the
  lookup failed deterministically on every start and the app settled on the
  English fallback for translations, CLDR date/number formatting, collation
  and text direction. Apps that happened to call `AddTranslations*` from their
  own `main()` self-healed by accident; everything else stayed English. The
  locale is now re-resolved at app creation (the JNI context is available
  before Go's `main` starts, so UI built there — including `lang.L` strings
  captured at build time — sees the real locale from the first frame), and
  again on each foregrounding, which picks up a system language switched while
  the app was backgrounded (Android recreates the activity, not the process).
  A change fires the same relayout as `lang.SetLocale`. The re-resolve — new
  exported seam `lang.ResolveSystemLocale` — respects an explicit `SetLocale`
  override, and `AddTranslations*` after `SetLocale` no longer silently
  reverts the app to the system locale (the same override guard, previously
  missing from the shared re-derivation path). Verified on the Android bench;
  other platforms resolve at init successfully and are unaffected.

* **Android: canvas scale is derived arithmetically from the reported
  density** ([#272](https://github.com/belluikit/bellui/issues/272)),
  matching `DisplayMetrics.density` (densityDpi/160), instead of a bucket
  lookup that rounded off-bucket densities to the nearest step. Devices
  configured between the classic buckets — 356, 420, 440dpi panels are
  common — previously rendered everything derived from dp (touch-target
  floors, text sizes) up to ~12% off in physical units; bucket densities
  (160/240/320/480/640) map exactly as before. The one bucket that does
  change is tvdpi (213), which now scales by 1.33125 as the platform does
  rather than the 1.5 the old ladder rounded it to, so an Android TV panel
  configured at tvdpi renders ~11% smaller. The "unknown density" log for
  off-bucket values is gone, and the `ACONFIGURATION_DENSITY_ANY` sentinel no
  longer reads as a 65534dpi screen.

* **wasm: the browser picker overrides no longer fire under the test driver**
  ([#257](https://github.com/belluikit/bellui/issues/257)). In a js/wasm test
  binary, `FileOpen.Show()` clicked a real hidden `<input type="file">` in the
  harness page and `FileSave.Show()` handed out a writer whose Close triggered
  a real browser download. The web driver's window now implements
  `driver.NativeWindow` — delivering the new `driver.BrowserWindowContext`,
  which carries the window's `<canvas>` element — and the wasm overrides
  assert it on the dialog's parent exactly like the desktop native backends,
  so the test driver falls back to the canvas dialog. An emulated extra window
  keeps the picker — it delegates to the root window it embeds and reports the
  root's `<canvas>`. A `nil` dialog parent no longer reaches the browser
  picker either: it now takes the canvas-dialog path, which — as on every
  desktop backend — requires a real parent window. The two #257-skipped
  cancel-shape subtests are back on js.

* **Mobile multi-select open uses the native pickers**
  ([#255](https://github.com/belluikit/bellui/issues/255)).
  `ShowFileOpenMultiple` on iOS and Android used to fall back to the canvas
  dialog, whose multi-select needs modifier keys a touch device does not
  have — picking more than one file was impossible. The iOS document picker
  now presents with `allowsMultipleSelection` (each URL crossing the bridge
  with its own retained security-scope closer) and Android's SAF intent sets
  `EXTRA_ALLOW_MULTIPLE` (verified live on the Android bench: two files
  chosen and both readers delivered; cancel reports `(nil, nil)`). An app
  whose platform provides no multi picker still falls back to the canvas
  dialog. Fixing the bridge also plugs a single-select leak: the picker's
  resource closer was assigned after the native reader/writer had captured
  its nil value, so the iOS security scope was never released.

* **The windowed canvas dialog's resize corner is grabbable on mobile**
  ([#263](https://github.com/belluikit/bellui/issues/263)). The
  `container.InnerWindow` resize corner was the #254 bug class left on
  desktop metrics: a 16dp drag target under every platform's touch floor,
  with the drivers' 8dp upward touch shift leaving ~8dp of it usable — a
  finger trying to resize a windowed dialog missed it most of the time. On
  mobile the corner now spans the frame-button touch target (48dp on the
  default theme) with the visual indicator unchanged, pinned bottom-right.
  The deliberate cost: a drag *starting* inside that corner square resizes
  the window instead of scrolling content beneath it; scrolls starting
  anywhere else are unaffected (the drag target latches at the gesture
  origin), and taps in the square pass through to the content — the corner
  captures drags only. Desktop keeps its 16dp corner.

* **The windowed canvas dialog's frame buttons are tappable on mobile**
  ([#254](https://github.com/belluikit/bellui/issues/254)). The
  `container.InnerWindow` chrome used desktop metrics everywhere, leaving the
  close button a 16dp target — under every platform's touch minimum — while
  the mobile drivers shift each touch up by 8dp of finger compensation, so on
  an iPad a tap on the X resolved above the button onto the inert title label
  nearly every time. On mobile devices each frame button now spans the theme's
  icon-button size plus a padding ring — 48dp on the default theme, the
  MD3/Android touch-target floor — with the visual button kept at the
  icon-button size carrying the standard icon; desktop chrome is unchanged.

* **Android: folders granted through the SAF picker's Download shortcut are
  accepted** ([#244](https://github.com/belluikit/bellui/issues/244)). The
  listability probe trusted `ContentResolver.getType`, which the downloads
  provider answers with null for a tree root the app holds a perfectly good
  grant for; the probe now falls back to attempting the children query that
  `List()` actually performs. Verified live on the Android 15/ColorOS bench
  device.

* **`driver.ObjectAtPosition`/`ObjectAtPositionMatching` cover every dispatch
  root** ([#245](https://github.com/belluikit/bellui/issues/245)). The public
  pick wrappers now probe through the canvas's own dispatch hit-walk when it
  exposes one, so the desktop menu bar and the mobile window head or open
  menu are pickable — and the spatial-index fast path is used — exactly as
  in tap dispatch; canvases without extra roots keep the overlays+content
  walk. Window-head objects also resolve through
  `Driver.AbsolutePositionForObject` now (their tree was missing from the
  canvas object trees).

* **`bellui package` fails fast, and clearly, when run outside a main
  package** ([#248](https://github.com/belluikit/bellui/issues/248)).
  Running it from the module root while the application lives in a
  subdirectory used to inject the generated metadata file into the non-main
  package and die later inside the build with go's cryptic "found packages X
  and main"; the packager now checks the source directory's own package up
  front and names it in the error. Wrapping a prebuilt `--executable` never
  builds or injects, so it stays exempt from the check.

* **File pickers: four fixes from the first on-device iOS pass** (#239).
  Closing a saved file no longer segfaults — the ObjC file bridge handed Go
  autoreleased `NSURL`/`NSFileHandle` pointers, now retained across the
  bridge and released on close, with the writer flushed before the security
  scope drops ([#250](https://github.com/belluikit/bellui/issues/250)).
  File filters actually filter — the picker moved off the deprecated
  UTI/`initWithDocumentTypes:` path to `UTType` +
  `initForOpeningContentTypes:`
  ([#251](https://github.com/belluikit/bellui/issues/251)). `SetFileName`
  reaches the save picker instead of every save offering the literal name
  "filename" ([#252](https://github.com/belluikit/bellui/issues/252)). And
  cancelling a multi-select canvas file dialog — every mobile multi-select —
  no longer panics on the callback shape
  ([#253](https://github.com/belluikit/bellui/issues/253)).

### Changed

* **A desktop primary press held on a `LongPressable` no longer delivers
  `Tapped`** ([#266](https://github.com/belluikit/bellui/issues/266)). The
  press-and-hold dispatch above is a behavior change for existing desktop
  apps: a click whose button stays down for 300 ms now dispatches
  `LongPressed` instead of `Tapped` to a widget implementing
  `event.LongPressable` — in this toolkit `Entry`, `RichEntry`, selectable
  text and the canvas file dialog's items; every other widget, `Button`
  included, taps on a slow click exactly as before. A `LongPressable` that
  declines the gesture must answer the press another way rather than ignore
  it (the file dialog's items fall back to the plain tap when the dialog is
  single-select or the item is a directory), because the withheld `Tapped`
  is not delivered afterwards.

* **Android's Back button now carries the Escape meaning**
  ([#259](https://github.com/belluikit/bellui/issues/259)). The mobile driver
  routes Back through the same dismissal walk as Escape, ahead of key
  delivery: it backs out of the innermost dismissible surface — a pop-up,
  menu, dialog, or the file dialog's touch selection mode — and a focused
  widget that claims Escape for a state of its own (`event.EscapeHandler`,
  e.g. an `Entry` with `OnEscape` set) receives the key as `KeyEscape`, the
  name its escape handling listens for. Only with nothing to dismiss does
  Back fall through to the app's `KeyBack` delivery (or, with no key hook,
  backgrounding the app). Previously Back backgrounded the app even while a
  modal dialog was open; apps that consumed `KeyBack` in `SetOnTypedKey`
  while an overlay was up will now see the overlay close instead.

* **Godoc `Since:` annotations on previously unreleased API now read
  `Since: 1.0`** ([#264](https://github.com/belluikit/bellui/issues/264)).
  API added after the fork's 2.9.0 release carried annotations from version
  lines that never shipped on this module path (2.10–2.19, 3.0); all are
  rewritten to the version that actually ships it, `v1.0.0`. Annotations from
  releases that did ship — upstream Fyne 1.0–2.7.3 and the fork's 2.8/2.9 —
  are unchanged. (This entry originally pointed to `docs/REBRAND.md` for the
  mapping; that file is not part of the public repository, and the rule above
  is the whole mapping.)

* **`driver.ObjectAtPosition`/`ObjectAtPositionMatching` now probe and report
  in the `Driver.AbsolutePositionForObject` space**
  ([#235](https://github.com/belluikit/bellui/issues/235)). The pick
  primitive's position and returned bounds are relative to the canvas
  interactive area rather than the raw canvas tree, so composing it with
  `AbsolutePositionForObject` — the pairing its doc comment recommends — is
  now exact on every driver. The interactive-area inset is zero on all
  desktop drivers, so behaviour changes only on mobile and the test canvas;
  the raw-tree helpers `GeometryForObject`/`ClipBoundsForObject` are
  deliberately unchanged. Pointer events still deliver `AbsolutePosition` in
  the raw-tree space — an event-sourced probe must subtract the inset first.

* **Native file dialogs are now the default**
  ([#226](https://github.com/belluikit/bellui/issues/226)).
  `dialog.UseNativeFileDialogs` flips from opt-in to opt-out: `FileOpen`,
  `FileSave` and `FolderOpen` are served by NSOpenPanel/NSSavePanel,
  IFileOpenDialog/IFileSaveDialog and the XDG FileChooser portal, with the
  canvas-drawn dialog as the fallback whenever the native backend cannot serve
  a request (no portal on the bus, unsupported platform, or a parent window
  with no platform handle — which keeps the test driver on the canvas dialog).
  Pass `UseNativeFileDialogs(false)` to restore the previous behaviour. The
  `SetShowHidden` toggle remains canvas-dialog-only.

## v1.0.0-rc.5 — 2026-07-29

**`x/editor` and `x/tables` were consolidated from the wrong source and are
re-cut here.** Anyone on rc.1–rc.4 using either module is on stale code and
should upgrade; `x/tables` users are the exposed ones, by a wide margin.
`x/charts` and `x/dag` are unchanged and correct.

**Unlike rc.3 and rc.4, this is not an `x/`-only release.** It is cut from
`main`, which has moved substantially since rc.4 was tagged on 2026-07-27 — 38
core commits, including the whole `webview` subsystem (macOS, iOS, Android,
wasm and Windows/WebView2 backends), `NativeViewHost`, the single-tree-walk
`nativeview` refactor, the fsnotify fork switch, and a reworked `RangeSlider`
with a disabled state and keyboard operation. Consumers should expect real
behaviour and rendering changes, not just an `x/` bump: the `RangeSlider` rework
alone changes rendered markup, so any golden/master files covering it need
regenerating.

Note that parts of the webview work have open issues at the time of tagging —
[#79](https://github.com/belluikit/bellui/issues/79) (wasm test leg),
[#82](https://github.com/belluikit/bellui/issues/82) (WebView2 not yet verified
on a real GPU session), [#83](https://github.com/belluikit/bellui/issues/83)
(a `Call` against a page with no shim stays pending). Weigh that before
adopting rc.5 for anything but the `x/` fix.

### Fixed

* **`x/editor` and `x/tables` were cut from each source repo's `chore/fyne-v3`
  branch instead of `main`** ([#85](https://github.com/belluikit/bellui/issues/85)).
  Those branches were abandoned duplicates: the real fyne-v3 migration had
  merged to `main` weeks earlier (`fytables` PR #79, `fycoder` #172) and
  development continued there, leaving each branch parked at its old fork point
  with one stale re-do commit on top. `x/tables` shipped **50 commits** behind
  its source and `x/editor` **14**.

  Re-cut from `origin/main`, recorded per module in `PROVENANCE.md` as
  `<repo>@<ref>@<sha>`:

  | Module | Was | Now |
  |---|---|---|
  | `x/editor` | `chore/fyne-v3`, ≈ `fycoder` v0.6.1-rc.1 | `main` @ `c766960` = **v0.12.0** |
  | `x/tables` | `chore/fyne-v3`, ≈ `fytables` v0.3.0-rc.2 | `main` @ `bb45169` = **v0.14.0** |

  `x/tables` regains two real defect fixes — `ColumnDef.Field` now resolves
  against struct rows and honours the dot notation it has always documented
  (`fytables` #84) — plus `event.Scrollable`, so AT scroll gestures page the
  grid (#82); `DisplayRow`/`ScrollToRow`, `CommitEditing`,
  `OnCellContextMenu`, `ColumnDef.CanEdit`/`.Editor`; and the `export/pdf` and
  `export/pxf` exporters, which were absent from the published module
  altogether. `x/editor` regains LSP semantic-token highlighting
  (`SemanticToken`, `SetSemanticTokens`) and `accessibility.go`
  (`SetAccessibilityLabel`, exposing the editor as a text area).

  Neither module's API changed relative to its source; the rewrites were
  mechanical. No new external dependency was added.

### Note

This shipped green and stayed undetected for four release candidates because a
stale branch still compiles — the consuming project (Bellui Player) built,
tested and released against it without a failure, since every API it *called*
still existed (belluikit/player#4). Nothing in the pipeline asks whether a
consolidated source was the newest code, only whether it builds. The
countermeasure is the `PROVENANCE.md` record plus the pre-flight check now
documented in `docs/REBRAND.md` in belluikit/player.

`x/charts` and `x/dag` came through the original consolidation correctly, but by
coincidence rather than by process: all four modules were cut the same wrong
way, and those two happened to sit on branches within a commit or two of their
tips. Their provenance is still unrecorded and should be established (#85).

All five `chore/fyne-v3` branches — `fytables`, `fycoder`, `fycharts`, `fydag`
and `extensions` — have since been deleted, each after confirming its PR was
merged and its remaining commits only re-applied the same import rewrite on a
stale base.

## v1.0.0-rc.4 — 2026-07-27

An `x/charts` fix release, cut promptly because the defect it carries **freezes
the application**. The core toolkit is unchanged from v1.0.0-rc.2; the core
module is re-tagged only to keep it in lockstep with the `x/` umbrella modules,
which pin it.

### Fixed

* **`x/charts`: a `ChartGroup`'s hover sync recursed forever, hanging the app**
  ([#36](https://github.com/belluikit/bellui/issues/36),
  [#37](https://github.com/belluikit/bellui/pull/37)). `wireHoverSync`
  registers, for every pane, an `EventMouseOut` listener that calls `HoverOut()`
  on every *other* pane — and `Chart.HoverOut()` re-emitted `EventMouseOut`
  unconditionally, even when nothing was hovered. Two panes were enough: A's
  mouse-out drove B, B emitted, which drove A, which emitted, without end. Go
  grows the goroutine stack rather than overflowing quickly, so it presented as
  a hang rather than a crash — reproduced in the gallery by leaving a chart on
  the Geo tab, and by switching from Geo to Financial (the tab change moves the
  pointer out of a chart), and diagnosed from a `sample(1)` of the wedged
  process. Two guards now break the cycle: `HoverOut` is idempotent (it
  repaints and emits only when it actually cleared a visible tooltip), and the
  group's hover fan-out takes a re-entrancy flag — the structural counterpart of
  the zoom path's `FromUser` check, which already guarded the same hazard one
  function above. Anyone on v1.0.0-rc.3 with a multi-pane `ChartGroup` is
  exposed and should upgrade.

### Known issues

* `x/charts`: the Geo/choropleth chart downloads and parses a 14 MB GeoJSON
  synchronously on the render goroutine (no timeout, failures not negatively
  cached) and re-rasterises the whole country set on every repaint — 226 ms per
  repaint against 15 ms for an ordinary chart, measured with the software
  painter ([#35](https://github.com/belluikit/bellui/issues/35)). Slow, but not
  the freeze above.
* `canvas.Raster` streaming is GL-only: on the default WebGPU painter a nil
  `Generator` segfaults and a `Streaming` raster's texture is never refreshed
  ([#34](https://github.com/belluikit/bellui/issues/34)).

## v1.0.0-rc.3 — 2026-07-27

An `x/editor` fix release. The core toolkit is unchanged from v1.0.0-rc.2; the
core module is re-tagged only to keep it in lockstep with the `x/` umbrella
modules, which pin it. All three fixes came from using the editor, and each
carries a regression test that fails on the previous release.

### Fixed

* **`x/editor`: line numbers detached from the content below a collapsed fold**
  ([#29](https://github.com/belluikit/bellui/issues/29),
  [#32](https://github.com/belluikit/bellui/pull/32)). The gutter rows drawn
  past the end of the document — the phantom numbers that keep the gutter
  spanning the full editor height on a short document — were positioned from the
  row *index* (`topOffset + row*stride`). A collapsed fold removes height
  without removing rows, so anything folded left those numbers hanging below the
  text with a gap the size of the folded region. Both the gutter and the
  decorator overlay now continue from where the laid-out content actually ends.
* **`x/editor`: gutter markers pushed the line numbers away from the code**
  ([#30](https://github.com/belluikit/bellui/issues/30)). `CompositeGutter`
  rendered its providers strictly in the order passed, so the common
  `[numbers, diagnostics, breakpoints]` composition drew a strip of unrelated
  markers between a number and its line. Providers now declare a column via the
  new `GutterColumnProvider` and the composite arranges them —
  `[markers][line numbers][fold]` — whatever order the host passes. Providers
  sharing a column keep their relative order; a provider that declares nothing
  counts as a marker, so a host's own provider stays on the leading edge;
  `NewCompositeGutterInOrder` opts out for a bespoke layout. `Width`, `Render`,
  the column hit-test (`TappedAtX`, including the #109 margin-tap fallback) and
  `breakpointProvider` all walk the arranged order. **This changes rendering and
  gutter tap x-positions for existing hosts with no call-site change** — the
  point of the fix, but hosts should re-check any coordinates they assumed.
* **`x/editor`: the completion popup ignored the editor's bottom edge**
  ([#31](https://github.com/belluikit/bellui/issues/31)).
  `widget.CompletionPopup.ShowAtCaret` picks its side against the *canvas*, so an
  editor that does not reach the bottom of its window opened the list downwards
  past its own bottom edge, over whatever sat beneath it. `popoverPositionIn`
  now separates the side choice (the editor viewport) from the clamp (the
  canvas), and the editor places the popup itself; the hover and signature cards
  use the same seam. The deliberate overflow of #19 survives: a list taller than
  a compact editor still opens downwards into the surrounding window rather than
  being squeezed above the caret.

### CI

* The macOS WebGPU runner fails `TestWindowedSurfaceCapture` whenever its
  display is asleep — `CGDisplayCopyDisplayMode` returns NULL, so no real
  surface exists and the capture reads back black
  ([#33](https://github.com/belluikit/bellui/issues/33)). Diagnosed and worked
  around per run; the durable wrapper is still to do.

## v1.0.0-rc.2 — 2026-07-27

Second release candidate of the rebrand soak. The substance is the WebGPU
bindings rebase: the previous binding lineage declared itself frozen
([#1](https://github.com/belluikit/bellui/issues/1)), so the fork now tracks a
maintained one, which also moves the toolkit onto a much newer `wgpu-native`.
Everything else is the fallout from that and the CI work needed to catch it.

### Changed

* WebGPU bindings rebased from `cogentcore/webgpu` onto `oliverbestmann/webgpu`
  via `github.com/belluikit/webgpu` v1.0.8 → **v1.1.0**, carrying `wgpu-native`
  v22.1.0.5 → **v29.0.1.1**
  ([#8](https://github.com/belluikit/bellui/pull/8),
  [#1](https://github.com/belluikit/bellui/issues/1)). The painter is ported to
  the new API surface (fallible `TryCreate*` constructors, `ShaderSourceWGSL`,
  tri-state depth-write, two-argument `Surface.Configure`, `TexelCopyTextureInfo`).
  No public Bellui API changes.
* `BELLUI_WEBGPU_BACKENDS` no longer accepts `dx11`/`d3d11` — wgpu-native
  dropped the D3D11 backend. `dx12`/`d3d12`, `vulkan`, `metal`, `gl`, `primary`,
  `secondary` and `all` are unchanged.

### Added

* `x/dag`: graph serialization restored without pulling gRPC into the build
  graph ([#3](https://github.com/belluikit/bellui/issues/3),
  [#14](https://github.com/belluikit/bellui/pull/14)).

### Fixed

* **WebGPU on Windows/D3D12: a fatal abort on every window resize**
  ([#20](https://github.com/belluikit/bellui/issues/20)). The surface texture
  acquired each frame holds a reference to a swapchain back buffer, and the
  present path dropped the handle without releasing it; DXGI's `ResizeBuffers`
  rejects an outstanding back-buffer reference, and wgpu-native turns that
  refusal into a Rust panic across cgo — killing the process rather than
  returning an error. The present path now releases the texture, matching what
  the discard path always did. Verified on a real desktop session (RTX 2070
  SUPER); this is only correct with the rebased bindings, which balance the
  device reference on acquire.
* WebGPU: a surface advertising no usable capabilities is no longer configured
  ([#20](https://github.com/belluikit/bellui/issues/20)). Both call sites
  indexed `caps.AlphaModes[0]` unguarded, so a surface that cannot present on
  the adapter produced a Go index panic on the way into an unrecoverable
  wgpu-native one. `NewDevice` now reports it (the painter logs and leaves the
  window unpainted) and `RecreateSurface` leaves the surface unconfigured.
* `internal/rasterx` cleared under staticcheck; `internal/render` import
  grouping and an `internal/ico` test path assumption fixed — the analyzer half
  of the house-style pass over the newly vendored code.

### CI

* New test legs: Linux x86_64 and Windows ARM64
  ([#16](https://github.com/belluikit/bellui/pull/16)).
* `gocyclo` scoped to first-party main-module code, so it stops descending into
  vendored third-party and the `x/` submodules
  ([#13](https://github.com/belluikit/bellui/issues/13),
  [#18](https://github.com/belluikit/bellui/issues/18)).
* The Windows x64 runner now runs in a real desktop session instead of as a
  session-0 service ([#25](https://github.com/belluikit/bellui/issues/25)). A
  service's window station has no display surfaces, so D3D12 cannot create a
  swapchain there — which is why the windowed present/capture coverage of #329
  could not run on it under the new bindings.

## v1.0.0-rc.1 — 2026-07-26

First release under the **Bellui** name and the `github.com/belluikit/bellui`
module path. This is the ecosystem rebrand cutover (pre-rebrand tracker #730, epic
pre-rebrand tracker #732); the toolkit's own features are unchanged from the `v3.0.0-rc` line of the
`trendvidia/fyne` fork, which is preserved frozen as the history reference.

Release-candidate while the rebrand soaks.

### Changed

* Module path `github.com/trendvidia/fyne/v3` → `github.com/belluikit/bellui`
  (no major suffix — a new module path under a new brand restarts the version
  line). Package `fyne` → `bellui`.
* CLI `fyne` → `bellui`; environment variables `FYNE_*` → `BELLUI_*`. Hard
  cutover, no back-compat fallbacks. App metadata was already `manifest.pxf`.
* Dependencies: the seven toolkit-specific forks moved to `belluikit`
  (`systray`, `glfw`, `glfw-js`, `gl-js`, `webgpu`, `typesetting`, `render`).
  `github.com/trendvidia/protowire-go` deliberately stayed put — it is a
  standalone public library, not a fork of ours.
* `github.com/srwiley/{oksvg,rasterx}`, `github.com/fyne-io/image/ico` and
  `belluikit/render` are now vendored under `internal/`. **The build graph and
  `go.sum` contain zero `srwiley` and zero `fyne-io`/`fyne.io` references**;
  build modules dropped from 33 to 30.

### Added

* `x/` umbrella: `x/{tables,editor,dag,charts}`, each its own module, migrated
  from `trendvidia/{fytables,fycoder,fydag,fycharts}`.

### Notes

* The `belluikit/*` repositories are **private** during the rebrand soak.
  Building outside the org requires `GOPRIVATE=github.com/belluikit/*` and git
  credentials.

## Unreleased

### Fixed

* A visible `widget.Tooltip` no longer consumes the click aimed at the widget beneath it (pre-rebrand tracker #424). The tooltip bubble is no longer a `PopUp`: it is hosted in a dedicated input-pass-through overlay, and overlay hit-testing (`internal/driver.FindObjectAtPositionMatching`) now falls through to the roots beneath when the top overlay implements the new `InputPassThrough` marker and nothing inside it matches. Dialogs/popups are unaffected (`PopUp` remains an input barrier).

### Added

* IME / input-method composition support — §9 (pre-rebrand tracker #35): new `event.Composable` interface (`Compose`/`ComposeCommit`/`ComposeCancel`) and `event.PreeditEvent`. `widget.Entry` shows in-progress preedit text inline at the caret (underlined) without touching `Text`, `OnChanged`, undo or bindings, committing the finalized text as a single edit. **Desktop** (macOS/Linux/Windows) is wired through the forked GLFW binding (GLFW 3.4 preedit callbacks): preedit display, `GLFW_IME` mode toggled when a composable widget is focused, and candidate-window positioning via the new `event.PreeditPositioner` (pre-rebrand tracker #125). **WebAssembly** via `compositionstart`/`compositionupdate`/`compositionend` DOM events. **Mobile** Go-side seam in place (composition event + `keyboardComposing`/`keyboardComposeCommit` cgo exports + driver dispatch); native Android/iOS composition is a tracked follow-up (pre-rebrand tracker #126). Test helpers `test.Compose`/`ComposeCommit`/`ComposeCancel`.

### Changed

* Desktop GLFW binding swapped from `github.com/go-gl/glfw/v3.3/glfw` to the `github.com/belluikit/glfw` fork (vendored GLFW bumped 3.3.10 → 3.4.0), which adds the IME preedit API. No behaviour change beyond IME support.

## 2.9.0 - 24 June 2026

The 2.8.1–2.8.3 tags were CI/build patch releases; this entry covers the
feature and fix work accumulated since 2.8.3.

### Added

* RTL right-to-left polish — §8 Phase 4 (pre-rebrand tracker #21): directional icons (back/forward, submenu chevron, mail reply/forward) mirror under a right-to-left layout via a new SVG horizontal-flip; horizontal scroll containers open at their right edge and reverse wheel/drag; and Tab/Shift+Tab focus order follows the visual right-to-left arrangement. `widget.Select` Left/Right cycling flips to match.
* Live localization — locale changes now re-translate static text: `binding.NewLocalizedString` + `lang.AddLocaleChangeListener`, `lang.BindText` for non-binding text (button labels, window titles, menu items), and key/plural `binding` localized-string variants; `lang.SetLocale` re-lays-out and repaints running windows (pre-rebrand tracker #87, pre-rebrand tracker #90, pre-rebrand tracker #92, pre-rebrand tracker #93).
* `widget` rich text: per-run `StyleRun` style fields and `Entry.CaretRect` column geometry (F1/F2).

### Fixed

* WebAssembly / WebGL: per-draw vertex-attribute state for WebGL conformance (pre-rebrand tracker #102), browser event callbacks marshalled onto the main loop (pre-rebrand tracker #113), and ripple shader program initialization (panic on button tap).
* Isolated the flaky `TestPreferences_Save` (pre-rebrand tracker #94).

### Changed

* Text stack self-hosted: the `trendvidia/typesetting` and `trendvidia/render` forks are imported directly (no `go.mod` replace), rebased onto upstream, and `GOPRIVATE` dropped from CI (pre-rebrand tracker #38, pre-rebrand tracker #106, pre-rebrand tracker #107). The WebAssembly `gl-js` dependency is imported from the trendvidia fork (pre-rebrand tracker #101).

## 2.8.0 - 10 June 2026

First release under the `github.com/trendvidia/fyne/v2` module path.

### Changed

* Module path moved from `fyne.io/fyne/v2` to `github.com/trendvidia/fyne/v2`
* Geometry, event, and core interface types live canonically in `geo`, `event`, and `core` (root aliases removed)
* `App`/`Window`/`Driver`/`Preferences` interfaces decomposed; optional capabilities via extension interfaces and generic free functions
* Default theme implements the Material Design 3 color system, typography scale, and component visuals
* Demo apps removed; the MD3 gallery lives at `github.com/trendvidia/gallery`

### Added

* MD3 component set (36 widgets), DatePicker/TimePicker/DateTime widgets, TreeView, Paned, SearchField, SpinBox, Toast, Tooltip and more
* Rendering engine: persistent canvas FBO, Gaussian-blur shadows, shader ripple, stencil clipping, RepaintBoundary, multi-stop gradients, Scene3D
* Advanced text: per-glyph weight/line-height/letter-spacing, text-on-path, per-character styled text, HTML parsing, syntax-highlighted code blocks
* Dev tools package (`dev/`): inspector overlay, tree dump, trace, hot reload

### Fixed

* Popup/overlay rendering with the canvas FBO; render-target restoration after offscreen GL work
* Hover dispatch: labels no longer swallow hover from enclosing rows; TreeView row actions stay visible and clickable
* Stale-frame bleed-through on borders while scrolling (opaque canvas FBO blit)
* Data race on the font cache access timestamp; Toast auto-dismiss timer leak
* Mobile and wasm builds of the GL painter

## 2.7.3 - 21 Feb 2026

### Fixed

* systray not functional within snap sandbox (systray#64)
* A dialog with a table. Selecting a cell and closing the dialog produces a Fyne error (fyne-io/fyne#5166)
* Fix JNI crash due to invalid global reference when accessing Android file streams (fyne-io/fyne#6067)
* Low frame rate with animated rectangles on 2.7.x (fyne-io/fyne#6040)
* Random panic when launching the app (fyne-io/fyne#5981)
* Tree Widget constantly calls IsBranch and ChildrenUIs (fyne-io/fyne#4339)
* Font scanning fails in Flatpak due to broken symlinks
* Correct system tray icon on Linux when using SVG
* Button press animation sometimes leaks outside the bounds of the button (fyne-io/fyne#6096)
* SVG cannot be loaded if it has currentColor at root element (fyne-io/fyne#6102)
* Wrapped text is rendered poorly (blurred) (fyne-io/fyne#4390)
* Blurry text in mobile Linux (fyne-io/fyne#2280)
* On 2.3.0 text is fuzzy (fyne-io/fyne#3464)
* Correct presentation of HTML entities when using markdown in RichText
* Accordion kills CPU by inefficient MinSize implementation (fyne-io/fyne#6108)
* Tree with no data (with Accordion?) - triggers endless layout loop (fyne-io/fyne#6094)
* NewRichTextFromMarkdown can't handle nested lists (fyne-io/fyne#6113)


## 2.7.2 - 6 Jan 2026

### Changed

* Added Japanese translation

### Fixed

* Crash in accordion (fyne-io/fyne#3966)
* Extended list does not focus correctly when tapped (fyne-io/fyne#5997)
* TextGrid: Ensure we are refreshing the correct row (fyne-io/fyne#6023)
* Entry doesn't lose focus when clicking on button elsewhere (fyne-io/fyne#5107)
* Mouse clicks missed (fyne-io/fyne#4672)
* Possible crash when form is truncated when an Entry is focused
* Alt-Tab'ing should not open MainMenu (fyne-io/fyne#2998)
* Wayland: ALT key steals focus to the menubar if it exists (fyne-io/fyne#5960)
* Extending the build-In theme only works fully when theme is set after app has started (fyne-io/fyne#6056)
* Multi-segment RichText not right aligning correctly (fyne-io/fyne#6060)


## 2.7.1 - 14 Nov 2025

### Fixed

* Ensure tar files created in cli tool contain a root directory
* 2.7 regression in GL performance on Mac (fyne-io/fyne#6010)
* GridWrap keyboard navigation does not change to next row at the end of previous row (fyne-io/fyne#5994)
* Image border radius not always shown on image fill cover (fyne-io/fyne#5980)
* Speed up file dialog rendering


## 2.7.0 - 16 Oct 2025

### Added

* Canvas types: Arc, Polygon, Square (and Rectangle.Aspect)
* "Cover" image fill
* Fully rounded corner radius ("pill" rectangle) & per-corner
* Image corner radius
* New containers: Navigation & Clip
* New Embedded driver (for running on non-standard drvices)
* RowWrap layout
* Add Generics to List and Tree for data binding
* Support for IPv6 addresses in URIs
* Add storage.RemoveAll to recursively delete from a repository
* Added Portuguese, Russian & Chinese (Simplified)
* Support left-tap systray to show window (SetSystemTrayWindow)
* Support JSON theme with fallback
* Add RichText bullet start number
* Option to always show Entry validation

### Changed

* Massive performance increases on rendering
* optimisations galore in data handling, custom themes and TextGrid
* Smooth infinite progress by reversing animation not just looping
* Numerous memory leaks and potential race conditions addressed

### Fixed

* Theme Override container cannot be used to override layout paddings (fyne-io/fyne#5019)
* Tree widget: First selected node stays highlighted after switching selection (fyne-io/fyne#5796)
* Brave browser: Web app via fyne serve blinks and closes , works correctly in Chrome (and Edge) (fyne-io/fyne#5705)
* layout.formLayout render canvas.Text as labels and values outside/off minSize area (fyne-io/fyne#5163)
* Android: EGL_BAD_SURFACE fails to redraw app after OpenFile dialog closes (fyne-io/fyne#3541)
* Tab and Shift +Tab not behaving as expected on folder dialog (fyne-io/fyne#5974)

### New Contributors

Code in v2.7.0 contains work from the following first time contributors:

* @kruitbosdotdev
* @cognusion
* @r3quie
* @redawl
* @generikvault
* @cwarden
* @Vinci10
* @cpustejovsky
* @xfaris
* @rad756
* @ystepanoff


## 2.6.3 - 21 August 2025

### Fixed

* Resolve compile issue with Go 1.25.0 caused by golang.org/x/tools conflict


## 2.6.2 - 28 July 2025

### Changed

* Added Czechoslovakia translations

### Fixed

* bounds check panic on undo after ctrl-deleting text on last line from MultiLineEntry (fyne-io/fyne#5714)
* Entry OnChanged does not get triggered on undo and redo (fyne-io/fyne#5710)
* SetText in TextGrid leaves trailing content (fyne-io/fyne#5722)
* Desktop app with system tray hangs on app.Quit (fyne-io/fyne#5724)
* CenterOnScreen Regression (fyne-io/fyne#5733)
* TextGrid CursorLocationForPosition reports wrong location when scrolled (fyne-io/fyne#5745)
* Language is always "en" on macOS (fyne-io/fyne#5760)
* TextGrid is glitchy when calling SetText on a scrolled container. (fyne-io/fyne#5762)
* When running for the second time in window.CenterOnScreen(), it will get stuck when running on the main thread (fyne-io/fyne#5766)
* Text entry widget backspace deletes two characters on Android (fyne-io/fyne#2774)
* Secondary windows is not refreshed correctly (fyne-io/fyne#5782)
* Clicking a button rapidly causes the click animation to extend outside of the button (fyne-io/fyne#5785)
* WASM cursor was ignored
* Corrected date format for Germany
* Hide() doesn't work at startup for widgets/containers (fyne-io/fyne#5597)
* Android GBoard: first character typed into Entry is prefixed with "0" after focus (fyne-io/fyne#5666)
* Use Scaled Monitor Size For Monitor Detection (fyne-io/fyne#5802)
* Don't override user choice if a) xdg lookup fails or b) system updates (fyne-io/fyne#5851)
* Entry with mobile.NumberKeyboard does not Type comma and separators (fyne-io/fyne#5101)
* Padding value is ignored in ThemeOverride container
* Performance improvements in TextGrid and object positioning
* Improvements in WASM rendering performance


## 2.6.1 - 8 May 2025

### Changed

 * Added Russian translations

### Fixed

 * Activity indicator is light and not visible when a light theme is active (fyne-io/fyne#5661)
 * Unsafe use of map in RichText on 2.6.0 (fyne-io/fyne#5639)
 * Image translucency causes blurriness on small icons (fyne-io/fyne#5476)
 * Infinite progress bar snapping and doesn't loop nicely (fyne-io/fyne#5433)
 * RichTextSegment SizeName is not SizeNameText by default (fyne-io/fyne#5307)
 * When there is an offline netdrive, the file dialog will freeze (fyne-io/fyne#2411)
 * Correctly reset image cache when Resource goes to nil
 * Data race after migration to v2.6.0 (fyne-io/fyne#5713)

## 2.6.0 - 10 April 2025

### Added

 * Added [fyne.Do] and [fyne.DoAndWait] to call from goroutines. This makes it possible to eliminate race conditions.
 * Add "Migrations" section to FyneApp.toml to mark migrations like `fyneDo = true`
 * Add Calendar and DateEntry widgets
 * Add a third state ([Check.Partial]) to the check widget (fyne-io/fyne#3576)
 * Add ability to select label text using new [Label.Selectable]
 * Support for storage on web driver (fyne-io/fyne#4634)
 * test: Add RenderToMarkup and RenderObjectToMarkup (fyne-io/fyne#5124)
 * Add ability to choose a text size for label widget (fyne-io/fyne#5561)
 * Show soft keyboard on Web build with mobile device (fyne-io/fyne#4394)
 * APIs for testing dialogs (fyne-io/fyne#2771)
 * Add `ScrollToOffset` functions to collection widgets
 * Add Prepend method to Accordion (fyne-io/fyne#5418)
 * Support Apple intel apps on M1/2 (using Rosetta) (fyne-io/fyne#3971)
 * Ability to turn off re-scaling when a window moves between monitors for Linux (fyne-io/fyne#5164)
 * Add functions to get text location for position (and vice-versa) with a TextGrid
 * Add support for scrolling many lines in TextGrid
 * Add `Append` function to TextGrid
 * Add `Prepend` function to Accordion
 * Support custom titles in file dialogs using `SetTitleText`
 * Add utility methods to handle colouring of SVG images
 * Add preference bind APIs for list (slice) types
 * Added Greek, Ukrainian & Chinese (Simplified) translations

### Changed

 * All callbacks from Fyne drivers and widgets now call on the same goroutine
 * Shortcuts on menu items are now called before widget or canvas shortcuts (fyne-io/fyne#2627)
 * ActionItems in an Entry should now match the standard button size
 * Tidy the fyne CLI and moved to tools repo (fyne-io/fyne#4920)
 * When scroll bar is expanded, clicking above or below the bar scrolls up or down (fyne-io/fyne#4922)
 * Add generics to data binding package
 * File picker now ignores case (fyne-io/fyne#5113)
 * Consistent callback order for dialogs - data before OnClosed
 * Improve drop-shadow to show light from top position
 * load markdown images from origin when not a URL
 * Debug now disabled by default for WASM builds
 * Updated theme of inner window borders with circle style and more customisations
 * Change Accordion.OpenAll when used with single-open accordion to open the first item instead of none

### Fixed

 * Fixed all known race conditions
 * Decouple clipboard from fyne.Window enhancement (fyne-io/fyne#4418)
 * Odd looking SelectEntry with long PlaceHolder (fyne-io/fyne#4430)
 * Crash when resizing mobile simulator window (fyne-io/fyne#5397)
 * Deadlock when creating widget too fast (fyne-io/fyne#3203)
 * Application crashes on .Resize() (fyne-io/fyne#5206)
 * Linux (ubuntu) menu shortcuts not working blocker (fyne-io/fyne#5355)
 * Slider snaps back to min-value on Android (fyne-io/fyne#5430)
 * SoftwareCanvas resize only works properly if it's the last step bug (fyne-io/fyne#5548)
 * Showing a disabled menu items with a non-SVG icon generates Fyne error bug (fyne-io/fyne#5557)
 * Trying to hide subsequently created popups in a goroutine results in Fyne runtime panic (fyne-io/fyne#5564)
 * Table passes negative index to onSelected function (fyne-io/fyne#4917)
 * storage.Move() fails to move directories (fyne-io/fyne#5493)
 * Tree and Table widgets refresh full content on any scroll or resize (fyne-io/fyne#5456)
 * Memory leak from widget renderers never being destroyed blocker (fyne-io/fyne#4903)
 * On MacOS SysTray menu does not show when clicked on an active space in second monitor (fyne-io/fyne#5223)
 * On MacOs systray tooltip does not show when full window app is active (fyne-io/fyne#5282)
 * Panic when opening and closing windows quickly bug (fyne-io/fyne#3280)
 * Goroutines showing same window at similar times can forget size races (fyne-io/fyne#4535)
 * Panic when confirming or dismissing file open dialog races (fyne-io/fyne#3279)
 * richImage may freeze the application in some cases. (fyne-io/fyne#3510)
 * Memory usage increases significantly per character in Entry (fyne-io/fyne#2969)
 * Submenus not working on mobile (fyne-io/fyne#5398)
 * ListWidget with data index out of bounds when modified bound data (fyne-io/fyne#5227)
 * After scrolling, first selection in a list jumps that item to the bottom of the container (fyne-io/fyne#5605)
 * Accordion could have incorrect layout with multiple items open
 * Prevent tapping within a popup from dismissing it, even if non-modal (fyne-io/fyne#5360)
 * Resolved performance issues in text and custom theme handling


## 2.5.5 - 13 March 2025

### Fixed

* Correct wasm build for Go 1.24 onwards


## 2.5.4 - 1 February 2025

### Changed

* Added Tamil translation

### Fixed

* Checkbox not responding to click because it is too "large"? (fyne-io/fyne#5331)
* Fix progressbar not showing label until first refresh
* FyneApp.toml causes BadLength error (fyne-io/fyne#5272)
* Test suite: failure with locale/language different from 'en' (fyne-io/fyne#5362)
* fix GridWrap crash when resizing to same size without creating renderer
* Submenus not working on mobile (fyne-io/fyne#5398)
* Subtle scrolling bug in List when the last two items are of different size (fyne-io/fyne#5281)
* File picker does not ignore case (fyne-io/fyne#5113)
* Tab "OnSelected" doesn't appear to allow focussing tab content (fyne-io/fyne#5454)
* Documentation fixes


## 2.5.3 - 15 December 2024

### Changed

* Smoothly decelerate scroll on mobile
* Added Spanish translation

### Fixed

* Starting location can be wrong in file dialogs with custom repository (fyne-io/fyne#5200)
* Improve how shortcut special keys for menu items are rendered on Windows and Linux (fyne-io/fyne#5108)
* Blank page in Chrome for Android
* Mobile Entry: cursor arrows don't work (fyne-io/fyne#5258)
* FileDialog does not handle relative file URIs well. (fyne-io/fyne#5234)
* [Linux] Only change variant when color scheme changes
* [Linux] Window with list flickers in Wayland (fyne-io/fyne#5133)
* Package command fails on OpenBSD (fyne-io/fyne#5195)
* System theme fallback is not working with custom themes
* Translucency and images with Alpha channel (fyne-io/fyne#1977)
* Performance regression when scrolling inside the file dialog (fyne-io/fyne#4307)
* Empty but visible images consume high CPU on 2.4.x (fyne-io/fyne#4345)
* Improved performance of text render caching
* nil pointer dereference in dialog.Resize() for color picker (fyne-io/fyne#5236)
* Tiny files written in iOS may be empty
* Some SVG resources don't update appearance correctly with the theme (fyne-io/fyne#3900)


## 2.5.2 - 15 October 2024

### Fixed

* Sometimes fyne fails to setup dark mode on Windows (fyne-io/fyne#4758)
* Memory leak in fontMetrics cache on desktop driver (fyne-io/fyne#4010)
* Fix possible crash with badly formatted json translation
* Tree widget doesn't display higher elements until a user selects one (fyne-io/fyne#5095, fyne-io/fyne#4346)
* Update to the latest breaking API changes in go-text
* Fix wrapping / truncation of multi-font text runs (fyne-io/fyne#4998)
* Test window title is not always set (fyne-io/fyne#5116)
* Deadlock in Button CreateRenderer() (fyne-io/fyne#5114)
* Fix possible crash in theme watching for windows
* Fix issue with Movies folder on darwin file dialog
* widget.Entry: validate when pasting from clipboard (fyne-io/fyne#5058, fyne-io/fyne#5028)
* Reduce contention in some widget locks
* Add Swedish translation
* Improvements to documentation
* Improved temp check for windows/msys
* Fix split in a theme override container


## 2.5.1 - 24 August 2024

### Fixed

 * Apps with translations in new languages would not be recognised (fyne-io/fyne#5015)
 * App ID can be ignored from metadata file with go build/run
 * Typing Chinese characters in widget.Entry and perform undo/redo crashes the app (fyne-io/fyne#5001)
 * Assets would render Low-DPI before attaching to a canvas
 * Single click in file dialog enters two directories (fyne-io/fyne#5053)
 * Light/Dark mode detection no longer works on Ubuntu with Fyne 2.5 (fyne-io/fyne#5029)
 * Scroll acceleration logic causes scrolling to "jump" suddenly on macOS (fyne-io/fyne#5067)
 * SetSystemTrayMenu doesn't work when run in goroutine (fyne-io/fyne#5039)
 * stack overflow when calling SetRowHeight in table UpdateCell callback (fyne-io/fyne#5007)
 * Resizing List causes visible items to refresh instead of resize (fyne-io/fyne#4080)
 * Child widget with Hoverable() interface keeps parent widget's Tapped() function from being called. (fyne-io/fyne#3906)
 * App Translation file is ignored / tries to load BR (fyne-io/fyne#5015, fyne-io/fyne#5040)
 * Missing theme variant auto-switching (dark/light) for Windows (fyne-io/fyne#4537)
 * Get DoubleTapDelay from the OS if an API is available (fyne-io/fyne#4448)
 * Entry cursor is not visible with animations off (fyne-io/fyne#4508) 
 * Redundant justify-content properties in CSS for centered-container class (fyne-io/fyne#5045)
 * Update go-text/render to avoid crashing when rendering certain bitmap fonts (fyne-io/fyne#5042)
 * Using container.NewThemeOverride leaks memory until window closing (fyne-io/fyne#5000)


## 2.5.0 - 14 July 2024

### Added

 * Internationalisation support and translations (fyne-io/fyne#605, fyne-io/fyne#3249)
 * Look up system fonts for glyphs that cannot be found embedded (fyne-io/fyne#2572, fyne-io/fyne#1579)
 * Completed support for Wayland on Linux
 * Completed support for the Web driver (*except file handling)
 * Add support for XDG Desktop Portals when built with `-tags flatpak` on Linux
 * Activity indicator widget
 * InnerWindow and MultipleWindows containers
 * ThemeOverride container for grouping items with a different theme
 * Add `NativeWindow.RunNative` to use a native window handle (fyne-io/fyne#4483)
 * Ability to request display stays on - `SetDisableScreenBlanking` (fyne-io/fyne#4534, fyne-io/fyne#3007)
 * Add Undo/Redo support for widget.Entry (fyne-io/fyne#436)
 * Add AppendMarkdown function to RichText
 * Add option in List to hide separators (fyne-io/fyne#3756)
 * New CustomPaddedLayout for more fine-grained container padding
 * Add SizeName property to Hyperlink widget
 * Support Ctrl+[backspace/delete] to delete the word to the left or right of the cursor
 * Add IntToFloat (and FloatToInt) in bindings Data binding (fyne-io/fyne#4666)
 * Add ScrollToOffset/GetScrollOffset APIs for collection widgets
 * Add ColumnCount API to GridWrap widget
 * Disable and Enable for Slider widget (fyne-io/fyne#3551)
 * Function `Remove` added to List bindings (fyne-io/fyne#3100)
 * Form layout can now be vertical (labels above field) or adaptive for mobile
 * Add support for Bold, Italic and Underline for TextGrid (fyne-io/fyne#1237)
 * Add support for setting a custom resource as the font source for text (fyne-io/fyne#808)
 * New `test` functions `NewTempApp`, `NewTempWindow` and `TempWidgetRenderer` to free resources automatically

### Changed

 * Fyne now depends on Go 1.19 at a minimum
 * Round the corners of scroll bars with new theme value
 * Improve contrast of text on highlight background colours
 * Layout of iOS and Android apps will adapt when keyboard appears (fyne-io/fyne#566, fyne-io/fyne#2371)
 * FyneApp.toml will now be loaded with `go build` (fyne-io/fyne#4688)
 * Text wrapping will now wrap in dialogs (fyne-io/fyne#2602)
 * System tray and tray menu icons on will now match theme on macOS (fyne-io/fyne#4549)
 * Triple clicking in an Entry widget now selects current line (fyne-io/fyne#4328)
 * About menu items will now override the macOS default About
 * System tray no longer shows tooltips (until we have full support)
 * Double tapping an item in a file dialog now selects and returns
 * Widgets should now use `theme.ForWidget()` instead of `theme.Default()` or static helpers

### Fixed

 * Kannada characters not rendering correctly (fyne-io/fyne#2654)
 * Notifications are not working on iOS (fyne-io/fyne#4966)
 * Incorrect scaling on Steam Deck screen Accessibility (fyne-io/fyne#4896)
 * Sometimes the last list row that should be visible doesn't show (fyne-io/fyne#4909)
 * RichText swallowing whitespace after Markdown links (fyne-io/fyne#4613, fyne-io/fyne#4340)
 * Disabled app tabs can still be selected in popup menu (fyne-io/fyne#4935)
 * Don't show title when mouse hover on Systray menu (fyne-io/fyne#4916)
 * Trying to access a URL through canvas.NewImageFromURI() in a test results in a panic (fyne-io/fyne#4863)
 * Don't insert tab character in Entry when Shift+Tab typed
 * Select Does Not Gain Focus When Tapped (fyne-io/fyne#4767)
 * binding.Untyped crashes when set to nil bug (fyne-io/fyne#4807)
 * Label and Slider not aligned in a FormItem (fyne-io/fyne#4714)
 * Windows: App Icon in Notification (fyne-io/fyne#2592)
 * Fix possible writing of empty preference data in some quit events
 * Allow application to set default view (list/grid) of file dialog before showing it (fyne-io/fyne#4595)
 * Fix ScrollToOffset when viewport is larger than content size
 * Incorrect row header width in widget.Table (fyne-io/fyne#4370)
 * Add missed truncation mode for hyperlink (fyne-io/fyne#4335)
 * AppTab does not display blue indicator line if you create it empty and then Append items to it later.
 * Many optimisations in animation, draw speed, layout and widget size calculations
 * DocTabItem update text doesn't update the underline select bar (graphic glitch) (fyne-io/fyne#3106)


## 2.4.5 - 15 April 2024

### Fixed

* iOS files write would fail when over 16KB
* storage.Delete not supported on Android/iOS (fyne-io/fyne#2120)
* layout.formLayout do not handle canvas.Text well in second column (fyne-io/fyne#4665)
* Fix building with ios17.4 (fyne-io/fyne#4741)
* Support template icon for system tray menu icons
* Fix recognition of missing XDG user directories (fyne-io/fyne#4650)
* FileDialog.SetOnClosed not always working (fyne-io/fyne#4651)
* Upgrade GLFW for performance improvements and bug fixes
* Multiple select popups can crash during background operations (fyne-io/fyne#4730)
* Controlling a negative slider with the left arrow key blocks after 8 steps (fyne-io/fyne#4736)
* cmd/fyne: command "get" is broken with Go 1.22 (fyne-io/fyne#4684)
* Race condition during system tray menu refresh (fyne-io/fyne#4697)
* Fyne release on Linux does not set Metadata().Release to true (fyne-io/fyne#4711)
* RichText leaks memory when replacing segments (fyne-io/fyne#4723)


## 2.4.4 - 13 February 2024

### Fixed

* Spaces could be appended to linux Exec command during packaging
* Secondary mobile windows would not size correctly when padded
* Setting Icon.Resource to nil will not clear rendering
* Dismiss iOS keyboard if "Done" is tapped
* Large speed improvement in Entry and GridWrap widgets
* tests fail with macOS Assertion failure in NSMenu (fyne-io/fyne#4572)
* Fix image test failures on Apple Silicon
* High CPU use when showing CustomDialogs (fyne-io/fyne#4574)
* Entry does not show the last (few) changes when updating a binding.String in a fast succession (fyne-io/fyne#4082)
* Calling Entry.SetText and then Entry.Bind immediately will ignore the bound value (fyne-io/fyne#4235)
* Changing theme while application is running doesn't change some parameters on some widgets (fyne-io/fyne#4344)
* Check widget: hovering/tapping to the right of the label area should not activate widget (fyne-io/fyne#4527)
* Calling entry.SetPlaceHolder inside of OnChanged callback freezes app (fyne-io/fyne#4516)
* Hyperlink enhancement: underline and tappable area shouldn't be wider than the text label (fyne-io/fyne#3528)
* Fix possible compile error from go-text/typesetting


## 2.4.3 - 23 December 2023

### Fixed

* Fix OpenGL init for arm64 desktop devices 
* System tray icon on Mac is showing the app ID (fyne-io/fyne#4416)
* Failure with fyne release -os android/arm (fyne-io/fyne#4174)
* Android GoBack with forcefully close the app even if the keyboard is up (fyne-io/fyne#4257)
* *BSD systems using the wrong (and slow) window resize
* Optimisations to reduce memory allocations in List, GridWrap, driver and mime type handling
* Reduce calls to C and repeated size checks in painter and driver code


## 2.4.2 - 22 November 2023

### Fixed

* Markdown only shows one horizontal rule (fyne-io/fyne#4216)
* Spacer in HBox with hidden item will cause an additional trailing padding (fyne-io/fyne#4259)
* Application crash when fast clicking the folders inside the file dialog (fyne-io/fyne#4260)
* failed to initialise OpenGL (fyne-io/fyne#437)
* App panic when clicking on a notification panel if there's a systray icon (fyne-io/fyne#4385)
* Systray cannot be shown on Ubuntu (fyne-io/fyne#3678, fyne-io/fyne#4381)
* failed to initialise OpenGL on Windows dual-chip graphics cards (fyne-io/fyne#437)
* Reduce memory allocations for each frame painted
* RichText may not refresh if segments manually replaced
* Correct URI.Extension() documentation
* Update for security fixes to x/sys and x/net
* Inconsistent rendering of Button widget (fyne-io/fyne#4243)
* PasswordEntry initial text is not obscured (fyne-io/fyne#4312)
* Pasting text in Entry does not update cursor position display (fyne-io/fyne#4181)


## 2.4.1 - 9 October 2023

### Fixed

* Left key on tree now collapses open branch
* Avoid memory leak in Android driver code
* Entry Field on Android in Landscape Mode Shows "0" (fyne-io/fyne#4036)
* DocTabs Indicator remains visible after last tab is removed (fyne-io/fyne#4220)
* Fix mobile simulation builds on OpenBSD
* Fix alignment of menu button on mobile
* Fix Compilation with Android NDK r26
* Clicking table headers causes high CPU consumption (fyne-io/fyne#4264)
* Frequent clicking on table may cause the program to not respond (fyne-io/fyne#4210)
* Application stops responding when scrolling a table (fyne-io/fyne#4263)
* Possible crash parsing malformed JSON color (fyne-io/fyne#4270)
* NewFolderOpen: incomplete filenames (fyne-io/fyne#2165)
* Resolve issue where storage.List could crash with short URI (fyne-io/fyne#4271)
* TextTruncateEllipsis abnormally truncates strings with multi-byte UTF-8 characters (fyne-io/fyne#4283)
* Last character doesn't appear in Select when there is a special character (fyne-io/fyne#4293)
* Resolve random crash in DocTab (fyne-io/fyne#3909)
* Selecting items from a list caused the keyboard to popup on Android (fyne-io/fyne#4236)


## 2.4.0 - 1 September 2023

### Added

* Rounded corners in rectangle (fyne-io/fyne#1090)
* Support for emoji in text
* Layout debugging (with `-tags debug` build flag) (fyne-io/fyne#3314)
* GridWrap collection widget
* Add table headers (fyne-io/fyne#1658, fyne-io/fyne#3594)
* Add mobile back button handling (fyne-io/fyne#2910)
* Add option to disable UI animations (fyne-io/fyne#1813)
* Text truncation ellipsis (fyne-io/fyne#1659)
* Add support for binding tree data, include new `NewTreeWithData`
* Add support for OpenType fonts (fyne-io/fyne#3245)
* Add `Window.SetOnDropped` to handle window-wide item drop on desktop
* Add lists to the types supported by preferences API
* Keyboard focus handling for all collection widgets
* Add APIs for refreshing individual items in collections (fyne-io/fyne#3826)
* Tapping slider moves it to that position (fyne-io/fyne#3650)
* Add `OnChangeEnded` callback to `Slider` (fyne-io/fyne#3652)
* Added keyboard controls to `Slider`
* Add `NewWarningThemedResource` and `NewSuccessThemedResource` along with `NewColoredResource` (fyne-io/fyne#4040)
* Custom hyperlink callback for rich text hyperlinks (fyne-io/fyne#3335)
* Added `dialog.NewCustomWithoutButtons`, with a `SetButtons` method (fyne-io/fyne#2127, fyne-io/fyne#2782)
* Added `SetConfirmImportance` to `dialog.ConfirmDialog`.
* Added `FormDialog.Submit()` to close and submit the dialog if validation passes
* Rich Text image alignment (fyne-io/fyne#3810)
* Bring back `theme.HyperlinkColor` (fyne-io/fyne#3867)
* Added `Importance` field on `Label` to color the text
* Navigating in entry quickly with ctrl key (fyne-io/fyne#2462)
* Support `.desktop` file metadata in `FyneApp.toml` for Linux and BSD
* Support mobile simulator on FreeBSD
* Add data binding boolean operators `Not`, `And` and `Or`
* Added `Entry.Append`, `Select.SetOptions`, `Check.SetText`, `FormDialog.Submit`
* Add `ShowPopUpAtRelativePosition` and `PopUp.ShowAtRelativePosition`
* Add desktop support to get key modifiers with `CurrentKeyModifiers`
* Add geometry helpers `NewSquareSize` and `NewSquareOffsetPos`
* Add `--pprof` option to fyne build commands to enable profiling
* Support compiling from Android (termux)

### Changed

* Go 1.17 or later is now required.
* Theme updated for rounded corners on buttons and input widgets
* `widget.ButtonImportance` is now `widget.Importance`
* The `Max` container and layout have been renamed `Stack` for clarity
* Refreshing an image will now happen in app-thread not render process, apps may wish to add async image load
* Icons for macOS bundles are now padded and rounded, disable with "-use-raw-icon" (fyne-io/fyne#3752)
* Update Android target SDK to 33 for Play Store releases
* Focus handling for List/Tree/Table are now at the parent widget not child elements
* Accordion widget now fills available space - put it inside a `VBox` container for old behavior (fyne-io/fyne#4126)
* Deprecated theme.FyneLogo() for later removal (fyne-io/fyne#3296)
* Improve look of menu shortcuts (fyne-io/fyne#2722)
* iOS and macOS packages now default to using "XCWildcard" provisioning profile
* Improving performance of lookup for theme data
* Improved application startup time

### Fixed

* Rendering performance enhancements
* `dialog.NewProgressInfinite` is deprecated, but dialog.NewCustom isn't equivalent
* Mouse cursor desync with Split handle when dragging (fyne-io/fyne#3791)
* Minor graphic glitch with checkbox (fyne-io/fyne#3792)
* binding.String===>Quick refresh *b.val will appear with new data reset by a call to OnChange (fyne-io/fyne#3774)
* Fyne window becomes unresponsive when in background for a while (fyne-io/fyne#2791)
* Hangs on repeated calls to `Select.SetSelected` in table. (fyne-io/fyne#3684)
* `Select` has wrong height, padding and border (fyne-io/fyne#4142)
* `widget.ImageSegment` can't be aligned. (fyne-io/fyne#3505)
* Memory leak in font metrics cache (fyne-io/fyne#4108)
* Don't panic when loading preferences with wrong type (fyne-io/fyne#4039)
* Button with icon has wrong padding on right (fyne-io/fyne#4124)
* Preferences don't all save when written in `CloseIntercept` (fyne-io/fyne#3170)
* Text size does not update in Refresh for TextGrid
* DocTab selection underline not updated when deleting an Item (fyne-io/fyne#3905)
* Single line Entry throws away selected text on submission (fyne-io/fyne#4026)
* Significantly improve performance of large `TextGrid` and `Tree` widgets
* `List.ScrollToBottom` not scrolling to show the totality of the last Item (fyne-io/fyne#3829)
* Setting `Position1` of canvas.Circle higher than `Position2` causes panic. (fyne-io/fyne#3949)
* Enhance scroll wheel/touchpad scroll speed on desktop (fyne-io/fyne#3492)
* Possible build issue on Windows with app metadata
* `Form` hint text has confusing padding to next widget (fyne-io/fyne#4137)
* `Entry` Placeholder Style Only Applied On Click (fyne-io/fyne#4035)
* Backspace and Delete key Do not Fire OnChanged Event (fyne-io/fyne#4117)
* Fix `ProgressBar` text having the wrong color sometimes
* Window doesn't render when called for the first time from system tray and the last window was closed (fyne-io/fyne#4163)
* Possible race condition in preference change listeners
* Various vulnerabilities resolved through updating dependencies 
* Wrong background for color dialog (fyne-io/fyne#4199)


## 2.3.5 - 6 June 2023

### Fixed

* Panic with unsupported font (fyne-io/fyne#3646)
* Temporary manifest file not closed after building on Windows
* Panic when using autogenerated quit menu and having unshown windows (fyne-io/fyne#3870)
* Using `canvas.ImageScaleFastest` not working on arm64 (fyne-io/fyne#3891)
* Disabled password Entry should also disable the ActionItem (fyne-io/fyne#3908)
* Disabled RadioGroup does not display status (fyne-io/fyne#3882)
* Negative TableCellID Row (fyne-io/fyne#2857)
* Make sure we have sufficient space for the bar as well if content is tiny (fyne-io/fyne#3898)
* Leak in image painter when replacing image.Image source regularly
* Links in Markdown/Rich Text lists breaks formatting (fyne-io/fyne#2911)
* Crash when reducing window to taskbar with popup opened (fyne-io/fyne#3877)
* RichText vertical scroll will truncate long content with horizontal lines (fyne-io/fyne#3929)
* Custom metadata would not apply with `fyne release` command
* Horizontal CheckGroup overlap when having long text (fyne-io/fyne#3005)
* Fix focused colour of coloured buttons (fyne-io/fyne#3462)
* Menu separator not visible with light theme (fyne-io/fyne#3814)


## 2.3.4 - 3 May 2023

### Fixed

* Memory leak when switching theme (fyne-io/fyne#3640)
* Systray MenuItem separators not rendered in macOS root menu (fyne-io/fyne#3759)
* Systray leaks window handles on Windows (fyne-io/fyne#3760)
* RadioGroup miscalculates label widths in horizontal mode (fyne-io/fyne#3386)
* Start of selection in entry is shifted when moving too fast (fyne-io/fyne#3804)
* Performance issue in widget.List (fyne-io/fyne#3816)
* Moving canvas items (e.g. Images) does not cause canvas repaint (fyne-io/fyne#2205)
* Minor graphic glitch with checkbox (fyne-io/fyne#3792)
* VBox and HBox using heap memory that was not required
* Menu hover is slow on long menus

## 2.3.3 - 24 March 2023

### Fixed

* Linux, Windows and BSD builds could fail if gles was missing


## 2.3.2 - 20 March 2023

### Fixed

* Fyne does not run perfectly on ARM-based MacOS platforms (fyne-io/fyne#3639) *
* Panic on closing window in form submit on Мac M2 (fyne-io/fyne#3397) *
* Wobbling slider effect for very small steps (fyne-io/fyne#3648)
* Fix memory leak in test canvas refresh
* Optimise text texture memory by switching to single channel
* Packaging an android fyne app that uses tags can fail (fyne-io/fyne#3641)
* NewAdaptiveGrid(0) blanks app window on start until first resize on Windows (fyne-io/fyne#3669)
* Unnecessary refresh when sliding Split container
* Linux window resize refreshes all content
* Themed and unthemed svg resources can cache collide
* When packaging an ampersand in "Name" causes an error (fyne-io/fyne#3195)
* Svg in ThemedResource without viewBox does not match theme (fyne-io/fyne#3714)
* Missing menu icons in Windows system tray
* Systray Menu Separators don't respect the submenu placement (fyne-io/fyne#3642)
* List row focus indicator disappears on scrolling (fyne-io/fyne#3699)
* List row focus not reset when row widget is reused to display a new item (fyne-io/fyne#3700)
* Avoid panic if accidental 5th nil is passed to Border container
* Mobile simulator not compiling on Apple M1/2
* Cropped letters in certain cases with the new v2.3.0 theme (fyne-io/fyne#3500)

Many thanks indeed to [Dymium](https://dymium.io) for sponsoring an Apple
M2 device which allowed us to complete the marked (*) issues.


## 2.3.1 - 13 February 2023

### Changed

* Pad app version to ensure Windows packages correctly (fyne-io/fyne#3638)

### Fixed

* Custom shortcuts with fyne.KeyTab is not working (fyne-io/fyne#3087)
* Running a systray app with root privileges resulted in panic (fyne-io/fyne#3120)
* Markdown image with no title is not parsed (fyne-io/fyne#3577)
* Systray app on macOS panic when started while machine sleeps (fyne-io/fyne#3609)
* Runtime error with VNC on RaspbianOS (fyne-io/fyne#2972)
* Hovered background in List widget isn't reset when scrolling reuses an existing list item (fyne-io/fyne#3584)
* cmd/fyne package can't find FyneApp.toml when -src option has given (fyne-io/fyne#3459)
* TextWrapWord will cause crash in RichText unverified (fyne-io/fyne#3498)
* crash in widget.(*RichText).lineSizeToColumn (fyne-io/fyne#3292)
* Crash in widget.(*Entry).SelectedText (fyne-io/fyne#3290)
* Crash in widget.(*RichText).updateRowBounds.func1 (fyne-io/fyne#3291)
* window is max size at all times (fyne-io/fyne#3507)
* systray.Quit() is not called consistently when the app is closing (fyne-io/fyne#3597)
* Software rendering would ignore scale for text
* crash when minimize a window which contains a stroked rectangle (fyne-io/fyne#3552)
* Menu item would not appear disabled initially
* Wrong icon colour for danger and warning buttons
* Embedding Fyne apps in iFrame alignment issue
* Generated metadata can be in wrong directory
* Android RootURI may not exist when used for storage (fyne-io/fyne#3207)


## 2.3.0 - 24 December 2022

### Added

* Shiny new theme that was designed for us
* Improved text handling to support non-latin alphabets
* Add cloud storage and preference support
* Add menu icon and submenu support to system tray menus
* More button importance levels `ErrorImportance`, `WarningImportance`
* Support disabling of `AppTabs` and `DocTabs` items
* Add image support to rich text (fyne-io/fyne#2366)
* Add CheckGroup.Remove (fyne-io/fyne#3124)

### Changed

* The buttons on the default theme are no longer transparent, but we added more button importance types
* Expose a storage.ErrNotExists for non existing documents (fyne-io/fyne#3083)
* Update `go-gl/glfw` to build against latest Glfw 3.3.8
* List items in `widget.List` now implement the Focusable interface

### Fixed

* Displaying unicode or different language like Bengali doesn't work (fyne-io/fyne#598)
* Cannot disable container.TabItem (fyne-io/fyne#1904)
* Update Linux/XDG application theme to follow the FreeDesktop Dark Style Preference (fyne-io/fyne#2657)
* Running `fyne package -os android` needs NDK 16/19c (fyne-io/fyne#3066)
* Caret position lost when resizing a MultilineEntry (fyne-io/fyne#3024)
* Fix possible crash in table resize (fyne-io/fyne#3369)
* Memory usage surge when selecting/appending MultilineEntry text (fyne-io/fyne#3426)
* Fyne bundle does not support appending when parameter is a directory
* Crash parsing invalid file URI (fyne-io/fyne#3275)
* Systray apps on macOS can only be terminated via the systray menu quit button (fyne-io/fyne#3395)
* Wayland Scaling support: sizes and distances are scaled wrong (fyne-io/fyne#2850)
* Google play console minimum API level 31 (fyne-io/fyne#3375)
* Data bound entry text replacing selection is ignored (fyne-io/fyne#3340)
* Split Container does not respect item's Visible status (fyne-io/fyne#3232)
* Android - Entry - OnSubmitted is not working (fyne-io/fyne#3267)
* Can't set custom CGO_CFLAGS and CGO_LDFLAGS with "fyne package" on darwin (fyne-io/fyne#3276)
* Text line not displayed in RichText (fyne-io/fyne#3117)
* Segfault when adding items directly in form struct (fyne-io/fyne#3153)
* Preferences RemoveValue does not save (fyne-io/fyne#3229)
* Create new folder directly from FolderDialog (fyne-io/fyne#3174)
* Slider drag handle is clipped off at minimum size (fyne-io/fyne#2966)
* Entry text "flickering" while typing (fyne-io/fyne#3461)
* Rendering of not changed canvas objects after an event (fyne-io/fyne#3211)
* Form dialog not displaying hint text and validation errors (fyne-io/fyne#2781)


## 2.2.4 - 9 November 2022

### Fixes

* Iphone incorrect click coordinates in zoomed screen view (fyne-io/fyne#3122)
* CachedFontFace seems to be causing crash (fyne-io/fyne#3134)
* Fix possible compile error if "fyne build" is used without icon metadata
* Detect and use recent Android NDK toolchain
* Handle fyne package -release and fyne release properly for Android and iOS
* Fix issue with mobile simulation when systray used
* Fix incorrect size and position for radio focus indicator (fyne-io/fyne#3137)


## 2.2.3 - 8 July 2022

### Fixed

* Regression: Preferences are not parsed at program start (fyne-io/fyne#3125)
* Wrappable RichText in a Split container causes crash (fyne-io/fyne#3003, fyne-io/fyne#2961)
* meta.Version is always 1.0.0 on android & ios (fyne-io/fyne#3109)


## 2.2.2 - 30 June 2022

### Fixed

* Windows missing version metadata when packaged (fyne-io/fyne#3046)
* Fyne package would not build apps using old Fyne versions
* System tray icon may not be removed on app exit in Windows
* Emphasis in Markdown gives erroneous output in RichText (fyne-io/fyne#2974)
* When last visible window is closed, hidden window is set visible (fyne-io/fyne#3059)
* Do not close app when last window is closed but systrayMenu exists (fyne-io/fyne#3092)
* Image with ImageFillOriginal not showing (fyne-io/fyne#3102)


## 2.2.1 - 12 June 2022

### Fixed

* Fix various race conditions and compatibility issues with System tray menus
* Resolve issue where macOS systray menu may not appear
* Updated yaml dependency to fix CVE-2022-28948
* Tab buttons stop working after removing a tab (fyne-io/fyne#3050)
* os.SetEnv("FYNE_FONT") doesn't work in v2.2.0 (fyne-io/fyne#3056)


## 2.2.0 - 7 June 2022

### Added

* Add SetIcon method on ToolbarAction (fyne-io/fyne#2475)
* Access compiled app metadata using new `App.Metadata()` method
* Add support for System tray icon and menu (fyne-io/fyne#283)
* Support for Android Application Bundle (.aab) (fyne-io/fyne#2663)
* Initial support for OpenBSD and NetBSD
* Add keyboard shortcuts to menu (fyne-io/fyne#682)
* Add technical preview of web driver and `fyne serve` command
* Added `iossimulator` build target (fyne-io/fyne#1917)
* Allow dynamic themes via JSON templates (fyne-io/fyne#211)
* Custom hyperlink callback (fyne-io/fyne#2979)
* Add support for `.ico` file when compiling for windows (fyne-io/fyne#2412)
* Add binding.NewStringWithFormat (fyne-io/fyne#2890)
* Add Entry.SetMinRowsVisible
* Add Menu.Refresh() and MainMenu.Refresh() (fyne-io/fyne#2853)
* Packages for Linux and BSD now support installing into the home directory
* Add `.RemoveAll()` to containers
* Add an AllString validator for chaining together string validators

### Changed

* Toolbar item constructors now return concrete types instead of ToolbarItem
* Low importance buttons no longer draw button color as a background
* ProgressBar widget height is now consistent with other widgets
* Include check in DocTabs menu to show current tab
* Don't call OnScrolled if offset did not change (fyne-io/fyne#2646)
* Prefer ANDROID_NDK_HOME over the ANDROID_HOME ndk-bundle location (fyne-io/fyne#2920)
* Support serialisation / deserialisation of the widget tree (fyne-io/fyne#5)
* Better error reporting / handling when OpenGL is not available (fyne-io/fyne#2689)
* Memory is now better reclaimed on Android when the OS requests it
* Notifications on Linux and BSD now show the application icon
* Change listeners for preferences no longer run when setting the same value
* The file dialog now shows extensions in the list view for better readability
* Many optimisations and widget performance enhancements
* Updated various dependencies to their latest versions

### Fixed

* SendNotification does not show app name on Windows (fyne-io/fyne#1940)
* Copy-paste via keyboard don't work translated keyboard mappings on Windows (fyne-io/fyne#1220)
* OnScrolled triggered when offset hasn't changed (fyne-io/fyne#1868)
* Carriage Return (\r) is rendered as space (fyne-io/fyne#2456)
* storage.List() returns list with nil elements for empty directories (fyne-io/fyne#2858)
* Entry widget, position of cursor when clicking empty space (fyne-io/fyne#2877)
* SelectEntry cause UI hang (fyne-io/fyne#2925)
* Font cutoff with bold italics (fyne-io/fyne#3001)
* Fyne error: Preferences load error (fyne-io/fyne#2936, 3015)
* Scrolled List bad redraw when window is maximized (fyne-io/fyne#3013)
* Linux and BSD packages not being installable if the name contained spaces


## 2.1.4 - 17 March 2022

### Fixed

* SetTheme() is not fully effective for widget.Form (fyne-io/fyne#2810)
* FolderOpenDialog SetDismissText is ineffective (fyne-io/fyne#2830)
* window.Resize() does not work if SetFixedSize(true) is set after (fyne-io/fyne#2819)
* Container.Remove() race causes crash (fyne-io/fyne#2826, fyne-io/fyne#2775, fyne-io/fyne#2481)
* FixedSize Window improperly sized if contains image with ImageFillOriginal (fyne-io/fyne#2800)


## 2.1.3 - 24 February 2022

### Fixed

* The text on button can't be show correctly when use imported font (fyne-io/fyne#2512)
* Fix issues with DocTabs scrolling (fyne-io/fyne#2709)
* Fix possible crash for tapping extended Radio or Check item
* Resolve lookup of relative icons in FyneApp.toml
* Window not shown when SetFixedSize is used without Resize (fyne-io/fyne#2784)
* Text and links in markdown can be rendered on top of each other (fyne-io/fyne#2695)
* Incorrect cursor movement in a multiline entry with wrapping (fyne-io/fyne#2698)


## 2.1.2 - 6 December 2021

### Fixed

* Scrolling list bound to data programmatically causes nil pointer dereference (fyne-io/fyne#2549)
* Rich text from markdown can get newlines wrong (fyne-io/fyne#2589)
* Fix crash on 32bit operating systems (fyne-io/fyne#2603)
* Compile failure on MacOS 10.12 Sierra (fyne-io/fyne#2478)
* Don't focus widgets on mobile where keyboard should not display (fyne-io/fyne#2598)
* storage.List doesn't return complete URI on Android for "content:" scheme (fyne-io/fyne#2619)
* Last word of the line and first word of the next line are joined in markdown parse (fyne-io/fyne#2647)
* Support for building `cmd/fyne` on Windows arm64
* Fixed FreeBSD requiring installed glfw library dependency (fyne-io/fyne#1928)
* Apple M1: error when using mouse drag to resize window (fyne-io/fyne#2188)
* Struct binding panics in reload with slice field (fyne-io/fyne#2607)
* File Dialog favourites can break for certain locations (fyne-io/fyne#2595)
* Define user friendly names for Android Apps (fyne-io/fyne#2653)
* Entry validator not updating if content is changed via data binding after SetContent (fyne-io/fyne#2639)
* CenterOnScreen not working for FixedSize Window (fyne-io/fyne#2550)
* Panic in boundStringListItem.Get() (fyne-io/fyne#2643)
* Can't set an app/window icon to be an svg. (fyne-io/fyne#1196)
* SetFullScreen(false) can give error (fyne-io/fyne#2588)


## 2.1.1 - 22 October 2021

### Fixed

* Fix issue where table could select cells beyond data bound
* Some fast taps could be ignored (fyne-io/fyne#2484)
* iOS app stops re-drawing mid-frame after a while (fyne-io/fyne#950)
* Mobile simulation mode did not work on Apple M1 computers
* TextGrid background color can show gaps in render (fyne-io/fyne#2493)
* Fix alignment of files in list view of file dialog
* Crash setting visible window on macOS to fixed size (fyne-io/fyne#2488)
* fyne bundle ignores -name flag in windows (fyne-io/fyne#2395)
* Lines with nil colour would crash renderer
* Android -nm tool not found with NDK 23 (fyne-io/fyne#2498)
* Runtime panic because out of touchID (fyne-io/fyne#2407)
* Long text in Select boxes overflows out of the box (fyne-io/fyne#2522)
* Calling SetText on Label may not refresh correctly
* Menu can be triggered by # key but not always Alt
* Cursor position updates twice with delay (fyne-io/fyne#2525)
* widgets freeze after being in background and then a crash upon pop-up menu (fyne-io/fyne#2536)
* too many Refresh() calls may now cause visual artifacts in the List widget (fyne-io/fyne#2548)
* Entry.SetText may panic if called on a multiline entry with selected text (fyne-io/fyne#2482)
* TextGrid not always drawing correctly when resized (fyne-io/fyne#2501)


## 2.1.0 - 17 September 2021

### Added

* DocTabs container for handling multiple open files
* Lifecycle API for handling foreground, background and other event
* Add RichText widget and Markdown parser
* Add TabWidth to TextStyle to specify tab size in spaces
* Add CheckGroup widget for multi-select
* Add FyneApp.toml metadata file to ease build commands
* Include http and https in standard repositories
* Add selection color to themes
* Include baseline information in driver font measurement
* Document storage API (App.Storage().Create() and others)
* Add "App Files" to file dialog for apps that use document storage
* Tab overflow on AppTabs
* Add URI and Unbound type to data bindings
* Add keyboard support for menus, pop-ups and buttons
* Add SimpleRenderer to help make simple widgets (fyne-io/fyne#709)
* Add scroll functions for List, Table, Tree (fyne-io/fyne#1892)
* Add selection and disabling to MenuItem
* Add Alignment to widget.Select (fyne-io/fyne#2329)
* Expose ScanCode for keyboard events originating from hardware (fyne-io/fyne#1523)
* Support macOS GPU switching (fyne-io/fyne#2423)

### Changed

* Focusable widgets are no longer focused on tap, add canvas.Focus(obj) in Tapped handler if required
* Move to background based selection for List, Table and Tree
* Update fyne command line tool to use --posix style parameters
* Switch from gz to xz compression for unix packages
* Performance improvements with line, text and raster rendering
* Items not yet visible can no longer be focused
* Lines can now be drawn down to 1px (instead of 1dp) (fyne-io/fyne#2298)
* Support multiple lines of text on button (fyne-io/fyne#2378)
* Improved text layout speed by caching string size calculations
* Updated to require Go 1.14 so we can use some new features
* Window Resize request is now asynchronous
* Up/Down keys take cursor home/end when on first/last lines respectively

### Fixed

* Correctly align text tabs (fyne-io/fyne#1791)
* Mobile apps theme does not match system (fyne-io/fyne#472)
* Toolbar with widget.Label makes the ToolbarAction buttons higher (fyne-io/fyne#2257)
* Memory leaks in renderers and canvases cache maps (fyne-io/fyne#735)
* FileDialog SetFilter does not work on Android devices (fyne-io/fyne#2353)
* Hover fix for List and Tree with Draggable objects
* Line resize can flip slope (fyne-io/fyne#2208)
* Deadlocks when using widgets with data (fyne-io/fyne#2348)
* Changing input type with keyboard visible would not update soft keyboards
* MainMenu() Close item does NOT call function defined in SetCloseIntercept (fyne-io/fyne#2355)
* Entry cursor position with mouse is offset vertically by theme.SizeNameInputBorder (fyne-io/fyne#2387)
* Backspace key is not working on Android AOSP (fyne-io/fyne#1941)
* macOS: 'NSUserNotification' has been deprecated (fyne-io/fyne#1833)
* macOS: Native menu would add new items if refreshed
* iOS builds fail since Go 1.16
* Re-add support for 32 bit iOS devices, if built with Go 1.14
* Android builds fail on Apple M1 (fyne-io/fyne#2439)
* SetFullScreen(true) before ShowAndRun fails (fyne-io/fyne#2446)
* Interacting with another app when window.SetFullScreen(true) will cause the application to hide itself. (fyne-io/fyne#2448)
* Sequential writes to preferences does not save to file (fyne-io/fyne#2449)
* Correct Android keyboard handling (fyne-io/fyne#2447)
* MIUI-Android: The widget’s Hyperlink cannot open the URL (fyne-io/fyne#1514)
* Improved performance of data binding conversions and text MinSize


## 2.0.4 - 6 August 2021

### Changed

* Disable Form labels when the element it applys to is disabled (fyne-io/fyne#1530)
* Entry popup menu now fires shortcuts so extended widgets can intercept
* Update Android builds to SDK 30

### Fixed

* sendnotification show appID for name on windows (fyne-io/fyne#1940)
* Fix accidental removal of windows builds during cross-compile
* Removing an item from a container did not update layout
* Update title bar on Windows 10 to match OS theme (fyne-io/fyne#2184)
* Tapped triggered after Drag (fyne-io/fyne#2235)
* Improved documentation and example code for file dialog (fyne-io/fyne#2156)
* Preferences file gets unexpectedly cleared (fyne-io/fyne#2241)
* Extra row dividers rendered on using SetColumnWidth to update a table (fyne-io/fyne#2266)
* Fix resizing fullscreen issue
* Fullscreen changes my display resolution when showing a dialog (fyne-io/fyne#1832)
* Entry validation does not work for empty field (fyne-io/fyne#2179)
* Tab support for focus handling missing on mobile
* ScrollToBottom not always scrolling all the way when items added to container.Scroller
* Fixed scrollbar disappearing after changing content (fyne-io/fyne#2303)
* Calling SetContent a second time with the same content will not show
* Drawing text can panic when Color is nil (fyne-io/fyne#2347)
* Optimisations when drawing transparent rectangle or whitespace strings


## 2.0.3 - 30 April 2021

### Fixed

* Optimisations for TextGrid rendering
* Data binding with widget.List sometimes crash while scrolling (fyne-io/fyne#2125)
* Fix compilation on FreeBSD 13
* DataLists should notify only once when change.
* Keyboard will appear on Android in disabled Entry Widget (fyne-io/fyne#2139)
* Save dialog with filename for Android
* form widget can't draw hinttext of appended item. (fyne-io/fyne#2028)
* Don't create empty shortcuts (fyne-io/fyne#2148)
* Install directory for windows install command contains ".exe"
* Fix compilation for Linux Wayland apps
* Fix tab button layout on mobile (fyne-io/fyne#2117)
* Options popup does not move if a SelectEntry widget moves with popup open
* Speed improvements to Select and SelectEntry drop down
* theme/fonts has an apache LICENSE file but it should have SIL OFL (fyne-io/fyne#2193)
* Fix build requirements for target macOS platforms (fyne-io/fyne#2154)
* ScrollEvent.Position and ScrollEvent.AbsolutePosition is 0,0 (fyne-io/fyne#2199)


## 2.0.2 - 1 April 2021

### Changed

* Text can now be copied from a disable Entry using keyboard shortcuts

### Fixed

* Slider offset position could be incorrect for mobile apps
* Correct error in example code
* When graphics init fails then don't try to continue running (fyne-io/fyne#1593)
* Don't show global settings on mobile in fyne_demo as it's not supported (fyne-io/fyne#2062)
* Empty selection would render small rectangle in Entry
* Do not show validation state for disabled Entry
* dialog.ShowFileSave did not support mobile (fyne-io/fyne#2076)
* Fix issue that storage could not write to files on iOS and Android
* mobile app could crash in some focus calls
* Duplicate symbol error when compiling for Android with NDK 23 (fyne-io/fyne#2064)
* Add internet permission by default for Android apps (fyne-io/fyne#1715)
* Child and Parent support in storage were missing for mobile appps
* Various crashes with Entry and multiline selections (including fyne-io/fyne#1989)
* Slider calls OnChanged for each value between steps (fyne-io/fyne#1748)
* fyne command doesn't remove temporary binary from src (fyne-io/fyne#1910)
* Advanced Color picker on mobile keeps updating values forever after sliding (fyne-io/fyne#2075)
* exec.Command and widget.Button combination not working (fyne-io/fyne#1857)
* After clicking a link on macOS, click everywhere in the app will be linked (fyne-io/fyne#2112)
* Text selection - Shift+Tab bug (fyne-io/fyne#1787)


## 2.0.1 - 4 March 2021

### Changed

* An Entry with `Wrapping=fyne.TextWrapOff` no longer blocks scroll events from a parent

### Fixed

* Dialog.Resize() has no effect if called before Dialog.Show() (fyne-io/fyne#1863)
* SelectTab does not always correctly set the blue underline to the selected tab (fyne-io/fyne#1872)
* Entry Validation Broken when using Data binding (fyne-io/fyne#1890)
* Fix background colour not applying until theme change
* android runtime error with fyne.dialog (fyne-io/fyne#1896)
* Fix scale calculations for Wayland phones (PinePhone)
* Correct initial state of entry validation
* fix entry widget mouse drag selection when scrolled
* List widget panic when refreshing after changing content length (fyne-io/fyne#1864)
* Fix image caching that was too aggressive on resize
* Pointer and cursor misalignment in widget.Entry (fyne-io/fyne#1937)
* SIGSEGV Sometimes When Closing a Program by Clicking a Button (fyne-io/fyne#1604)
* Advanced Color Picker shows Black for custom primary color as RGBA (fyne-io/fyne#1970)
* Canvas.Focus() before window visible causes application to crash (fyne-io/fyne#1893)
* Menu over Content (fyne-io/fyne#1973)
* Error compiling fyne on Apple M1 arm64 (fyne-io/fyne#1739)
* Cells are not getting draw in correct location after column resize. (fyne-io/fyne#1951)
* Possible panic when selecting text in a widget.Entry (fyne-io/fyne#1983)
* Form validation doesn't enable submit button (fyne-io/fyne#1965)
* Creating a window shows it before calling .Show() and .Hide() does not work (fyne-io/fyne#1835)
* Dialogs are not refreshed correctly on .Show() (fyne-io/fyne#1866)
* Failed creating setting storage : no such directory (fyne-io/fyne#2023)
* Erroneous custom filter types not supported error on mobile (fyne-io/fyne#2012)
* High importance button show no hovered state (fyne-io/fyne#1785)
* List widget does not render all visible content after content data gets shorter (fyne-io/fyne#1948)
* Calling Select on List before draw can crash (fyne-io/fyne#1960)
* Dialog not resizing in newly created window (fyne-io/fyne#1692)
* Dialog not returning to requested size (fyne-io/fyne#1382)
* Entry without scrollable content prevents scrolling of outside scroller (fyne-io/fyne#1939)
* fyne_demo crash after selecting custom Theme and table (fyne-io/fyne#2018)
* Table widget crash when scrolling rapidly (fyne-io/fyne#1887)
* Cursor animation sometimes distorts the text (fyne-io/fyne#1778)
* Extended password entry panics when password revealer is clicked (fyne-io/fyne#2036)
* Data binding limited to 1024 simultaneous operations (fyne-io/fyne#1838)
* Custom theme does not refresh when variant changes (fyne-io/fyne#2006)


## 2.0 - 22 January 2021

### Changes that are not backward compatible

These changes may break some apps, please read the 
[upgrading doc](https://developer.fyne.io/api/v2.0/upgrading) for more info
The import path is now `fyne.io/fyne/v2` when you are ready to make the update.

* Coordinate system to float32
  * Size and Position units were changed from int to float32
  * `Text.TextSize` moved to float32 and `fyne.MeasureText` now takes a float32 size parameter
  * Removed `Size.Union` (use `Size.Max` instead)
  * Added fyne.Delta for difference-based X, Y float32 representation
  * DraggedEvent.DraggedX and DraggedY (int, int) to DraggedEvent.Dragged (Delta)
  * ScrollEvent.DeltaX and DeltaY (int, int) moved to ScrollEvent.Scrolled (Delta)

* Theme API update
  * `fyne.Theme` moved to `fyne.LegacyTheme` and can be load to a new theme using `theme.FromLegacy`
  * A new, more flexible, Theme interface has been created that we encourage developers to use

* The second parameter of `theme.NewThemedResource` was removed, it was previously ignored
* The desktop.Cursor definition was renamed desktop.StandardCursor to make way for custom cursors
* Button `Style` and `HideShadow` were removed, use `Importance`

* iOS apps preferences will be lost in this upgrade as we move to more advanced storage
* Dialogs no longer show when created, unless using the ShowXxx convenience methods
* Entry widget now contains scrolling so should no longer be wrapped in a scroll container

* Removed deprecated types including:
  - `dialog.FileIcon` (now `widget.FileIcon`)
  - `widget.Radio` (now `widget.RadioGroup`)
  - `widget.AccordionContainer` (now `widget.Accordion`)
  - `layout.NewFixedGridLayout()` (now `layout.NewGridWrapLayout()`)
  - `widget.ScrollContainer` (now `container.Scroll`)
  - `widget.SplitContainer` (now `container.Spilt`)
  - `widget.Group` (replaced by `widget.Card`)
  - `widget.Box` (now `container.NewH/VBox`, with `Children` field moved to `Objects`)
  - `widget.TabContainer` and `widget.AppTabs` (now `container.AppTabs`)
* Many deprecated fields have been removed, replacements listed in API docs 1.4
  - for specific information you can browse https://developer.fyne.io/api/v1.4/

### Added

* Data binding API to connect data sources to widgets and sync data
  - Add preferences data binding and `Preferences.AddChangeListener`
  - Add bind support to `Check`, `Entry`, `Label`, `List`, `ProgressBar` and `Slider` widgets
* Animation API for handling smooth element transitions
  - Add animations to buttons, tabs and entry cursor
* Storage repository API for connecting custom file sources
  - Add storage functions `Copy`, `Delete` and `Move` for `URI`
  - Add `CanRead`, `CanWrite` and `CanList` to storage APIs
* New Theme API for easier customisation of apps
  - Add ability for custom themes to support light/dark preference
  - Support for custom icons in theme definition
  - New `theme.FromLegacy` helper to use old theme API definitions
* Add fyne.Vector for managing x/y float32 coordinates
* Add MouseButtonTertiary for middle mouse button events on desktop
* Add `canvas.ImageScaleFastest` for faster, less precise, scaling
* Add new `dialog.Form` that will phase out `dialog.Entry`
* Add keyboard control for main menu
* Add `Scroll.OnScrolled` event for seeing changes in scroll container
* Add `TextStyle` and `OnSubmitted` to `Entry` widget
* Add support for `HintText` and showing validation errors in `Form` widget
* Added basic support for tab character in `Entry`, `Label` and `TextGrid`

### Changed

* Coordinate system is now float32 - see breaking changes above
* ScrollEvent and DragEvent moved to Delta from (int, int)
* Change bundled resources to use more efficient string storage
* Left and Right mouse buttons on Desktop are being moved to `MouseButtonPrimary` and `MouseButtonSecondary`
* Many optimisations and widget performance enhancements

* Moving to new `container.New()` and `container.NewWithoutLayout()` constructors (replacing `fyne.NewContainer` and `fyne.NewContainerWithoutLayout`)
* Moving storage APIs `OpenFileFromURI`, `SaveFileToURI` and `ListerForURI` to `Reader`, `Writer` and `List` functions

### Fixed

* Validating a widget in widget.Form before renderer was created could cause a panic
* Added file and folder support for mobile simulation support (fyne-io/fyne#1470)
* Appending options to a disabled widget.RadioGroup shows them as enabled (fyne-io/fyne#1697)
* Toggling toolbar icons does not refresh (fyne-io/fyne#1809)
* Black screen when slide up application on iPhone (fyne-io/fyne#1610)
* Properly align Label in FormItem (fyne-io/fyne#1531)
* Mobile dropdowns are too low (fyne-io/fyne#1771)
* Cursor does not go down to next line with wrapping (fyne-io/fyne#1737)
* Entry: while adding text beyond visible reagion there is no auto-scroll (fyne-io/fyne#912)


## 1.4.3 - 4 January 2021

### Fixed

* Fix crash when showing file open dialog on iPadOS
* Fix possible missing icon on initial show of disabled button
* Capturing a canvas on macOS retina display would not capture full resolution
* Fix the release build flag for mobile
* Fix possible race conditions for canvas capture
* Improvements to `fyne get` command downloader
* Fix tree, so it refreshes visible nodes on Refresh()
* TabContainer Panic when removing selected tab (fyne-io/fyne#1668)
* Incorrect clipping behaviour with nested scroll containers (fyne-io/fyne#1682)
* MacOS Notifications are not shown on subsequent app runs (fyne-io/fyne#1699)
* Fix the behavior when dragging the divider of split container (fyne-io/fyne#1618)


## 1.4.2 - 9 December 2020

### Added

* [fyne-cli] Add support for passing custom build tags (fyne-io/fyne#1538)

### Changed

* Run validation on content change instead of on each Refresh in widget.Entry

### Fixed

* [fyne-cli] Android: allow to specify an inline password for the keystore
* Fixed Card widget MinSize (fyne-io/fyne#1581)
* Fix missing release tag to enable BuildRelease in Settings.BuildType()
* Dialog shadow does not resize after Refresh (fyne-io/fyne#1370)
* Android Duplicate Number Entry (fyne-io/fyne#1256)
* Support older macOS by default - back to 10.11 (fyne-io/fyne#886)
* Complete certification of macOS App Store releases (fyne-io/fyne#1443)
* Fix compilation errors for early stage Wayland testing
* Fix entry.SetValidationError() not working correctly


## 1.4.1 - 20 November 2020

### Changed

* Table columns can now be different sizes using SetColumnWidth
* Avoid unnecessary validation check on Refresh in widget.Form

### Fixed

* Tree could flicker on mouse hover (fyne-io/fyne#1488)
* Content of table cells could overflow when sized correctly
* file:// based URI on Android would fail to list folder (fyne-io/fyne#1495)
* Images in iOS release were not all correct size (fyne-io/fyne#1498)
* iOS compile failed with Go 1.15 (fyne-io/fyne#1497)
* Possible crash when minimising app containing List on Windows
* File chooser dialog ignores drive Z (fyne-io/fyne#1513)
* Entry copy/paste is crashing on android 7.1 (fyne-io/fyne#1511)
* Fyne package creating invalid windows packages (fyne-io/fyne#1521)
* Menu bar initially doesn't respond to mouse input on macOS (fyne-io/fyne#505) 
* iOS: Missing CFBundleIconName and asset catalog (fyne-io/fyne#1504)
* CenterOnScreen causes crash on MacOS when called from goroutine (fyne-io/fyne#1539)
* desktop.MouseHover Button state is not reliable (fyne-io/fyne#1533)
* Initial validation status in widget.Form is not respected
* Fix nil reference in disabled buttons (fyne-io/fyne#1558)


## 1.4 - 1 November 2020

### Added (highlights)

* List (fyne-io/fyne#156), Table (fyne-io/fyne#157) and Tree collection Widgets
* Card, FileItem, Separator widgets
* ColorPicker dialog
* User selection of primary colour
* Container API package to ease using layouts and container widgets
* Add input validation
* ListableURI for working with directories etc
* Added PaddedLayout

* Window.SetCloseIntercept (fyne-io/fyne#467)
* Canvas.InteractiveArea() to indicate where widgets should avoid
* TextFormatter for ProgressBar
* FileDialog.SetLocation() (fyne-io/fyne#821)
* Added dialog.ShowFolderOpen (fyne-io/fyne#941)
* Support to install on iOS and android with 'fyne install'
* Support asset bundling with go:generate
* Add fyne release command for preparing signed apps
* Add keyboard and focus support to Radio and Select widgets 

### Changed

* Theme update - new blue highlight, move buttons to outline
* Android SDK target updated to 29
* Mobile log entries now start "Fyne" instead of "GoLog"
* Don't expand Select to its largest option (fyne-io/fyne#1247)
* Button.HideShadow replaced by Button.Importance = LowImportance

* Deprecate NewContainer in favour of NewContainerWithoutLayout
* Deprecate HBox and VBox in favour of new container APIs
* Move Container.AddObject to Container.Add matching Container.Remove
* Start move from widget.TabContainer to container.AppTabs
* Replace Radio with RadioGroup
* Deprecate WidgetRenderer.BackgroundColor

### Fixed

* Support focus traversal in dialog (fyne-io/fyne#948), (fyne-io/fyne#948)
* Add missing AbsolutePosition in some mouse events (fyne-io/fyne#1274)
* Don't let scrollbar handle become too small
* Ensure tab children are resized before being shown (fyne-io/fyne#1331)
* Don't hang if OpenURL loads browser (fyne-io/fyne#1332)
* Content not filling dialog (fyne-io/fyne#1360)
* Overlays not adjusting on orientation change in mobile (fyne-io/fyne#1334)
* Fix missing key events for some keypad keys (fyne-io/fyne#1325)
* Issue with non-english folder names in Linux favourites (fyne-io/fyne#1248)
* Fix overlays escaping screen interactive bounds (fyne-io/fyne#1358)
* Key events not blocked by overlays (fyne-io/fyne#814)
* Update scroll container content if it is changed (fyne-io/fyne#1341)
* Respect SelectEntry datta changes on refresh (fyne-io/fyne#1462)
* Incorrect SelectEntry dropdown button position (fyne-io/fyne#1361)
* don't allow both single and double tap events to fire (fyne-io/fyne#1381)
* Fix issue where long or tall images could jump on load (fyne-io/fyne#1266, fyne-io/fyne#1432)
* Weird behaviour when resizing or minimizing a ScrollContainer (fyne-io/fyne#1245)
* Fix panic on NewTextGrid().Text()
* Fix issue where scrollbar could jump after mousewheel scroll
* Add missing raster support in software render
* Respect GOOS/GOARCH in fyne command utilities
* BSD support in build tools
* SVG Cache could return the incorrect resource (fyne-io/fyne#1479)

* Many optimisations and widget performance enhancements
* Various fixes to file creation and saving on mobile devices


## 1.3.3 - 10 August 2020

### Added

* Use icons for file dialog favourites (fyne-io/fyne#1186)
* Add ScrollContainer ScrollToBottom and ScrollToTop

### Changed

* Make file filter case sensitive (fyne-io/fyne#1185)

### Fixed

* Allow popups to create dialogs (fyne-io/fyne#1176)
* Use default cursor for dragging scrollbars (fyne-io/fyne#1172)
* Correctly parse SVG files with missing X/Y for rect
* Fix visibility of Entry placeholder when text is set (fyne-io/fyne#1193)
* Fix encoding issue with Windows notifications (fyne-io/fyne#1191)
* Fix issue where content expanding on Windows could freeze (fyne-io/fyne#1189)
* Fix errors on Windows when reloading Fyne settings (fyne-io/fyne#1165)
* Dialogs not updating theme correctly (fyne-io/fyne#1201)
* Update the extended progressbar on refresh (fyne-io/fyne#1219)
* Segfault if font fails (fyne-io/fyne#1200)
* Slider rendering incorrectly when window maximized (fyne-io/fyne#1223)
* Changing form label not refreshed (fyne-io/fyne#1231)
* Files and folders starting "." show no name (fyne-io/fyne#1235)


## 1.3.2 - 11 July 2020

### Added

* Linux packaged apps now include a Makefile to aid install

### Changed

* Fyne package supports specific architectures for Android
* Reset missing textures on refresh
* Custom confirm callbacks now called on implicitly shown dialogs
* SelectEntry can update drop-down list during OnChanged callback
* TextGrid whitespace color now matches theme changes
* Order of Window Resize(), SetFixedSize() and CenterOnScreen() does no matter before Show()
* Containers now refresh their visuals as well as their Children on Refresh()

### Fixed

* Capped StrokeWidth on canvas.Line (fyne-io/fyne#831)
* Canvas lines, rectangles and circles do not resize and refresh correctly
* Black flickering on resize on MacOS and OS X (possibly not on Catalina) (fyne-io/fyne#1122)
* Crash when resizing window under macOS (fyne-io/fyne#1051, fyne-io/fyne#1140)
* Set SetFixedSize to true, the menus are overlapped (fyne-io/fyne#1105)
* Ctrl+v into text input field crashes app. Presumably clipboard is empty (fyne-io/fyne#1123, fyne-io/fyne#1132)
* Slider default value doesn't stay inside range (fyne-io/fyne#1128)
* The position of window is changed when status change from show to hide, then to show (fyne-io/fyne#1116)
* Creating a windows inside onClose handler causes Fyne to panic (fyne-io/fyne#1106)
* Backspace in entry after SetText("") can crash (fyne-io/fyne#1096)
* Empty main menu causes panic (fyne-io/fyne#1073)
* Installing using `fyne install` on Linux now works on distributions that don't use `/usr/local`
* Fix recommendations from staticcheck
* Unable to overwrite file when using dialog.ShowFileSave (fyne-io/fyne#1168)


## 1.3 - 5 June 2020

### Added

* File open and save dialogs (fyne-io/fyne#225)
* Add notifications support (fyne-io/fyne#398)
* Add text wrap support (fyne-io/fyne#332)
* Add Accordion widget (fyne-io/fyne#206)
* Add TextGrid widget (fyne-io/fyne#115)
* Add SplitContainer widget (fyne-io/fyne#205)
* Add new URI type and handlers for cross-platform data access
* Desktop apps can now create splash windows
* Add ScaleMode to images, new ImageScalePixels feature for retro graphics
* Allow widgets to influence mouse cursor style (fyne-io/fyne#726)
* Support changing the text on form submit/cancel buttons
* Support reporting CapsLock key events (fyne-io/fyne#552)
* Add OnClosed callback for Dialog
* Add new image test helpers for validating render output
* Support showing different types of soft keyboard on mobile devices (fyne-io/fyne#971, fyne-io/fyne#975)

### Changed

* Upgraded underlying GLFW library to fix various issues (fyne-io/fyne#183, fyne-io/fyne#61)
* Add submenu support and hover effects (fyne-io/fyne#395)
* Default to non-premultiplied alpha (NRGBA) across toolkit
* Rename FixedGridLayout to GridWrapLayout (deprecate old API) (fyne-io/fyne#836)
* Windows redraw and animations continue on window resize and move
* New...PopUp() methods are being replaced by Show...Popup() or New...Popup().Show()
* Apps started on a goroutine will now panic as this is not supported
* On Linux apps now simulate 120DPI instead of 96DPI
* Improved fyne_settings scale picking user interface
* Reorganised fyne_demo to accommodate growing collection of widgets and containers
* Rendering now happens on a different thread to events for more consistent drawing
* Improved text selection on mobile devices

### Fixed (highlights)

* Panic when trying to paste empty clipboard into entry (fyne-io/fyne#743)
* Scale does not match user configuration in Windows 10 (fyne-io/fyne#635)
* Copy/Paste not working on Entry Field in Windows OS (fyne-io/fyne#981)
* Select widgets with many options overflow UI without scrolling (fyne-io/fyne#675)
* android: typing in entry expands only after full refresh (fyne-io/fyne#972)
* iOS app stops re-drawing mid frame after a while (fyne-io/fyne#950)
* Too many successive GUI updates do not properly update the view (904)
* iOS apps would not build using Apple's new certificates
* Preserve aspect ratio in SVG stroke drawing (fyne-io/fyne#976)
* Fixed many race conditions in widget data handling
* Various crashes and render glitches in extended widgets
* Fix security issues reported by gosec (fyne-io/fyne#742)


## 1.2.4 - 13 April 2020

### Added

 * Added Direction field to ScrollContainer and NewHScrollContainer, NewVScrollContainer constructors (fyne-io/fyne#763)
 * Added Scroller.SetMinSize() to enable better defaults for scrolled content
 * Added "fyne vendor" subcommand to help packaging fyne dependencies in projects
 * Added "fyne version" subcommand to help with bug reporting (fyne-io/fyne#656)
 * Clipboard (cut/copy/paste) is now supported on iOS and Android (fyne-io/fyne#414)
 * Preferences.RemoveValue() now allows deletion of a stored user preference

### Changed

 * Report keys based on name not key code - fixes issue with shortcuts with AZERTY (fyne-io/fyne#790)

### Fixed

 * Mobile builds now support go modules (fyne-io/fyne#660)
 * Building for mobile would try to run desktop build first
 * Mobile apps now draw the full safe area on a screen (fyne-io/fyne#799)
 * Preferences were not stored on mobile apps (fyne-io/fyne#779)
 * Window on Windows is not controllable after exiting FullScreen mode (fyne-io/fyne#727)
 * Soft keyboard not working on some Samsung/LG smart phones (fyne-io/fyne#787)
 * Selecting a tab on extended TabContainer doesn't refresh button (fyne-io/fyne#810)
 * Appending tab to empty TabContainer causes divide by zero on mobile (fyne-io/fyne#820)
 * Application crashes on startup (fyne-io/fyne#816)
 * Form does not always update on theme change (fyne-io/fyne#842)


## 1.2.3 - 2 March 2020

### Added

 * Add media and volume icons to default themes (fyne-io/fyne#649)
 * Add Canvas.PixelCoordinateForPosition to find pixel locations if required
 * Add ProgressInfinite dialog

### Changed

 * Warn if -executable or -sourceDir flags are used for package on mobile (fyne-io/fyne#652)
 * Update scale based on device for mobile apps
 * Windows without a title will now be named "Fyne Application"
 * Revert fix to quit mobile apps - this is not allowed in guidelines

### Fixed

 * App.UniqueID() did not return current app ID
 * Fyne package ignored -name flag for ios and android builds (fyne-io/fyne#657)
 * Possible crash when appending tabs to TabContainer
 * FixedSize windows not rescaling when dragged between monitors (fyne-io/fyne#654)
 * Fix issues where older Android devices may not background or rotate (fyne-io/fyne#677)
 * Crash when setting theme before window content set (fyne-io/fyne#688)
 * Correct form extend behaviour (fyne-io/fyne#694)
 * Select drop-down width is wrong if the drop-down is too tall for the window (fyne-io/fyne#706)


## 1.2.2 - 29 January 2020

### Added

* Add SelectedText() function to Entry widget
* New mobile.Device interface exposing ShowVirtualKeyboard() (and Hide...)

### Changed

* Scale calculations are now relative to system scale - the default "1" matches the system
* Update scale on Linux to be "auto" by default (and numbers are relative to 96DPI standard) (fyne-io/fyne#595)
* When auto scaling check the monitor in the middle of the window, not top left
* bundled files now have a standard header to optimise some tools like go report card
* Shortcuts are now handled by the event queue - fixed possible deadlock

### Fixed

* Scroll horizontally when holding shift key (fyne-io/fyne#579)
* Updating text and calling refresh for widget doesn't work (fyne-io/fyne#607)
* Corrected visual behaviour of extended widgets including Entry, Select, Check, Radio and Icon (fyne-io/fyne#615)
* Entries and Selects that are extended would crash on right click.
* PasswordEntry created from Entry with Password = true has no revealer
* Dialog width not always sufficient for title
* Pasting unicode characters could panic (fyne-io/fyne#597)
* Setting theme before application start panics on macOS (fyne-io/fyne#626)
* MenuItem type conflicts with other projects (fyne-io/fyne#632)


## 1.2.1 - 24 December 2019

### Added

* Add TouchDown, TouchUp and TouchCancel API in driver/mobile for device specific events
* Add support for adding and removing tabs from a tab container (fyne-io/fyne#444)

### Fixed

* Issues when settings changes may not be monitored (fyne-io/fyne#576)
* Layout of hidden tab container contents on mobile (fyne-io/fyne#578)
* Mobile apps would not quit when Quit() was called (fyne-io/fyne#580)
* Shadows disappeared when theme changes (fyne-io/fyne#589)
* iOS apps could stop rendering after many refreshes (fyne-io/fyne#584)
* Fyne package could fail on Windows (fyne-io/fyne#586)
* Horizontal only scroll container may not refresh using scroll wheel


## 1.2 - 12 December 2019

### Added

* Mobile support - iOS and Android, including "fyne package" command
* Support for OpenGL ES and embedded linux
* New BaseWidget for building custom widgets
* Support for diagonal gradients
* Global settings are now saved and can be set using the new fyne_settings app
* Support rendering in Go playground using playground.Render() helpers
* "fyne install" command to package and install apps on the local computer
* Add horizontal scrolling to ScrollContainer
* Add preferences API
* Add show/hide password icon when created from NewPasswordEntry
* Add NewGridLayoutWithRows to specify a grid layout with a set number of rows
* Add NewAdaptiveGridLayout which uses a column grid layout when horizontal and rows in vertical


### Changed

* New Logo! Thanks to Storm for his work on this :)
* Applications no longer have a default (Fyne logo) icon
* Input events now execute one at a time to maintain the correct order
* Button and other widget callbacks no longer launch new goroutines
* FYNE_THEME and FYNE_SCALE are now overrides to the global configuration
* The first opened window no longer exits the app when closed (unless none others are open or Window.SetMaster() is called)
* "fyne package" now defaults icon to "Icon.png" so the parameter is optional
* Calling ExtendBaseWidget() sets up the renderer for extended widgets
* Entry widget now has a visible Disabled state, ReadOnly has been deprecated
* Bundled images optimised to save space
* Optimise rendering to reduce refresh on TabContainer and ScrollContainer


### Fixed

* Correct the color of Entry widget cursor if theme changes
* Error where widgets created before main() function could crash (fyne-io/fyne#490)
* App.Run panics if called without a window (fyne-io/fyne#527)
* Support context menu for disabled entry widgets (fyne-io/fyne#488)
* Fix issue where images using fyne.ImageFillOriginal may not show initially (fyne-io/fyne#558)


## 1.1.2 - 12 October 2019

### Added

### Changed

* Default scale value for canvases is now 1.0 instead of Auto (DPI based)

### Fixed

* Correct icon name in linux packages
* Fullscreen before showing a window works again
* Incorrect MinSize of FixedGrid layout in some situations
* Update text size on theme change
* Text handling crashes (fyne-io/fyne#411, fyne-io/fyne#484, fyne-io/fyne#485)
* Layout of image only buttons
* TabItem.Content changes are reflected when refreshing TabContainer (fyne-io/fyne#456)

## 1.1.1 - 17 August 2019

### Added

* Add support for custom Windows manifest files in fyne package

### Changed

* Dismiss non-modal popovers on secondary tap
* Only measure visible objects in layouts and minSize calculations (fyne-io/fyne#343)
* Don't propagate show/hide in the model - allowing children of tabs to remain hidden
* Disable cut/copy for password fields
* Correctly calculate grid layout minsize as width changes
* Select text at end of line when double tapping beyond width

### Fixed

* Scale could be too large on macOS Retina screens
* Window with fixed size changes size when un-minimized on Windows (fyne-io/fyne#300)
* Setting text on a label could crash if it was not yet shown (fyne-io/fyne#381)
* Multiple Entry widgets could have selections simultaneously (fyne-io/fyne#341)
* Hover effect of radio widget too low (fyne-io/fyne#383)
* Missing shadow on Select widget
* Incorrect rendering of subimages within Image object
* Size calculation caches could be skipped causing degraded performance


## 1.1 - 1 July 2019

### Added

* Menubar and PopUpMenu (fyne-io/fyne#41)
* PopUp widgets (regular and modal) and canvas overlay support (fyne-io/fyne#242)
* Add gradient (linear and radial) to canvas
* Add shadow support for overlays, buttons and scrollcontainer
* Text can now be selected (fyne-io/fyne#67)
* Support moving through inputs with Tab / Shift-Tab (fyne-io/fyne#82)
* canvas.Capture() to save the content of a canvas
* Horizontal layout for widget.Radio
* Select widget (fyne-io/fyne#21)
* Add support for disabling widgets (fyne-io/fyne#234)
* Support for changing icon color (fyne-io/fyne#246)
* Button hover effect
* Pointer drag event to main API
* support for desktop mouse move events
* Add a new "hints" build tag that can suggest UI improvements

### Changed

* TabContainer tab location can now be set with SetTabLocation()
* Dialog windows now appear as modal popups within a window
* Don't add a button bar to a form if it has no buttons
* Moved driver/gl package to internal/driver/gl
* Clicking/Tapping in an entry will position the cursor
* A container with no layout will not change the position or size of its content
* Update the fyne_demo app to reflect the expanding feature set

### Fixed

* Allow scrollbars to be dragged (fyne-io/fyne#133)
* Unicode char input with Option key on macOS (fyne-io/fyne#247)
* Resizng fixed size windows (fyne-io/fyne#248)
* Fixed various bugs in window sizing and padding
* Button icons do not center align if label is empty (fyne-io/fyne#284)


## 1.0.1 - 20 April 2019

### Added

* Support for go modules
* Transparent backgrounds for widgets
* Entry.OnCursorChanged()
* Radio.Append() and Radio.SetSelected() (fyne-io/fyne#229)

### Changed

* Clicking outside a focused element will unfocus it
* Handle key repeat for non-runes (fyne-io/fyne#165)

### Fixed

* Remove duplicate options from a Radio widget (fyne-io/fyne#230)
* Issue where paste shortcut is not called for Ctrl-V keyboard combination
* Cursor position when clearing text in Entry (fyne-io/fyne#214)
* Antialias of lines and circles (fyne-io/examples#14)
* Crash on centering of windows (fyne-io/fyne#220)
* Possible crash when closing secondary windows
* Possible crash when showing dialog
* Initial visibility of scroll bar in ScrollContainer
* Setting window icon when different from app icon.
* Possible panic on app.Quit() (fyne-io/fyne#175)
* Various caches and race condition issues (fyne-io/fyne#194, fyne-io/fyne#217, fyne-io/fyne#209).


## 1.0 - 19 March 2019

The first major release of the Fyne toolkit delivers a stable release of the
main functionality required to build basic GUI applications across multiple
platforms.

### Features

* Canvas API (rect, line, circle, text, image)
* Widget API (box, button, check, entry, form, group, hyperlink, icon, label, progress bar, radio, scroller, tabs and toolbar)
* Light and dark themes
* Pointer, key and shortcut APIs (generic and desktop extension)
* OpenGL driver for Linux, macOS and Windows
* Tools for embedding data and packaging releases

