# #803 — Wide-gamut and HDR output (design proposal)

Status: **proposal, for the owner to decide.** No code. Tracks #803 (steps 3
and 4 of #373). Every claim is marked **measured** (with how) or **inferred**.

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

**Not measured:** Windows (D3D12 swap chain colour space for `rgba16float`,
whether wgpu-native sets scRGB), Android (Vulkan `VK_EXT_swapchain_colorspace`
through wgpu), Linux (Wayland colour management through wgpu), the web
(WebGPU canvas `colorSpace: "display-p3"` and `toneMapping: "extended"`), and
iOS. Each needs the same capability read-back on that platform before the
design commits to it.

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

**Leaning: A.** *Local to the painter, reversible, inferred* (the pass cost is
not measured). **What would flip it:** the final pass costing a noticeable
share of a phone's frame time; measure it on the Pixel before committing.

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

## What I'd measure before you decide

1. **Windows** (omen): surface formats and the colour space wgpu-native sets
   for `rgba16float`. Falsifies D1-A if there's no extended-sRGB path.
2. **Android** (Pixel 9a): the same through Vulkan. Same falsifier.
3. **D2-A's final pass** on the Pixel: frame time with and without it.
4. **macOS, by eye and capture on the P3 Mac:** a `WideColor` P3 red next to
   sRGB red, in a throwaway prototype, to confirm the path end to end.

## Out of scope

Colour-managing the UI for colour-blind or high-contrast needs (themes do
that), print colour, and CMYK.
