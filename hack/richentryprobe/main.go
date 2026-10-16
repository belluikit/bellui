// Command richentryprobe is the live-verify harness for widget.RichEntry (pre-rebrand tracker #667),
// the editable rich-text surface (epic pre-rebrand tracker #648). RichEntry had only unit tests and
// software-rendered masters — this drives it in a running app on the default
// WebGPU/Metal painter so the things golden masters can't fully vouch for get a
// human eye: the caret across style runs and at image rows, the empty-field
// caret + MD3 chrome (pre-rebrand tracker #749), selection highlights spanning runs, in-memory image
// rendering, per-run restyle (ApplyStyle), and undo/redo.
//
// Needs a human:
//   - Type into the seeded editor; confirm the caret tracks correctly across the
//     bold/italic/colored runs and sits at the LEADING edge of the empty field
//     below (not stranded at the trailing edge — the pre-rebrand tracker #749 iPad symptom).
//   - Select across runs (shift-arrows, or drag once pre-rebrand tracker #750 lands) → the highlight
//     should span the whole selection, not just one run.
//   - Toolbar: Bold/Italic/Underline/Strike restyle the selection (or set the
//     typing style with no selection); Image inserts a generated block image at
//     the caret; Undo/Redo step through every edit; Filled/Outlined toggles the
//     chrome.
//   - The readout under the editor shows String() + a per-segment style summary,
//     driven by OnContentChanged, so a screenshot captures the live document.
//
// On-GPU is the point (per reference_desktop_webgpu_verify): run it with the
// default painter, e.g.
//
//	go run ./hack/richentryprobe
package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	// Bundle the CJK fallback font so the IME live-verify (pre-rebrand tracker #674) shows real
	// glyphs on every target — wasm has no system fonts to scan, so composed
	// CJK commits render as tofu without it.
	_ "github.com/belluikit/bellui/theme/cjk"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("RichEntry live-verify (#667)")

	// A live log so a screenshot captures the OnContentChanged stream.
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
		lines = append(lines, msg)
		if len(lines) > 10 {
			lines = lines[len(lines)-10:]
		}
		bellui.Do(func() { logLbl.SetText(strings.Join(lines, "\n")) })
	}

	// Subject: a RichEntry seeded with mixed styles + a block image, so the caret
	// must cross runs and an image row from the first keystroke.
	re := widget.NewRichEntry()
	re.SetSegments([]widget.RichTextSegment{
		&widget.TextSegment{Style: inline(bellui.TextStyle{}), Text: "normal "},
		&widget.TextSegment{Style: inline(bellui.TextStyle{Bold: true}), Text: "bold "},
		&widget.TextSegment{Style: inline(bellui.TextStyle{Italic: true}), Text: "italic "},
		&widget.TextSegment{Style: colored(theme.ColorNamePrimary), Text: "colored\n"},
		&widget.ImageSegment{Image: swatch(color.NRGBA{R: 0x3f, G: 0x8f, B: 0xff, A: 0xff})},
		&widget.TextSegment{Style: inline(bellui.TextStyle{}), Text: "after image"},
	})

	readout := widget.NewLabel("")
	readout.Wrapping = bellui.TextWrapWord
	re.OnContentChanged = func(segs []widget.RichTextSegment) {
		readout.SetText(fmt.Sprintf("String()=%q\n%s", re.String(), segSummary(segs)))
	}
	// Prime the readout once (OnContentChanged fires only on edits).
	readout.SetText(fmt.Sprintf("String()=%q\n%s", re.String(), segSummary(re.Segments())))

	apply := func(name string, s bellui.TextStyle) *widget.Button {
		return widget.NewButton(name, func() {
			re.ApplyStyle(inline(s))
			logf("ApplyStyle %s (selection or typing style)", name)
		})
	}

	toolbar := container.NewHBox(
		apply("Bold", bellui.TextStyle{Bold: true}),
		apply("Italic", bellui.TextStyle{Italic: true}),
		apply("Underline", bellui.TextStyle{Underline: true}),
		apply("Strike", bellui.TextStyle{Strikethrough: true}),
		widget.NewButton("Plain", func() {
			re.ApplyStyle(inline(bellui.TextStyle{}))
			logf("ApplyStyle Plain")
		}),
		widget.NewButton("Image", func() {
			re.InsertImage(swatch(color.NRGBA{R: 0xff, G: 0x6f, B: 0x3f, A: 0xff}))
			logf("InsertImage")
		}),
		widget.NewButton("Undo", func() { re.Undo(); logf("Undo") }),
		widget.NewButton("Redo", func() { re.Redo(); logf("Redo") }),
	)

	// Chrome toggle: eyeball filled vs outlined MD3 chrome (pre-rebrand tracker #749).
	styleToggle := widget.NewButton("Chrome: Filled", nil)
	styleToggle.OnTapped = func() {
		if re.Style == widget.EntryStyleFilled {
			re.Style = widget.EntryStyleOutlined
			styleToggle.SetText("Chrome: Outlined")
		} else {
			re.Style = widget.EntryStyleFilled
			styleToggle.SetText("Chrome: Filled")
		}
		re.Refresh()
		logf("Style=%v", re.Style)
	}

	// A second, empty RichEntry: verify the empty-field caret sits at the LEADING
	// edge and the chrome renders (the pre-rebrand tracker #749 regressions).
	empty := widget.NewRichEntry()

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("RichEntry live-verify (#667)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("Type/select across runs; toolbar restyles, inserts images, undo/redo."),
		toolbar,
		styleToggle,
		widget.NewLabel("Subject (seeded: mixed styles + block image):"),
		re,
		widget.NewLabel("Empty field — caret must sit at the LEADING edge (#749):"),
		empty,
		widget.NewSeparator(),
		widget.NewLabel("Live document (OnContentChanged):"),
		readout,
		widget.NewSeparator(),
		logLbl,
	))
	w.Resize(geo.NewSize(760, 820))

	logf("ready — drive the editor on the default (WebGPU/Metal) painter")
	bellui.ShowAndRun(w)
}

// inline builds an inline text style with the given font style.
func inline(s bellui.TextStyle) widget.RichTextStyle {
	return widget.RichTextStyle{Inline: true, TextStyle: s}
}

// colored builds an inline text style tinted with a theme color.
func colored(name bellui.ThemeColorName) widget.RichTextStyle {
	return widget.RichTextStyle{Inline: true, ColorName: name}
}

// swatch returns a small solid-color image used as an in-memory block image.
func swatch(c color.NRGBA) image.Image {
	const w, h = 64, 40
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// A thin darker border so the image bounds are obvious on screen.
			if x < 2 || y < 2 || x >= w-2 || y >= h-2 {
				img.Set(x, y, color.NRGBA{A: 0xff})
				continue
			}
			img.Set(x, y, c)
		}
	}
	return img
}

// segSummary renders a one-line-per-segment style digest for the readout.
func segSummary(segs []widget.RichTextSegment) string {
	var b strings.Builder
	for i, seg := range segs {
		switch s := seg.(type) {
		case *widget.TextSegment:
			var tags []string
			if s.Style.TextStyle.Bold {
				tags = append(tags, "b")
			}
			if s.Style.TextStyle.Italic {
				tags = append(tags, "i")
			}
			if s.Style.TextStyle.Underline {
				tags = append(tags, "u")
			}
			if s.Style.TextStyle.Strikethrough {
				tags = append(tags, "s")
			}
			if s.Style.ColorName != "" {
				tags = append(tags, "color:"+string(s.Style.ColorName))
			}
			fmt.Fprintf(&b, "%d text[%s] %q\n", i, strings.Join(tags, ","), s.Text)
		case *widget.ImageSegment:
			fmt.Fprintf(&b, "%d image\n", i)
		default:
			fmt.Fprintf(&b, "%d %T\n", i, seg)
		}
	}
	return b.String()
}
