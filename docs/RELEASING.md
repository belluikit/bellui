# Releasing

A release here is **annotated tags plus a `CHANGELOG.md` entry plus a GitHub
Release on the main-module tag** (step 7). The Releases page is part of the
release: the newest stable is marked **latest**, rc tags are marked
pre-release, and the `x/*` and `cmd/bellui` tags deliberately get no Release
object of their own (they always accompany a main-module release; a Release
per tag would bury the real entries under six echoes). This reverses the earlier
"tags only, `gh release list` empty by design" stance — decided in
[#136](https://github.com/belluikit/bellui/issues/136) (2026-08-05) so that
the Releases page users actually look at is authoritative. There is no
release automation yet
(a release-train Action was tracked as pre-rebrand tracker #731); this
document is the procedure.

The repository is one main module plus **five** nested `x/*` modules
(`charts`, `dag`, `editor`, `mmedia`, `tables`), and `cmd/bellui` — the CLI —
which is its own module too. The `x/*` modules and `cmd/bellui` pin the
toolkit by version and carry the same version as the toolkit, even when their
own code is unchanged. So `go install
github.com/belluikit/bellui/cmd/bellui@<version>` works for every release, and
the install lines in the docs can name the toolkit's version
([#1464](https://github.com/belluikit/bellui/issues/1464)). Until v1.5.0 the
command was tagged only when it changed, and v1.4.0 has no `cmd/bellui` tag.

Version line: `v1.0.0-rc.N` while the rebrand soaks, then `v1.0.0`.

Each step below gives the commands; the root `Makefile` has a target for most
of them, noted under the step. `make release-status` lists what is still open
in the release's milestone, the open pull requests and main's latest CI runs.

## Before the steps: validate by hand

Unit tests and CI do not see a phone's keyboard, a screen reader or a locale
change at run time. `hack/releasecheck` walks a tester through the release on
a desktop, Android, iOS and the web, with steps and expected results for each
check, and writes a report ([its README](../hack/releasecheck/README.md)):

```sh
make release-setup P=desktop    # build the bellui command, the gallery and the companion app from this checkout
make release-run P=desktop      # one check at a time; exits non-zero if any failed
make release-list               # the whole checklist as release-validation-<version>.md
```

`P` is `desktop`, `android`, `ios` or `web`. `make gallery`, `make gallery-web`,
`make release-app` and `make release-app-web` start the apps the checks open.
Tag only on reports with no failure; file each failure first.

## The one trap: ordering is forced

**The main-module tag must exist and be fetchable before the `x/*` pins move.**
`go get github.com/belluikit/bellui@<version>` in an `x/` module resolves the
tag (from GitHub directly on a machine whose `GOPRIVATE` covers belluikit,
from the proxy elsewhere) — bumping the pins first simply fails. That is why the
`x/*` tags of past releases sit on a commit *after* the main tag rather than on
it. Do not try to make them share a commit.

## Checklist

### 1. Changelog

Entries for the release are files in `changelog.d/` (#511). Fold them into
`CHANGELOG.md`, which also deletes them:

```sh
go run ./hack/changelog release v1.0.0-rc.7
```

With make: `make release-changelog VERSION=v1.0.0-rc.7`.

Then write the section's opening summary under its heading, as earlier
releases have one. Do the same for the `bellui` command's changelog, with the
same version:

```sh
go run ./hack/changelog release -C cmd/bellui v1.0.0-rc.7
```

When the command has no fragments, the tool refuses to write an empty section.
Add it by hand below the Unreleased section of `cmd/bellui/CHANGELOG.md`:

```markdown
## v1.0.0-rc.7 — 2026-11-25

Cut with toolkit v1.0.0-rc.7. No changes of its own. Built against bellui
v1.0.0-rc.7.
```

Commit both as `docs: changelog for <version>`.

A `bellui` command that writes PXF translation catalogs (#522) is released
with or after the first toolkit release that reads them, never before:
`bellui init` resolves the newest toolkit, and v1.2.0 fails on a PXF catalog
with `no unmarshaler registered for pxf`.

### 2. Green CI on main

The tag should name a verified commit. Wait for main's checks — a tag is
cheap to cut and awkward to move.

### 3. Tag the main module

`make release-tag-commands VERSION=v1.0.0-rc.7` prints this step's and step
5's commands; it does not run them.

```sh
git tag -a v1.0.0-rc.7 -m "v1.0.0-rc.7"
git push origin v1.0.0-rc.7
```

The changelog entry was dated when it was written in step 1. If that is not
the date you are tagging on, correct it in a commit first and wait for its
checks — the entry, the tag and the GitHub Release should name the same day.

Then ask the public Go proxy for it. `make release-proxy VERSION=v1.0.0-rc.7`
does this:

```sh
curl -fsS https://proxy.golang.org/github.com/belluikit/bellui/@v/v1.0.0-rc.7.info
```

A consumer gets the module from proxy.golang.org, so that is the check that
counts. Asking the proxy for the version is also what makes it fetch the tag
and add the version to index.golang.org, the index pkg.go.dev reads. Until
something asks, pkg.go.dev does not list the release. If the request fails
right after the push, wait a few minutes and run it again.

Do not use `go list -m github.com/belluikit/bellui@<version>` for this. On a
machine whose `GOPRIVATE` covers `github.com/belluikit/*` (the maintainer's
does), `go` fetches straight from GitHub and never asks the proxy, even with
a fresh `GOMODCACHE`. Clearing the setting with `GOPRIVATE=` does not help:
an empty variable falls back to the value `go env -w` stored. v1.5.0 passed
`go list -m` and was reported as served by the proxy. The proxy first fetched
it about 90 minutes later, and pkg.go.dev listed it only then
([#1463](https://github.com/belluikit/bellui/issues/1463)).

### 4. Bump the `x/*` and `cmd/bellui` pins, and verify each by hand

`make release-pins VERSION=v1.0.0-rc.7` runs the loop below for every module
(`X="charts tables"` narrows the `x/*` ones; `cmd/bellui` is always included).

For each of `x/charts x/dag x/editor x/mmedia x/tables cmd/bellui`:

```sh
cd $m
go get github.com/belluikit/bellui@v1.0.0-rc.7
go mod tidy          # see below — not optional
go build ./... && go vet ./... && go test ./...
cd ../..
```

**Why this is manual even though `x_tests.yml` exists.** That workflow builds,
vets and tests all five modules on every PR — but it first rewrites each
`go.mod` with an ephemeral `replace` pointing at the checkout (its "Resolve the
toolkit from this checkout" step, never committed). So CI proves the *code*
works against the current tree; it never resolves the module against a
*published* pin. Those are different failures, and only the second one is what
a consumer hits.

That is not hypothetical: at rc.2, `x/charts` did not build after the pin bump —
the rebased WebGPU bindings pull in per-OS `wgpu-native` libs modules whose
`go.sum` entries `go get` does not add, and only that one module was short them
([#13 comment](https://github.com/belluikit/bellui/issues/13#issuecomment-5091126506)).
`go mod tidy` is what fixes that class, which is why it is in the loop above.

Then tidy the nested modules that build against an `x/*` checkout through
`replace ../..` (today `x/editor/examples/wasm`, `x/mmedia/hack/*` and
`x/tables/hack/a11yprobe`):

```sh
for d in $(find x -mindepth 3 -name go.mod -exec dirname {} \;); do
  (cd "$d" && go mod tidy && go build -o "$(mktemp -d)/" ./... && go vet ./...) || echo "FAILED: $d"
done
```

Nothing else resolves them: the loop above names only the five modules, and
`x_tests.yml` never enters these directories. Whenever an `x/*` module's
requirements move, each of them stops resolving (`go: updates to go.mod
needed`) until it is tidied — v1.1.0's fsnotify bump did that to
`x/editor/examples/wasm`. The wasm example builds for `GOOS=js GOARCH=wasm`;
build it that way too.

Commit all of these `go.mod`/`go.sum` changes together, push.

> This step will still be needed after
> [#13](https://github.com/belluikit/bellui/issues/13) closes. #13 is about CI
> covering these modules at all; the gap here is the ephemeral replace, which
> is deliberate (it is what lets a PR change both sides at once) and is not
> going away.

### 5. Tag the `x/*` modules and `cmd/bellui`

First check that no module already has the version. An `x/*` module can be
released on its own between toolkit releases, and a version the Go proxy has
served can never be moved:

```sh
git fetch origin --tags
for m in charts dag editor mmedia tables; do git tag -l "x/$m/v1.0.0-rc.7"; done
git tag -l "cmd/bellui/v1.0.0-rc.7"
```

Any output names a version that is taken. Give that module the next free
version and say so in the changelog entry. `make release-tag-commands` prints a
warning for each one. v1.3.0-rc.2 hit this: x/mmedia had shipped its own rc.2
on 2026-11-02, so with toolkit rc.2 it went out as rc.3.

Then, on the commit from step 4:

```sh
for m in x/charts x/dag x/editor x/mmedia x/tables cmd/bellui; do
  git tag -a "$m/v1.0.0-rc.7" -m "$m v1.0.0-rc.7"
done
git push origin --tags
```

Then ask the public proxy for each of them, as in step 3:

```sh
make release-proxy-modules VERSION=v1.0.0-rc.7
```

It prints `served:` and the proxy's answer for each module, or `NOT SERVED:`
and fails. A module that has no tag at the version fails here: run on
v1.4.0, it reports `cmd/bellui`, which that release did not tag.

**Do not skip this.** rc.6 did: the pins were bumped but the tags were never
cut, so the `x/*` modules have had no tag since rc.5 and `x/mmedia` has never
been tagged at all — [#148](https://github.com/belluikit/bellui/issues/148) is
that gap. A consumer of `x/charts` sees rc.5 as the newest release while the
toolkit is at rc.6.

### 6. Fresh-consumer smoke check

`make release-smoke VERSION=v1.0.0-rc.7` runs this recipe in a fresh
temporary directory.

Cheap, and it catches a release that only works from inside this checkout.
It resolves everything the way a consumer without `GOPRIVATE` does: through
proxy.golang.org only, with no fallback to GitHub, into an empty module
cache (a module already in the cache is never fetched again). The cache sits
beside the smoke module, not inside it, or `go mod tidy` reads the cache as
the module's own packages.

The `GOPRIVATE` variables must be empty, and `GOENV=off` is what makes them
empty: with the go env file in use, an empty value falls back to the stored
one (step 3). A pattern that matches nothing is not the same as empty. Any
non-empty `GONOPROXY`, set directly or through `GOPRIVATE`, makes the go
command look in the module cache before the proxy. Once bellui is cached at a
version, `go install github.com/belluikit/bellui/cmd/bellui@<that version>`
then fails with `module github.com/belluikit/bellui@<version> found, but does
not contain package github.com/belluikit/bellui/cmd/bellui`. That is a go
command bug ([golang/go#58116](https://github.com/golang/go/issues/58116)),
and the website's install section tells consumers who meet it what to run
([#1469](https://github.com/belluikit/bellui/issues/1469)). `GOENV=off` also
drops the stored `GOCACHE`, so the recipe passes it through.

The command is installed after the build, with bellui already in the cache,
because that is the order a consumer meets it in.

```sh
GOCACHE="$(go env GOCACHE)"   # read before GOENV=off hides the stored value
d="$(mktemp -d)" && mkdir "$d/smoke" && cd "$d/smoke"
export GOENV=off GOCACHE GOPRIVATE= GONOPROXY= GONOSUMDB= \
  GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOMODCACHE="$d/modcache"
go mod init smoke
cat > main.go <<'EOF'
package main

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("smoke")
	w.SetContent(widget.NewLabel("hello"))
	bellui.ShowAndRun(w) // a free function, NOT a Window method
}
EOF
go mod edit -require=github.com/belluikit/bellui@v1.0.0-rc.7
go mod tidy   # resolves the imports AND writes the transitive go.sum entries
go build .
GOBIN="$d/bin" go install github.com/belluikit/bellui/cmd/bellui@v1.0.0-rc.7
"$d/bin/bellui" version
```

Two things this recipe gets right on purpose, both verified against rc.6 while
writing this:

- **Write the program before resolving the module.** Running
  `go get github.com/belluikit/bellui@<version>` in an empty module adds the
  module but not the sums for the packages you then import, so the build fails
  with `missing go.sum entry for module providing package …/go-locale`. Letting
  `go mod tidy` resolve real imports is what produces a complete `go.sum` — the
  same failure mode as the `x/charts` one in step 4, seen from the consumer
  side.
- **`bellui.ShowAndRun(w)` is a package-level function** in Bellui, not
  `w.ShowAndRun()`, and the program therefore imports the root package. Getting
  that wrong is the usual way this smoke test "fails" for the wrong reason.

Repeat with one `x/` module (`go get github.com/belluikit/bellui/x/charts@…`),
in the same environment, to prove a consumer can build against step 5's tags.
Then remove the cache. Its files are read-only, so `rm -r` cannot remove it:

```sh
go clean -modcache
```

### 7. GitHub Release on the main-module tag

Last on purpose: the Release is the announcement, so it goes up only after the
pins, tags and smoke check have proven the version actually resolves for a
consumer. For an rc:

```sh
gh release create v1.0.0-rc.7 --verify-tag --prerelease \
  --title "v1.0.0-rc.7" \
  --notes "Release candidate — see the v1.0.0-rc.7 section of CHANGELOG.md."
```

For a stable, mark it **latest** and carry the changelog entry as the notes
(for `v1.0.0` itself, the v1 line summary that
[#136](https://github.com/belluikit/bellui/issues/136) describes):

```sh
gh release create v1.0.0 --verify-tag --latest \
  --title "v1.0.0" --notes-file /tmp/v1-notes.md
```

`--verify-tag` makes the command fail rather than invent a tag if step 3 was
skipped. Only the main-module tag gets a Release; the `x/*` and
`cmd/bellui` tags do not (see the top of this document).

## Linux webview releases carry a browser engine

If the release changes the pinned CEF payload, the bump procedure and the
security obligations that come with redistributing Chromium are in
[docs/CEF-MAINTENANCE.md](CEF-MAINTENANCE.md).
