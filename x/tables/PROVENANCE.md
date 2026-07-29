# x/tables — consolidated from github.com/trendvidia/fytables

Copied from `github.com/trendvidia/fytables` @ `origin/main` @
`bb451696c4ddf9ad3c7656b6ca28b429babca4f6` (= tag `v0.14.0`, 2026-07-24).

Only mechanical rewrites were applied: the module path became
`github.com/belluikit/bellui/x/tables`, `package fytables` became
`package tables`, and `github.com/trendvidia/fyne/v3` became
`github.com/belluikit/bellui`. Comments that name `fytables` historically are
left as written. No API changed.

## Re-cut history

| Ref | Source | Why |
|---|---|---|
| `v1.0.0-rc.5` | `main` @ `bb45169` (v0.14.0) | Corrects the below |
| `v1.0.0-rc.1`…`rc.4` | `chore/fyne-v3` @ fork point ≈ v0.3.0-rc.2 | **Wrong source** |

The original consolidation was cut from the source repo's `chore/fyne-v3`
branch rather than `main` (belluikit/bellui#85). That branch was an abandoned
duplicate — the real fyne-v3 migration had merged to `main` as PR #79 weeks
earlier and development continued there — so it sat **50 commits** behind. The
published module was missing, among much else:

* `fix: resolve ColumnDef.Field against struct rows, not only maps`
* `fix: implement the dot notation ColumnDef.Field has always documented (#84)`
* `feat(a11y): implement event.Scrollable so AT scroll gestures page the grid (#82)`
* `DisplayRow` / `ScrollToRow`, `CommitEditing`, `OnCellContextMenu`,
  `ColumnDef.CanEdit` / `ColumnDef.Editor`
* the `export/pdf.go` and `export/pxf.go` exporters entirely

Bellui Player shipped `v1.0.0-rc.1` on that older grid with a fully green suite
(belluikit/player#4) — a stale branch still compiles, and nothing in the
pipeline asks whether the source was current.

**The lesson this file exists to enforce:** record the ref *and* the sha, not a
version. The stale content corresponded to no tag at all, which is why every
attempt to identify it by version number failed. Verify the source is
`origin/main` before copying — a branch whose name matches what you are looking
for is not evidence that it is current.
