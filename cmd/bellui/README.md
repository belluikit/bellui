# bellui command-line tool

The `bellui` CLI packages, bundles, installs and manages Bellui applications.

This is a **nested Go module** (`github.com/belluikit/bellui/cmd/bellui`) inside
the toolkit repo. It keeps its packaging-only dependencies (icns, goversioninfo,
rsrc, image codecs, …) out of the `github.com/belluikit/bellui` library module,
while living in the same repo as the toolkit so the embedded Android dex is
regenerated from the toolkit's own Java sources — no cross-repo drift.

## Install

```sh
go install github.com/belluikit/bellui/cmd/bellui@latest
```

Releases are tagged `cmd/bellui/vX.Y.Z` on this repo (nested-module tag prefix).

## Android dex

`internal/mobile/dex.go` embeds a compiled dex extending `NativeActivity`
(`internal/driver/mobile/app/GoNativeActivity.java` in the toolkit). It is
checked in and regenerated with the Android SDK on the `PATH`:

```sh
cd internal/mobile && go generate    # go run gendex/gendex.go -o dex.go
```

Because the CLI now lives beside the toolkit, `gendex` reads the toolkit's Java
directly (`../../../../internal/driver/mobile/app/*.java`). **Any change to that
Java must land with a matching dex regen in the same commit** — CI verifies the
committed dex matches a fresh regen.

## History

Migrated from `github.com/trendvidia/tools` (which mirrored `fyne.io/tools`);
that repo is archived. The move eliminated the cross-repo coupling where a dex
regen could ship ahead of the toolkit's native signatures.
