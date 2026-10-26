Thanks for your interest in contributing to Bellui!

Bellui is open source under the [BSD 3-Clause license](LICENSE). By sending a
pull request you agree that your contribution is released under the same
license. Everyone taking part is expected to follow the
[code of conduct](CODE_OF_CONDUCT.md).

## Asking a question

For help with your own code, or a question about how something works, ask in
[Discussions](https://github.com/belluikit/bellui/discussions/categories/q-a)
(Q&A), or see the [support page](https://bellui.com/support/). Ideas that are
not yet a concrete feature request belong in
[Discussions → Ideas](https://github.com/belluikit/bellui/discussions/categories/ideas).
The issue tracker is for bugs and feature requests.

## Reporting a bug

1. Search the [issue list](https://github.com/belluikit/bellui/issues) first.
   If the bug is already there, add what you know to it.
2. Otherwise open a new issue with the
   [bug report form](https://github.com/belluikit/bellui/issues/new?template=bug_report.yml).
   A small program that shows the problem, and the platform you saw it on, are
   the most useful things you can include.

Security problems are not reported in public issues: see
[SECURITY.md](SECURITY.md).

## Fixing a bug

1. Read [Contributing code](#contributing-code) below.
2. Write a test that fails because of the bug.
3. Fix it, and check that the test now passes.
4. Run the checks listed below.
5. [Open a pull request](https://github.com/belluikit/bellui/compare) against
   `main` and say which issue it fixes (`Fixes #123`).

## Adding a feature

Bellui is maintained by TrendVidia. For anything larger than a small fix, open
a [feature request](https://github.com/belluikit/bellui/issues/new?template=feature_request.yml)
first and describe what you want to build and why, so we can agree on the
approach before you write the code. [ROADMAP.md](ROADMAP.md) lists what is
planned, and [docs/notes/](docs/notes/) explains how each area is designed.

Bellui runs on desktop, mobile and the web, so a new feature is normally
expected to work on all of them, or to say clearly where it does not (an
optional interface the app can check, rather than a silent no-op).

# Contributing code

## Setting up

You need the Go version named in [go.mod](go.mod), a C compiler and your
platform's development packages; [the getting started guide](https://bellui.com/develop/)
lists them per platform.

The repository holds several Go modules: the toolkit at the root, the command
line tool in `cmd/bellui`, and the extensions in `x/charts`, `x/dag`,
`x/editor`, `x/mmedia` and `x/tables`. Run the tests in each module you change.

## Checks

CI runs these on every pull request; running them first saves a round trip.

```sh
go test ./...                  # in each module you changed
go vet ./...
gofumpt -l .                   # formatting (stricter than gofmt)
goimports -l .                 # import grouping
staticcheck ./...
```

Install the tools with:

```sh
go install mvdan.cc/gofumpt@latest
go install golang.org/x/tools/cmd/goimports@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

The renderer also has an OpenGL build; if you touch rendering code, run
`go vet -tags gl ./...` too.

### With make

The root `Makefile` wraps these and the other CI legs; `make` lists every
target. The common ones:

```sh
make lint                       # formatting, vet, cyclomatic complexity, changelog
make test                       # main module, as CI's Linux leg (-race -tags ci)
make test-gl                    # with the OpenGL painter (under Xvfb if there is no display)
make test-mobile                # as the mobile driver
make test-a11y                  # the accessibility bridges
make test-x X=charts            # an x/ module against this checkout, as CI tests it
make test-cli                   # the bellui command
make test-wasm PKG=./lang/      # packages under GOOS=js, in Node
make test-pkg PKG=./widget/ RUN=TestEntry
make stress PKG=./internal/driver/embedded/ RUN=TestNoOSDriver_TouchStopsDragCoast COUNT=50
make bench PKG=./widget/ RUN=BenchmarkRefresh > new.txt   # then: benchstat old.txt new.txt
make tools                      # install the lint tools at the versions CI uses
```

`make stress` runs a test with every CPU core busy, which is how a timing
flake that only shows on loaded CI runners is reproduced locally. A full test
run can rewrite golden images under `internal/{oksvg,rasterx,render}/testdata`;
`make restore-goldens` puts them back, and discards any change to them.

## Style

- Everything new gets a unit test where that is possible.
- New public API has a doc comment ending in a `Since:` line with the version
  it first appears in.
- A user-visible change gets a changelog entry: one file in
  [`changelog.d/`](changelog.d/README.md) (or `cmd/bellui/changelog.d/` for the
  command line tool), not an edit to `CHANGELOG.md`, so that open pull
  requests do not conflict there.
- Breaking changes need a deprecation path, or an issue where they were agreed
  first.
- Data formats Bellui defines are PXF: files it reads or writes, data it
  stores, messages between its own processes. JSON is only for the edge, where
  another system defines the format: OAuth and OpenID Connect, web APIs,
  messages to and from JavaScript. Moving an existing JSON format keeps
  reading the old one for a while, as preferences did (`app/preferences.go`).
