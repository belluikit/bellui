# Maintaining the bundled Chromium (CEF)

On Linux the `webview` package renders with CEF, and **the application bundles
the engine** — `bellui package` lays a pinned payload out beside the binary
(#60, #105, #107). Every other platform borrows the OS's own web engine, so its
security updates arrive with the OS. Linux does not: a bellui app ships the
Chromium *we* pinned, and it stays at that version until we move it.

That makes this project a browser-engine redistributor for one platform. This
document is the obligation that comes with it — settled in #98.

## Cadence

**Security-driven, with a quarterly floor.**

- **Security-driven.** When a Chromium stable release carries a High or
  Critical CVE reachable from what we expose — the renderer, its IPC, the
  network stack, V8 — bump within about **two weeks** of the Chrome advisory.
  "Reachable" is a judgement call, and it is meant to be: an app embedding a
  web view is exposed to renderer bugs in a way a headless parser is not, so
  when in doubt, treat it as reachable and bump.
- **Quarterly floor.** Bump at least once a quarter even in a quiet stretch, so
  the pin never drifts more than ~3 Chromium majors behind stable. A pin that
  is merely *old* is a liability whether or not a specific CVE is named.

That lands around 6–8 bumps a year. Tracking every Chromium stable (~13/year)
was considered and rejected: each bump is a payload rebuild plus a `gocef`
header re-vendor if the API moved, and the marginal safety over the policy
above did not justify the standing cost.

## Owner

**bene** owns the bump.

There is no named backup today. If the owner is unavailable past a bump's
target window, the escalation is to **file a P0 in this repo** naming the
pending version — an app shipping a known-vulnerable Chromium is not a quiet
problem, and it should be visible in the backlog rather than waiting.

When a second maintainer exists, name them here; that is the only change this
section needs.

## Signal — how we learn a bump is due

- **[Chrome Releases blog](https://chromereleases.googleblog.com/)**, "Stable
  Channel Update for Desktop" posts. These carry the CVE list and severities;
  this is the primary source.
- **[CEF forum](https://magpcss.org/ceforum/) and
  [chromiumembedded/cef releases](https://github.com/chromiumembedded/cef/releases)**
  for when a matching CEF build exists — Chromium's fix is not actionable here
  until CEF has built it.
- **The automated freshness check** below, which is the backstop for both.

## Where the pin lives

Three places, in dependency order — a bump moves them in this order:

1. **`belluikit/gocef-libs`** mirrors the built CEF payload per platform.
   `manifest/manifest.go` carries `CEFVersion` (the full
   `<cef>+g<hash>+chromium-<version>` string) and the per-asset SHA-256. This
   is the authoritative pin.
2. **`belluikit/gocef`** vendors CEF's C API headers and pins `APIVersion`
   (currently 13300, deliberately at `CEF_API_VERSION_MIN`). A bump only moves
   this if the new build needs API the pin does not cover — re-check the
   `CEF_API_ADDED` guards for anything the shim binds before moving it.
3. **`bellui`** consumes both: `go.mod` here for `gocef`, and
   `cmd/bellui/go.mod` for `gocef-libs` (the CLI is what downloads and verifies
   the payload at package time — see `cmd/bellui/internal/commands/cefbundle.go`,
   which refuses an asset with no checksum).

A consumer picks a bump up with `go get`. That really is the whole story for an
app: no separate download step, no system package.

**Verify after a bump** — the payload is what every Linux webview app ships:

- `gocef`'s integration test against the new payload (`GOCEF_PAYLOAD=… go test
  -run TestCEFRuntime ./...`, plus the `GOCEF_EXTERNAL_PUMP=1` leg).
- `hack/cefhostprobe` headless, and `-live` on a real session — the #127
  matrix (both painters × X11/Wayland, popups, HiDPI, real input) is the
  acceptance shape.

## Staleness visibility

`.github/workflows/cef_freshness.yml` runs weekly. It reads the pinned
Chromium version out of the `gocef-libs` manifest, asks Google's version-history
API for the current Linux stable, and compares:

- **Same version** — quiet.
- **Behind, within the floor** — logs the drift and links the Chrome Releases
  blog, so the security-driven half of the policy gets a human read. No
  failure: being one patch behind is normal between bumps.
- **Three or more Chromium majors behind** — fails, and opens (or updates) a
  tracking issue in this repo. Chromium ships a major roughly every four weeks,
  so three majors is the quarterly floor expressed in the units the API
  actually gives us.

The check deliberately does not try to decide CVE reachability — no API answers
"is this bug reachable from an embedded renderer". It makes the drift
*impossible to not notice*; the judgement stays with the owner.
