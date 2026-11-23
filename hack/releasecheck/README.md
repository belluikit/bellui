# Release check

A guided manual validation of a release on desktop, Android, iOS and the web.
It builds the [gallery](https://github.com/belluikit/gallery) from this checkout,
plus a small companion app (`app/`) for the behaviours the gallery does not show:
locale switching, right-to-left scopes and windows, theme overrides, menus,
touch and keys, disabled widgets and containers, the navigation drawer, swipes
that start on nested widgets, and wrapping and long-pressed text.

```sh
go run ./hack/releasecheck setup -platform desktop -run   # build everything the checks open
go run ./hack/releasecheck run -platform desktop          # answer each check; writes a report
go run ./hack/releasecheck list > checklist.md            # the whole checklist, to read or print
```

Platforms: `desktop`, `android`, `ios`, `web`. `-area` narrows a run or list to
some of `smoke, widgets, touch, i18n, rtl, a11y, charts, tables, dag, media, cli,
edge`.
Checks marked SPECIAL are edge cases or the release's riskiest changes.

`run` appends every verdict to the report as it is given (a fail or a blocked
check asks what was seen), prints how to resume after `q`, and exits 1 if any
check failed. `setup -run` runs only the build steps it can; packaging and
installing on a phone, and starting the apps, are printed for you to run.

## The macOS probe

On a Mac, the companion app can walk the checks a script can drive by itself:

```sh
go run ./hack/releasecheck/app -probe /tmp/rc-probe   # writes report.md and captures, exits 1 on a failure
```

It posts clicks, drags and keys to the window through AppKit, so they take the
path real input does: GLFW, then the driver's hit test, focus manager and
gestures. A first step clicks an enabled button and stops the run unless the
tap arrives, so a check that expects nothing to happen cannot pass because
input never arrived. It asserts what it can read back (status lines, chip
states, focus, positions, stroke widths in pixels) for WG-01–03, WG-05,
WG-07, RTL-07, SM-07, SM-08, TK-06, TK-13, I18N-01/03/04 and ED-05, and saves
real-painter captures, listed as LOOK in the report, for a person to judge.

The driver reads a click's position from the system cursor, so **the probe
moves the mouse pointer while it runs: leave the mouse alone until it quits**
(about a minute). It needs no Accessibility permission. It does not cover the
gallery, a screen reader, sound, the OS appearance switch or dragging the
window by hand; those checks stay with `run`.

The checks live in `checks.go` and name the release they were written for. For
the next release, copy them forward: keep what still applies and replace the
checks that named this release's changes.
