# Release check

A guided manual validation of a release on desktop, Android, iOS and the web.
It builds the [gallery](https://github.com/belluikit/gallery) from this checkout,
plus a small companion app (`app/`) for the behaviours the gallery does not show:
locale switching, right-to-left scopes and windows, theme overrides, menus, and
touch and keys.

```sh
go run ./hack/releasecheck setup -platform desktop -run   # build everything the checks open
go run ./hack/releasecheck run -platform desktop          # answer each check; writes a report
go run ./hack/releasecheck list > checklist.md            # the whole checklist, to read or print
```

Platforms: `desktop`, `android`, `ios`, `web`. `-area` narrows a run or list to
some of `smoke, touch, i18n, rtl, a11y, charts, tables, dag, media, cli, edge`.
Checks marked SPECIAL are edge cases or the release's riskiest changes.

`run` appends every verdict to the report as it is given (a fail or a blocked
check asks what was seen), prints how to resume after `q`, and exits 1 if any
check failed. `setup -run` runs only the build steps it can; packaging and
installing on a phone, and starting the apps, are printed for you to run.

The checks live in `checks.go` and name the release they were written for. For
the next release, copy them forward: keep what still applies and replace the
checks that named this release's changes.
