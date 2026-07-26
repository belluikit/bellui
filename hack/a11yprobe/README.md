# a11yprobe — accessibility backend live-verify harness

The probe app plus platform client drivers used to verify the screen-reader
backends (epic #490) against a real windowing system, from the outside, the
way assistive technology sees them. Golden tests cover the neutral semantic
layer; this harness covers what they cannot: the platform bridge, per-window
state, focus routing and action dispatch. Backends: macOS NSAccessibility
(#485), Windows UIA (#487), Linux AT-SPI2 (#981).

## Probe app

```sh
go run -tags accessibility ./hack/a11yprobe
```

Two windows. **Probe A** carries the feature coverage — slider
(AXIncrement/AXDecrement), multi-line + password entries (text protocol,
secure-field subrole), markdown headings (virtual nodes / heading rotor),
selectable label (read-only text protocol), TreeView with rename + delete
(outline rows, disclosure, custom actions), Toast/Snackbar triggers
(live-region announcements). **Probe B** is minimal and exists to verify
disjoint per-window element trees and key-window focus gating (#554).

Widget callbacks print `PROBE`-prefixed lines to stdout so a driver can
confirm an action reached the widget, not just the bridge.

## macOS clients (`darwin/`)

```sh
./hack/a11yprobe/darwin/build.sh
./hack/a11yprobe/darwin/axset  $(pgrep -f a11yprobe | head -1)  # entry text protocol
./hack/a11yprobe/darwin/axtree $(pgrep -f a11yprobe | head -1)  # outline rows + disclosure
./hack/a11yprobe/darwin/axsel  $(pgrep -f a11yprobe | head -1)  # selectable-label selection
# off-viewport virtualized-cell realize (#637 Stage 4) — needs a11yrealizeprobe:
go run -tags accessibility ./hack/a11yrealizeprobe &                    # note its pid
./hack/a11yprobe/darwin/axcell $(pgrep -f a11yrealizeprobe) 500000 1    # AXCellForColumnAndRow
```

`axcell` requests a cell far past the viewport of a 1,000,000-row virtual grid
through the `AXCellForColumnAndRow` parameterized attribute (which the bridge
maps to `accessibilityCellForColumn:row:`) — the same call VoiceOver's table
navigation makes. It asserts the returned `AXCell`'s title / row-column index
range match the request, that re-requesting returns an equal element (stable
realized-cell identity), that an out-of-range coordinate is rejected, and that
`AXPress` on the off-screen cell reaches `AccessibleScrollCellIntoView` (watch
the probe's `PROBE scrollIntoView` line). The VoiceOver *speech* pass for
off-viewport table navigation is the remaining human step.

Raw `AXUIElement` clients exist because **System Events cannot deliver
range-typed attribute sets**: `set value of attribute "AXSelectedTextRange"`
silently no-ops (its list→AXValue coercion never happens), while booleans
(`AXDisclosing`) and strings (`AXValue`) set fine. Everything read-only, and
performing actions, works from System Events / osascript.

The terminal running the clients (or osascript) needs Accessibility
permission: System Settings → Privacy & Security → Accessibility.

## Operational gotchas

- **Keep the display awake**: `caffeinate -d -u` for the whole session.
  While the display sleeps, System Events reports 0 windows and
  `entire contents` comes back empty — indistinguishable from a broken
  bridge.
- The app takes several seconds to register its windows with AX after
  launch (GPU init); poll `count windows` before diagnosing.
- Nodes outside a height-starved widget's clip are **culled** from the
  accessibility tree. If rows seem missing, give the widget real space
  before concluding the bridge dropped them (the probe uses a Border layout
  for the tree for exactly this reason).
- Custom actions surface in System Events under `actions of <element>`
  (with their human titles), **not** as the value of the `AXCustomActions`
  attribute (reads as `missing value`). Perform by index:
  `perform (action N of el)`.
- In System Events, injected bellui elements come **before** the titlebar
  buttons: `button 1 of window 1` is the first bellui button.
- bellui window titles may read as empty via `name of windows`; address
  windows by title in a `tell` block (`window "Probe A"`) or by index.

## Windows clients (`windows/`)

PowerShell UIA clients over `System.Windows.Automation` (no install needed),
used to verify the UIA backend (#487) the way Narrator/NVDA query it:

```powershell
# from the repo root, with the probe running:
go run -tags accessibility ./hack/a11yprobe    # note its pid
powershell -File hack\a11yprobe\windows\uiatree.ps1 <pid>          # control view + patterns
powershell -File hack\a11yprobe\windows\uiatree.ps1 <pid> -Raw     # raw view (incl. structural groups)
powershell -File hack\a11yprobe\windows\uiaact.ps1 <pid> "Tap me" invoke
powershell -File hack\a11yprobe\windows\uiaact.ps1 <pid> "" setrange 80    # empty name → first RangeValue element (the slider)
powershell -File hack\a11yprobe\windows\uiaact.ps1 <pid> "" setvalue "hi"  # empty name → first Value-settable element (the entry)
powershell -File hack\a11yprobe\windows\uiatext.ps1 <pid>          # Text pattern navigation
```

`uiatree` prints each element's control type, name, supported patterns,
value/range/toggle/selection/grid state and bounds; `uiaact` performs
Invoke/Toggle/Select/Expand/Collapse/SetFocus/RangeValue.SetValue/
Value.SetValue on the first element matching a Name; `uiatext` walks the
Text pattern (document range, word/line units, settable selection).
Cross-check the `PROBE` stdout lines to confirm actions reached the widget.

### Off-viewport cell realize (`uiarealize.ps1`, #637 Stage 3)

`uiarealize.ps1` verifies the virtualized-cell realize seam: a UIA client asks
a grid's `GridPattern.GetItem(row, col)` for a cell **far past the viewport**
(none is materialized), which the bridge realizes on demand through
`internal/accessibility.RealizeCell` — the same call NVDA's table navigation
makes. It uses the dedicated `hack/a11yrealizeprobe` (a widget reporting a
1,000,000×3 extent with zero realized cells), not the main probe's small Table.

```powershell
go run -tags accessibility ./hack/a11yrealizeprobe    # note its pid
powershell -File hack\a11yprobe\windows\uiarealize.ps1 <pid> 500000 1
```

It asserts the returned element's `Name`/`GridItem` coordinates match the
request, that `ContainingGrid` round-trips, that re-reading the same cell yields
the same element (`Automation.Compare` — stable realized-cell identity), that an
out-of-range `GetItem` is rejected, and that `VirtualizedItem.Realize()` reaches
`AccessibleScrollCellIntoView` (watch the probe's `PROBE scrollIntoView` line).
Verified end-to-end on the console session via `schtasks /IT` (an SSH session
lives in a session-0 window station where GLFW windows are invisible and UIA
cannot enumerate them — schedule probe + client as one interactive task onto the
logged-in desktop and read the output from a file). The NVDA *speech* pass for
off-viewport table navigation is the remaining human step.

### NVDA speech capture (`nvda_speechcapture.ps1` + `speechcapture/`)

The UIA clients above verify the *provider* tree the way a screen reader
queries it. `nvda_speechcapture.ps1` goes one layer further: it runs the real
**NVDA** screen reader with the `speechcapture/` global plugin loaded (which
logs every queued utterance via `speech.extensions.pre_speechQueued`), drives
a scripted walk over Probe A + Probe B, and merges NVDA's speech with per-step
markers into a diffable transcript. This turns the human "does it sound right"
listen (#566) into a reproducible artifact.

```powershell
# one-time: a portable NVDA copy (any recent version)
nvda_2026.1.1.exe --create-portable-silent --portable-path C:\nvda_portable --minimal
go build -tags accessibility -o a11yprobe.exe ./hack/a11yprobe
powershell -File hack\a11yprobe\windows\nvda_speechcapture.ps1 `
  -NvdaExe C:\nvda_portable\nvda.exe -ProbeExe .\a11yprobe.exe
```

Captured **reliably** (these fire on UIA events, independent of key injection):
focus tracking, role/label/value/state on focus, password masking (speaks
`protected` + bullet count, never the content), per-window identity (focus
follows the key window), and live-region announcements (Toast/Snackbar speak
unprompted). A clean run's transcript reads e.g. `slider 50`, `edit hello`,
`edit protected … 6 bullet`, `Show Toast button`, `announcement test toast`.

**Not** reliably driven — needs physical keys + a human listen: widget
behaviour reached by **arrow keys** (slider increment, tree-row navigation,
caret motion). NVDA installs a low-level keyboard hook that filters
*synthetically injected* keystrokes, so those steps behave differently than a
real keypress (injected character keys, e.g. typing into an entry, *do* reach
the widget). That residual is the human half of #566; the provider side of
those same behaviours is already covered by `uiaact`/`uiatext` above.

Same physical-session requirement as the UIA clients: over SSH the app's
windows land in a session-0 window station (invisible to UIA, never
foreground), so focus tracking never engages. Use RDP or the console.

## Linux clients (AT-SPI2, `busctl`)

The pure-Go AT-SPI2 backend (#981) speaks the `org.a11y.atspi.*` D-Bus
protocol, so **any** AT client reads it — no bespoke driver binaries needed.
`busctl` walks the exact methods Orca calls; no audio required.

```sh
go run -tags accessibility ./hack/a11yprobe &          # opens Probe A + Probe B

A11Y=$(busctl --user call org.a11y.Bus /org/a11y/bus org.a11y.Bus GetAddress \
        | sed 's/^s "//;s/"$//')

# Find our app under the AT-SPI desktop root (ToolkitName == "Bellui"):
for n in $(busctl --address "$A11Y" call org.a11y.atspi.Registry \
    /org/a11y/atspi/accessible/root org.a11y.atspi.Accessible GetChildren \
    | grep -oP ':[0-9.]+'); do
  busctl --address "$A11Y" get-property "$n" /org/a11y/atspi/accessible/root \
    org.a11y.atspi.Application ToolkitName | grep -q Bellui && echo "$n" && break
done
NAME=:1.73   # the bus name printed above

# Two frames (Probe A / Probe B), each with the real widget tree:
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/root \
  org.a11y.atspi.Accessible GetChildren

# Role / name / state / extents of any node (exactly what Orca reads):
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/9 \
  org.a11y.atspi.Accessible GetRoleName        # -> "slider"
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/10 \
  org.a11y.atspi.Accessible GetState           # editable, focusable, multiline, ...

# Reverse path — an AT action reaching back into Go:
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/9 \
  org.a11y.atspi.Component GrabFocus           # -> b true

# --- P1 (#986): Text / Value / Action / EditableText -----------------------
# GetInterfaces reports which of these a node supports; query accordingly.
ENTRY=/org/a11y/atspi/accessible/10; SLIDER=/org/a11y/atspi/accessible/9

# Text (entry): content, caret, word/line segmentation:
busctl --address "$A11Y" get-property $NAME $ENTRY org.a11y.atspi.Text CharacterCount
busctl --address "$A11Y" call $NAME $ENTRY org.a11y.atspi.Text GetText ii 0 13
busctl --address "$A11Y" call $NAME $ENTRY org.a11y.atspi.Text GetStringAtOffset iu 0 1  # word
busctl --address "$A11Y" call $NAME $ENTRY org.a11y.atspi.Text GetStringAtOffset iu 0 3  # line

# Value (slider): min / max / current:
busctl --address "$A11Y" get-property $NAME $SLIDER org.a11y.atspi.Value CurrentValue

# Action (button node 17): enumerate + invoke:
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/17 \
  org.a11y.atspi.Action GetActions                            # -> a(sss) press ...
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/17 \
  org.a11y.atspi.Action DoAction i 0                          # -> b true (fires the tap)

# EditableText: drive an edit; watch object:text-changed / text-caret-moved fire:
busctl --address "$A11Y" call $NAME $ENTRY \
  org.a11y.atspi.EditableText InsertText isi 0 "X" 1          # -> b true

# --- P2 (#987): Cache fast path -------------------------------------------
# One round-trip snapshot of the whole tree (what Orca prefers over walking):
busctl --address "$A11Y" call $NAME /org/a11y/atspi/cache \
  org.a11y.atspi.Cache GetItems                              # -> a((so)(so)(so)iiassusau) N ...

# Watch AddAccessible / RemoveAccessible as widgets appear and windows close
# (RemoveAccessible on teardown is mandatory — GNOME bug 650090):
dbus-monitor --address "$A11Y" \
  "type='signal',interface='org.a11y.atspi.Cache'"

# Watch focus / structure events the way Orca subscribes (no audio):
dbus-monitor --address "$A11Y" \
  "type='signal',interface='org.a11y.atspi.Event.Object'" \
  "type='signal',interface='org.a11y.atspi.Event.Window'"
```

The final **Orca speech + focus-tracking** pass (window actually focused) is
the human-in-the-loop acceptance step, like the darwin/UIA backends:

```sh
gsettings set org.gnome.desktop.a11y.applications screen-reader-enabled true
orca --replace &
```

Node paths are stable per object across frames but assigned at first sight, so
the exact numbers above vary per run — walk `GetChildren` to discover them.

## Extending to other platforms

The probe app is platform-neutral — only the client drivers are
platform-specific. macOS (`darwin/`), Windows (`windows/`) and Linux
(AT-SPI2, above) are covered; a new backend needs an equivalent client set
against the same probe, and the announcement and text-protocol seams it
consumes are already exercised here.
