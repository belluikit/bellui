# orcaspeech — Orca speech-capture harness (Linux, #981)

The `busctl` / `dbus-monitor` recipes in [`hack/a11yprobe`](../a11yprobe/README.md)
verify the AT-SPI2 **provider tree** the way Orca queries it. This harness goes
one layer further: it records what **Orca actually speaks**, turning the human
"does it sound right" listen into a diffable transcript — the Linux counterpart
of the Windows NVDA speech capture (`hack/a11yprobe/windows/nvda_speechcapture.ps1`).

## How it works

Orca speaks through **speech-dispatcher**'s default output module. The harness
installs a `sd_generic` module whose synth command *appends each utterance's
text to a log file* instead of (or as well as) producing audio, and points the
default module at it. Every string Orca would speak lands in the transcript.

No GPU, no audio device, and no screen reader are needed to validate the capture
itself — a plain `spd-say` walks the exact same path (`selftest`).

## Usage

```sh
hack/orcaspeech/capture.sh setup       # install the logging module (idempotent)
hack/orcaspeech/capture.sh selftest    # prove capture works via spd-say (no Orca)
hack/orcaspeech/capture.sh run         # enable Orca; live-tail the transcript
hack/orcaspeech/capture.sh run --probe # + build/launch hack/a11yprobe, walk its focus
hack/orcaspeech/capture.sh teardown    # remove the module; restore normal speech
```

Transcript path: `$BELLUI_ORCA_SPEECH_LOG`
(default `~/.cache/bellui-orca-speech/utterances.log`).

**Always `teardown` when done** — the log-only module leaves Orca *silent*
(no audio) until removed. Set `BELLUI_ORCA_SPEAK=1` before `setup` to also hear
each utterance through the system default while capturing.

`selftest` is the reproducible, dependency-free proof:

```
$ hack/orcaspeech/capture.sh selftest
[orcaspeech] PASS — spd-say utterance captured:
    push button, Click me
```

## Reading the bellui backend with Orca (the acceptance pass)

`run --probe` enables Orca and launches [`hack/a11yprobe`](../a11yprobe/README.md).
Orca reads the **active** window, so the probe must hold keyboard focus for its
widgets to be spoken — **click the probe window** (or Alt-Tab to it), then arrow
through it while the transcript records. Each focused widget should speak its
role + name, e.g. `push button, Click me` / `slider, 50` / the entry's text.

### Window-focus caveat (GNOME/Wayland)

Under a Wayland session, an app can't reliably give *its own* window keyboard
focus, and `xdotool windowactivate` on the XWayland surface is refused
(`_NET_WM_DESKTOP failed`). So the focus step is the human "click the window"
action — the same human-in-the-loop the on-device screen-reader passes have on
every platform. Driving focus via `Component.GrabFocus` over `busctl` only takes
effect once the probe is already the key window (the backend's `curWindow` focus
gating, matching the macOS key-window rule), so it complements the click, it
doesn't replace it.

If the probe is not the foreground window the transcript will fill with whatever
Orca *is* reading (the terminal, the desktop) — that is expected, not a harness
fault.

## Environment

Verified on the linux_parallels box: Orca 50.2, speech-dispatcher with the
`sd_generic` module, AT-SPI2 2.60.0, GNOME/Wayland. The capture mechanism
(`setup` + `selftest`) and Orca routing (Orca's live utterances landing in the
transcript) are both confirmed here. The full probe-reading pass is gated only
by the Wayland window-focus step above.
