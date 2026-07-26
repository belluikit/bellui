# internal/render — vendored from github.com/belluikit/render

Vendored from `belluikit/render` v0.2.4 (formerly `trendvidia/render`), itself a
fork of `go-text/render`. Glyph rasterization for the text stack: outline,
bitmap and SVG-table colour glyphs.

License: UNLICENSE / BSD-3-Clause (SPDX dual) — see `LICENSE`, retained in full.

## Why vendored

This was the **last holder of `github.com/srwiley/{oksvg,rasterx}`** in the
dependency graph. Those are vendored at `internal/{oksvg,rasterx}`, but Go's
internal rule scopes them to `github.com/belluikit/bellui/...` — a separate
module such as `belluikit/render` cannot import them. The choice was therefore
to duplicate oksvg+rasterx inside `render`, or to bring `render` here. It is
501 LOC with exactly one consumer (`internal/painter`, using only
`render.Renderer`), so absorbing it was clearly cheaper than a second copy of
~5.5k LOC.

Result: **srwiley is fully retired** from the module graph.

Note `svg.go`'s `drawSVG` is reachable (called from `render.go`) — it draws
SVG-table colour glyphs — so the oksvg dependency is genuine, not dead code.

## Updating

Treat as owned code; patch here. `belluikit/render` is now unused by bellui and
is frozen unless another consumer appears.

### Local modifications
* `github.com/srwiley/{oksvg,rasterx}` retargeted to `internal/{oksvg,rasterx}`.
