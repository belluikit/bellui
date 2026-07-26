# internal/ico — vendored from github.com/fyne-io/image/ico

Vendored from `github.com/fyne-io/image` v0.1.1 (`ico/` package only). Only the
package import path was changed.

## Licensing — two licenses, both apply

This package has mixed provenance and ships **both** upstream license files:

| File | Covers | License |
|---|---|---|
| `LICENSE-reader` | `reader.go` | Apache-2.0 |
| `LICENSE-writer` | `writer.go` | BSD-3-Clause, Copyright (c) 2012 The Go Authors |

Both are retained in full as their licenses require. Do not consolidate them.

## Why vendored

bellui used only `image/ico` (391 LOC) out of the upstream module; the other
package, `image/xpm` (1,269 LOC), was entirely unused. Vendoring the one package
we need drops the dead weight, removes a fork repo from the release train, and
adds no dependencies. Windows `.ico` is a frozen format, so upkeep is ~nil.

Consumers: `internal/driver/glfw` (window icons) and the `bellui` CLI's Windows
packaging.

## Updating

Treat as owned code; patch here.

### Local modifications
* Import path only (`github.com/fyne-io/image/ico` → this package).
* Reformatted with `gofumpt`/`goimports` to match this repo's enforced house style.
