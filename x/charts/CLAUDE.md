# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build all packages
go build ./...

# Run static analysis
go vet ./...

# Run the demo app (requires display / Fyne GL context)
go run ./cmd/demo/

# Run a single test (when tests exist)
go test ./axis/ -run TestNiceTicks -v
```

**Critical**: The `go.mod` has `replace github.com/belluikit/bellui => ../fyne` pointing to the local Fyne fork at `../fyne`. This sibling directory must exist for builds to work.

## Architecture

fycharts is a native Go charting library rendering ECharts-style charts with Fyne canvas primitives. The project has ~50 Go source files (~10,000 lines of code) across 12 packages. The core flow is:

```
User creates chart.Option (declarative config)
  -> chart.New(opt) creates a Fyne widget
  -> chartRenderer.rebuild() runs the pipeline:
      1. Classify series (cartesian vs pie vs 3D)
      2. Build axis scales, apply zoom window
      3. Build coordinate systems per Y-axis
      4. Apply LTTB downsampling when data > pixel width
      5. Render: background -> title -> breadcrumb -> axes -> grid lines
           -> series -> overlays -> legend -> zoom buttons -> toolbox
           -> brush overlay -> tooltip
      6. Register hit test data for interactivity
      7. Trigger entrance animation on first render (if Animation=true)
  -> Fyne displays the flat []CanvasObject list
```

### Package Dependencies

```
chart/  ──→  series/  (renderer registry, 21 chart types)
   │    ──→  coord/   (data-to-pixel mapping)
   │    ──→  axis/    (scales, ticks)
   │    ──→  component/ (tooltip, legend, title, zoom buttons, overlays, breadcrumb, toolbox, visualmap)
   │    ──→  interact/  (hit testing, event bus, zoom/brush state)
   │    ──→  style/   (theme resolver)
   │    ──→  animate/ (entrance + transition animation controller)
   └────→  internal/mathutil/ (nice numbers, line clipping, LTTB downsampling)

builder/ ──→  chart/, series/, axis/, component/, interact/ (fluent API)
dataset/ ──→  (standalone: column storage, Encode, Transform pipeline)
```

### 21 Chart Types

All series types are defined in `series/series.go` and each has its own file:

| Type Constant | File | Coord System |
|---------------|------|--------------|
| `TypeLine` | `line.go` | Cartesian2D |
| `TypeBar` | `bar.go` | Cartesian2D |
| `TypeScatter` | `scatter.go` | Cartesian2D |
| `TypePie` | `pie.go` | None |
| `TypeCandlestick` | `candlestick.go` | Cartesian2D |
| `TypeHeatmap` | `heatmap.go` | Cartesian2D |
| `TypeRadar` | `radar.go` | Polar |
| `TypeGauge` | `gauge.go` | None |
| `TypeFunnel` | `funnel.go` | None |
| `TypeBoxplot` | `boxplot.go` | Cartesian2D |
| `TypeTreemap` | `treemap.go` | None |
| `TypeSankey` | `sankey.go` | None |
| `TypeSunburst` | `sunburst.go` | None |
| `TypeGraph` | `graph.go` | None |
| `TypeTree` | `tree.go` | None |
| `TypeParallel` | `parallel.go` | Cartesian2D |
| `TypePictorial` | `pictorial.go` | Cartesian2D |
| `TypeEffectScatter` | `effect_scatter.go` | Cartesian2D |
| `TypeCustom` | `custom.go` | Cartesian2D |
| `TypeBar3D` | `bar3d.go` | Scene3D |
| `TypeScatter3D` | `scatter3d.go` | Scene3D |
| `TypeSurface` | `surface.go` | Scene3D |

### Key Patterns

**Sealed Series interface** (`series/series.go`): All series types must live in the `series/` package because `Series` has an unexported `sealed()` method. To add a new chart type:
1. Add a struct implementing `Series` (include `func (s *MyType) sealed() {}`)
2. Add a `Renderer` implementation
3. Call `RegisterRenderer(TypeName, factory)` in `init()`
4. Add hit test registration in `chart/renderer.go`'s type switch

**Renderer registry**: `series.NewRenderer(type)` does dynamic dispatch via `init()`-registered factories. If a type isn't registered, it returns nil and rendering silently skips it.

**Stacking**: `LineSeries.Stack` and `BarSeries.Stack` specify a stack group name. The `RenderContext.StackOffsets` map tracks cumulative Y offsets per category index for each stack group. Each series reads its base offset, renders at base+value, then writes the new cumulative offset for the next series in the group.

**LTTB downsampling** (`internal/mathutil/downsample.go`): `LTTB()` reduces N data points to a threshold count while preserving visual shape. `DownsampleFloat64()` wraps it for Y-only data with index-based X. The line renderer auto-applies this when data count exceeds the pixel width of the plot area.

**Overlays** (`chart/option.go`): `ReferenceLine`, `ReferenceBand`, and `Annotation` implement the sealed `Overlay` interface. They are stored in `Option.Overlays` and rendered by `component.RenderOverlays()` using the coordinate systems to map data-space values to pixel positions.

**Drilldown** (`chart/option.go` + `component/breadcrumb.go`): `DrilldownConfig.DataProvider` is called on click with the current breadcrumb path. It returns a child `Option` (or nil for no drilldown). The breadcrumb UI (`RenderBreadcrumb`) provides a back button and clickable crumb segments to navigate up the hierarchy.

**Toolbox** (`component/toolbox.go`): `ToolboxConfig` enables save-as-image and magic type switching buttons. `RenderToolbox` produces button objects and a `ToolboxLayout` for click hit-testing. Actions are returned as strings ("save", "magicType:line", etc.).

**VisualMap** (`component/visualmap.go`): `VisualMapConfig` maps a data dimension to colors (gradient interpolation) and/or symbol sizes. `MapColor()` and `MapSize()` normalize the value to [0,1] and interpolate across gradient stops or size range.

**3D charts**: `Bar3DSeries`, `Scatter3DSeries`, and `SurfaceSeries` bypass the 2D coordinate system and render directly to `canvas.Scene3D` from the Fyne fork. They set up a `Camera3D`, create `Mesh3D` geometry (e.g., `makeBox` for bars), and return a single Scene3D canvas object. The scene supports orbit rotation and scroll zoom natively.

**Multi-Y-axis**: The renderer builds one `Cartesian2D` per `YAxis` config. Each series specifies `YAxisIndex` to select its coordinate system. Data ranges are scanned per-axis.

**Widget threading**: All chart state is behind `Chart.mu` (RWMutex). Interaction handlers (hover, tap, scroll, drag) acquire the lock, update state, release, then call `Refresh()`. Never call `Refresh()` while holding the lock.

**Hit testing rebuilds every render**: `HitTester` is cleared and repopulated during each `rebuild()`. Hidden series (legend-toggled off) aren't registered, so they can't be clicked.

**Line clipping**: When zoomed, lines are clipped to the plot area via Cohen-Sutherland (`internal/mathutil/clip.go`). Symbols outside the viewport are skipped.

**Animation** (`animate/animate.go`): `Controller` drives entrance (400ms, `EaseStandardDecelerate`) and transition (300ms, `EaseStandard`) animations via Fyne's `bellui.Animation`. Series renderers read `AnimProgress` in `RenderContext` to interpolate from 0 (hidden) to 1 (final state). The chart triggers entrance on first render when `Option.Animation = true`.

**Builder** (`builder/builder.go`): `ChartBuilder` wraps `chart.Option` with method chaining. Sub-builders (`LineBuilder`, `BarBuilder`, etc.) configure series-specific options and return to the parent via `Done()`. Overlay builders (`ReferenceLineBuilder`, etc.) and `Drilldown()` are also chained. `Widget()` calls `chart.New(b.Build())` to produce the final widget.

**Never block the render goroutine; never re-rasterise an unchanged view** (#35): the choropleth used to `http.Get` + parse 14 MB of GeoJSON *inside* `Render` (427 ms of frozen UI, unbounded on a slow network because `http.DefaultClient` has no timeout, and retried every frame when offline because failures were not cached), then rebuild its whole raster on every paint (226 ms/repaint). Both are now handled by patterns any series with remote data or expensive rasterisation should copy:
- **Async load** (`series/geojson_loader.go`, modelled on `internal/tiles.Cache`): `geoJSONLoader.Get(url, onLoaded)` returns nil while loading, keeps one in-flight request per URL, caches the parsed `FeatureCollection`, gives the request a 30 s timeout and a 64 MiB read cap, and **caches failures with exponential backoff** (2 s doubling to 5 min) so an offline app degrades to "no overlay" instead of re-dialling per frame. `fetch`/`now` are injected test seams — the loader tests use neither network nor wall clock.
- **Repaint seam**: `series.RenderContext.Invalidate func()` (built once in `chartRenderer.seriesRenderContext`, → `chartRenderer.invalidate` → `bellui.Do(refreshDirty(dirtyFull))`) is how a renderer asks for another render from a background goroutine. Safe from any goroutine, may be nil.
- **Rasterisation memo**: `choroplethRaster.image(key, fc, w, h, build)` returns the cached image unless the `choroplethKey` — viewport, centre, zoom, resolved data range, an **order-independent** FNV digest of `RegionData` (Go randomises map iteration, so a naive digest would miss every time) and a digest of the colour stops — the feature collection *pointer*, or the pixel size changed. Inline `GeoJSON` is memoised too (`inlineGeoJSON`, keyed on backing-array identity via `sameBytes`), since it was being re-parsed per render. Measured: 6.5 ms → 17 µs per repaint (`BenchmarkChoroplethRepaint` vs `BenchmarkChoroplethFirstRasterize`).
- **`MapSeries` (#39)** got the same two treatments. Tile arrival now repaints: `tiles.Cache` gained `SetListener(key, fn)`/`HasListener(key)` because a cache is shared **by URL** across every map while the repaint seam is **per chart**, so arrival fans out rather than notifying one owner. The registration is **one-shot and only while tiles are outstanding** (the repaint re-renders, which re-registers if anything is still missing), so a fully-loaded map leaves nothing behind holding the chart's `invalidate` closure alive in the process-global cache. The composite is memoised on `mapKey` (viewport, centre, zoom, pixel size, digest of the *loaded* tile set — an arriving tile changes the digest and nothing else does), and the per-pixel `img.Set`/`tileImg.At` sampling loop became `blitTile`, which clips once and indexes both pixel buffers directly (`fillNRGBA` likewise fills one row and copies it down). Measured: 889 µs → 16 µs per repaint (`BenchmarkMapRepaint` vs `BenchmarkMapComposite`), and the blit itself 1.15 ms → 122 µs (`BenchmarkBlitTile` vs `BenchmarkBlitTileReference`, which keeps the original loop as a correctness oracle). `tiles.Cache.Seed` populates a cache without the network for tests/benchmarks.
- **Verify**: `cmd/geoprobe -auto` builds the demo's Geo tab against the real data and times round trips onto the main goroutine — that latency *is* the beachball. Post-startup worst case is ~2-3 ms across the whole download, parse, first rasterisation and tile arrival. It also reports the rasterisation cost directly, which doubles as proof the data actually arrived (a quiet run must not pass by having drawn nothing).
- **First rasterisation of a *new* view (#40)** — the memo above elides repaints, but the first paint of any new view (data arrival, and every pan or zoom step) still ran the generator in full, at ~205 ms on the real country data. It is now **~17 ms**, a 11× cut, from three fixes, none of which needed a background goroutine — measured on the actual 14 MB `countries.geojson` by `BenchmarkChoroplethFirstRasterizeRealData` (`BELLUI_GEOJSON=<path>`; skipped without it, so the suite needs no fixture), with `BenchmarkChoroplethFirstRasterizeBig` as the network-free synthetic gate:
  - **Decode geometry once, at parse time** (194 → 33 ms, the big one). `geojson.Geometry` kept its coordinates as a `json.RawMessage` and `Polygons()` re-`Unmarshal`ed them *on every call* — so the choropleth re-parsed the entire coordinate payload on every rasterisation, about half the total cost. `Geometry.UnmarshalJSON` now resolves the rings up front and `Polygons()` returns them (shared, read-only, concurrency-safe). Dropping the RawMessage also stops retaining a second copy of every coordinate as raw JSON for the life of the collection. A malformed geometry is recorded on the geometry and reported by `Polygons()`, **not** returned from `Parse` — one bad feature must not cost the caller every other feature, which is how the lazy version behaved.
  - **`coord.Projection`** (33 → 24 ms), an immutable snapshot of a `GeoCoord`'s zoom/centre/viewport. `GeoCoord.DataToPixel` recomputed the zoom scale (a `Pow`) and the **centre pixel** (a `Log`/`Tan`/`Cos` triple) *per point* — hundreds of thousands of times per view. `GeoCoord.Projection()` resolves them once; `DataToPixel` now delegates to a one-shot snapshot so it is unchanged for single-point callers. Being a **value that copies what it needs**, it also removes the trap #40 flagged for ever moving this work off the render goroutine: rasterising against a snapshot never reads the shared `GeoCoord` that `Render` mutates via `SetBounds` on the next frame.
  - **Active-edge scanline sweep** (24 → 17 ms; 4× on the ring alone). `geojson.RasterizePolygon` tested *every* edge against *every* scanline — O(scanlines × points), which on a country ring (thousands of points over hundreds of scanlines) dominated what was left — and allocated its intersection list per scanline. It now builds the edge list once, sorts by top, and sweeps an active list, filling spans by writing straight into the pixel buffer. `geojson.Rasterizer` is the buffer-reusing form the choropleth drives across all rings (not safe for concurrent use — one per goroutine). Output is **byte-identical**: `rasterizePolygonReference` in `rasterize_test.go` keeps the original loop as a correctness oracle (the `BenchmarkBlitTileReference` pattern), asserted over fixed shapes, four-sided clipping, degenerate rings and 300 random ones.
  - **Async rasterisation was considered and not done.** #40 suggested returning the previous image and rasterising in the background, but explicitly invited checking whether making it fast enough removed the need — it does. At ~17 ms a new view costs about one frame instead of a twelfth of a second, while async would show a stale or blank frame during a pan (worse for a map than the frame it saves), needs the resolved colours snapshotted to stop racing on `RegionData`, and only *moves* the remaining projection/fill work rather than removing it. Revisit if a consumer needs a genuinely hitch-free continuous pan on a much denser data set — `Projection` has already removed the hard part.

**Dataset** (`dataset/dataset.go`): Column-oriented storage with typed dimensions (DimFloat, DimString, DimTime). `Encode` maps dimension indices to visual roles. `Transform` interface with `FilterTransform` and `SortTransform` implementations. `Pipeline` chains transforms sequentially.

### Coordinate Systems

| Type | Use | Data -> Pixel |
|------|-----|-------------|
| `Cartesian2D` | Line, Bar, Scatter, Candlestick, Heatmap, Boxplot, Parallel, Pictorial, EffectScatter, Custom | Normalizes via axis Scale, flips Y |
| `Polar` | Radar, Sunburst | (radius, angle) -> (x, y) |
| `None` | Pie, Gauge, Funnel, Treemap, Sankey, Graph, Tree | Identity; series positions itself |
| `Scene3D` | Bar3D, Scatter3D, Surface | 3D camera projection via Fyne fork |

### CategoryScale Zoom

`CategoryScale.SetRange(min, max)` narrows the visible categories. Unlike `LinearScale`, it uses band positioning `(value - rangeMin + 0.5) / visibleCount` so items never sit on axis edges.
