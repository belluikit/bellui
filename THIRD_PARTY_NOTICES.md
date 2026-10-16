# Third-Party Notices

Bellui is a derivative work of the [Fyne](https://github.com/fyne-io/fyne)
toolkit. Fyne's copyright notice and BSD 3-Clause license are reproduced in
[`LICENSE`](LICENSE), and its contributors are listed in [`AUTHORS`](AUTHORS).

This file lists everything else in this repository that comes from a third
party: code, fonts, icons and images. Each entry names where the license text
lives in the tree.

## Code

### fyne-x

Bellui ports or adapts several widgets from Fyne's separate extension project,
[fyne-x](https://github.com/fyne-io/fyne-x) (`fyne.io/x/fyne`):

- `widget/gif.go` (AnimatedGif) — adapted from `widget/gif.go`
- `widget/filetree.go` (FileTree) — adapted from `widget/filetree.go`
- `layout/responsive.go` — inspired by `layout/responsive.go`
- `dialog/about.go` — follows the API of `dialog/about.go`
- `widget/toolbar_twostate.go` — follows the API of `widget/twostatetoolbaraction.go`
- `widget/testdata/gif/` — test images and their README, copied unchanged

fyne-x is distributed under the following license:

```text
BSD 3-Clause License

Copyright (C) 2020 Fyne.io developers and community (see AUTHORS)
All rights reserved.


Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of Fyne.io nor the names of its contributors may be
      used to endorse or promote products derived from this software without
      specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### Vendored packages

Each of these directories keeps its upstream license file. Its
`PROVENANCE.md` records the upstream version and what changed.

| Directory | Upstream | License | License file |
|---|---|---|---|
| `internal/oksvg` | github.com/fyne-io/oksvg (fork of srwiley/oksvg) | BSD-3-Clause, Copyright (c) 2018 Steven R Wiley | `internal/oksvg/LICENSE` |
| `internal/rasterx` | github.com/srwiley/rasterx | BSD-3-Clause, Copyright (c) 2018 Steven R Wiley | `internal/rasterx/LICENSE` |
| `internal/ico` (`reader.go`) | github.com/fyne-io/image | Apache-2.0 | `internal/ico/LICENSE-reader` |
| `internal/ico` (`writer.go`) | github.com/fyne-io/image | BSD-3-Clause, Copyright (c) 2012 The Go Authors | `internal/ico/LICENSE-writer` |
| `internal/render` | github.com/belluikit/render (fork of go-text/render) | Unlicense OR BSD-3-Clause | `internal/render/LICENSE` |

### Files with their own notice

- Some files in `internal/driver/mobile` and `cmd/bellui/internal/mobile` come
  from `golang.org/x/mobile` by way of Fyne. Those files keep their
  `Copyright … The Go Authors` header. The Go Authors' BSD 3-Clause text is in
  `internal/ico/LICENSE-writer`.
- `cmd/bellui/internal/templates/data/webgl-debug.js` — Copyright (c) 2012
  The Khronos Group Inc. Its MIT-style license is in the file header.

## Fonts

| Font | Files | Copyright | License | License file |
|---|---|---|---|---|
| Inter | `theme/font/Inter-Regular.ttf`, `InterSymbols-Regular.ttf` | Copyright 2020 The Inter Project Authors | SIL OFL 1.1 | `theme/font/LICENSE_Inter.txt` |
| Noto Sans | `theme/font/NotoSans-*.ttf`, `NotoSansPunct-bellui.ttf` | Copyright 2015 Google Inc. | SIL OFL 1.1 | `theme/font/LICENSE.txt` |
| Noto Sans Symbols, Symbols 2, Math | `theme/font/NotoSansSymbols-bellui.ttf`, `NotoSansSymbols2-bellui.ttf`, `NotoSansMath-bellui.ttf` | Copyright 2022 The Noto Project Authors; Copyright 2022 Google LLC | SIL OFL 1.1 | `theme/font/LICENSE_NotoSansSymbols.txt` |
| Noto Sans CJK SC | `theme/cjk/NotoSansCJKsc-bellui.otf` | © 2014-2021 Adobe | SIL OFL 1.1 | `theme/cjk/LICENSE-NotoSansCJK.txt` |
| Roboto | `theme/font/Roboto-*.ttf` | Copyright 2011 The Roboto Project Authors | SIL OFL 1.1 | `theme/font/LICENSE_Roboto.txt` |
| Office Code Pro | `theme/font/OfficeCodePro-*.otf` | © 2015 Adobe & Nathan Rutzky | SIL OFL 1.1 | `theme/font/LICENSE.txt` |
| DejaVu Sans Mono for Powerline | `theme/font/DejaVuSansMono-Powerline.ttf` | Copyright (c) 2003 Bitstream, Inc.; DejaVu changes are public domain | Bitstream Vera / DejaVu License | `theme/font/LICENSE_DejaVu-Powerline.txt` |
| EmojiOne Color | `theme/font/EmojiOneColor.otf`, `internal/render/testdata/EmojiOneColor.otf` | Copyright 2016 Adobe Systems Incorporated | MIT | `theme/font/LICENSE_EmojiOneColor.txt` |

Fonts used only by tests:

| Font | Files | Copyright | License | License file |
|---|---|---|---|---|
| Noto Sans, Noto Mono, Noto Sans CJK | `internal/render/testdata/NotoSans-*.ttf`, `theme/testdata/NotoMono-Regular.ttf`, `internal/painter/testdata/NotoCJKtest.otf` | Google / Adobe | SIL OFL 1.1 | `internal/render/testdata/OFL.txt` |
| Greybeard (port of UW ttyp0) | `internal/render/testdata/Greybeard-22px.ttf` | Copyright (c) 2012-2015 Uwe Waldmann; modified work Copyright 2018 Andy Walker | MIT (ttyp0 variant) | `internal/render/testdata/LICENSE-Greybeard.txt` |

## Icons and images

- **Material Design icons.** Most icons in `theme/icons/` are Google's
  [Material Design icons](https://github.com/google/material-design-icons),
  inherited from Fyne. They are licensed under Apache-2.0, which is reproduced in
  `theme/icons/LICENSE-MaterialIcons.txt`.
- **Go gopher.** `default-icon.png`, `spinner_dark.gif` and `spinner_light.gif`
  in `cmd/bellui/internal/templates/data/` show a gopher. The Go gopher was
  designed by Renée French and is licensed under
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
  `default-icon.png` is AI-generated: it was made with Google Gemini. Google's
  [Terms of Service](https://policies.google.com/terms/archive/20260730)
  (effective 30 July 2026) say "Google won't claim ownership" of content
  generated with its services.
- **earth.gif.** `widget/testdata/gif/earth.gif` and `earth-once.gif` are CC BY-SA 3.0,
  by Wikipedia user Marvel, derived from NASA imagery
  ([source](https://commons.wikimedia.org/wiki/File:Rotating_earth_(large).gif)).
  They came by way of fyne-x.
- **SVG test icons.** The test icons in `internal/oksvg/testdata/` are from
  Freepik / Flaticon, under the attribution terms in
  `internal/oksvg/testdata/LICENSE`.
- **Sign-in provider logos.** `auth/logos/` holds the Apple, GitHub, Google,
  Microsoft and Okta logos. They are trademarks of their owners and are included
  only to identify those sign-in providers on login buttons. Each owner's brand
  guidelines govern their use.

## Go dependencies

Go module dependencies are listed in `go.mod` and `go.sum`; their licenses
can be inspected with `go-licenses` or similar tooling. Dependencies are
limited to the following licenses: Apache-2.0, BSD-2-Clause, BSD-3-Clause, MIT,
ISC, Unlicense (when dual-licensed with one of these), or SIL OFL-1.1.
