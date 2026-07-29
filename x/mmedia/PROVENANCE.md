# x/mmedia — consolidated from github.com/trendvidia/fyplayer

Copied from `github.com/trendvidia/fyplayer` @ `origin/main` @
`57fd6fba5cbff907628410d1654c707e97762dd7` (2026-07-28).

**That sha carries no tag** — it is three commits past `v0.3.0`, the newest tag
in the source repo. Recording the ref and the sha rather than a version is the
rule `x/tables` exists to enforce; here it is not a formality, because no
version number identifies this content.

The source ref was checked before copying, as `docs/REBRAND.md` requires:
`HEAD` equalled `origin/main`, and no remote branch carried unmerged commits.
`fyplayer` does have a `chore/fyne-v3` branch — the same name that sent
`x/tables` and `x/editor` out stale — but here it is fully merged into `main`
and holds nothing `main` lacks.

## What changed

Mechanical rewrites only. **28 of the 32 Go files are reproduced byte-for-byte**
by applying this set to the source, and the `internal/source` test media and
`.gitignore` are byte-identical:

| From | To |
|---|---|
| `github.com/trendvidia/fyplayer` | `github.com/belluikit/bellui/x/mmedia` |
| `package fyplayer` | `package mmedia` |
| `github.com/trendvidia/fyne/v3` | `github.com/belluikit/bellui` |
| `fyne.{Do,Container,TextStyle,TextAlign}` | `bellui.*` |
| `github.com/trendvidia/goffmpeg` | `github.com/belluikit/goffmpeg` |
| prose: "Fyne widget/renderer/app", "fyne's painter selection / thread checker" | "Bellui …", "bellui's …" |

No API changed. The internal packages (`audio`, `pipeline`, `source`,
`subtitle`) keep their names.

`videosurface_test.go` quotes a toolkit panic string in a comment; it was
updated with the rest of the prose and still matches `app.go` verbatim
("Attempt to access current Bellui app when none is started").

**The other four files carry one further change: `gofumpt -w`.** `fyplayer`
gated on `gofmt` only, so its tree was never gofumpt-clean; bellui's
`static_analysis` runs `gofumpt -d -e .` over the whole working tree, x/
modules included, and that command exits 1 on any finding. Leaving these as
found would have kept the gate red. The edit is whitespace and nothing else — a
leading blank line removed after the opening brace of
`new{Audio,Media,Video}PlayerFromPipeline`, and a blank line added between
`FFmpegSource.VideoSize` and `Duration`:

| File | Change |
|---|---|
| `audioplayer.go` | −1 blank line |
| `player.go` | −1 blank line |
| `videoplayer.go` | −1 blank line |
| `internal/source/ffmpeg.go` | +1 blank line |

`git diff -w` against the rewritten source is empty for all four, so the
byte-for-byte guarantee above still holds up to whitespace across the whole
module.

`go.sum` was **regenerated, never substituted** — its hashes are keyed by
module path, so rewriting `trendvidia`→`belluikit` inside it would produce
entries that verify against nothing. It contains zero `trendvidia` lines.

## Two things this move is not

**The FFmpeg binding advanced two minor versions.** `fyplayer` pinned
`trendvidia/goffmpeg v0.2.0`; `belluikit/goffmpeg` publishes only `v0.4.0`,
which pulls `goffmpeg-libs` `v0.1.0` → `v0.3.0`. The delta is additive
on the surface this module uses — `Open`, `OpenReader`, `IDecoder`,
`VideoInfo`, `AudioInfo`, `MediaType*` are unchanged — and brings an FFmpeg
bump past the upstream v4l2 fix plus a v4l2 buffer-unmap leak fix. It is called
out because "mechanical rewrite" does not cover a dependency bump, not because
anything here had to adapt to it.

This is a version bump, not a fork divergence: `belluikit/goffmpeg` is in sync
with its source. `v0.4.0` is an annotated tag whose commit is
`trendvidia/goffmpeg`'s `origin/main` exactly (`c897242`, zero commits after
it), and 175 of the 179 files match after the module-path rewrite. The four
that do not are `go.mod`, a regenerated `go.sum`, and two files —
`.github/workflows/build.yml` and `.gitignore` — that still name
`trendvidia/goffmpeg-libs` where `go.mod` requires the `belluikit` one. That
last pair is a defect in `goffmpeg`'s own build workflow, not in the path this
module consumes: `x/mmedia` resolves the published modules, which is what the
verification below exercises.

`goffmpeg-libs` is also why this module declares `go 1.26.2` while its `x/`
siblings declare `1.25.0`.

**It has no CI.** `fyplayer` carried `.github/workflows/ci.yml`; it was dropped
rather than copied, because a workflow file inside a subdirectory of another
repo never runs, and this one named runners and a `GOPRIVATE` value that no
longer apply. `x/tables` and `x/editor` kept theirs and they are dead files.
The `x/` modules have no CI at all — belluikit/bellui#13.

What that issue must cover for this module specifically: **both painters**.
The video surface takes a different path under each — GL updates its texture in
place, WebGPU (the default) goes through the raster generator — and
`streaming_gl.go` / `streaming_webgpu.go` are selected by build tag, so only
building and testing both catches a change that compiles against one and breaks
the other. The source repo's CI did this and it is the leg most easily lost.

## Verification

Run from this directory, at the copy:

```sh
env GOWORK=off go build ./...
env GOWORK=off go build -tags gl ./...
env GOWORK=off go vet ./...
env GOWORK=off go test -race -count=1 ./...     # 5 packages
env GOWORK=off go test -tags gl -count=1 ./...  # 5 packages
gofmt -l . && goimports -l .                    # both empty
grep -rn "trendvidia\|fyne" --include='*.go' .  # empty
```

All green, both painters, on darwin/arm64 — the platform `goffmpeg-libs` ships
linkable artifacts for on this machine.
