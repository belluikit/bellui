# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## About

Bellui is a cross-platform GUI toolkit for Go. Module path: `github.com/belluikit/bellui`. Supports desktop (Linux, macOS, Windows), mobile (iOS, Android), and web (WebAssembly).

## Common Commands

### Testing

```bash
# Run all tests (all platforms)
go test -race -tags ci github.com/belluikit/bellui/...

# Run a single test
go test -race -tags ci -run TestButtonMinSize ./widget/

# Run tests for a specific package
go test -race -tags ci ./container/...

# The x/ submodules are separate Go modules, so the command above does not
# reach them. Test one from its own directory.
cd x/mmedia && go test -race -tags gl,ci ./...
```

CI requires minimum 62% test coverage on the main module. The `x/` submodules
are covered by `x_tests.yml`, which builds/vets/tests each of them against the
toolkit code in the checkout (an ephemeral `replace`, never committed) and holds
a **per-module** coverage floor — they do not inherit the 62%. See #91, #13.

### Static Analysis

```bash
# Formatting (gofumpt, not gofmt)
gofumpt -d -e .
goimports -e -d .

# Vet and lint
go vet ./...
staticcheck ./...

# Cyclomatic complexity (max 30). Vendored third-party (internal/oksvg,
# internal/rasterx, internal/render) and the x/ submodules are scoped out —
# gocyclo walks the filesystem, so unlike `go vet`/`staticcheck` it would
# otherwise descend into code this module does not own. See #13, #18.
gocyclo -over 30 -ignore 'internal/oksvg|internal/rasterx|internal/render|^x/' .
```

### Code Generation

```bash
# Generate data binding types
go generate ./data/binding/

# Bundle assets (SVGs, icons)
go generate ./cmd/bellui_settings/data/
```

## Architecture

### Core Interface Pattern

Everything builds on `bellui.CanvasObject` (defined in `canvasobject.go`): MinSize, Move, Position, Resize, Size, Hide, Show, Visible, Refresh.

Widgets implement `bellui.Widget` which adds `CreateRenderer() WidgetRenderer`. The renderer handles layout and drawing, separating logic from presentation. Key interfaces are defined in root-level files: `app.go`, `window.go`, `canvas.go`, `canvasobject.go`, `widget.go`, `driver.go`.

Behavioral capabilities are composed via interfaces: `Tappable`, `Draggable`, `Focusable`, `Scrollable`, `Hoverable`, `LongPressable`, `Disableable`, etc. (all in `event/event.go`).

### Interface Composition Pattern

Core interfaces (`App`, `Window`, `Driver`) are kept minimal. Optional capabilities are exposed via small extension interfaces and type assertions:

**App optional interfaces:** `CloudApp` (cloud provider), `NotifierApp` (system notifications). Standalone functions: `bellui.OpenURL()`, `bellui.SendNotification()`, `bellui.ShowAndRun(w)`.

**Window optional interfaces:** `MasterWindow` (`SetMaster()`), `DroppableWindow` (`SetOnDropped()`).

**Driver optional interfaces:** `DoubleTapDriver` (`DoubleTapDelay()`), `ScreenBlankingDriver` (`SetDisableScreenBlanking()`). Standalone: `bellui.DoubleTapDelay()`.

**Preferences** uses generic free functions instead of typed methods:
```go
// Interface: Value(key) (any, bool), SetValue(key, value), RemoveValue(key), AddChangeListener(func())
bellui.SetPref(prefs, "key", 42)            // type-safe via generics
val := bellui.PrefOr(prefs, "key", 0)       // returns int with fallback
bellui.SetPrefList(prefs, "items", []string{"a", "b"})
items := bellui.PrefListOr(prefs, "items", []string{})
```

### Platform Abstraction

Platform-specific code uses build tags, not runtime checks:
- `mobile` / `ios` / `android` - mobile platforms
- `no_glfw` - skip GLFW-dependent tests (test-only; set in macOS/Wayland CI). NOT a driver selector — all desktop platforms (macOS, Linux, Windows) use the go-gl/glfw driver by default
- `no_native_menus` - disable the native macOS NSMenu bar, falling back to canvas-drawn menus
- `wayland` - **forced** Linux Wayland-only build (single backend). Rarely needed — the default build now selects X11/Wayland at *runtime* (see Runtime Wayland/X11 Selection below), so a plain build runs on both. Kept for packagers who want a lean single-backend binary.
- `x11` - **forced** Linux X11-only build (single backend), the counterpart to `wayland`.
- `wasm` / `js` - WebAssembly
- `ci` - CI test behavior
- `migrated_belluido` - test compatibility tag
- `accessibility` - opt-in screen-reader support (semantic tree walker + platform backends; see Accessibility Semantic Layer below)

Driver implementations live in `internal/driver/` with `common/`, `mobile/`, and shared logic.

### Key Packages

- `app/` - Application lifecycle, preferences, settings
- `canvas/` - Drawing primitives (text, shapes, images, gradients)
- `container/` - Layout containers (VBox, HBox, Border, Tab, Split, Grid)
- `widget/` - UI widgets (Button, Entry, Label, List, Table, Tree, etc.)
- `layout/` - Layout managers
- `dialog/` - Modal dialogs (file, color, confirmation)
- `data/binding/` - Type-safe data binding with change listeners
- `storage/` - URI handling and file system repository pattern
- `theme/` - Theming system and bundled icons
- `test/` - Test utilities and assertions
- `internal/` - Cache, painters (software/GL), SVG handling

### Testing Patterns

Tests use `github.com/stretchr/testify`. The `test` package provides:

- `test.NewTempApp(t)` - Creates a test app with automatic cleanup
- `test.AssertObjectRendersToImage(t, "master.png", obj)` - Visual regression testing against `testdata/` baselines
- `test.AssertObjectRendersToMarkup(t, "master.xml", obj)` - Markup-based rendering verification
- `test.AssertCanvasTappableAt(t, canvas, pos)` - Interaction testing
- `test.TempWidgetRenderer(t, widget)` - Access widget renderer in tests

Failed visual tests write output to `testdata/failed/` for comparison.

For locale-dependent tests in widget packages, use `lang.SetLocaleForTests("en-US")` to override the system locale.

## Fork Conventions

Rules that apply to *new* code anywhere in the fork. The reasoning behind each lives in
the `docs/notes/` file for its area.

- **Optional capability interfaces, not fat interfaces.** Core interfaces stay minimal;
  new capabilities are small extension interfaces discovered by type assertion (see
  Interface Composition above). A failed assertion must degrade visibly or fall back —
  never a silent no-op.
- **Never embed `bellui.Container`.** The object-tree walker
  (`internal/driver.walkObjectTree`, behind painting, hit-testing, a11y and test markup)
  matches `*bellui.Container` by *exact type*, so an embedder's children lay out but are
  never painted or hit-testable (#767). Use a plain `*bellui.Container` (config in the
  Layout or a lookaside) or a real `core.Widget` with a renderer.
- **`Bind` never claims a public callback field.** A binder gets its own unexported hook,
  invoked before the public callback, and `Unbind` clears only that hook (#47, #49).
- **`SetX` reports, `SetXSilently` does not.** A plain setter fires `OnChanged` /
  `OnChangeEnded` and propagates to bound data; the `*Silently` variant is the quiet
  counterpart. Fork-wide.
- **Heavy or platform-bound code is an optional import**, linked only when imported:
  `theme/cjk`, `webview`, `analytics`, `dev/inspector`. Keeps default and wasm binaries lean.
- **Forks are consumed by import rewrite, never a `replace` directive** —
  `belluikit/glfw`, `belluikit/glfw-js`, `belluikit/systray`, `belluikit/fsnotify`.
- **Main-goroutine discipline.** Widget mutation, layout and platform calls run on the main
  goroutine; cross-thread work hops via `bellui.Do` / `bellui.DoAndWait`. Native callbacks
  that arrive on a platform thread must be queued, not applied in place.
- **wasm build tags:** when adding a `wasm`-tagged file, its `!wasm` counterpart and its own
  tag must together cover `wasm && ci` — otherwise nothing builds and the symbol is
  undefined on that leg (#73, #77).
- **Touching `app/GoNativeActivity.java` requires the tools-repo dex regen**
  (`go run gendex/gendex.go -o dex.go` with `ANDROID_HOME` set).
- **Live-verify harnesses live in `hack/*probe`**, most with an `-auto` scripted mode. A
  change to a platform bridge is not verified by unit tests alone — run the probe, on device
  or on GPU where the claim requires it.

## Fork Features

This fork extends upstream Bellui substantially. Design decisions, invariants, platform
gotchas and recorded non-goals live in `docs/notes/` — **read the file for an area before
changing it**; the list below is only an index.

- **Text, IME & editors** — `docs/notes/text.md`
  `TextStyle.Weight`/`LineHeightScale`/`LetterSpacing` are `*pointer` fields (wrap with
  `bellui.Ptr(...)` in literals), plus `Language`; underline/strikethrough on general text;
  per-rune font fallback (`RegisterFontFallback`, `theme/cjk`, bundled symbol subsets);
  per-subtree/per-window RTL `DirectionScope`; HTML + markdown parsing and its
  untrusted-content defenses (`AllowRemoteContent` defaults **false**); `Entry` rope storage
  (`internal/textbuf`, string-identity contract) and `DeferTextSync`; IME via
  `event.Composable` + the `internal/textinput` mirror seam; mobile touch selection;
  `widget.RichEntry`.
- **Widgets, events & layouts** — `docs/notes/widgets.md`
  `event.Hoverable`/`HoverEvent`, `LongPressable`, `KeyEvent.Modifier`, `ScrollEvent.Source`,
  nested-scroll propagation; the 36 MD3 components; Date/Time/DateTime pickers and entries;
  `CompletionPopup`, `ContextMenu`, `Paned`, `SearchField`, `SpinBox`, `StatusBar`, `Toast`,
  `AnimatedGif`, `FileTree`; `ShrinkLayout`, `ResponsiveLayout`; `dialog.NewAbout`.
- **Rendering & canvas** — `docs/notes/rendering.md`
  `canvas.StyledText` (per-run style), `TextOnPath`, streaming rasters for video playback
  (`Streaming` + `SetPixels` + `PixelImage`), shader ripple, FBO gaussian shadows, backdrop
  blur, stencil rounded clipping, group opacity, `RepaintBoundary`, dirty-region culling,
  GPU `Scene3D`.
- **MD3 foundation, theming & i18n** — `docs/notes/theme-i18n.md`
  `DefaultTheme()` is `enhancedTheme`: HCT tonal palettes → ~30 MD3 color roles, 15-token
  type scale, state layers, shape/animation tokens; `DenseTheme()`; `widget.ThemeSelector`;
  the typed `RenderStyleTheme`/`MobileTheme` interfaces. Also the whole `lang/` surface —
  locale override + live re-translation, CLDR date/time/currency/list/unit formatting,
  collation, grammatical select, and the `bellui translate` catalog workflow.
- **Platform integration** — `docs/notes/platform.md`
  Native/portal file dialogs (`dialog.UseNativeFileDialogs`, default off); **runtime**
  Wayland/X11 selection (`build.IsWayland` is a runtime var; `BELLUI_PLATFORM` pins it);
  window positioning/screens, window-hosted dialogs with native parent + modality;
  per-platform multi-window semantics; single-instance mode and open-with/deep-link events;
  clipboard flavors (image/files/HTML/multi); rich notifications; the forked fsnotify and its
  Windows fixes; system-tray Linux delivery caveat; recorded non-goals (printing,
  session-end veto).
- **Data model layer** — `docs/notes/data-model.md`
  `data/model`: observable `Model`, `SortProxy`/`FilterProxy` (rebuilt per change, never per
  row), identity-keyed `SelectionModel`, `MutableModel` write-through, `TreeProxy`; widget
  adapters `BindListModel`/`BindTableModel`/`BindTableSort`/`BindListEditing`.
- **Accessibility** — `docs/notes/accessibility.md`
  Public semantics in `core/accessible.go` (always compiled); bounds-aware walker and five
  platform backends behind `-tags accessibility` — macOS NSAccessibility, Windows UIA, Linux
  AT-SPI2 (pure Go), Android virtual nodes, iOS. `StateDisabled` is derived structurally.
  Announcements via `bellui.AnnounceToScreenReader`. Verify with `hack/a11yprobe`.
- **Native view embedding** — `docs/notes/native-embedding.md`
  `widget.NativeViewHost` is a **hole punch**: bellui draws nothing there, so every overlay,
  dialog and popup is occluded and `Canvas.Capture` sees a hole. Optional `webview` package
  with macOS/iOS/Android/Windows/wasm backends and an origin-gated bridge. Deliberately not
  a Tab stop (WCAG 2.1.2 keyboard trap). **No Linux backend, by decision** — WPE rejected;
  CEF is the only candidate if ever revisited.
- **DevTools & analytics** — `docs/notes/devtools.md`
  `dev/inspector` — opt-in, read-only widget-tree inspector in its own window with pick mode.
  `analytics` — opt-in, consent-first event package (starts **disabled**, generates no device
  identifier, queue + disk spool + batching + backoff), plus semantic auto-capture that reads
  role/label/state only, never values.

### MD3 Gallery App

The Material Design 3 component showcase lives at `github.com/trendvidia/gallery` (checked
out as a sibling of this repo, resolved via the workspace `go.work`). Use it to manually
verify widget, theme and rendering changes.

## Code Style

- Formatting: `gofumpt` (stricter than `gofmt`)
- Imports: `goimports` ordering
- Dependency licenses must be: Apache-2.0, BSD-2-Clause, BSD-3-Clause, MIT, or ISC

## Backlog Protocol

The toolkit backlog lives in the **Bellui Backlog** GitHub project (trendvidia/projects/2):
fields Workstream / Priority (P0–P2) / Tranche (Now, Next, Later, Parked), plus native
blocked-by edges between issues. Before picking up work:

1. Run `hack/plan/plan.sh` — renders the live plan: startable items (unblocked, by tranche
   then priority), blocked items with their open blockers, parked items. Work only unblocked
   items; Parked means decline-unless-triggered (the trigger is recorded in the issue).
2. The issue body is the work spec; the project answers *what/when*. New issues are
   auto-added to the project by `.github/workflows/add-to-project.yml` — set their
   Workstream/Priority/Tranche when triaging.
3. Issues labeled `design-tool` are additionally tracked in the AppViewer Platform project
   (trendvidia/projects/1) as part of the cross-repo design-tool story — its plan wins for
   sequencing those (see trendvidia/appviewer docs/STORY.md).

### Follow-up rule (no silent tech debt)

While executing any issue, discovered work is never left as a TODO comment, a "later" note in
the PR description, or unstated scope creep. Exactly one of:

1. **In-scope**: small, same repo, and needed for this issue's acceptance → do it in the same PR.
2. **File it now**: everything else → open an issue *before* the originating PR merges —
   in this repo, or in the owning trendvidia repo when the gap belongs to a dependency we own
   (glfw, glfw-js, protowire-go, govm, fycoder, tools/CLI, …). Triage it on the spot
   (Workstream/Priority/Tranche here; `design-tool` label if story-related), wire a blocked-by
   edge if it blocks planned work, and cross-link it from the originating issue/PR.
3. Third-party (not ours) → record the limitation in the issue/CLAUDE.md with the upstream link.

A phase/issue is not done while a discovered follow-up exists only in prose.
