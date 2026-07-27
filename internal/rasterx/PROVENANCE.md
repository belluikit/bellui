# internal/rasterx — vendored from github.com/srwiley/rasterx

Vendored at commit `2ab79fcdd4ef` (module version
`v0.0.0-20220730225603-2ab79fcdd4ef`, the last upstream release).

**Copyright (c) 2018, Steven R Wiley.** BSD 3-Clause — see `LICENSE`, retained
in full. Only the package import path was changed; the code is otherwise
upstream's.

## Why vendored

Upstream has been dormant since 2022-07-30. rasterx is the software painter's
2-D rasterizer (fills, strokes, dashes, caps, joins, gradients) and depends only
on `golang.org/x/image`, which bellui already requires — so vendoring adds no
modules and lets us patch it directly instead of carrying a fork repo.

Note `golang.org/x/image/vector` supplies the underlying scanline fill; rasterx
adds the stroking/dashing layer on top, which is why x/image alone is not a
drop-in replacement.

## Updating

Upstream is effectively frozen. Treat this as owned code: patch here and note
the change below.

### Local modifications
* Import path only (`github.com/srwiley/rasterx` → this package).
* Reformatted with `gofumpt`/`goimports` to match this repo's enforced house style.
* Cleared `staticcheck` (15 findings), the analyzer counterpart to the gofumpt
  pass above. All mechanical, no behaviour change:
  - `x == false` → `!x` in `dash.go`, `geomx.go`, `gradient.go`, `stroke.go`
    (S1002). Only the ten *code* occurrences; the identical phrasing inside
    doc comments ("returns with `intersects == false` if …") is prose and was
    left alone.
  - dropped a redundant trailing `return` in `stroke.go` `StrokeGap` (S1023).
  - `image.ZR` → `image.Rectangle{}` in `scan.go` and `rasterx_test.go`
    (SA1019, deprecated since Go 1.13). In the two `scan.go` comparisons the
    literal needs parentheses — `== (image.Rectangle{})` — since a bare
    composite literal in an `if` condition is ambiguous with the block brace.

  Verified by this package's tests plus its downstream consumers
  (`internal/render`, `internal/oksvg`, `internal/painter/...`, whose
  golden-image tests would catch any rasterization change).

  Still outstanding: three functions exceed the repo's cyclomatic-complexity
  limit (`readStyleAttr` 47, `addSeg` 44 in oksvg; `strokeEdge` 43 here). They
  are scoped out of the gocyclo gate pending a real refactor — tracked in #18,
  deliberately not attempted alongside a mechanical cleanup.
