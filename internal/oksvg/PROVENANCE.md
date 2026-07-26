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
