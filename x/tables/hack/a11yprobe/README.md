# a11yprobe — DataGrid accessibility live-verify

A probe app + Windows UIA driver that verify the `DataGrid`'s **real platform
accessibility tree** (issues #15/#16) — the layer the Go unit tests can't reach.
It mirrors the fyne fork's `hack/a11yprobe` pattern and reuses its app-agnostic
UIA client scripts.

This is a **separate module** (its own `go.mod`, `replace ../..`) so fytables'
library `go.mod` stays free of the GL/app dependency tree.

## What it checks

`main.go` puts one grid on screen: pinned-left (`id`) + 3 main columns + pinned-
right (`status`), grouped by `region`, 500 rows so the view virtualizes. Named
"Problems" for a stable UIA name. `windows/uia_assert.ps1` launches it, dumps the
live UIA tree with the fork's `uiatree.ps1`, and asserts:

- **#15** — a `Table` with a Grid pattern reporting **5 columns** (absolute
  order: pinned-left + main + pinned-right); `DataItem` cells with a GridItem
  pattern carrying row/column in that absolute space, **including both pinned
  columns**; region group-header rows as `Group` elements with ExpandCollapse.
- **#16** — the Grid pattern's `RowCount` (full extent) is far larger than the
  count of materialized `DataItem` cells: the grid virtualizes, surfacing
  "row R of N" without realizing every row.

`main.go -checkbox` adds the leading checkbox-selection column and pre-selects
two rows; `windows/uia_assert_31.ps1` drives that mode and asserts the semantics
finished in #29 (issue #31):

- **#29 checkbox cell** — the Grid pattern reports **6 columns** (checkbox +
  pinned-left + main + pinned-right); a `DataItem` at absolute column **0** wraps
  an interactive `CheckBox` (Toggle pattern); every data column shifts one place
  right (pinned-left → 1 … pinned-right → 5); and the pre-selected rows' column-0
  checkboxes read `Toggle=On` while the rest read `Off`.
- **#29 group action** — invoking the `ExpandCollapse` pattern's
  `Collapse()`/`Expand()` on a `Group` element actually toggles the group: the
  state flips **and** the Grid `RowCount` drops then restores as the group's
  child rows leave and re-enter the display, proving the call is routed through
  `ToggleGroup` (not a read-only state).

## Session requirement (important)

UIA/screen readers only see a window in an **interactive desktop session**
(console/RDP with an autologon user). Over SSH, or from a runner installed as a
**Windows service (session 0)**, the window lands in an invisible window station
and the tree is empty. The org's existing `windows_parallels` runners are
session-0 services (fine for their headless CI) — they **cannot** run this. It
must run in the logged-on user's session (e.g. launched via a scheduled task
with `/it`, or by hand over RDP).

## Run it (interactive session)

```powershell
cd hack\a11yprobe
go build -tags accessibility -o a11yprobe.exe .          # activates the UIA backend
$fyne = go list -m -f '{{.Dir}}' github.com/trendvidia/fyne/v2
powershell -ExecutionPolicy Bypass -File windows\uia_assert.ps1 `
  -ProbeExe .\a11yprobe.exe `
  -UiaTree (Join-Path $fyne 'hack\a11yprobe\windows\uiatree.ps1')
```

The probe's default mode groups by `region`; `-masterdetail` swaps in expandable
master-detail rows (#45) and `-tree` swaps in a 3-level hierarchical tree (#47),
where parent rows expose a `TreeItem` with `AccessibleLevel` (1..3) and an
`ExpandCollapse` pattern routed to `ToggleTreeNode`. In `-tree` the leftmost
column (`id`) is pinned-left, so the disclosure chevron/indent renders on that
pinned column — the pinned-hierarchy path of #50. These modes are mutually
exclusive; `-checkbox` composes with any of them.

For the #31 checkbox + group-action checks, and the #45 master-detail check,
build once and run the extra scripts against the same probe (each passes its own
`-checkbox` / `-masterdetail` / `-tree` flag to it):

```powershell
powershell -ExecutionPolicy Bypass -File windows\uia_assert_31.ps1 `
  -ProbeExe .\a11yprobe.exe `
  -UiaTree (Join-Path $fyne 'hack\a11yprobe\windows\uiatree.ps1')

powershell -ExecutionPolicy Bypass -File windows\uia_assert_45.ps1 `
  -ProbeExe .\a11yprobe.exe `
  -UiaTree (Join-Path $fyne 'hack\a11yprobe\windows\uiatree.ps1')
```

The `.github/workflows/a11y-uia.yml` job wraps these for a future **interactive**
self-hosted runner (manual dispatch only).

## Verified result

Captured against the real Windows UIA backend (windows/arm64, v2.17.0),
interactive session:

```
[Window] "fytables a11y probe" {Window,Transform} FOCUSED
  [Table] "Problems" {Invoke,Grid} grid=504x5
    [DataItem] {GridItem} cell=1,0 …        # pinned-left  (absolute col 0)
    [Group] "North (125)" {ExpandCollapse} expand=Expanded
    [DataItem] {GridItem} cell=1,1 … 9,3    # main cols 1–3
    [DataItem] {GridItem} cell=1,4 …        # pinned-right (absolute col 4)
```

`grid=504x5` (500 data + 4 group headers × 5 columns) with only **45 cells
materialized** across all 5 columns (9 rows each, pinned included) — the absolute
column model (#15) and virtualized extent (#16) confirmed end-to-end through the
platform, plus the group-header ExpandCollapse semantics (#15).

## Verified result (#31 — checkbox cell + group action)

Captured with `-checkbox` against the real Windows UIA backend (windows/arm64,
v2.17.0), interactive session:

```
[Table] "Problems" {Invoke,Grid} grid=504x6
  [DataItem] "1000"        {Invoke,GridItem,SelectionItem} selected=True  cell=1,0   # pinned-left 'id' -> col 0
  [Group] "North (125)" {ExpandCollapse} expand=Expanded
  [DataItem]               {Invoke,GridItem,SelectionItem} selected=True  cell=1,1   # checkbox cell -> col 1 (visual slot)
    [CheckBox] {Toggle} toggle=On                                                    #   pre-selected row: checked
  [DataItem] "Problem 000" {Invoke,GridItem,SelectionItem} selected=True  cell=1,2   # main cols 2-4
  [DataItem]               {Invoke,GridItem,SelectionItem} selected=False cell=3,1
    [CheckBox] {Toggle} toggle=Off                                                   #   unselected row: unchecked
  [DataItem] "open"        {Invoke,GridItem,SelectionItem} selected=True  cell=1,5   # pinned-right 'status' -> col 5

EXPANDCOLLAPSE before:        state=Expanded  gridRows=504
EXPANDCOLLAPSE afterCollapse: state=Collapsed gridRows=379   # ToggleGroup ran: 125 North rows left
EXPANDCOLLAPSE afterExpand:   gridRows=504                    # round-trip restored
A11Y UIA #31 ASSERTIONS PASSED
```

`grid=504x6` — the checkbox column is rendered in its visual slot just right of
the pinned-left band, so pinned-left `id` stays at column 0 and the checkbox cell
reports column 1, matching visual left-to-right order (#35); a nameless
`DataItem` wraps an interactive `CheckBox` whose `Toggle` tracks the row's
selection (`On` only for the two pre-selected rows). Every cell exposes
`SelectionItem` with `selected=` reflecting `StateSelected` — the whole selected
row reads `selected=True` (#34). Driving the `Group`'s `ExpandCollapse` pattern
collapses and re-expands the group for real — RowCount 504 → 379 → 504 —
confirming the action is wired through `ToggleGroup` (#29).

Per-cell NVDA speech carries the synthetic-key limitation noted below.

## Verified result (#45 — master-detail disclosure)

Captured with `-masterdetail` (rows 0 and 2 pre-expanded) against the real
Windows UIA backend (windows/arm64, v2.17.0), interactive session:

```
[Button] "1000 details" {ExpandCollapse} expand=Expanded
[Button] "1001 details" {ExpandCollapse} expand=Collapsed
[Button] "1002 details" {ExpandCollapse} expand=Expanded
...
MASTERDETAIL disclosures: 7 total, 2 expanded, 5 collapsed
MASTERDETAIL '1000 details' after Collapse(): Collapsed
MASTERDETAIL '1000 details' after Expand():  Expanded
A11Y UIA #45 ASSERTIONS PASSED
```

Each master row exposes a `Button` carrying the `ExpandCollapse` pattern (the
`masterRowA11y` disclosure node) labelled by the row's first value; the
pre-expanded rows read `Expanded`, the rest `Collapsed`. Driving one disclosure's
`Collapse()`/`Expand()` flips its state for real — the call is wired through
`ToggleDetail` (#45), matching the group-header `ExpandCollapse` behavior.
Keyboard `Enter`/`Space` toggles the same detail band (covered by Go tests).

## NVDA speech capture (#566)

`windows/nvda_capture.ps1` runs real NVDA (portable) with the fork's
`speechcapture` global plugin, launches the probe, drives a focus walk, and
merges NVDA's spoken utterances into a transcript:

```powershell
# one-time: portable NVDA (any recent build)
nvda_2026.1.1.exe --create-portable-silent --portable-path C:\nvda_portable --minimal
$fyne = go list -m -f '{{.Dir}}' github.com/trendvidia/fyne/v2
powershell -ExecutionPolicy Bypass -File windows\nvda_capture.ps1 `
  -NvdaExe C:\nvda_portable\nvda.exe -ProbeExe .\a11yprobe.exe `
  -PluginSrc (Join-Path $fyne 'hack\a11yprobe\windows\speechcapture')

# add -Checkbox for the #36 checkbox-cell + group-toggle steps
powershell -ExecutionPolicy Bypass -File windows\nvda_capture.ps1 -Checkbox `
  -NvdaExe C:\nvda_portable\nvda.exe -ProbeExe .\a11yprobe.exe `
  -PluginSrc (Join-Path $fyne 'hack\a11yprobe\windows\speechcapture')
```

Captured on the real screen reader (NVDA 2026.1.1, interactive session):

```
### grid window foreground
   | Problems table
```

NVDA announces the grid as **"Problems table"** — the container role + accessible
name are spoken, not just present in the UIA tree.

### Checkbox cell + group toggle (#36)

`nvda_capture.ps1 -Checkbox` runs the probe with its checkbox-selection column
and adds two steps driven by **real UIA events** (not injected keys): SetFocus on
the interactive `CheckBox` in a pre-selected row, and driving the group's
`ExpandCollapse` pattern. Captured on the real screen reader (NVDA 2026.1.1):

```
### focus (checkbox cell in a pre-selected row)
   | data item selected row 2 column 2
   | check box checked

### ExpandCollapse.Collapse() / Expand()
   | not checked
   | checked
```

NVDA **speaks the selection and the checkbox state**: a selected cell is spoken
as *"data item selected row 2 column 2"* (role + `SelectionItem` selected state +
grid position, confirming #29/#34), and the select control as *"check box
checked"* / *"not checked"* — driven by focus/state-change events, not injected
keys.

**Limitation (residual, #36 / #566).** Two things are **not** reliably driven by
a script: (1) per-cell arrow navigation (NVDA filters *synthetic* arrow keys),
and (2) the group's expanded/collapsed announcement — fyne group-header rows are
not keyboard/UIA-focusable, so NVDA never sits on the group to voice its
disclosure (during the toggle it keeps reporting the checkbox it was tracking,
hence the *checked/not checked* above). Both need a physical keypress + a human
listen; the provider side is already proven by the UIA dumps above (group
`ExpandCollapse` toggles the grid for real, RowCount 504 → 379 → 504).
