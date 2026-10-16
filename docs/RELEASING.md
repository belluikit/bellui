# Releasing

A release here is **annotated tags plus a `CHANGELOG.md` entry plus a GitHub
Release on the main-module tag** (step 7). The Releases page is part of the
release: the newest stable is marked **latest**, rc tags are marked
pre-release, and the `x/*` tags deliberately get no Release object of their
own (they always accompany a main-module release; a Release per tag would
bury the real entries under five echoes). This reverses the earlier
"tags only, `gh release list` empty by design" stance — decided in
[#136](https://github.com/belluikit/bellui/issues/136) (2026-08-05) so that
the Releases page users actually look at is authoritative. There is no
release automation yet
([#731](https://github.com/belluikit/bellui/issues/731) tracks the release-train
Action); this document is the procedure.

The repository is one main module plus **five** nested `x/*` modules
(`charts`, `dag`, `editor`, `mmedia`, `tables`), and `cmd/bellui` — the CLI —
which is its own module too. The `x/*` modules pin the toolkit by version and
carry the same version as the toolkit, even when their own code is unchanged.

Version line: `v1.0.0-rc.N` while the rebrand soaks, then `v1.0.0`.

## The one trap: ordering is forced

**The main-module tag must exist and be fetchable before the `x/*` pins move.**
`go get github.com/belluikit/bellui@<version>` in an `x/` module resolves the
tag from the proxy — bumping the pins first simply fails. That is why the
`x/*` tags of past releases sit on a commit *after* the main tag rather than on
it. Do not try to make them share a commit.

## Checklist

### 1. Changelog

Write the entry, commit as `docs: changelog for <version>`.

### 2. Green CI on main

The tag should name a verified commit. Wait for main's checks — a tag is
cheap to cut and awkward to move.

### 3. Tag the main module

```sh
git tag -a v1.0.0-rc.7 -m "v1.0.0-rc.7"
git push origin v1.0.0-rc.7
```

Confirm the proxy can see it before continuing — this is what step 4 depends on:

```sh
go list -m github.com/belluikit/bellui@v1.0.0-rc.7   # may take a moment to appear
```

### 4. Bump the `x/*` pins, and verify each by hand

For each of `charts dag editor mmedia tables`:

```sh
cd x/$m
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

Commit the five `go.mod`/`go.sum` changes together, push.

> This step will still be needed after
> [#13](https://github.com/belluikit/bellui/issues/13) closes. #13 is about CI
> covering these modules at all; the gap here is the ephemeral replace, which
> is deliberate (it is what lets a PR change both sides at once) and is not
> going away.

### 5. Tag the `x/*` modules

On the commit from step 4:

```sh
for m in charts dag editor mmedia tables; do
  git tag -a "x/$m/v1.0.0-rc.7" -m "x/$m v1.0.0-rc.7"
done
git push origin --tags
```

**Do not skip this.** rc.6 did: the pins were bumped but the tags were never
cut, so the `x/*` modules have had no tag since rc.5 and `x/mmedia` has never
been tagged at all — [#148](https://github.com/belluikit/bellui/issues/148) is
that gap. A consumer of `x/charts` sees rc.5 as the newest release while the
toolkit is at rc.6.

### 6. Fresh-consumer smoke check

Cheap, and it catches a release that only works from inside this checkout:

```sh
cd "$(mktemp -d)"
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

Repeat with one `x/` module (`go get github.com/belluikit/bellui/x/charts@…`)
to prove step 5's tags are visible to the proxy.

### 7. GitHub Release on the main-module tag

Last on purpose: the Release is the announcement, so it goes up only after the
pins, tags and smoke check have proven the version actually resolves for a
consumer. For an rc:

```sh
gh release create v1.0.0-rc.7 --verify-tag --prerelease \
  --title "v1.0.0-rc.7" \
  --notes "Release candidate — see the Unreleased section of CHANGELOG.md."
```

For a stable, mark it **latest** and carry the changelog entry as the notes
(for `v1.0.0` itself, the v1 line summary that
[#136](https://github.com/belluikit/bellui/issues/136) describes):

```sh
gh release create v1.0.0 --verify-tag --latest \
  --title "v1.0.0" --notes-file /tmp/v1-notes.md
```

`--verify-tag` makes the command fail rather than invent a tag if step 3 was
skipped. Only the main-module tag gets a Release; the `x/*` tags do not (see
the top of this document).

## Linux webview releases carry a browser engine

If the release changes the pinned CEF payload, the bump procedure and the
security obligations that come with redistributing Chromium are in
[docs/CEF-MAINTENANCE.md](CEF-MAINTENANCE.md).
