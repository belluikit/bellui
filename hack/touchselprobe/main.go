// Command touchselprobe is the on-device harness for the mobile touch
// text-selection stack (pre-rebrand tracker #945): pre-rebrand tracker #758 hold-fire long-press + selection handles,
// pre-rebrand tracker #942 drag magnifier/loupe, pre-rebrand tracker #943 caret drag-handle, pre-rebrand tracker #944 selectable
// Label/RichText parity, pre-rebrand tracker #888 RichEntry handles + styled-run loupe. Package
// it for a device with
//
//	bellui package --os android/arm64
//
// and drive the gestures by hand; every surface the acceptance criteria
// mention is on one of the tabs.
package main

import (
	"image"
	"image/color"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("touchselprobe")

	// Entry tab: multiline (long-press word select, handles, loupe, caret
	// handle, auto-scroll at the edges), single-line, and password (which
	// must offer no handles and no word-select).
	long := make([]string, 0, 30)
	long = append(
		long,
		"The quick brown fox jumps over the lazy dog near the riverbank today.",
		"Pack my box with five dozen liquor jugs before the delivery arrives.",
	)
	for i := 3; i <= 30; i++ {
		long = append(long, strings.Repeat("word ", 12)+"line-end")
	}
	multi := widget.NewMultiLineEntry()
	multi.SetText(strings.Join(long, "\n"))
	multi.Wrapping = bellui.TextWrapOff // horizontal auto-scroll is part of pre-rebrand tracker #945

	single := widget.NewEntry()
	single.SetText("single line entry with plenty of words to select")
	password := widget.NewPasswordEntry()
	password.SetText("secret words here")

	entryTab := container.NewBorder(nil,
		container.NewVBox(single, password), nil, nil, multi)

	// Selectable tab (pre-rebrand tracker #944): read-only Label and RichText with selection
	// enabled — long-press word select, handles, copy menu on lift.
	label := widget.NewLabel("This selectable label has several words to long-press, " +
		"select and copy. It never edits, it only selects.\nA second line keeps " +
		"multi-row selections honest.")
	label.Selectable = true
	rich := widget.NewRichTextFromMarkdown("**Bold words** and *italic words* mix in this " +
		"selectable rich text. Long-press any word to select it, then drag the " +
		"handles across style runs.\n\nA second paragraph keeps multi-row drags honest.")
	rich.Selectable = true
	rich.Wrapping = bellui.TextWrapWord
	selectableTab := container.NewVBox(label, widget.NewSeparator(), rich)

	// RichEntry tab (pre-rebrand tracker #888/#1000): the editable rich-text surface — long-press
	// word select (nearest-word snap on whitespace), drag handles across
	// styled runs and around the inline image atom, the styled-run loupe
	// (bold/heading runs must magnify true to their drawn widths), and the
	// editor context menu on lift / handle tap.
	richEntry := widget.NewRichEntry()
	richEntry.Wrapping = bellui.TextWrapWord
	inline := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			inline.Set(x, y, color.RGBA{R: 220, G: 60, B: 60, A: 255})
		}
	}
	richEntry.SetSegments([]widget.RichTextSegment{
		&widget.TextSegment{
			Style: widget.RichTextStyle{Inline: true, SizeName: theme.SizeNameHeadlineSmall, TextStyle: bellui.TextStyle{Bold: true}},
			Text:  "Heading run",
		},
		&widget.TextSegment{Style: widget.RichTextStyleInline, Text: " then plain words "},
		&widget.TextSegment{
			Style: widget.RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}},
			Text:  "bold words",
		},
		&widget.TextSegment{Style: widget.RichTextStyleInline, Text: " and "},
		&widget.TextSegment{
			Style: widget.RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}},
			Text:  "italic words",
		},
		&widget.TextSegment{Style: widget.RichTextStyleInline, Text: " to long-press and drag across.\nSecond line flows an inline "},
		&widget.ImageSegment{Image: inline, InlineImage: true},
		&widget.TextSegment{Style: widget.RichTextStyleInline, Text: " image atom to drag a selection over."},
	})
	richEntryTab := container.NewVBox(richEntry)

	tabs := container.NewAppTabs(
		container.NewTabItem("Entry", entryTab),
		container.NewTabItem("Selectable", selectableTab),
		container.NewTabItem("RichEntry", richEntryTab),
	)
	w.SetContent(tabs)
	bellui.ShowAndRun(w)
}
