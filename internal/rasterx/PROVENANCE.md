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
