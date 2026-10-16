// Command richimeprobe verifies widget.RichEntry IME composition on real targets
// (pre-rebrand tracker #674), side by side with a plain widget.Entry as the control. Both share the
// same native mobile IME seam (pre-rebrand tracker #126, iOS GoInputView = a UITextField subclass),
// so comparing them isolates whether a missing Pinyin candidate bar / conversion
// is RichEntry-specific or a shared mobile-IME gap.
//
// Needs a human: tap each field, switch to a CJK keyboard (Pinyin), type "nihao".
// Expected: an inline underlined preedit + the iOS candidate bar above the
// keyboard; picking a candidate commits and logs the field's text once. The log
// tags which field changed so a screenshot shows the A/B result.
package main

import (
	"fmt"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	// Register a bundled CJK font so committed Chinese/Japanese/Korean text
	// renders as real glyphs instead of tofu boxes (the bundled default fonts
	// are Latin-only; iOS system-font scan is unreliable). This isolates IME
	// correctness from font coverage when verifying pre-rebrand tracker #771/#773.
	_ "github.com/belluikit/bellui/theme/cjk"
)

func main() {
	a := app.New()
	w := a.NewWindow("IME A/B Probe (#674)")

	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
		lines = append(lines, msg)
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}
		out := ""
		for _, l := range lines {
			out += l + "\n"
		}
		bellui.Do(func() { logLbl.SetText(out) })
	}

	// Control: plain Entry (its iOS IME was validated in pre-rebrand tracker #126).
	entry := widget.NewEntry()
	entry.SetPlaceHolder("A) plain Entry — type Pinyin here")
	entry.OnChanged = func(s string) { logf("Entry.OnChanged=%q", s) }

	// Pre-filled so pre-rebrand tracker #748 (select-all + type replaces, not appends) is testable:
	// long-press → Select All → type a char; expect the field to become that char.
	entry.SetText("select me and type")

	// Subject: RichEntry.
	re := widget.NewRichEntry()
	re.OnContentChanged = func(segs []widget.RichTextSegment) {
		logf("RichEntry.String()=%q", re.String())
	}

	// Regression guard for the iOS mirror (pre-rebrand tracker #771): plain caret input still routes
	// through TypedRune, so NumericEntry's filter must reject letters.
	num := widget.NewNumericEntry()
	num.SetPlaceHolder("C) NumericEntry — letters must be rejected")
	num.OnChanged = func(s string) { logf("NumericEntry.OnChanged=%q", s) }

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("IME A/B: Entry vs RichEntry (#674, #771)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("Tap a field → CJK (Pinyin) keyboard → type \"nihao\"."),
		widget.NewLabel("Watch for an inline preedit + candidate bar; pick a candidate."),
		widget.NewLabel("A) plain Entry (control, pre-filled — try Select All + type):"),
		entry,
		widget.NewLabel("B) RichEntry (subject):"),
		re,
		widget.NewLabel("C) NumericEntry (filter must still reject letters):"),
		num,
		widget.NewSeparator(),
		logLbl,
	))
	w.Resize(geo.NewSize(700, 780))

	logf("ready — compare candidate behaviour in A (Entry) vs B (RichEntry)")
	bellui.ShowAndRun(w)
}
