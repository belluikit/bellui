# Roadmap

Structural gaps and improvements for Bellui, ordered by priority.
Accessibility items are tracked separately in [ACCESSIBILITY.md](ACCESSIBILITY.md).

## 1. Thread Safety

### Round 2

Thread safety issues found during full codebase audit (2026-04-19).

~~**RichText `visualCache` unprotected (`widget/richtext.go:75`):**~~
~~Fixed: added `visualMu sync.Mutex` to RichText protecting `cachedSegmentVisual()` and `cleanVisualCache()`.~~

~~**Theme render stack not thread-safe (`internal/theme/render.go`):**~~
~~Fixed: replaced global slice with per-goroutine stacks via `sync.Map` keyed by `thread.GoroutineID()`. Each goroutine gets its own isolated stack — no mutex needed on individual entries, no cross-goroutine interference.~~

~~**Preferences `ChangeListeners()` returns raw slice (`internal/preferences.go`):**~~
~~Fixed: renamed to `ChangeListenersCopy()` with `RLock` + snapshot copy. No callers of old name remain.~~

~~**Streaming texture GL race (`internal/painter/gl/texture.go`):**~~
~~Fixed: restructured `updateStreamingTexture()` to hold `StreamMu` through `TexSubImage2D` call for the `StreamPix` path. Generator path releases lock before generating (no race — fresh image).~~

### Round 3

Thread safety issues found during second-pass audit (2026-04-19).

~~**TreeView `open` map unprotected (`widget/treeview.go:335-393`):**~~
~~Fixed: extended `tv.mu` to protect all `open` map access — `OpenBranch`, `CloseBranch`, `IsBranchOpen`, `OpenAllBranches`, `CloseAllBranches`, `openAncestors`, `finalizeDrag`. Removed `ensureOpenMap()`, inlined nil checks under lock.~~

~~**TreeView `selected` field unprotected (`widget/treeview.go:410-452`):**~~
~~Fixed: extended `tv.mu` to protect all `selected` access — `Select`, `Unselect`, `UnselectAll`, `Selected`, `TypedKey`, and renderer `Refresh`. Callbacks called outside lock to avoid deadlocks.~~

~~**Canvas `baseObject` position/size fields unprotected (`canvas/base.go:15-22`):**~~
~~Fixed: added `propMu sync.RWMutex` to `baseObject` protecting `size`, `position`, `Hidden`, `min`. All subtypes (Text, Rectangle, Image, Raster, etc.) inherit protection via method delegation. Circle and Line (which don't embed baseObject) received their own `propMu` covering `Position1`/`Position2`/`Hidden`/`transform`.~~

~~**Canvas Image File/Resource TOCTOU (`canvas/image.go:180-197`):**~~
~~Fixed: `Refresh()` now snapshots `File`, `Resource`, `Image` under `propMu` at method entry. `updateReader()` accepts file/resource as parameters instead of re-reading from struct fields.~~

---

## 2. Resource Leaks — Renderers & Goroutines

Widget renderer `Destroy()` cleanup audit (2026-04-19).

~~**Tooltip renderer leaks timer and popup (`widget/tooltip.go:230`):**~~
~~Fixed: `Destroy()` now calls `cancelTimer()` + `hidePopUp()`.~~

~~**Toast renderer leaks auto-dismiss timer (`widget/toast.go`):**~~
~~Fixed: replaced `SimpleRenderer` with custom `toastRenderer` whose `Destroy()` calls `Dismiss()`.~~

~~**RepaintBoundary FBO never freed (`container/repaint_boundary.go:92`):**~~
~~Fixed: GL painter `Free()` now calls `FreeBoundaryFBO(obj)`, releasing the FBO/texture when the object is freed.~~

~~**FontPicker goroutine without cancellation (`widget/font_picker.go:622-658`):**~~
~~Fixed: added `cancelled atomic.Bool`. Custom `fontPickerRenderer.Destroy()` sets it. Goroutine checks before and inside `fyne.Do()` callback.~~

~~**Settings change listener goroutine leak (`app/settings.go:114`):**~~
~~Fixed: removed fire-and-forget goroutine. When channel is full, update is dropped — `s` is a pointer so the receiver always sees the latest state.~~

**Empty `Destroy()` methods (no resources, low risk but should be reviewed) (pre-rebrand tracker #28):**
`widget/context_menu.go:129`, `widget/datetime_picker.go:178`, `widget/scroll_indicator.go:135`, `widget/search_select.go:281`, `widget/select.go:402`, `widget/code_segment.go:145`, `widget/treeview_render.go:395,876`, `container/paned.go:127`.

---

## 3. Scene3D Rendering Issues

Issues found in `canvas/scene3d.go` during audit (2026-04-19).

~~**Backface culling doesn't cull (`canvas/scene3d.go:269-273`):**~~
~~Fixed: Added `BackfaceCulling bool` field to `Scene3D`. When true, back-facing triangles are skipped (`continue`). When false (default), normals are flipped toward the viewer for correct double-sided rendering. Misleading "Backface culling" comment corrected.~~

~~**Software vs GL renderer divergence (normals):**~~
~~Fixed: `buildScene3DVerts()` in the GL renderer now applies the same camera-relative normal correction as the software renderer — normals facing away from the camera are flipped (or the face is culled when `BackfaceCulling` is true). Both renderers now produce identical lighting for the same mesh and camera configuration.~~

**Remaining divergence (low impact) (pre-rebrand tracker #29):** Software clips entire triangles when any vertex has Z < -1 in NDC; GL hardware clips triangle geometry at all frustum planes. This produces slightly different results only for triangles that straddle the near clipping plane.

---

## 4. Data Binding Listener Accumulation

~~**No duplicate detection or max count (`data/binding/binding.go:68-72`):**~~
~~Fixed: `AddListener` now checks for identity-equal duplicates before appending. Adding the same listener a second time is a no-op. Prevents accumulation from repeated bind-without-unbind patterns.~~

~~**basicBinder race between Bind/Unbind (`widget/bind_helper.go:20-61`):**~~
~~Fixed: moved `data.AddListener(listener)` inside the lock, after `unbindLocked()`. The old listener's `RemoveListener` is now always enqueued before the new `AddListener`, and concurrent `Bind` calls are fully serialized.~~

---

## 5. Test Coverage Gaps (partially done)

**Done:**
- `canvas/scene3d_test.go` — 35 tests: Vec3/Mat4 math, Scene3D object, software rasterizer (triangles, z-buffer, backgrounds, edge cases), internal helpers (rasterizeTriangle, min3f, max3f).
- `layout/shrinklayout_test.go` — 5 tests: MinSize (0,0), Layout positioning, multiple children, empty input. Full coverage.
- `internal/theme/render_test.go` — expanded: empty stack fallback, nested LIFO push/pop, balanced cycles, rapid stress test.
- Stale golden files cleaned — deleted 550 files across 9 `testdata/failed/` directories. `.gitignore` already prevents re-accumulation.
- ~~Locale test isolation~~ — `lang.SetLocaleForTests()` now accepts `testing.TB` and auto-restores locale via `t.Cleanup()`. All 40 call sites updated. Prevents `TestEntry_FocusWithPopUp` / `TestEntry_TappedSecondary` deterministic failures from locale state leaking between tests.
- ~~Canvas primitive test expansion~~ — Ripple (8→3 new), BoxShadow (8→3 new), BackdropBlur (8→3 new): Move/Resize/Hide/Show/Visible, MinSize, property defaults, BlurRadius mutation.
- ~~TextOnPath/StyledText test expansion~~ — TextOnPath (15→5 new): Resize, SetMinSize no-op, Refresh, nil buildPath, bounds tracking. StyledText (16→5 new): SetMinSize no-op, Refresh, bold run width, mixed sizes, empty runs.
- ~~Painter MeasureStyledRuns/DrawStyledRuns/ClearFontCache~~ — 11 new tests: single/multi/empty/nil runs, size fallback, different sizes, pixel output verification, color fallback, cache clear safety.

**Remaining zero coverage (pre-rebrand tracker #30):**
- `canvas/glview.go` — GL context management, custom shader rendering
- `internal/painter/gl/` (~1%) — GL painter almost entirely untested (requires live GL context)

**Remaining thin coverage (<40%) (pre-rebrand tracker #31):**
- `internal/painter/` — gradient rendering (gradient.go well-tested at pixel level, but font_internal_test.go mostly commented out)

**Remaining test infrastructure issues (pre-rebrand tracker #32):**
- 4 flaky test files excluded from CI via build tags: `apptabs_desktop_flakey_test.go`, `apptabs_mobile_flakey_test.go`, `canvas_flakey_test.go`, `window_flakey_test.go`.

---

## 6. Build & CI

~~**Local `replace` directive in go.mod:**~~
~~Fixed (2026-06-10): the typesetting replace now points at `github.com/belluikit/typesetting` pinned by pseudo-version. Builds require `GOPRIVATE=github.com/trendvidia`.~~
~~Resolved (2026-06-23, pre-rebrand tracker #38): the `replace` directive is **gone**. Both forks now self-host under the `trendvidia` module path (gl-js precedent) and are imported directly: `github.com/belluikit/typesetting v0.3.4` (carries the Face glyph/cmap cache data-race fix) and `github.com/belluikit/render v0.2.2` (rebased on go-text/render v0.2.1, repointed at the typesetting fork — needed because `go-text/render` shares `shaping.Output`/`*font.Face` types with typesetting, so a partial swap would mismatch). Both repos are public, so `GOPRIVATE` is no longer required (verified: clean-cache fetch via proxy.golang.org + sum.golang.org). Each fork's `main` mirrors clean upstream; the `trendvidia` branch (default) carries the fork delta.~~
Follow-ups: ~~**pre-rebrand tracker #107** drop the now-unnecessary `GOPRIVATE` (forks are public) — done 2026-06-23: removed from README and all 5 CI workflow steps (the conditional `GO_MODULES_TOKEN` rewrite stays as forward-looking defense for any future *private* dep).~~ **pre-rebrand tracker #106** (fork posture) — **the original "retire once go-text/typesetting#256 lands" exit condition is dead**: upstream **closed pre-rebrand tracker #256 unmerged** as policy (`font.Face` is intentionally *not* concurrent-safe; toolkits must synchronize themselves — same precedent as go-text/typesetting#197), so the data-race fix will never be upstreamed and the fork is effectively permanent. The only retire path now is implementing fyne-side `Face` synchronization (wrap/guard the shared `Face`), tracked separately as **pre-rebrand tracker #110**. **Posture chosen: keep the fork, stay current.** Rebased the fork onto upstream `main` HEAD (`684cf8f`, post-v0.3.4 — note v0.3.4 was *behind* the prior fork base) carrying the data-race fix forward, and bumped fyne to `trendvidia/typesetting v0.3.5` + `trendvidia/render v0.2.3` (2026-06-23). Each fork's `main` mirror was refreshed to upstream HEAD; re-sync by re-rebasing the `trendvidia` branch + re-tagging when upstream moves.

~~**CI workflows have never executed:**~~
Resolved 2026-06-21: all workflows run green (Platform Tests ×{linux, macOS, windows-x64, windows-arm64}, Static Analysis, Web Tests, Mobile Tests). **Migrated fully to self-hosted runners** (zero GitHub-hosted minutes): Linux on `trader` (3 systemd runner instances), macOS on a logged-in Mac mini, Windows x64 on `windows_omen` and arm64 on `windows_parallels` (scheduled tasks in the interactive desktop session, isolated `HOME` so the user's ssh `insteadOf` git rewrite doesn't break `actions/checkout`). Setup recipe + gotchas live in [.github/SELF_HOSTED_RUNNERS.md](.github/SELF_HOSTED_RUNNERS.md).

**Self-hosted-runner stabilisation** (the shared-host issues GitHub-hosted VMs never expose), all fixed:
- `xvfb-run -a` — parallel runners colliding on Xvfb display `:99`.
- `flock /tmp/fyne-ci-apt.lock` around apt — concurrent `apt-get update` failing on the lists lock (`DPkg::Lock::Timeout` only covers the dpkg lock).
- `cache: false` on `actions/setup-go` — its tarball restore untarred over the persistent on-disk Go cache (`tar: File exists`).
- `concurrency:` groups + `push` restricted to `trendvidia` — duplicate `push`+`pull_request` runs and stale runs were oversubscribing the 8-core box.
- Single Go version (`stable`) on the two heaviest `-race` workflows, matching `windows_tests`.
- 16 GB swap on `trader` — OOM-killed `-race` linkers (`link: cannot write …: EOF`).
- windows/arm64 specifics: `-race` is unsupported there (dropped for that leg), and `internal/driver/mobile` is skipped (golang.org/x/mobile pulls a Windows ANGLE binary Google doesn't ship for arm64).

~~**Cyclomatic complexity over the gocyclo limit (pre-rebrand issue #4):**~~
Done 2026-06-21 (pre-rebrand PR #12): all 11 functions over 30 refactored below it by pure restructuring — theme size/colour switches became lookup maps, the GL paint walks moved onto state-struct methods, and the widget dispatch/loops were split into helpers (e.g. `denseTheme.Size` 45→2, `glCanvas.paint` 43→4, `renderNode` 41→21). `gocyclo -over 30 .` is empty repo-wide and the CI gate is restored to 30.

**Platform-specific test failures (first CI runs, 2026-06-10):**
- ~~Linux: `TestMenu_Layout/menu_with_shortcuts`~~ Fixed 2026-06-11: the `layout_shortcuts_other` masters were untouched pre-fork upstream files (old gradient-strip shadows, old metrics); the subtest silently returns on darwin and windows (upstream symbol-font TODO at menu_desktop_test.go), so Linux was the only platform that ever compared them — and Linux CI had never run. Masters regenerated on Linux. The darwin/windows skip is still in place and worth revisiting.
- ~~Windows~~ Green as of 2026-06-10: the job was missing the `ci` build tag, and the mobile package's TestMain lived in a file excluded on windows+ci (now in untagged main_test.go).
- ~~Mobile tag: AppTabs/theme golden tests fail.~~ Fixed 2026-06-11: two causes — the `migrated_fynedo` build tag had lost its tagged file so every `fyne.Do` in mobile-tag tests took the error path (dozens of binding/widget failures), and the mobile golden masters were stale relative to the text-metrics change desktop already absorbed. Tag wiring restored, goldens regenerated, desktop-pointer tests guarded. Mobile Tests workflow green.

**Flaky on CI (intermittent, ~1 in 6 full-suite runs) (pre-rebrand tracker #33):**
`TestList_ThemeOverride` pixel-diff on ubuntu-latest; never reproduced locally (30x isolated, 4x full-package on the Linux VM). Suspected race between theme-override refresh queueing and the capture; an explicit Refresh was added before the capture as a stabilizer. If it recurs, the root cause hunt starts at the canvas refresh queue vs Capture ordering in the test driver.

**~~Flaky on CI: `app.TestPreferences_Save` (and siblings) on linux~~ Fixed 2026-06-23 (PR pre-rebrand tracker #98 / pre-rebrand tracker #94):**
Surfaced on pre-rebrand PR #91 (pre-rebrand tracker #90), whose diff touched zero lines under `app/`: only `platform_tests (stable, linux)` failed (`expected "value", actual ""`, coverage line 61%) while macOS/Windows/mobile/web passed on the same commit; re-running the linux job went green. Root cause: under `-tags ci` the config root is a single process-wide temp dir, so every `TestPreferences_*` using app-ID `"dummy"` shared one `preferences.json`; cleanup stopped the file watcher but not the 100ms debounce timer (`resetSavedRecently` re-arms on every save), so a prior test's leaked timer (e.g. `TestPreferences_Remove`, which removes `keyString`) fired during a later test and clobbered the shared file. Fix: namespace each test's storage id by `t.Name()` and cancel the debounce timer in cleanup. Verified with `go test -race -tags ci -count=30 -run TestPreferences ./app/`.

**Mobile/wasm runtime validation (pre-rebrand tracker #34 mobile on-device; pre-rebrand tracker #7 wasm GL stubs):**
Both targets compile (fixed 2026-06-10) but have never executed the FBO/dirty-region/shadow pipeline on a device or browser. **Update (2026-06-23): the wasm target has now been exercised in a browser and the WebGL backend renders an interactive UI — see §11 / pre-rebrand tracker #95. Update (2026-06-24): the wasm GL context no longer stubs FBO operations — real FBO support landed (pre-rebrand tracker #7/#34, pre-rebrand PR #134), so shadows/dirty-region/RepaintBoundary are wired on web; browser-verify of those paths is pending (pre-rebrand tracker #133). Mobile on-device validation (pre-rebrand tracker #34) is still outstanding. Update (2026-06-26, v2.10.0): validated the imported tag end-to-end on **wasm** (headless WebGL — renders + virtualised deep-scroll, zero console errors) and **Android** (Pixel_3a API 34 emulator — renders + virtualised touch-scroll). **iOS** compiles/links/launches but the simulator's deprecated GLES on Apple Silicon makes it unverifiable there — real-device verify tracked by pre-rebrand tracker #195. Surfaced: a real mobile bug (pre-rebrand tracker #193 — incomplete `fyne.Do` migration: GUI mutations in `main()`/widget setters run off the goroutine the mobile loop registers as main, flooding the thread guard on startup + interaction; non-fatal, redispatched via `DoAndWait`) and toolchain/build gotchas (pre-rebrand tracker #194 — wasm needs `-tags osusergo` for the Go 1.26 `os/user` regression; deprecated CLI APK compresses `resources.arsc` (repackage for API 30+); host `CGO_LDFLAGS` leaks the Apple-ld flag into the Android NDK link).**

---

## 7. Code Quality & Tech Debt

~~**TODO/FIXME comments indicating incomplete features:**~~
~~- `widget/button.go:560` — "TODO support disabling bitmap resource not just SVG"~~
~~Fixed: removed SVG guard — `NewDisabledResource` is now applied to all icon resources (SVG colorization for SVGs, widget-level opacity for bitmaps).~~
~~- `widget/richtext.go:73` — "TODO improve pre-rebrand tracker #2464" (minsize/scroller coupling)~~
~~- `widget/richtext.go:828` — "TODO resolve this in a similar way to pre-rebrand tracker #2991"~~
~~Fixed: replaced dangling TODO markers with upstream issue references for traceability.~~
~~- `widget/richtext_objects.go:792` — "TODO maybe return ---\n?"~~
~~Fixed: removed TODO — returning "" is correct (separators are visual, not text content).~~
~~- `widget/popup.go:294,300` — scroll handling for oversized popups~~
~~Fixed: popup now clamps its size to canvas bounds so content doesn't overflow.~~
~~- `widget/tree.go:272` — "scrolling to a node should open all parents"~~
~~Fixed: `ScrollTo()` now calls `openAncestors()` which walks the tree to find the path to the target node and opens all closed ancestor branches before scrolling.~~
~~- `widget/fileicon.go:74` — "remove background when SetSelected is gone"~~
~~Fixed: removed deprecated `Selected` field, `SetSelected()` method, and the background rectangle from the renderer. No callers existed.~~

~~**Deprecation without clear migration:**~~
~~- `container.go:26-57` — 3 deprecated constructors, migration path unclear~~
~~Fixed: added `Since` tags to all three deprecated constructors.~~
~~- `window.go:122-125` — `Clipboard()` deprecated, no `Since` tag~~
~~Fixed: `Clipboard()` was already removed during the Window interface cleanup.~~
~~- `text.go:43-47` — `TextTruncate` deprecated for `TextTruncateClip` across different enums~~
~~Fixed: added `Since: 1.0` tag to the `TextTruncate` deprecation.~~

~~**Preferences docstring typo:**~~
~~- `preferences.go:60` — says "SetIntList saves a list of string values", should say "int values"~~
~~Fixed: the old `SetIntList` API with the typo was removed during the Preferences interface cleanup (replaced with generic `SetPref`/`PrefList`).~~

~~**Font cache unbounded (`internal/painter/font.go`):**~~
~~Fixed: added `lastAccess` timestamp to `FontCacheItem`. `SweepFontCache()` evicts entries not accessed within `ValidDuration`. Registered as a `CleanHook` in the cache `Clean()` cycle.~~

~~**Widget renderer cache cleanup gap (`internal/cache/widget.go`):**~~
~~Fixed: `CleanCanvas()` now calls `destroyExpiredRenderers()` after cleaning canvas-associated widgets, ensuring expired renderers with lost canvas associations are cleaned up at window close.~~

---

## 8. RTL Text Direction & Layout Mirroring

HarfBuzz in the go-text/typesetting fork fully supports Arabic shaping (`ot_shape_arabic.go`), and `di.Direction` defines `DirectionLTR`/`DirectionRTL`/`DirectionTTB`/`DirectionBTT`.

> **Update (2026-06-21): Phase 1 done — and the original premise was wrong.**
> Fyne does *not* hardcode `DirectionLTR` "in every shaping call". The
> `trendvidia/typesetting` fork's `shaping.Segmenter.Split()` (called inside
> `walkString`) already runs the bidi algorithm (UAX #9), detects script per
> run, and resolves language per run — and those per-run values flow straight
> into HarfBuzz via `shapeCallback`. So Arabic/Hebrew already join, reorder, and
> select script-specific features. The lone hardcoded `DirectionLTR` was only
> the bidi *paragraph base direction*, which the segmenter overrides per run.
> See the Completed section for what landed (base-direction plumbing,
> `TextStyle.Direction`, `lang.IsRTL()`). This also closes most of §10.

**Text direction:**
- Detect script from text content or locale and pass correct `shaping.Input.Direction` to HarfBuzz
- Propagate direction through `canvas.Text`, `widget.Label`, `widget.RichText`, `widget.Entry`
- Apply Unicode Bidirectional Algorithm (UAX #9) for mixed LTR/RTL text

**Layout mirroring:**
- `TextAlignLeading`/`TextAlignTrailing` exist but are applied as fixed left/right — make them direction-aware
- Add `LayoutDirection` to `fyne.App` or `fyne.Canvas` (auto-detected from locale, overridable)
- Mirror horizontal layouts (`HBox`, `Border`, `Form`, `Grid`) when direction is RTL ✅ Phase 3
- Flip scroll direction, navigation order, and icon placement for RTL ✅ Phase 4 (pre-rebrand tracker #21)
- Mirror custom-renderer widget internals + flip their spatial arrow nav ✅ Phase 5 (pre-rebrand tracker #122, pre-rebrand tracker #123)

### Starter brief (for a dedicated RTL session)

RTL is large and spans three subsystems (shaping, bidi, layout). Tackle it in
phases — **do not attempt it in one pass.** Note that §10 (Script & Language
Tag Propagation) is really part of the same foundation: HarfBuzz needs
*direction + script + language together*, so fold §10 into Phase 1.

Recommended first step in the new session: a short **design/plan pass** (the
Plan agent is good for this) to confirm the call sites below and decide the API
surface — where `LayoutDirection` lives (`fyne.App` vs `fyne.Canvas`), and how a
per-widget language/direction override is exposed — *before* writing code.

**Phase 1 — Shaping foundation (✅ DONE 2026-06-21; absorbed §10).** Arabic/Hebrew
join and reorder; script + language reach HarfBuzz. What was actually needed
turned out to be small, because the segmenter already did the bidi/script/
language detection (see the Update above). Delivered:
- `fyne.TextStyle.Direction` (`TextDirectionAuto`/`LTR`/`RTL`); zero value Auto
  is backward compatible. Flows through `canvas.Text`/`Label`/`RichText`/`Entry`
  for free because it rides the existing `TextStyle` struct.
- `lang.IsRTL()` / `lang.IsRTLLocale()` — locale→direction via the script of the
  active locale tag (honours `SetLocaleForTests`, consistent with the other
  `lang` formatting helpers).
- `resolveBaseDirection()` in `internal/painter/font.go` maps style+locale to the
  bidi paragraph base; wired into `walkString` (also sets `shaping.Input.Language`
  from the locale for §10's script-feature selection) and the `DrawStyledRuns`
  metric pass. `DrawString`/`MeasureString` inherit it via `walkString`.
- Tests: `lang/direction_test.go`, `internal/painter/font_rtl_test.go`
  (deterministic direction/script assertions — glyph coverage is font-dependent
  so a cross-platform Arabic *pixel* test was deliberately avoided).

Not done in Phase 1 (correctly deferred): per-run *visual ordering* across many
mixed runs is handled by the segmenter (runs are returned in visual order), but
treating the whole paragraph's bidi reordering at the widget level — wrapping,
caret/selection mapping, `canvas.StyledText` cross-run RTL — is Phase 2.

**Phase 2 — Bidi at the widget level (✅ MERGED 2026-06-22, PR pre-rebrand tracker #23; pre-rebrand tracker #19).**
Built one shared per-line primitive — `painter.LineVisualLayout` — on the
typesetting fork's existing bidi segmenter (`Segmenter.Split` returns runs in
visual order; no new dependency). It returns a logical-rune↔visual-x caret table,
per-rune glyph extents, and helpers `CaretX`/`HitTest`/`VisualNeighbor`/
`SelectionSpans`, gated by a cheap `MayContainRTL` pre-filter so pure-LTR text
keeps the exact legacy path (incl. tab expansion). Wired into:
- `canvas.StyledText` — `DrawStyledRuns` now bidi-reorders runs (visual order via
  the segmenter) instead of drawing them in array order.
- `widget.Entry` — caret pixel position (`lineSizeToColumn`→`rtlCaretX`), click
  hit-testing (`cursorColAt`→`HitTest`), multi-rectangle selection for visually
  disjoint ranges (`buildSelection`→`rtlSelectionSpans`), and visual-order
  Left/Right arrow navigation (`visualColNeighbor`→`VisualNeighbor`).
- Wrapped/multi-row text resolves per row automatically (helpers key on the
  row's own runes). Forward caret affinity at direction boundaries.

**Phase 2.1 — Cross-segment bidi within one RichText row (✅ MERGED 2026-06-22, PR pre-rebrand tracker #27; pre-rebrand tracker #24).**
Extended the widget layer to the multi-segment case Phase 2 deferred: a row built
from several differently-styled `TextSegment`s (e.g. an LTR span beside an RTL
span) is now bidi-resolved across segment boundaries. New primitive
`painter.StyledLineVisualLayout` assembles the row into one logical rune sequence,
runs a single bidi pass for visual order, and shapes each style run with its own
font/size — returning the same `LineLayout` (caret table + per-rune extents) as
the single-style path, indexed by global logical offset. `RichText.rowVisualLayout`
unifies the single- and multi-segment cases; `rtlCaretX`/`rtlSelectionSpans`/
`visualColNeighbor` and `selectable.cursorColAt` route through it, so caret x,
click hit-testing, multi-rect selection and visual arrow-nav all work across the
boundary. The bidi ordering loop in `DrawStyledRuns`/`styledRunVisualOrder` was
factored into a shared `lineVisualSeq`. Each style run is still shaped
independently, so cross-run cursive joining stays deferred to pre-rebrand tracker #26. Tests: a
same-style split reproduces `LineVisualLayout` exactly (pins the model), plus
hit-test round-trip, visual walk and disjoint-selection on a genuinely
multi-style mixed row.

Scoped to rows of all-`TextSegment`s; the remaining multi-segment gaps it
surfaced were grouped under **Phase 2.2**
(pre-rebrand tracker #44).

**Phase 2.2 — Non-text segments & exact alignment in bidi rows (✅ MERGED 2026-06-22, PR pre-rebrand tracker #45; pre-rebrand tracker #44).**
Closed the two open multi-segment gaps from Phase 2.1.
**Non-text segments** (pre-rebrand tracker #41): a row
mixing RTL text with an inline `HyperlinkSegment`/image is now bidi-resolved
across the boundary. `StyledRun` gained `Box`/`BoxWidth`;
`StyledLineVisualLayout` interleaves each non-text segment as an opaque box — a
`U+FFFC` placeholder gives it a slot in the single bidi pass and it consumes its
measured width, but it contributes **no caret column**, so the caret/extent
tables (and `row()`/`SelectedText`/`rowLength`/the all-text fast path) stay
text-only. `boxExtent` + `SelectionSpans` fill a box iff the rune it trails is
selected, matching the legacy single-rectangle behaviour; `rowStyledRuns` emits
box runs instead of bailing on non-text (still bails on concealed/password).
**Alignment-offset drift** (pre-rebrand tracker #42):
`selectable.alignOffsetX` now uses `RichText.rowWidth`, which sums each segment's
own-style measurement (and reuses the bidi layout's total width for RTL rows), so
center/trailing-aligned multi-segment rows hit-test and draw selection rects
exactly. Tests: painter box width/caret/selection (LTR + RTL), widget
RTL-row-with-link layout/width/selection/hit-test, per-segment alignment offset.

It surfaced two fast-follows (both filed under pre-rebrand tracker #44): copied selection drops a
non-text segment's text across the box (`SelectedText` is text-only,
pre-rebrand tracker #46) and caret height ignores a
tall inline image (`rowVisualHeight` skips non-text,
pre-rebrand tracker #47).

**Phase 2.3 — Opaque-box selection-copy fidelity (✅ MERGED 2026-06-22, PR pre-rebrand tracker #49; pre-rebrand tracker #46).**
Closed the pre-rebrand tracker #46 fast-follow: copying a selection that visually covered an inline
non-text segment dropped the segment's text, because `SelectedText()` rebuilt the
string from the text-only `row()` ("see [docs] now" → "see  now"). New
`RichText.rowSelectedText(row, startCol, endCol)` walks the row's segments in
logical order, emits each `TextSegment`'s selected runes, and splices in a
non-text segment's `Textual()` when its opaque box is covered — using the **same**
owner-rune test `SelectionSpans` uses to fill the box rectangle (a box's owner is
the text rune it trails, `counted-1`; included iff `owner ∈ [startCol, endCol)`).
So the copied text now matches the highlighted rectangles for both LTR and RTL
rows. `SelectedText()` constrains only the first/last rows to their columns and
delegates per-row extraction to the helper. Tests: inline-link copy (LTR + RTL),
owner-boundary inclusion/exclusion. Surfaced one fast-follow (filed under pre-rebrand tracker #44):
standalone **block** non-text segments (own-row image/table/separator, zero text
columns) are still skipped by both selection highlight and copy
(pre-rebrand tracker #50).

**Phase 2.4 — Mixed-row caret height & block-segment selection (✅ MERGED 2026-06-22, PR pre-rebrand tracker #52; pre-rebrand tracker #47, pre-rebrand tracker #50).**
Closed the last two pre-rebrand tracker #44 sub-issues. **Caret height** (pre-rebrand tracker #47):
`rowVisualHeight` measured only `TextSegment`s, so on a row whose tallest element
is an inline image/box the bidi caret (`rtlCaretX`) drew at the text height; it now
measures non-text segments via the same cached visual height that supplies their
box width, so the caret spans the full row height (matching `RowGeometry`).
**Block-segment selection** (pre-rebrand tracker #50):
a standalone block non-text segment (image/table/separator) that starts a fresh
row sits alone with **zero text columns** (box owner `-1`), so `SelectedText`
dropped its `Textual()` and `buildSelection` drew a zero-width sliver. Such a row
is now handled **all-or-nothing**: when it is fully spanned (interior, `row <
selectEndRow`) `SelectedText` splices the new `RichText.rowBlockText(row)` and
`buildSelection` draws a full-box-width highlight — both gated on the same
condition so copy and highlight stay in sync (LTR + RTL). Blank text rows
(`rowWidth == 0`) keep the legacy sliver path; a block trailing inline text on a
shared row was already covered by pre-rebrand tracker #46. Tests: tall-inline-box caret height;
block-row copy + full-width highlight, last-row exclusion (copy/highlight agree),
RTL surrounding rows. Surfaced one fast-follow: a block on the **last/only** row
can't be fully selected (zero-column rows have a single caret position, so
select-all ending on a trailing block omits it)
(pre-rebrand tracker #53).

With pre-rebrand tracker #41/#42/#46/#47/#50 merged, the **pre-rebrand tracker #44 epic is complete**; arbitrary
multi-segment RichText rows are bidi-correct for layout, alignment, hit-testing,
caret position/height, and selection copy/highlight.

**Phase 2.5 — Last/only-row block selection (✅ MERGED 2026-06-22, PR pre-rebrand tracker #55; pre-rebrand tracker #53).**
Closed the pre-rebrand tracker #53 fast-follow: pre-rebrand tracker #50 gated block-row copy/highlight on `row <
selectEndRow`, so a block on the **last** selected row — or the **only** row — was
excluded (select-all on content ending with a block omitted it; a sole-block
RichText couldn't be selected at all). A block row is atomic, so reaching its row
selects it: `SelectedText`/`buildSelection` now cover any block row a
non-degenerate selection reaches (gates dropped), and `selectAll()` gives a sole
trailing block a synthetic `cursorColumn = 1` so the range encloses it. New
`RichText.isBlockRow` centralises the zero-column-with-non-text-segment test.
Behaviour change: selecting *onto* a trailing block now includes it. Tests:
last-row reached, select-all trailing block, select-all sole block row. Surfaced
one fast-follow: triple-tap and single-row drag still can't select a standalone
block (same degenerate zero-column range, not yet given the synthetic column)
(pre-rebrand tracker #56).

**Phase 2.6 — Triple-tap & single-row block selection (✅ MERGED 2026-06-22, PR pre-rebrand tracker #58; pre-rebrand tracker #56).**
Closed the pre-rebrand tracker #53 fast-follow: the single-row interaction paths — triple-tap
(`selectCurrentRow`) and a drag confined to one block row — still produced a
degenerate zero-column range (`select == cursor` at column 0) and so selected
nothing, even though pre-rebrand tracker #53 had already made a standalone block selectable via
select-all and multi-row selection. Both paths now apply the same
synthetic-trailing-column treatment `selectAll` uses: when the target row
`isBlockRow` and the range would be degenerate, the cursor column is set to 1 so
the atomic block is enclosed. `SelectedText`/`buildSelection` already cover any
block row a non-degenerate selection reaches, so copy and highlight follow with
no further change. Tests: triple-tap selects+highlights a block row; a drag
within a block row selects+highlights it.

**Phase 2.7 — Cursor affinity at direction boundaries (✅ MERGED 2026-06-22, PR pre-rebrand tracker #60; pre-rebrand tracker #25).**
At an LTR↔RTL boundary a single logical caret offset has two valid visual x
positions; Phase 2 always picked the forward (downstream) one, so the caret
could not stick to the side the user arrived from — typing a character before an
RTL run threw the caret to the far side of the run instead of beside the new
glyph. `LineLayout` now records a parallel upstream caret table (`caretsBwd`, the
trailing edge of the preceding rune) alongside the forward `Carets`;
`assignCluster` fills both deterministically, so `Carets` is forward-affinity by
construction (was last-writer-wins) and identical away from boundaries. New
`CaretAffinity` with `CaretXAt(offset, affinity)` (downstream ≡ the legacy
`CaretX`) and `HitTestAffinity(x) → (offset, affinity)`, which weighs both
candidate positions so a click resolves to the side it landed on. `Entry` gains a
`cursorUpstream` bit plumbed into `CursorPosition` (`rtlCaretX`/`lineSizeToColumn`
took a `downstream` argument): set upstream on insert (typing/paste) so the caret
hugs the new text, reset to downstream by navigation keys, and set from the
clicked side via `cursorColAt`/`getRowCol`. Only the rendered caret x changes —
the logical column and navigation stop set are untouched. Tests: painter
affinity/`HitTestAffinity` units (boundary differs, endpoints fold, downstream ≡
`CaretX`, click resolves side, LTR stays downstream) and Entry caret-x/typing/
arrow-reset/click-side affinity.

**Phase 2.8 — Cross-run cursive joining (✅ MERGED 2026-06-23, PR pre-rebrand tracker #62; pre-rebrand tracker #26).**
Both styled-text call sites bidi-reordered runs but shaped each one
independently, so an Arabic word split across two same-font runs — e.g. to
colour part of it differently — did not join cursively at the seam: the boundary
glyphs took isolated forms instead of medial/initial/final. Adjacent runs that
share a face (size, style, font) and both `MayContainRTL` are now grouped and
shaped as one unit so joining survives the boundary. In `DrawStyledRuns`,
`groupStyledRunsForDraw` concatenates such runs while recording each rune's
resolved draw colour; the joined run is shaped once and `drawColoredOutput` draws
it as maximal same-colour glyph slices (each chained off the previous slice's end
x), so every part keeps its own colour. Group visual order reuses the existing
`styledRunVisualOrder`. In `StyledLineVisualLayout`, `joinAdjacentStyledRuns`
merges the same runs before shaping; concatenation order and rune count are
preserved, so the text-only caret model and global text-offset indexing are
unchanged. Merging is gated on both neighbours containing RTL, so pure-LTR text
and any face change keep the exact legacy one-run-at-a-time path, and boxes never
merge — the common LTR syntax-highlighting case is byte-for-byte unaffected.
Tests in `font_styledjoin_test.go`: split-vs-whole layout width/caret equality,
pixel-identical rendering of a split vs whole word, and grouping unit coverage,
each skip-guarded against missing Arabic glyph coverage (and confirmed to fail
without the fix).

This was the last RTL §8 Phase 2 follow-up. Remaining RTL work is the
independent Phase 3 (layout mirroring,
pre-rebrand tracker #20) and Phase 4 (polish,
pre-rebrand tracker #21). The low-priority
per-widget language override (pre-rebrand tracker #22)
is ✅ **DONE** — see the §10 update below.

**Phase 3 — Layout mirroring (✅ MERGED 2026-06-23, PR pre-rebrand tracker #64; pre-rebrand tracker #20; independent subsystem).**
Unified direction into one ambient concept: canonical `fyne.Direction`
(`DirectionAuto`/`LTR`/`RTL`), with `fyne.TextDirection` kept as an alias so
`TextStyle.Direction` reads naturally. The process-wide setting
(`fyne.SetLayoutDirection`/`fyne.CurrentLayoutDirection`) drives layout mirroring,
leading/trailing alignment **and** the default text base direction — all resolve
through the single `lang.IsRTLDirection()` (override else locale via `lang.IsRTL()`).
`resolveBaseDirection()` Auto now consults it, so an RTL override seeds text
ordering under an LTR locale (explicit `TextStyle.Direction` still wins).
`lang.ResolveAlign()` swaps `TextAlignLeading`/`Trailing` under RTL (an
involution, so LTR is identity) and is consulted by the software/GL painters
(`canvas.Text`, `canvas.StyledText`), `RichText`/`Label`, `Entry` (cursor),
`Hyperlink` (focus box) and `selectable` (selection highlight). `HBox`, `Border`,
`Grid` and `Form` mirror horizontally via a post-pass reflection
(`mirrorChildren`) gated on `lang.IsRTLDirection()`. Default LTR is byte-for-byte
unchanged (all reflection/swaps/seeds are no-ops), so existing visual baselines
are untouched. Follow-ups: runtime direction-change refresh landed in Phase 3.1
below (pre-rebrand tracker #65); per-canvas /
mixed-direction layout (pre-rebrand tracker #66;
out of scope here because text measurement is canvas-free) — since delivered
across Phases 6–7 and closed. Remaining
nuance: multi-segment `RichText` rows mirror by edge selection but keep within-row
segment order from the bidi layer. Phase 4 (pre-rebrand tracker #21)
still covers scroll direction, focus/nav order and icon placement.

**Phase 3.1 — Runtime direction-change refresh (✅ MERGED 2026-06-23, PR pre-rebrand tracker #86; pre-rebrand tracker #65).**
`fyne.SetLayoutDirection` was a process-wide atomic setter, so flipping the
direction after the UI was built left existing windows with stale mirroring,
leading/trailing alignment and text base direction until an unrelated relayout
happened. It now re-lays-out and repaints every running window when the direction
**actually changes**, reusing the theme-change fan-out: a root-package
registration hook (`fyne.RegisterLayoutDirectionRefresh`, mirroring the existing
`RegisterTextMeasure` pattern to dodge an import cycle into the app/driver
internals) whose callback — installed by the app and test packages — calls
`settings.apply()`. Both the GLFW (`internal/driver/glfw/loop.go`) and mobile
(`internal/driver/mobile/driver.go`) drivers already listen on `apply()` and do a
full relayout + repaint + RepaintBoundary-FBO invalidation, so every window
refreshes on every driver with no new canvas-walk code. Setting the same direction
is a no-op (atomic `Swap` guard), and with no app running the change just takes
effect for UI built afterwards; refresh follows the same main-goroutine threading
rule as `Settings.SetTheme`. Tests: hook fires once on a real change / no-op on a
repeat set / fires again on change-back, plus a test-app check that the settings
listeners actually fire. Surfaced one follow-up: the analogous live refresh for
`lang.SetLocale` (translations/formatting/locale-derived direction), which landed
under pre-rebrand tracker #87 (PR
pre-rebrand tracker #89; previously mis-filed under pre-rebrand tracker #65),
with static-text re-translation completed in
pre-rebrand tracker #90 (PR
pre-rebrand tracker #91) — see §10.

**Phase 4 — Polish (✅ MERGED 2026-06-24, PR pre-rebrand tracker #121; pre-rebrand tracker #21).**
The affordance polish that sits above shaping (Phase 1), bidi (Phase 2) and
layout mirroring (Phase 3), all gated on the single `lang.IsRTLDirection()`
resolver so the default left-to-right path is byte-for-byte unchanged.

- **Icon placement.** A new `internal/svg.FlipHorizontal` reflects an SVG about
  its viewBox centre by wrapping every top-level drawable in a group carrying a
  `matrix(-1,0,0,1,w,0)` transform — piggybacking on the same typed-XML round-trip
  `Colorize` already uses, so every bundled icon survives it losslessly. A thin
  `theme.directionalResource` wrapper marks the *directional* icons (back/forward
  arrows, the submenu chevron `MenuExpand`, the mail reply/forward glyphs) and,
  **only** under RTL, returns flipped `Content()` and an `rtl_`-prefixed `Name()`
  (a distinct SVG-raster-cache key, so the mirrored art never collides with the
  upright one and a runtime direction flip repaints to the right variant).
  Non-directional icons (search, settings, clock) and vertical arrows
  (`MoveUp`/`MoveDown`, drop-down carets) are deliberately left upright. Media
  transport glyphs are also left upright, matching the platform convention that
  play/skip do not mirror under RTL.
- **Scroll direction.** `internal/widget.Scroll` now opens horizontal content at
  its **right edge** the first time it overflows under RTL (`pinToRTLStart`, a
  one-shot that a later user/explicit scroll supersedes via `rtlPinned`), and
  reverses the horizontal wheel/drag delta (`resolveScrollDelta`) so the natural
  forward scroll advances toward the content's left (far) end. The offset stays
  measured from the left, so the scrollbar thumb and the edge shadows — which
  track clipping, not reading order — need no change and remain correct.
- **Focus / navigation order.** `internal/app.FocusManager` Tab/Shift+Tab
  traversal is re-sequenced into **visual** order under RTL: focusables are
  collected in tree order (with their absolute positions and own sizes — the
  walker's trailing size is the clip rect, so `obj.Size()` is used), grouped into
  rows by vertical-span overlap, ordered top-to-bottom, and within each row
  right-to-left. A vertical column has no rows to reverse and is left untouched,
  while a toolbar / segmented button / navigation bar reverses; this is generic,
  so it covers any horizontally-arranged focusables without per-widget code. The
  LTR path skips the reorder entirely (identical to before). `widget.Select`'s
  Left/Right option cycling is flipped under RTL to match the reversed reading
  order.
- **Deferred → ✅ landed in Phase 5 (below).** Per-widget *spatial* arrow
  navigation for widgets whose internal layout is not yet RTL-mirrored
  (`GridWrap`'s grid, `SegmentedButton`) was left alone here on purpose: flipping
  their Left/Right keys without mirroring their pixels would desync keys from what
  the user sees. It landed together with RTL mirroring of those custom renderers —
  mirror the internal layout first, then flip the keys to match — in Phase 5
  (pre-rebrand tracker #123), alongside the
  `TopAppBar` RTL layout bug (pre-rebrand tracker #122).
  Per-canvas direction (pre-rebrand tracker #66) is a
  distinct design item, since closed across Phases 6–7 (see below).

**Phase 5 — Custom-renderer widget mirroring (✅ MERGED 2026-06-25, PRs pre-rebrand tracker #172 / pre-rebrand tracker #173; pre-rebrand tracker #122 / pre-rebrand tracker #123).**
The custom-renderer MD3 widgets positioned their children from a fixed left origin
and never consulted `lang.IsRTLDirection()`, so they stayed LTR under an RTL
layout. Phase 5 sweeps them, gated on the same one resolver so the LTR path stays
byte-for-byte unchanged.

- **Shared helper.** Phase 3's `layout.mirrorChildren` post-pass is promoted to
  exported `layout.MirrorChildren`, and a `widget.mirrorIfRTL(objects, width)`
  wrapper (`widget/direction_internal.go`) is the custom-renderer analogue.
  Reflecting each child's x-span (`new_x = width − x − childWidth`) **reverses** a
  horizontal sequence, which is exactly the RTL arrangement; full-width
  backgrounds/dividers map onto themselves, so a blanket pass is safe.
- **`TopAppBar` (pre-rebrand tracker #122).** Its
  title `canvas.Text` was `Move`d but never `Resize`d, so the painter's
  leading→trailing flip drew it over the navigation icon. The title is now sized
  to its measured box (making the alignment flip a no-op) and the renderer applies
  `mirrorIfRTL`. The stale-`canvas.Text`-width hazard recurs in a few element-row
  widgets, fixed the same way.
- **Uniform post-pass.** `BottomAppBar`, `NavigationBar`, `ButtonGroup`,
  `SplitButton`, `Chip`, `AuthButton`, `Badge`, `SpinBox`, `Check` and the
  `SegmentedButton` outer row mirror via `mirrorIfRTL` at the end of `Layout`
  (`Chip`/`SplitButton`/`AuthButton` labels sized first).
- **Hand-tuned.** `GridWrap` is virtualised, so the column flip lives in the grid
  math (`x = (colCount−1−col)·stride` under RTL; rows and scrolling unchanged).
  `Carousel` mirrors via the blanket pass (prev/next swap, dots reverse, the
  centred content page maps onto itself). `SegmentedButton` bakes its pill corner
  rounding from `first`/`last`, so that assignment is swapped under RTL to keep the
  pill ends on the outer edges after the row reflects.
- **Spatial arrow nav.** `GridWrap` `KeyLeft`/`KeyRight` swap under RTL (extracted
  `focusPrevItem`/`focusNextItem`), and `SegmentedButton` gains `event.Focusable`
  with RTL-aware arrow-key selection cycling (the `widget.Select` pattern) and a
  primary-coloured focus ring on the selected segment.
- **No change** (recorded): vertical widgets (`NavigationRail`, `Accordion`) and
  standard-layout-based widgets (`Toolbar`, `StatusBar`, `Button`, `Form`) already
  mirror or need none.

Tests are position-based rather than font-dependent pixel goldens (per the §10
convention); `widget/`+`layout/` suites pass under `-race`. Surfaced follow-ups:
`SegmentedButton` click-to-focus (✅ MERGED 2026-06-25, PR pre-rebrand tracker #180; pre-rebrand tracker #174) —
a segment tap now moves keyboard focus to the parent group via
`focusIfNotMobile` (mirroring `widget.Select.Tapped`), so click-then-arrow
works without a prior Tab; and end-to-end visual RTL verification in the gallery
(pre-rebrand tracker #175, still open). A design
write-up for per-canvas / mixed-direction layout was posted to
pre-rebrand tracker #66 (since delivered across
Phases 6–7 and closed).

**Phase 6 — Per-subtree / per-window direction (✅ MERGED 2026-06-25, PR pre-rebrand tracker #177; pre-rebrand tracker #66).**
Until now direction was a process global, so a mixed-direction UI (an LTR
settings dialog inside an RTL app) was impossible. The blocker was that text
measurement/shaping and the `Layout` interface are canvas-free, so a per-canvas
direction would desync a globally-measured `MinSize` from a per-canvas render.
The fix reuses the per-widget `ThemeOverride` two-seam pattern rather than
threading a canvas through the measurement API:

- An ambient direction is layered **under** the single `lang.IsRTLDirection()`
  resolver via a per-goroutine stack (`internal/uidir`, `Push`/`Pop`/`Current`
  keyed by `thread.GoroutineID()`, the direction analogue of the
  `internal/theme` rendering-theme stack). Because every consumer already
  funnels through `IsRTLDirection()` (mirroring, `lang.ResolveAlign`,
  `resolveBaseDirection`), this scopes them all with no call-site changes; the
  unscoped path stays a single global read.
- `container.DirectionScope` / `NewDirectionScope(content, dir)` pushes its
  direction around the renderer's `Layout`/`MinSize`, and is pushed per-node in
  `common.Canvas.EnsureMinSize` next to the existing `ThemeOverride` hook (the
  canvas drives `updateLayout` directly, bypassing the parent renderer push).
  `Canvas.SetDirection`/`Direction` (an optional `fyne.Canvas` interface) scopes
  a whole window: the per-canvas direction wraps both the shared `EnsureMinSize`
  walk and the GLFW paint walk, so a window scope also gets correct paint-time
  text base direction. Scopes nest (innermost wins) and compose with the global.
- Live in-place toggling without a relayout shares the global runtime-refresh
  limitation (pre-rebrand tracker #65).

**Phase 7 — Container-scope paint-time direction (Seam B) (✅ MERGED 2026-06-25, PR pre-rebrand tracker #182; pre-rebrand tracker #178).**
Phase 6 scoped a *container* `DirectionScope` at layout/measure time only —
paint is a separate per-driver tree walk, so inside such a scope the paint-time
base direction of mixed-bidi text (`resolveBaseDirection`) and the alignment of
slack-width standalone `canvas.Text`/`StyledText` primitives (`lang.ResolveAlign`
in the `gl`/`software` `draw.go`) still resolved against the process-global
direction. The fix mirrors the layout-time `EnsureMinSize` hook on the paint
side: each driver's paint walk pushes the scope's direction onto the per-goroutine
`uidir` stack around its subtree (`internal/driver.Push`/`PopDirectionScope`),
so the canvas-free consumers (`lang.IsRTLDirection`, `lang.ResolveAlign`) pick it
up with no call-site changes — both gaps close at once. The push/pop sit before
any RepaintBoundary/dirty-region early return so they always pair (`walkTree`
invokes both callbacks per node), and are replicated across the GLFW
(`glPaintWalk`), mobile (`mobileTreeWalker`) and software/test (`software.Painter`)
walks. `DirectionScope.ScopedDirection()` lets the painters detect the scope
structurally (an interface), avoiding a `container`→`internal/driver` import
cycle. Per-run `TextStyle.Direction` remains the escape hatch for mixed-bidi runs.
With this, a container `DirectionScope` is direction-correct at layout **and**
paint time on every driver. Tests: software-painter golden proving a
leading-aligned `canvas.Text` in an RTL container scope hugs the right edge while
the global stays LTR (verified to fail without the hook).

The one paint wrap this left desktop-only — the GLFW walk pushed `c.Direction()`
but the mobile `paintWindow` did not, so a *whole-window* `Canvas.SetDirection`
scope's paint-time base direction was desktop-only — was closed by the symmetric
mirror in mobile's `paintWindow` (✅ MERGED 2026-06-25, PR pre-rebrand tracker #185; pre-rebrand tracker #183).
**Both `DirectionScope` and per-canvas `Canvas.SetDirection` are now
direction-correct at layout and paint time on every driver, closing
pre-rebrand tracker #66.** The only remaining
limitation is live in-place direction toggling without a relayout, shared with the
global setting (pre-rebrand tracker #65); the new
direction is applied on the next relayout. The per-canvas paint path is also
golden-tested without a GPU (✅ MERGED 2026-06-25, PR pre-rebrand tracker #188; pre-rebrand tracker #186):
`software.Painter.Paint` wraps its walk in the canvas direction when the canvas
exposes the optional `Direction()` accessor (the software analogue of the GL
drivers' paint wrap), and the `test` canvas gained `SetDirection`/`Direction`
plus direction-scoped layout — so a leading-aligned `canvas.Text` on an
RTL-scoped canvas is verified to hug the right edge while the global stays LTR.
(The GL drivers' own paint path stays GL-bound and on-device-validated; the
`internal/uidir` mechanism is covered by that golden and the `uidir` unit tests.)

---

## 9. Input Method (IME) Support

Tracked in pre-rebrand tracker #35.

> **Update (2026-06-23): Phase 1 — composition core + WASM + mobile seam.**
> `widget.Entry` now has composition (preedit) state and a platform-agnostic
> driver interface:
> - New `event.Composable` interface (embeds `Focusable`) with `Compose`,
>   `ComposeCommit`, `ComposeCancel`, plus `event.PreeditEvent`. `Entry`
>   implements it.
> - `Entry` shows the in-progress preedit inline at the caret (underlined via
>   the existing `StyleRun` path) without touching `Text`, `OnChanged`, undo or
>   bindings — the committed text is inserted as a single edit on commit. Done
>   by splicing a *composed display string* through `syncSegments` (preedit
>   lives in the layout segment, never in `Text`).
> - Driver plumbing: `window.processCompose/Commit/Cancel` type-assert the
>   focused object to `event.Composable`.
> - **WebAssembly**: `compositionstart`/`compositionupdate`/`compositionend` DOM
>   events on the hidden input drive the focused widget (`device_wasm.go`); char
>   events are gated while composing to avoid double input.
> - **Mobile**: Go-side seam landed — `composition.Event` on the event pipe,
>   `keyboardComposing`/`keyboardComposeCommit` cgo exports, driver dispatch to
>   the focused `Composable`. The native Android `InputConnection`/composing-span
>   and iOS `UITextInput` marked-text wiring is a **device-tested follow-up**.
> - Test helpers `test.Compose/ComposeCommit/ComposeCancel`.

> **Update (2026-06-24): desktop wired (pre-rebrand tracker #125).**
> The GLFW binding was forked to `github.com/belluikit/glfw` (vendored GLFW
> bumped 3.3.10 → 3.4.0, adding the IME preedit API). The desktop driver registers
> `SetPreeditCallback` (preedit → `Compose`; empty preedit → `ComposeCancel`;
> committed text still arrives via the char callback → `TypedRune`), toggles the
> `GLFW_IME` input mode through `canvas.OnFocus` when a `Composable` widget is
> focused, and positions the candidate window via the new `event.PreeditPositioner`
> (`widget.Entry` implements it) translated through `SetPreeditCursorRectangle`.
> One binding covers macOS/Linux/Windows. Live per-platform IME still to be
> manually verified.

**Remaining (open follow-ups):**
- **Mobile native** (pre-rebrand tracker #126) —
  Android `InputConnection.setComposingText`/`finishComposingText` interception
  and iOS marked-text protocol, calling the existing `keyboardComposing`/
  `keyboardComposeCommit` exports; needs on-device testing.
- **WASM end-to-end browser verification** (pre-rebrand tracker #128).

---

## 10. Script & Language Tag Propagation

> **Update (2026-06-21): mostly resolved alongside §8 Phase 1.** The
> `shaping.Segmenter` already sets `Script` per run (from the Unicode script
> property) and resolves `Language` per run, and `walkString` now also seeds
> `shaping.Input.Language` from the active locale so matching-script runs use the
> locale's language.
>
> **Update (2026-06-22): §10 COMPLETE — per-widget language override landed
> (pre-rebrand tracker #22).** Added
> `fyne.TextStyle.Language` (a BCP 47 tag; zero value = the locale, unchanged
> behaviour). New `resolveLanguage(style)` in `internal/painter/font.go` mirrors
> `resolveBaseDirection`: a non-empty `Language` overrides the locale-derived
> `shaping.Input.Language`, otherwise the locale is used. Wired into all four
> shaping inputs (`walkString`, `LineVisualLayout`, `StyledLineVisualLayout`,
> `lineVisualSeq`) so the `DrawStyledRuns` draw path and measurement agree; the
> metrics cache key (`internal/cache/text.go`) now includes `language` so two
> styles differing only by language no longer collide. Like `Direction`, it only
> affects shaping of the text content, not font selection or bidi ordering.
>
> **Update (2026-06-23): app-level locale override exposed.** `lang.CurrentLocale()`
> (the override-aware dual of `SystemLocale`, pre-rebrand tracker #69)
> and the public `lang.SetLocale(fyne.Locale)`
> (pre-rebrand tracker #73) let an app override the
> active locale used for translations, formatting, text direction and shaping
> language (empty reverts to the system locale). Live refresh of already-built UI
> on a locale change now re-lays-out and repaints every running window
> (pre-rebrand tracker #87, PR pre-rebrand tracker #89),
> reusing the §8 Phase 3.1 mechanism — so direction mirroring, alignment, bidi
> ordering and format-on-refresh widgets all update live.
>
> **Update (2026-06-23): static-text re-translation landed
> (pre-rebrand tracker #90, PR pre-rebrand tracker #91).**
> The pre-rebrand tracker #87 relayout repaints a widget's *stored* text but does not re-evaluate the
> `lang.L`/`X`/`N` call that produced it, so a `widget.Label` built from a static
> `lang.L("Cancel")` kept its build-time string. Added the missing re-derivation
> half: `binding.NewLocalizedString(key, data...)` — a `binding.String` that
> re-runs `lang.Localize` on a locale change and pushes the new text to its bound
> widget (`widget.NewLabelWithData(binding.NewLocalizedString("Cancel"))`) — backed
> by `lang.AddLocaleChangeListener(fn) (remove func())`, a per-subscriber companion
> to the app-level `RegisterLocaleChangeRefresh` generalised from pre-rebrand tracker #87's single
> callback. The binding subscribes only while it has listeners (subscribe on first
> `AddListener`, unsubscribe on last), so a transient binding leaks no `lang`
> subscription and the default path stays zero-cost. Tests cover the subscription
> lifecycle, the binding re-derivation/unsubscribe, and the end-to-end Label
> scenario.
>
> **Update (2026-06-23): the two follow-ups landed, closing out live locale
> switching.** Key/plural binding variants — `NewLocalizedStringWithKey` (lang.X),
> `NewLocalizedPlural`/`NewLocalizedPluralWithKey` (lang.N/XN, count from a
> `binding.Int`, auto-exposing the live count as `{{.Count}}`) — shipped in PR
> pre-rebrand tracker #99 (pre-rebrand tracker #93).
> For text on surfaces with no data binding (a `widget.Button` label, a `Window`
> title, a `fyne.MenuItem`), `lang.BindText(key, apply, data...) (remove func())`
> calls the setter with the current translation immediately and on every locale
> change — PR pre-rebrand tracker #100
> (pre-rebrand tracker #92). Together with pre-rebrand tracker #87
> (relayout) and pre-rebrand tracker #65 (direction refresh), live locale **and** direction switching
> is now complete end to end.

`shaping.Input.Script` and `shaping.Input.Language` are never set in Fyne's text rendering paths. Without these, HarfBuzz cannot select script-specific OpenType features.

**Impact:**
- Serbian vs Russian Cyrillic may render with wrong localized glyph forms
- Script-specific ligatures and contextual alternates won't activate
- Vertical CJK features won't be selected (related to vertical text, but the tag issue is separate)

**Goal:**
- Detect script from text content via Unicode script property (or `language.Scripts` from the typesetting package)
- Set `shaping.Input.Language` from the app's locale or a per-widget language override
- Set `shaping.Input.Script` from detected script or explicit override
- Propagate through `walkString()`, `DrawString()`, `DrawStyledRuns()` in `internal/painter/font.go`

---

## 11. WebGL / WASM Web Target

Render real applications in the browser under `GOOS=js GOARCH=wasm`. Driven by a
planned [pxfed](https://github.com/trendvidia/pxfed) →
[fycoder](https://github.com/trendvidia/fycoder) port targeting the web; the one
hard dependency is that this fork can paint a UI under WebGL. This is the
runtime follow-through on the §6 "Mobile/wasm runtime validation" item
(pre-rebrand tracker #7).
**Tracking: pre-rebrand tracker #95.**

A `js/wasm` browser spike (2026-06-23; harness at
[`trendvidia/fycoder/examples/wasm`](https://github.com/trendvidia/fycoder/tree/main/examples/wasm))
found the *application* stack web-ready — fycoder plus a full editor app compile
and link, and IndexedDB storage + window creation work in-browser — but the
WebGL renderer is not. Two distinct defects surfaced one after another, which
implies the wasm GL path has never rendered a real UI; **expect more**.

- **~~Blocker 1 — FBO allocation~~ FIXED & adopted (pre-rebrand tracker #101).**
  `EnsureCanvasFBO` (`internal/painter/gl/canvas_fbo.go`) calls
  `TexImage2D(..., nil)`. The typed-nil `[]uint8` defeats gl-js's
  `SliceToTypedArray` `== nil` guard (a typed nil boxed in `any` is not `nil`)
  and becomes a zero-length `Uint8Array` — which WebGL rejects, and which
  previously panicked at `&s[0]` (`index out of range [0] with length 0`).
  Fixed in [trendvidia/gl-js#1](https://github.com/belluikit/gl-js/pull/1)
  (return `js.Null()` for empty/typed-nil slices, so `texImage2D(..., null)`
  allocates storage as on desktop). Adopted here by **redirecting the import** to
  the fork rather than a `replace`: the fork's `trendvidia` branch (now default)
  renames its module path to `github.com/belluikit/gl-js` (tag `v0.2.1`) and the
  three `*_wasm.go` consumers import that path directly, so there is no
  `replace`/import drift to maintain. The fork's `master` mirrors clean upstream.
- **~~Blocker 2 — non-conformant vertex-attribute state~~ FIXED (code), pending browser re-spike (pre-rebrand tracker #102).**
  `enableAttribArrays` (`internal/painter/gl/gl_wasm.go`) enabled each program's
  attributes once at creation; the painter never calls `DisableVertexAttribArray`
  and uses no VAOs. WebGL attribute-array enable state is global, so a later draw
  with a different attribute layout left an enabled attribute with no bound
  buffer → `INVALID_OPERATION: drawArrays: no buffer is bound to enabled
  attribute`, and no text/textured geometry rendered. Fixed with per-draw
  vertex-attribute reconciliation localised entirely to the WebGL context
  (`xjsContext`): it tracks which attribute locations are enabled and which were
  fed a buffer (`VertexAttribPointer`) since the last `UseProgram`, re-enables an
  attribute on feed, and disables any enabled-but-unfed array immediately before
  each `DrawArrays`. So only the attributes the current program supplies stay
  enabled at draw time, satisfying WebGL. Every painter draw already feeds all of
  its program's attributes before drawing, so the reconcile only ever drops
  genuine cross-program leftovers (e.g. scene3d's `color` location during a
  texture draw). Desktop GL parity is automatic — the change lives only in
  `gl_wasm.go` (built under `wasm`); the native/mobile
  contexts are untouched. Verified: `CGO_ENABLED=0 GOOS=js GOARCH=wasm go build`
  + `go vet` clean, desktop GL tests green. **Still to do:** re-run the browser
  spike (harness: `trendvidia/fycoder/examples/wasm`) to confirm a real UI paints
  now that both pre-rebrand tracker #101 and pre-rebrand tracker #102 are in — and watch for the next defect (the spike's
  "expect more" caveat). FBO-dependent paths (blur, dirty-region, RepaintBoundary)
  were stubbed on wasm at the time of this spike; real FBO support landed later
  (pre-rebrand tracker #7/#34, pre-rebrand PR #134) — see the resolved entry below.
- **~~Blocker 3 — browser event callbacks run off the main goroutine →
  `DoAndWait` deadlock on refresh~~ FIXED (code), pending browser re-spike
  (pre-rebrand tracker #113).**
  Found re-running the spike with pre-rebrand tracker #101+pre-rebrand tracker #102 in: a focus/blur event kills the
  program with a Go deadlock (`exit 2`). Desktop GLFW invokes window callbacks
  synchronously inside `pollEvents()` on the **main** goroutine, so a
  callback-triggered `Refresh` → `async.EnsureMain` runs inline. `fyne-io/glfw-js`
  instead delivers DOM events via JS event listeners on a **non-main** goroutine
  — and focus/blur/close run *synchronously* inside the `js.Func` callback (the
  rest are dispatched with `go`). The wasm window callbacks were inconsistent:
  some marshalled onto the main loop with the **waiting** `runOnMain`, while
  `keyPressed`/`charInput`/`focused` ran their `process*` handler inline. Either
  way the handler's first `Canvas.Refresh` → `EnsureMain` took the error fallback
  (`goroutine.go:49`, `fyne.DoAndWait`), which queues onto `funcQueue` and blocks
  on `<-done`. For the synchronous listeners the single-threaded wasm JS thread is
  parked *inside* the callback, so `runGL` can never drain the queue → deadlock.
  Fixed by marshalling **every** wasm window event callback onto the main loop
  *without* waiting via a new `runOnMainAsync` (`loop_wasm.go`,
  `runOnMainWithWait(fn, false)`): the JS callback returns immediately (the
  `funcQueue.In()` send is buffered), and `runGL` later runs the handler **on the
  main goroutine**, so `EnsureMain` runs inline — no `DoAndWait`, no deadlock, and
  the EnsureMain error-log slow path is gone for every event (focus/key/mouse/
  resize). FIFO `funcQueue` order is preserved, matching desktop GLFW which
  invokes callbacks synchronously on main inside `pollEvents`. Desktop parity is
  automatic — the change lives only in `*_wasm.go` (built under
  `wasm`). Verified: `GOWORK=off CGO_ENABLED=0 GOOS=js
  GOARCH=wasm go build` clean, desktop GLFW tests green, **and the browser spike
  confirmed** focus/blur no longer deadlocks the program.
- **~~Blocker 4 — wasm GL painter never initialises the ripple shader program →
  panic on first button tap~~ FIXED, browser-confirmed
  (pre-rebrand tracker #117).**
  Found immediately after pre-rebrand tracker #113, re-running the spike: tapping any MD3
  `widget.Button` killed the program (`exit 2`, nil-uniform deref inside
  `drawObject`→`drawRipple`, preceded by WebGL `bufferSubData: Buffer ... is
  null` warnings). The fork's shader programs are initialised per-context in
  `gl_es.go`/`gl_core.go`/`gl_gomobile.go`, but the **wasm** context has its own
  `painter.Init` in `gl_wasm.go` (the ES/core files are `&& !wasm`), and that copy
  set up every program *except* `rippleProgram`. A button tap shows the ripple, so
  the first post-tap paint walked into `drawRipple` with a zero `ProgramState`
  (zero `ref`, null vertex buffer, nil uniform maps) → null-buffer draw + nil
  `*UniformState` deref → panic. Fixed by adding the `rippleProgram` block
  (`ripple_es` shader, 16-float buffer, same uniform/attribute wiring) to
  `gl_wasm.go:Init`, mirroring `gl_es.go`. Ripple is a direct shader draw with no
  FBO, so it renders correctly on wasm. Verified: `GOWORK=off CGO_ENABLED=0
  GOOS=js GOARCH=wasm go build` clean, desktop `painter/gl`+`glfw` tests green.
  **Note:** `gl_wasm.go:Init` still omits the FBO-dependent blur programs
  (`blurHProgram`/`blurVProgram`/backdrop) that `gl_es.go` has, so a widget that
  draws a `canvas.BoxShadow`/`BackdropBlur` (e.g. an elevated card/FAB) would hit
  the same uninitialised-program panic. Those paths are FBO-gated (stubbed on
  wasm) and deferred; they need a graceful wasm no-op guard before any
  shadowed/blurred widget is safe on the web target
  (pre-rebrand tracker #118, relates pre-rebrand tracker #7).
- **~~Blocker 5 — FBO blur/shadow programs uninitialised → panic on
  `BoxShadow`/`BackdropBlur`~~ FIXED (pre-rebrand tracker #118).**
  The latent twin of pre-rebrand tracker #117 flagged above: `drawBoxShadow`/`drawBackdropBlur` run
  the two-pass FBO Gaussian-blur pipeline, but `gl_wasm.go:Init` never sets up
  `blurHProgram`/`blurVProgram` and the FBO ops are stubs, so any elevated
  card/FAB/`Modern` button would deref a nil `*UniformState` and panic (`exit 2`).
  Rather than initialise programs that still could not render (FBOs are stubbed —
  blocked on pre-rebrand tracker #7/#34), both draw paths now early-return behind a build-tagged
  `framebufferSupported` constant (`false` on wasm, `true` elsewhere): the
  shadowed/blurred widget renders without its shadow/frost instead of crashing.
  Desktop/mobile GL is unchanged (the const folds to a no-op). Verified:
  `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build` + desktop `painter/gl` tests green.
- **~~Real FBO support on wasm~~ DONE (pre-rebrand tracker #7/pre-rebrand tracker #34, PR pre-rebrand tracker #134, merged 2026-06-24).**
  The pre-rebrand tracker #118 guard's permanent replacement. Root cause: the painter's `Framebuffer`
  type was hard-coded `uint32` in `context.go`, but WebGL handles are `js.Value`
  wrappers, so `gl_wasm.go` could only stub the five framebuffer ops as no-ops.
  Fix mirrors the existing `Texture` platform-split — `Framebuffer` moves into
  `framebuffer.go` (`!wasm`: `uint32`) / `framebuffer_wasm.go` (`wasm`:
  `gl.Framebuffer`), each with a `defaultFramebuffer` sentinel (desktop zero /
  wasm `gl.NoFramebuffer`) replacing the two literal `BindFramebuffer(…, 0)`
  screen-binds. `gl_wasm.go` now drives the real gl-js framebuffer API and inits
  the `blur_h_es`/`blur_v_es` programs (shaders were embedded, programs never
  created); `framebufferSupported` flips to `true`, un-stubbing `drawBoxShadow`/
  `drawBackdropBlur`. Un-stubs canvas-FBO dirty-region culling, RepaintBoundary
  FBO caching, blur drop shadows and backdrop blur — all color-only RGBA8 FBOs,
  complete on WebGL1. Caveat: Scene3D's depth-texture FBO may be incomplete on
  WebGL1 (no `WEBGL_depth_texture`), but `checkFBOComplete` only logs, never
  panics — tracked separately. Desktop/mobile unchanged (`Framebuffer` folds back
  to `uint32`). Verified: `CGO_ENABLED=0 GOWORK=off GOOS=js GOARCH=wasm go build`
  (gl package + full `cmd/fyne_settings`) + desktop `painter/gl`+`glfw` tests
  green. **Browser-verify of the now-real rendering still pending — pre-rebrand tracker #133.**

**Spike outcome (browser-confirmed, harness `trendvidia/fycoder/examples/wasm`):**
with pre-rebrand tracker #101+pre-rebrand tracker #102+pre-rebrand tracker #113+Blocker 4 in, the wasm target **renders and is interactive** —
focus/blur, mouse clicks, the MD3 ripple animation, and keyboard typing into the
editor all work. The renderer half of §11 is functional. One known non-blocker
remains (the FBO blur-program gap is now guarded — pre-rebrand tracker #118 — so shadowed/blurred
widgets are safe; real FBO support has since landed — pre-rebrand tracker #7/#34, pre-rebrand PR #134 — so
shadows/blur/dirty-region now render, pending browser-verify pre-rebrand tracker #133): **large-document cost** — the harness's "Load 10k lines" button hangs the tab
because `widget.Entry` lays out **all** rows (unvirtualised), so a 10k-line
`SetText` is a multi-second non-yielding pass on the single wasm goroutine
(confirmed: the tap handler enters `SetText` and never returns while `runGL`
stops yielding). This is fyne's existing large-document limitation, not a
wasm/threading defect; smaller documents and normal editing are responsive.

**The fix — Entry/editor row virtualisation — has since landed**
(pre-rebrand tracker #119, pre-rebrand PR #137): a 10k-line
`SetText` now materialises ~26 rows in ~30ms. On top of it the fycoder editor
fork-dependency track (F1–F5) is **complete** — caret/row geometry (F1/F2,
pre-rebrand PR #116), virtualisation (F5 = pre-rebrand tracker #119), **edit veto / `ProtectedRanges`** (F3,
pre-rebrand tracker #140, pre-rebrand PR #142, **merged**), and
**row hiding / code folding `HiddenRanges`** (F4,
pre-rebrand tracker #141, pre-rebrand PR #143, **merged**) — F3
and F4 both compose with virtualisation. Downstream fycoder wiring (Phase 4/5)
and browser-verify of the complete editor in the wasm harness is tracked by
pre-rebrand tracker #150.

Also noted: `app/meta_development.go:54` logs `Executable not implemented for js`
on startup (non-fatal dev-metadata lookup); fat apps fail the wasm build on
transitive host deps (`bbolt`/`bleve` → `syscall.Flock`/`mmap`) — a
web-entrypoint packaging concern, not a fyne bug. Runtime UX checks (IME §9,
clipboard, large-file scrolling) are gated behind a working renderer. Full
analysis:
[`fycoder/docs/roadmap/10-wasm-web-target.md`](https://github.com/trendvidia/fycoder/blob/main/docs/roadmap/10-wasm-web-target.md).

---

## Remaining gaps (acceptable for now)

  ┌─────────────────────────────────────────┬────────────────────────────────────────────────────────────┐
  │                  Area                   │                           Status                           │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ GL painter (uploadInvTransform,         │ Requires live GL context — covered by visual testing, not  │
  │ transformedAABBQuad, etc.) (pre-rebrand tracker #30)        │ unit tests                                                 │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ Software drawTransformed (pre-rebrand tracker #36)          │ Requires visual golden images — would need render +        │
  │                                         │ compare test infrastructure                                │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ Test theme isolation (pre-rebrand tracker #37)              │ ApplyTheme now auto-restores via t.Cleanup, but some       │
  │                                         │ tests may still leak other global state. ApplyTheme uses   │
  │                                         │ busy-wait with no timeout (test/test_helper.go:145-161).   │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ typesetting/render forks (pre-rebrand tracker #38) — DONE   │ Resolved 2026-06-23: replace directive removed. Both forks │
  │                                         │ self-hosted under trendvidia module path and imported      │
  │                                         │ directly (typesetting v0.3.4 + render v0.2.2). Public      │
  │                                         │ repos, no GOPRIVATE. See §6 Build & CI.                     │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ FeatureTheme type safety (pre-rebrand tracker #39)          │ Feature(name string) interface{} uses type-erased lookups. │
  │                                         │ Callers must assert at runtime. Only 2 feature names       │
  │                                         │ exist. Low priority — working but fragile.                  │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ Global theme stack architecture         │ DONE — replaced global slice with per-goroutine stacks     │
  │                                         │ via sync.Map keyed by thread.GoroutineID(). Each goroutine │
  │                                         │ has its own isolated stack. Parallel rendering safe.        │
  │                                         │                                                            │
  ├─────────────────────────────────────────┼────────────────────────────────────────────────────────────┤
  │ TextStyle field defaults ambiguity      │ DONE — Weight, LineHeightScale, LetterSpacing changed to   │
  │                                         │ pointer types (*TextWeight, *float32). nil = "not set".     │
  │                                         │ Accessor methods ResolvedWeight/LineHeightScale/             │
  │                                         │ LetterSpacing resolve nil to semantic defaults. Cache keys   │
  │                                         │ flattened to resolved values for correct == comparison.      │
  └─────────────────────────────────────────┴────────────────────────────────────────────────────────────┘

## Completed

- **fycoder editor fork-deps — Entry row virtualisation + folding + protected ranges (F2–F5)** — The editor capabilities gating fycoder's web port. **F5 row virtualisation** (pre-rebrand tracker #119, pre-rebrand PR #137): `RichText.canVirtualize()` materialises only the rows whose Y band intersects the viewport (+8-row buffer) for the multi-line, wrap-off, single-`TextSegment` editor case, with full row geometry filled analytically so selection/cursor/external-gutter consumers see the whole document; a 10k-line `SetText` went from a multi-second freeze to ~30ms / ~26 row objects. **F3 edit veto / `ProtectedRanges`** (pre-rebrand tracker #140, pre-rebrand PR #142, merged): `Entry.BeforeChange func(sel TextRange, next string) bool` + `Entry.ProtectedRanges []ProtectedRange{Start,End,CaretOpaque}`; a single gate `editAllowed` sits below every mutation path (type / IME-commit / paste / cut / backspace / delete / word-delete / newline / select-delete / programmatic), vetoed edits never reach the provider or undo stack, and `CaretOpaque` bars caret entry (arrow + click). **F4 row hiding / code folding `HiddenRanges []LineRange`** (pre-rebrand tracker #141, pre-rebrand PR #143, merged): folded rows collapse to zero height and never materialise; the virtual window is computed on the visible-row axis (`winRows`) so a fold collapsing thousands of middle rows never materialises them; `RowPositions`/`RowMetrics`/`CaretRect` report visible-row geometry, the scroll extent shrinks, caret nav skips folds, and a selection spanning a fold keeps the hidden text in the model. F1/F2 (rich `StyleRun` fields + `CaretRect` column geometry) landed earlier in pre-rebrand PR #116. Follow-ups: pre-rebrand tracker #144–pre-rebrand tracker #147 (gutter alignment in the non-virtual path, wrapped-line folding, undo/redo-vs-protection policy, mobile caret-opaque), pre-rebrand tracker #149 (F3+F4 compose tests), pre-rebrand tracker #150 (downstream fycoder wiring + browser-verify).
- **RichText large-document virtualisation — wrapped + mixed/variable-height content (pre-rebrand tracker #139, closed)** — Extended F5's wrap-off single-segment window to the rest of `RichText`. **Phase A (lazy/windowed wrap measurement, pre-rebrand tracker #157, pre-rebrand PR #159):** a single-segment *wrapped* document keeps `RichText.canVirtualize()`'s uniform stride (every wrapped row shares one style) but measures only the logical lines spanning the viewport; per-logical-line `lineStarts`/`lineMeasured`/`lineWrap`/`lineRowCount`/`lineStartRow` prefix-sums (`widget/richtext_lazy.go`) serve the row model, with unmeasured lines carrying a `ceil(runeLen/estCharsPerRow)` estimate that refines to exact on measurement. Estimation engages only above 2000 logical lines; smaller or any folded doc measures eagerly. Chain of fixes — analytic `rowAtY` inverse + wrap-aware `SelectedText` (pre-rebrand tracker #160/pre-rebrand tracker #161, pre-rebrand PRs #162/#163) and a segment-boundary-wrap-copy guard test (pre-rebrand tracker #164, pre-rebrand PR #165) — all merged. **Phase B (variable-height multi-segment window, pre-rebrand tracker #158):** **Step 1** (pre-rebrand PR #166) a per-row measured-height cache (`rowHeightCache`, NaN sentinel, keyed by version/width/theme metrics) feeds the dense `calculateMin`/`Layout` without re-shaping unchanged rows; volatile rows (images, `HeightForWidth` tables/code) are never cached. **Step 2** (pre-rebrand PR #167) the `varHeight` model (`widget/richtext_varheight.go`): a `rowYPrefix` prefix-sum (folds + spacing baked in) gives O(1)/O(log n) `RowGeometry`/`rowAtY`, text rows measured exactly + cached and volatile rows estimated (running mean) until they enter the window, where only the window's objects materialise. Engages for multi-segment, truncation-off, wrap∈{off,word,break} documents over 2000 rows. Follow-ups: **pre-rebrand tracker #169 (opt-in vs auto-enable) + pre-rebrand tracker #170 (`virtualWidth` O(rows) wrap-off scan) — both merged** (pre-rebrand PR #190): scrollable-RichText virtualisation is now opt-in via the public `RichText.Virtualize bool` (default off keeps before pre-rebrand tracker #158 full materialisation; Entry's internal path unaffected), and the wrap-off widest-row estimate uses a bounded uniform sample (cap 512 rows) ∪ the materialised window, grown monotonically as wide rows scroll in, instead of shaping every row. Remaining: **pre-rebrand tracker #168 (browser/gallery-verify the variable-height window end-to-end)**, open. **Content-autosizing follow-on (pre-rebrand PR #192, merged):** because a wrapping/truncating RichText reports `MinSize().Width ≈ 0` (the virtualisation/`calculateMin` design only accumulates width when wrapping is off), the new public `RichText.IntrinsicSize() geo.Size` reports the natural unwrapped extent (drives a wrap-off/truncate-off refresh, reads `MinSize`, refreshes back; direct `MinSize` when wrapping is already off) — for sizing popovers/tooltips/containers to content. Follow-up: mirror on `Label`/`Entry`.
- **Resource leak fixes** — Tooltip renderer `Destroy()` stops timer + hides popup. Toast custom renderer calls `Dismiss()`. GL painter `Free()` calls `FreeBoundaryFBO()` for GPU memory cleanup. FontPicker goroutine cancellation via `atomic.Bool`. Settings `apply()` drops notification when channel full instead of spawning unbounded goroutines.
- **Interface bloat decomposition** — Preferences: 27 methods → 4 (`Value`/`SetValue`/`RemoveValue`/`AddChangeListener`) + generic `Pref[T]`/`PrefOr[T]`/`SetPref[T]` free functions with int/float64 JSON coercion. App: 17 → 12, extracted `CloudApp`, `NotifierApp` optional interfaces; `OpenURL()`/`SendNotification()` standalone functions. Window: 29 → 25, removed deprecated `Clipboard()`, moved `ShowAndRun()`/`SetMaster()`/`SetOnDropped()` to standalone functions + optional interfaces. Driver: 13 → 10, extracted `DoubleTapDriver`/`ScreenBlankingDriver` optional interfaces; `DoubleTapDelay()` standalone function.
- **Must-fix bugs** — Entry cursor tracking fix (cursorRow/cursorColumn), dedup queue race (individual delete instead of bulk Clear), captureImage bounds check, Paned weight persistence, binding listener snapshot, preferences synchronous flush on quit.
- **Security** — Path traversal fix in `AssetBundle.Resource()` with `isWithinDir()` containment check.
- **Thread safety (round 1)** — Transform stack mutex, Image metadata mutex, renderer double-creation prevention via `LoadOrStore`, streaming texture mutex, FocusManager mutex, preferences listener snapshot copy, Entry cursor position mutex.
- **Thread safety (rounds 2–3)** — RichText `visualCache` mutex, streaming texture lock held through GL upload, TreeView `open`/`selected` protected by `tv.mu`, Image `Refresh()` TOCTOU fix (source field snapshot), Preferences `ChangeListenersCopy` rename. `baseObject`/`Circle`/`Line` position/size/visibility protected by `propMu`.
- **GPU resource management** — Defer-based cleanup for blur FBO/textures, `Flush()` before FBO resize to prevent lost draws, degenerate triangle skip in Scene3D, `CopyTexSubImage2D` replaces `ReadPixels` in BackdropBlur. Mobile GL package extended with framebuffer/stencil/color-mask methods.
- **GPU / rendering issues** — FBO completeness checks (`checkFBOComplete()`) after all FBO creation sites (canvas, boundary, shadow, blur, Scene3D). Group opacity applied to `drawBoxShadow`, `drawBackdropBlur`, `drawRipple` via `effectiveAlpha()`. GLES shader precision already handled by separate `_es` variant files.
- **Widget & container tech debt** — RichText visual cache leak fix (always scan for stale entries), row bounds over-invalidation fix (content fingerprint skips redundant recomputation), gradient stop validation (sort + clamp to [0,1]), NavigationBar min-width from item MinSizes, Paned divider sync with hidden cleanup on removal.
- **Widget cleanup** — `entryRenderer.Destroy()` now hides and nils `e.popUp` when the renderer is destroyed. Cursor animation was already stopped by `entryContentRenderer.Destroy()`.
- **RTL polish — scroll, focus order, icon mirroring (§8 Phase 4; pre-rebrand tracker #21)** — The affordance layer above shaping/bidi/layout-mirroring, every piece gated on the one `lang.IsRTLDirection()` resolver so the default LTR path is byte-for-byte unchanged. **Icons:** a new `internal/svg.FlipHorizontal` reflects an SVG about its viewBox centre by wrapping every top-level drawable in a group carrying `matrix(-1,0,0,1,w,0)` — chosen over per-element transforms because it piggybacks on the exact typed-XML round-trip `Colorize` already uses, so every bundled icon survives it losslessly. A thin `theme.directionalResource` wrapper marks the *directional* icons (back/forward, the `MenuExpand` chevron, mail reply/forward) and, **only** under RTL, returns flipped `Content()` plus an `rtl_`-prefixed `Name()` — a distinct SVG-raster-cache key, so the mirrored art never collides with the upright one and the Phase 3.1 direction-change repaint re-reads the right variant. Non-directional icons, vertical arrows and media-transport glyphs stay upright by design. **Scroll:** `internal/widget.Scroll` keeps its left-measured offset (so the bar thumb and edge shadows, which track clipping not reading order, need no change) and adds just two RTL behaviours — open horizontal content at its right edge on first overflow (`pinToRTLStart`, a one-shot superseded by any later user/explicit scroll via `rtlPinned`) and reverse the horizontal wheel/drag delta (`resolveScrollDelta`) so the natural forward scroll advances toward the content's far (left) end. **Focus order:** `internal/app.FocusManager` Tab/Shift+Tab is re-sequenced into visual order under RTL — focusables collected in tree order with their absolute positions and **own** `obj.Size()` (the walker's trailing size is the clip rect, an easy trap), grouped into rows by vertical-span overlap, rows top-to-bottom, each row right-to-left. This is generic, so toolbars/segmented buttons/nav bars traverse visually with no per-widget code; a vertical column has no rows to reverse and is left untouched, and the LTR path skips the reorder entirely. `widget.Select`'s Left/Right option cycle is flipped to match. **Deliberately deferred:** per-widget *spatial* arrow nav for custom-renderer widgets whose internal layout isn't mirrored (`GridWrap`, `SegmentedButton`) — flipping their keys without mirroring their pixels would desync, so it must follow internal mirroring (pre-rebrand tracker #123). Tests: SVG flip (group-wrap + pixel mirror + fallbacks), directional-vs-non-directional resource, scroll right-edge start / wheel flip / LTR guards / user-supersedes-pin, focus row/grid/column ordering (LTR + RTL), Select RTL arrow flip. Merged via PR pre-rebrand tracker #121. Surfaced follow-ups: `TopAppBar` (and likely `BottomAppBar`/`Toolbar`/`NavigationBar`) lay out from a fixed left origin and don't mirror under RTL (pre-rebrand tracker #122); custom-renderer internal mirroring + spatial arrow nav (pre-rebrand tracker #123); per-canvas / mixed-direction layout since delivered across Phases 6–7 and closed (pre-rebrand tracker #66).
- **RTL layout mirroring (§8 Phase 3; pre-rebrand tracker #20)** — Modelled reading/flow direction as **one** ambient concept (as Flutter `Directionality` / CSS `dir` / Android `layoutDirection` do) rather than separate text vs layout knobs. Canonical `fyne.Direction` (`DirectionAuto`/`LTR`/`RTL`) lives in the root package; `fyne.TextDirection` is a type alias (with aliased `TextDirection*` consts) so `TextStyle.Direction` stays self-documenting. The process-wide setting `fyne.SetLayoutDirection`/`fyne.CurrentLayoutDirection` (atomic) is the single source of truth; `lang.IsRTLDirection()` resolves it (explicit override else locale via `lang.IsRTL()`) and is the one resolver behind layout mirroring, alignment **and** the default text base direction — `resolveBaseDirection()` Auto consults it, so an RTL override reorders Auto text even under an LTR locale while explicit `TextStyle.Direction` still wins. Process-global is the correct scope because text measurement/shaping is canvas-free (`resolveBaseDirection` runs inside `walkString`/`MeasureString` with no canvas), mirroring how `theme.Padding()` is already read globally inside layouts; per-canvas direction would desync MinSize from render and is out of scope. `lang.ResolveAlign()` swaps `TextAlignLeading`/`Trailing` under RTL (an involution → LTR is identity; center unaffected) and is applied at the choke points that turn leading/trailing into pixels: the software and GL painters (`canvas.Text`, `canvas.StyledText`), `RichText.layoutRow` (single-text x-pad + branch selection, with the multi-segment grow mechanic wrapping its physical `setAlign` in `ResolveAlign` to cancel the painter's single downstream flip), `Entry` cursor offset, `Hyperlink` focus box and `selectable` selection highlight. Horizontal layouts (`HBox`, `Border`, `Grid`, `Form`) mirror via a uniform post-pass that reflects every visible child's x-span within the container width (`mirrorChildren`), keeping the original LTR positioning code untouched; full-width children map onto themselves so `VBox`/top/bottom bars are no-ops. The whole feature is gated on the resolved direction, so default LTR is byte-for-byte unchanged (existing visual baselines pass). Tests: `lang` resolver + involution, base-direction-follows-override, layout mirror positions for all four layouts (+ an LTR-unchanged guard), and a deterministic painter render test asserting a leading-aligned glyph moves from the left half (LTR) to the right half (RTL). Merged via PR pre-rebrand tracker #64. Follow-ups: runtime direction-change refresh **done** (§8 Phase 3.1, PR pre-rebrand tracker #86 / pre-rebrand tracker #65); per-canvas / mixed-direction layout since delivered across Phases 6–7 and closed (pre-rebrand tracker #66); Phase 4 (pre-rebrand tracker #21) covers scroll direction, focus/nav order, icon placement. The analogous `lang.SetLocale` live refresh landed too (PR pre-rebrand tracker #89 / pre-rebrand tracker #87), and re-deriving static `lang.L` strings on a live locale switch landed via `binding.NewLocalizedString` + `lang.AddLocaleChangeListener` (PR pre-rebrand tracker #91 / pre-rebrand tracker #90), with the key/plural binding variants (PR pre-rebrand tracker #99 / pre-rebrand tracker #93) and `lang.BindText` for non-binding text — button labels, window titles, menu items (PR pre-rebrand tracker #100 / pre-rebrand tracker #92) — completing live locale switching.
- **RTL bidi at the widget level (§8 Phase 2; pre-rebrand PR #23, pre-rebrand tracker #19)** — Closed the gap above the shaping layer with one shared per-line primitive, `painter.LineVisualLayout`, built on the typesetting fork's existing bidi segmenter (`Segmenter.Split` already returns runs in visual order — no new dependency). It exposes a logical-rune↔visual-x caret table plus unambiguous per-rune glyph extents, with `CaretX`/`HitTest`/`VisualNeighbor`/`SelectionSpans`; a cheap `MayContainRTL` script pre-filter gates it so pure-LTR text keeps the exact legacy measurement path (incl. tab expansion). Wired into `canvas.StyledText` (`DrawStyledRuns` bidi-reorders runs instead of array order), and `widget.Entry`: caret pixel position (`rtlCaretX`), click hit-testing (`HitTest`), multi-rectangle selection for visually-disjoint ranges (`rtlSelectionSpans` + variable-length rect pool), and visual-order Left/Right arrow navigation (`visualColNeighbor`). Wrapped/multi-row text resolves per row automatically. Forward caret affinity at direction boundaries. Tests: caret round-trip + non-monotonicity, StyledText reversal (incl. pixel placement), Entry caret/click/selection/arrow nav for `"abc مرحبا def"`. Deferred follow-ups: cross-*segment* RichText bidi (pre-rebrand tracker #24), cursor affinity tracking (pre-rebrand tracker #25), cross-run cursive joining in StyledText (pre-rebrand tracker #26). Layout mirroring is Phase 3 (pre-rebrand tracker #20); polish Phase 4 (pre-rebrand tracker #21).
- **RTL shaping foundation (§8 Phase 1 + §10; pre-rebrand PR #18)** — Arabic/Hebrew now join, reorder, and select script-specific OpenType features. Discovery showed the `trendvidia/typesetting` `Segmenter` already does per-run bidi (UAX #9) + script + language detection, and those values already reach HarfBuzz via `shapeCallback`; the only true gap was the bidi *paragraph base direction* (hardcoded LTR). Added `fyne.TextStyle.Direction` (`TextDirectionAuto`/`LTR`/`RTL`, zero value Auto = backward compatible), `lang.IsRTL()`/`IsRTLLocale()` (active-locale script → direction), and `resolveBaseDirection()` wired into `walkString` (+ `shaping.Input.Language` from locale for §10) so measurement and drawing agree. Override rides the existing `TextStyle` struct, so it flows through `canvas.Text`/`Label`/`RichText`/`Entry` with no widget changes. Tests in `lang/direction_test.go` + `internal/painter/font_rtl_test.go` (deterministic direction/script assertions; cross-platform Arabic pixel tests avoided as glyph coverage is font-dependent). Bidi at the widget level (wrapping, caret/selection, cross-run StyledText) is Phase 2 (pre-rebrand tracker #19); layout mirroring Phase 3 (pre-rebrand tracker #20); polish Phase 4 (pre-rebrand tracker #21); per-widget language override (pre-rebrand tracker #22).
- **Locale-aware formatting** — CLDR date/time/currency patterns, 24h/12h auto-detection, `DateEntry`/`DateTimeEntry` widgets, auto-detect picker constructors, `FormatCurrency`/`CurrencySymbol`/`DefaultCurrency`. Thread-safe locale tag access via `sync.RWMutex`. Translation files (all 15 locales) updated.
- **Test coverage (partial)** — `canvas/scene3d_test.go` (35 tests: Vec3/Mat4 math, Scene3D object, software rasterizer), `layout/shrinklayout_test.go` (5 tests: full coverage), `internal/theme/render_test.go` expanded (4 new tests). 550 stale `testdata/failed/` files cleaned. Locale test isolation fixed (`SetLocaleForTests` auto-restores via `t.Cleanup`). Canvas primitives (Ripple, BoxShadow, BackdropBlur, TextOnPath, StyledText) expanded with Move/Resize/Hide/Show/property tests. Painter `MeasureStyledRuns`/`DrawStyledRuns`/`ClearFontCache` tests added (11 new).
- **Resource system** — `LazyResource` for on-demand loading, `StreamingResource` interface for large assets, `AssetBundle` for external asset directories, `fyne bundle --lazy` for lazy code generation.
- **Streaming texture path** — `canvas.Raster.Streaming` with `TexSubImage2D` for zero-churn video/real-time updates (commit `f03f66afc`).
- **GLView canvas primitive** — `canvas.GLView` with `GLContext` interface for custom GL rendering integrated into the scene graph (commit `ad96ebfdc`).
- **Animation API** — Composable groups (parallel/sequential/staggered), spring dynamics, keyframe support, value capture (commit `dd58bd69b`).
- **Build tag unification** — Dropped `migrated_fynedo` and `no_glfw` tags. Single `-tags ci` works on all platforms (macOS, Linux, Windows).
- **Data race fixes** — Thread-safe `async.Map` for `migrated_fynedo` path (sync.RWMutex + snapshot-based Range). Font cache races fixed in go-text/typesetting fork (go-text/typesetting#256).
- **Test suite fixes** — Fixed stale golden files (MD3 theme), hardcoded theme color assertions, `BoxShadow` markup renderer support, `goroutineID` test migration, `containerOnMain` nil-safety, `ApplyTheme` auto-restore via `t.Cleanup`, RichText row-bounds cache invalidation, Label `syncSegments` on Refresh.
- **GPU-accelerated 2D path rendering** — `canvas.Path` with MoveTo/LineTo/QuadTo/CubicTo/Close, stroke styles (dash, caps, joins, miter), fill rules (winding/even-odd) via stencil buffer, edge AA via fringe geometry, `container.PathClip` for path-based clipping.
- **Transform stack** — Per-object affine transforms on all 11 canvas primitives (including Circle/Line), world transform composition through tree, transform-aware hit testing, software painter parent transform propagation.
- **Dirty-region tracking** — `container.RepaintBoundary` for FBO-cached subtree isolation. Persistent full-canvas FBO (`DirtyRegionPainter`) retains frame content across double-buffer swaps. Scissor-based dirty-region culling skips draw calls for unchanged objects. **Partial repaints repaired and gated (pre-rebrand PR #8, pre-rebrand issue #5):** the original culling dropped content (TopAppBar vanishing mid-scroll, fycoder highlights blinking on Android) because the dirty region used stale pre-walk positions and ignored moves + draw-overflow. The repair (`Canvas.PartialDirtyRegion`) runs a draw-free compute-pass that unions both previous and current world-space bounds of every refreshed/moved object (plus refreshed-subtree overflow) and is correct by construction. Since #496 it is **on by default on desktop** (macOS, Linux, Windows), where it was validated pixel for pixel against a full repaint by `BELLUI_PARTIAL_VERIFY` (the gallery and the perf scenes on macOS WebGPU and GL, Linux WebGPU and GL, Windows D3D12), and cut GPU work 4.8× on a mostly static scene at no CPU cost worth measuring. It stays **off on mobile and the web** until validated on devices there (#593). `nopartialrepaint` turns it off at build time, `BELLUI_PARTIAL_REPAINT=0` at run time.
- **Text rendering enhancements** — `canvas.TextOnPath` for text along arbitrary paths (arc-length parameterized glyph placement, per-glyph rotation). `canvas.StyledText` for per-character/run styling at the canvas level (multi-color, multi-size, multi-font inline text). Plus text rotation, sub-pixel positioning, and driver-free text measurement.
- **Hit testing at scale** — Grid-based `SpatialIndex` for O(1) broadphase hit testing. 64px uniform grid built during `WalkTrees`, atomic publish for lock-free reads. ~253ns per query with 10K objects (0 allocs). Transform-aware narrowphase with inverse-matrix hit test. GLFW and mobile drivers integrated.
- **GPU-accelerated 3D pipeline** — `Scene3D` rendered via GL pipeline with vertex buffers, depth-tested FBO, and Phong-style lighting shader. Meshes expanded to interleaved VBOs (position + normal + color), MVP transform in vertex shader. FBO color texture cached in existing texture pipeline. `Uniform3f`, `UniformMatrix4fv` added to GL context. Software rasterizer preserved as fallback.
- **Accessibility core types** — `Accessible`, `AccessibleDescriber`, `AccessibleValuer`, `AccessibleStateful`, `AccessibleActioner` interfaces. 47 roles, 9 state flags, 8 action constants. 36 widgets annotated. Generic `WalkAccessibleTree`. (See [ACCESSIBILITY.md](ACCESSIBILITY.md) for remaining work.)
- **Missing event types** — `event.HoverEvent` + `event.Hoverable` interface (replaces `desktop.Hoverable` with `desktop.MouseEvent`; methods renamed `HoverIn`/`HoverMoved`/`HoverOut`; deprecated aliases in `driver/desktop`). `event.ScrollSource` enum (`MouseWheel`/`Touchpad`/`Touch`) + `Source` field on `ScrollEvent`. `event.LongPressable` interface with `LongPressed(*PointEvent)` — touch handler tries `LongPressable` first, falls back to `SecondaryTappable`; desktop right-click unchanged. `test.LongPress()`/`LongPressAt()` helpers added.
- **Code quality & tech debt** — Resolved all TODO/FIXME comments: button bitmap disable (removed SVG guard), RichText TODOs (upstream issue refs), SeparatorSegment.SelectedText, popup oversized clamping, Tree.ScrollTo opens ancestors, FileIcon.SetSelected removal. Added `Since` tags to deprecated constructors/enums. Font cache eviction via `SweepFontCache` with last-access timestamps, hooked into `Clean()` cycle. Renderer cache gap fixed: `CleanCanvas` sweeps expired renderers at window close.
