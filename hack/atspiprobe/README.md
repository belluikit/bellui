# atspiprobe — pure-Go AT-SPI2 provider spike (pre-rebrand tracker #489)

Research spike for **pre-rebrand tracker #489** (a11y Phase 6: Linux screen-reader support).
Proves that a Bellui app can expose an accessible tree to Linux assistive
technology (Orca) by speaking the `org.a11y.atspi.*` **AT-SPI2** D-Bus protocol
**directly in pure Go** — via `github.com/godbus/dbus/v5`, already a fork
dependency — with **no cgo and no Rust/AccessKit**.

It is deliberately standalone: it hand-builds a tiny static tree
(application → frame → { button, label }) and does **not** touch the bellui canvas
or `internal/accessibility`. The production backend is the follow-up
implementation issue; this only de-risks the handshake and wire format.

## Run

On a Linux box with the AT-SPI2 stack (`at-spi2-core`) and Orca installed:

```bash
go run ./hack/atspiprobe
```

The provider discovers the a11y bus (`org.a11y.Bus.GetAddress`), dials it,
exports its objects, registers with the registry via `Socket.Embed`, and stays
alive emitting a focus event on the button.

## Verify (no audio needed) — walk the tree as an AT client

```bash
A11Y=$(busctl --user call org.a11y.Bus /org/a11y/bus org.a11y.Bus GetAddress \
        | sed 's/^s "//;s/"$//')
NAME=:1.39   # the "our a11y bus name" the probe logs on startup

# Our app is a child of the AT-SPI desktop root — Orca's tree entry point:
busctl --address "$A11Y" call org.a11y.atspi.Registry \
  /org/a11y/atspi/accessible/root org.a11y.atspi.Accessible GetChildren

# Read role / name / state / extents of any node (this is exactly what Orca does):
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/2 \
  org.a11y.atspi.Accessible GetRoleName          # -> "push button"
busctl --address "$A11Y" get-property $NAME /org/a11y/atspi/accessible/2 \
  org.a11y.atspi.Accessible Name                 # -> "Click me"

# Reverse path — an AT action reaching back into Go:
busctl --address "$A11Y" call $NAME /org/a11y/atspi/accessible/2 \
  org.a11y.atspi.Component GrabFocus             # logs "GrabFocus ..." in the probe
```

## Verify with Orca itself

```bash
gsettings set org.gnome.desktop.a11y.applications screen-reader-enabled true
# monitor which methods Orca calls on our provider:
dbus-monitor --address "$A11Y" | grep "destination=$NAME"
orca --replace &
```

When Orca starts it probes every app on the bus. Against this provider it calls
`Application.GetApplicationBusAddress`, `Cache.GetItems`, and
`Properties.Get` on our root — i.e. it **discovered and read the pure-Go app**.
(`Cache.GetItems` is intentionally unimplemented here; Orca falls back to the
per-property tree walk, which is why `Properties.Get` follows. Implementing
`org.a11y.atspi.Cache` is a follow-up performance phase, not a blocker.)

## What this proves / what it doesn't

Proven on the linux_parallels box (arm64, GNOME/Wayland, Orca 50.2,
AT-SPI2 2.60.0):

- pure-Go `Socket.Embed` handshake → app appears under the desktop root
- `Accessible` + `Application` + `Component` reads (role/name/state/extents)
- focus `StateChanged` events in the verified `siiva{sv}` wire format
- reverse action path (`GrabFocus`) into Go
- **Orca actively discovers and queries the app**

Not covered here (all deferred to the implementation issue): wiring the real
`internal/accessibility.Node` tree, the `Cache`/`GetItems` fast path,
`Text`/`Value`/`Action`/`Selection`/`Table` interfaces, and the Wayland
absolute-coordinate limitation (an app cannot learn its window's screen
position under Wayland — a constraint shared with AccessKit).
