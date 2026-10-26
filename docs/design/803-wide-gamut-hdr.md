# #803 — Wide-gamut and HDR output (design proposal)

Status: **decided (owner, 2026-10-26), not built.** D1 and D3–D6 as proposed
below; D2 revised after measuring (see D2). Tracks #803 (steps 3 and 4 of
#373). Every claim is marked **measured** (with how) or **inferred**. The probe
code is on branch `probe/wide-surface-803` (not for merge).

## What the user would get

Today every colour Bellui draws is an 8-bit sRGB value, so even on a display
that can show more:
- a brand colour or a photo outside sRGB is clipped to the nearest sRGB colour
  (#802 converts a Display P3 photo to sRGB, and clips it);
- nothing can be brighter than white (no HDR video, no HDR photo highlights).

Wide gamut lets an app show colours outside sRGB on a display that has them.
HDR lets it show highlights brighter than UI white. HDR needs wide gamut
first: both need values outside sRGB's 0–1 range.

## How Bellui draws today

- **Colours** are Go `color.Color`, read through `RGBA()` as 16-bit
  premultiplied values, in practice 8-bit sRGB (`color.NRGBA` in themes, PXF
  theme files write `"#rrggbbaa"`).
- **The painter writes sRGB-encoded values** and blends them as they are, in
  gamma space, into a non-sRGB 8-bit surface (`chooseSurfaceFormat` picks
  `BGRA8Unorm`; `srgb_fidelity_test.go` pins it, #322). Translucency,
  anti-aliasing and gradients all look the way they do because of this.
- **The surface's colour space**, measured on macOS (#815): wgpu-native tags the
  8-bit surface sRGB, so macOS colour-matches it.

## Measured on macOS (M3 Max, wgpu-native as pinned, 2026-10-26)

- A Metal surface offers `bgra8unorm-srgb`, `bgra8unorm`, **`rgba16float`** and
  **`rgb10a2unorm`** (surface capabilities read back from a real device).
- When wgpu-native configures **`rgba16float`**, it sets the layer's colour space
  to **`kCGColorSpaceExtendedLinearSRGB`**. `rgb10a2unorm` gets plain
  `kCGColorSpaceSRGB`, which gives 10-bit precision but no wider gamut.
- So on macOS, extended-range linear sRGB is the wide-gamut path wgpu already
  takes. Values above 1.0 or below 0.0 are colours outside sRGB, and above 1.0
  is also brighter than white, which is where EDR (Apple's HDR) starts.

## Measured on Windows and Android (2026-10-26)

`hack/colorprobe` (probe branch) draws grey swatches at sRGB 64/128/192 with the
surface format forced. A surface shown as sRGB reads them back unchanged; one
composited as linear light reads 137/188/225, sRGB's encoding of 0.251/0.502/0.753.

- **Windows** (D3D12, standard-gamut ASUS monitor, captured with GDI): offers
  `rgba16float` and `rgb10a2unorm`; `rgba16float` reads **137/188/225**, so it
  is composited as linear (scRGB). The extended range is inferred: the monitor
  can't show beyond sRGB, but DXGI's only linear 709 colour space is scRGB.
- **Android** (OPPO OPD2403, Adreno 750, Vulkan, Android 16, P3 panel, captured
  with `screencap`, which comes out in Display P3 there): same formats, same
  **137/188/225**. A float clear colour of Display P3 red in extended linear
  sRGB, (1.2249, −0.0421, −0.0196), reads **255/0/0**, while the sRGB red
  swatch reads 234/51/35: **a colour outside sRGB reaches the screen.**

**Not measured:** Linux (Wayland colour management through wgpu), the web
(WebGPU canvas `colorSpace: "display-p3"` and `toneMapping: "extended"`), iOS,
and HDR on any platform.

## The decisions

### D1. How an app expresses a colour outside sRGB

| | Option | What changes | Reverts by | Notes |
|---|---|---|---|---|
| **A** | **A new colour type implementing `color.Color`**, e.g. `bellui.WideColor{R, G, B, A float32}` in extended linear sRGB, with constructors such as `DisplayP3(r, g, b)` | additive public type; its `RGBA()` returns the clipped sRGB value, so every existing code path keeps working, and the painter type-asserts it to use the wide value | revert, while nothing persists it | matches the fork's rule of optional capabilities found by type assertion; a wide colour degrades to its sRGB clip wherever it isn't supported |
| B | Change Bellui's colour type to float everywhere | every API that takes or returns a colour | migration | the most uniform, and the most expensive; breaks every app |
| C | Display P3 only (an 8- or 10-bit P3 type, like CSS `color(display-p3 …)`) | additive type, P3-tagged surface | revert | simpler for designers, but no HDR, and wgpu tags `rgb10a2unorm` sRGB on macOS, so Bellui would override the layer after every configure |

**Leaning: A**, classified *additive public API, reversible until persisted,
the macOS half measured, the rest inferred, latent* (no app asks for it yet).
**What would flip it:** an app whose colours mostly live outside sRGB (B
starts to pay), or Windows and Android offering only a P3 path, not extended
sRGB (C).

### D2. Where blending happens

The surface that carries wide colours is linear float. Blending linear values
changes how every translucent edge, shadow and gradient looks, which is the
#322 problem in reverse.

| | Option | Cost |
|---|---|---|
| **A** | **Keep blending in gamma space**: draw into an offscreen float target in sRGB-encoded (extended, sign-preserving) values, as today, and convert to linear in one final full-screen pass | one extra full-screen pass per frame on wide-gamut windows only; unchanged look |
| B | Blend linear | the physically correct result, but every app's translucency and anti-aliasing change, and every golden image changes |

**Measured:** the pass, at the OPPO tablet's full 2120×3000, costs **1.15 ms**
on its Adreno 750 (median; 1.07 ms into an 8-bit target, so reading the float
source dominates). At its 120 Hz that is **about 14% of every frame**: the
falsifier for A as proposed. An M3 Max takes 77 µs.

**Decided (owner, 2026-10-26): A, limited to what changed.** Keep gamma-space
blending, and run the conversion only over the region that changed this frame,
reusing partial repaint's damage regions. Where a platform offers an extended
*non-linear* sRGB surface, skip the pass (macOS has `kCGColorSpaceExtendedSRGB`;
Android's extended-range sRGB dataspace is inferred; Windows has none), as a
later optimisation. Scrolling or full-screen motion in a wide window still pays
the full pass. **What would flip it:** wide apps that scroll full-screen most of
the time on 120 Hz devices; then B, linear blending in wide windows only.

### D3. When a window draws wide

| | Option |
|---|---|
| **A** | **Opt-in per app** (an app setting, or the first `WideColor`/HDR image shown): 8-bit sRGB stays the default, at half the surface memory |
| B | Always, where the display supports it |

**Leaning: A**: no cost for apps that don't use it. *Reversible, inferred.*

### D4. Theme and data formats

Theme files write `"#rrggbbaa"`. A wide colour needs a syntax. Leaning: the
CSS Color 4 form, `"color(display-p3 1 0.2 0.1)"` and
`"color(srgb-linear …)"`, accepted next to hex. It's additive, and every
existing file still reads. *Wire-format addition, so foundational; reverts
only while no file uses it.*

### D5. Images

#802 converts a decoded image to sRGB and clips it. For a wide window, the
same converter would target extended linear sRGB at 16-bit float instead,
using twice the memory of 8-bit RGBA. *Local, reversible; follows D3.*

### D6. HDR (after wide gamut)

On macOS, values above 1.0 on an extended-linear surface are EDR once the
layer sets `wantsExtendedDynamicRangeContent` (inferred from Apple's
documentation, not measured). The headroom (`NSScreen.maximumExtendedDynamicRangeColorComponentValue`)
varies with the display and its brightness, so an HDR image or video needs
tone mapping to it. Windows (scRGB, HDR10), Android and the web each differ.
**Proposal:** decide D1–D5 first, ship wide gamut, then design HDR around the
first consumer (x/mmedia HDR video is the likely one) with its own
measurements.

## Probes

1. Windows: done, supports D1-A (above).
2. Android: done, supports D1-A, extended range measured (above). Run on the
   OPPO tablet; a Pixel was not attached.
3. D2's final pass: done, falsified the full-screen pass; D2 revised (above).
4. macOS end to end: done. On a MacBook Pro's built-in P3 screen (Liquid
   Retina XDR), an `rgba16float` surface reads the greys as linear
   (137/188/225), and the band cleared to Display P3 red as extended linear
   sRGB reads **255/1/0** in the P3-space capture, while sRGB red reads
   234/51/35: a colour outside sRGB reaches the screen.

## Out of scope

Colour-managing the UI for colour-blind or high-contrast needs (themes do
that), print colour, and CMYK.
