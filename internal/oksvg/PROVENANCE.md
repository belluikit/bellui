# internal/oksvg — vendored from github.com/fyne-io/oksvg

Vendored from `github.com/fyne-io/oksvg` v0.2.0 (itself a fork of
`github.com/srwiley/oksvg`). SVG parsing for theme icons.

**Copyright (c) 2018, Steven R Wiley.** BSD 3-Clause — see `LICENSE`, retained
in full.

## Why vendored

oksvg's public API is *typed against* rasterx: `SvgIcon.Draw` takes a
`*rasterx.Dasher`. Once rasterx was vendored to `internal/rasterx`, oksvg had to
come along or the two would reference incompatible `Dasher` types across a
module boundary. Vendoring both keeps the SVG pipeline internally consistent and
owned.

It also adds no modules — oksvg needs only `internal/rasterx`, `golang.org/x/image`
and `golang.org/x/net/html/charset`, all already required.

## Updating

Treat as owned code; patch here.

### Local modifications
* Import path, and `github.com/srwiley/rasterx` retargeted to
  `github.com/belluikit/bellui/internal/rasterx`.
* Reformatted with `gofumpt`/`goimports` to match this repo's enforced house style.
* `SvgPath.DrawTransformed` scales the stroke width, dash lengths and dash
  offset by the path's transform (#1292). Upstream passes them to the dasher in
  user units while the path's points arrive already transformed, so a stroke
  kept its width in pixels at every size: a 7-unit stroke on a 120-unit viewBox
  was 7 px at 40 px and at 240 px. Both srwiley/oksvg and fyne-io/oksvg still
  have this. For a uniform scale, with or without rotation, the scale is the
  square root of the transform's determinant.
* A stroke under a non-uniform transform stretches with the shape, as in a
  browser (#1295; upstream srwiley/oksvg#53, still open). The path is stroked
  under a uniform scale by the transform's larger axis scale, and the rest of
  the transform is applied to the outline by a scanner wrapper
  (`transformingScanner`) on its way to the rasterizer. The larger scale is
  deliberate: built at the smaller one, a thin stroke under a strong stretch
  rounds to zero width in the dasher's 1/64 px units and vanishes.
* `DefaultStyle` uses SVG's initial values for a stroke: a width of 1 and
  mitred joins (#1478). Upstream defaulted to a width of 2 and bevelled
  joins, so a stroke with neither attribute was twice as thick as in a
  browser and had its outer corners cut off. rasterx's `Miter` already falls
  back to a bevel past the miter limit, as SVG does.
