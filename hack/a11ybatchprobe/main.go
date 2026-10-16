// Command a11ybatchprobe is the on-device verification harness for the Android
// a11y/IME polish batch: IME batch-edit coalescing (pre-rebrand tracker #1015), partial
// extracted-text updates (pre-rebrand tracker #1009) and the visual-layout sidecar (pre-rebrand tracker #995). Tracked
// by pre-rebrand tracker #1071.
//
// It shows one multiline widget.Entry (soft-wrapped, so pre-rebrand tracker #995 visual LINE and
// the magnifier have something to bite on) plus controls that surface the
// otherwise-invisible acceptance signals:
//
//   - "Undo once"    — the pre-rebrand tracker #1015 granularity test. Select all in the field,
//     type a character, then tap this ONCE. Coalesced (fixed):
//     the ORIGINAL text returns. Two-step (bug): the field
//     goes EMPTY (the delete-all intermediate), and a second
//     Undo is needed. The status line shows the text after the
//     single undo, so the result is unambiguous.
//   - "Count undos"  — auto-driver: calls Entry.Undo() until the text stops
//     changing and reports how many steps it took. After
//     select-all + type one char, a coalesced edit is ONE
//     step; the bug is two (with an empty intermediate).
//   - "Load big doc" — fills the field with ~200 KB so pre-rebrand tracker #1009's partial-extract
//     path and pre-rebrand tracker #1015's diff run against a large document
//     (watch logcat for TransactionTooLargeException — the
//     whole-document ExtractedText push would blow the binder
//     limit; the partial update must not).
//   - "Clear"        — reset.
//
// Every bellui-side change is logged (tag "BelluiBatch"): text length + caret so a
// paired `adb logcat -e 'BelluiBatch'` captures the bellui side of each keystroke.
//
// HUMAN device pass (adb bypasses the soft keyboard, so the IME/TalkBack paths
// can only be driven by hand):
//  1. adb logcat -c ; adb logcat | grep -E "BelluiBatch|Bellui|TransactionTooLarge"
//  2. pre-rebrand tracker #1015: focus the field, Select all (long-press ▸ Select all, or the
//     handle menu), type "X", tap "Undo once" → expect the ORIGINAL text, not
//     an empty field. Repeat with "Count undos" → expect 1 step.
//  3. pre-rebrand tracker #1015 CJK regression: switch Gboard to Pinyin, compose a word inline
//     (underline), pick a candidate — must still work; "Count undos" of the
//     committed word is one step.
//  4. pre-rebrand tracker #1009: "Load big doc", type a few chars, keep predictive text /
//     suggestion strip working; logcat must show NO TransactionTooLargeException.
//  5. pre-rebrand tracker #995 (TalkBack on): explore the field, use character/word granularity —
//     the magnifier/highlight box must track each glyph; a soft-wrapped line
//     reads as one visual line.
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.a11ybatchprobe")
	w := a.NewWindow("A11y batch probe (#1015/#1009/#995)")

	status := widget.NewLabel("ready")
	status.Wrapping = bellui.TextWrapWord

	e := widget.NewMultiLineEntry()
	e.SetPlaceHolder("Focus me, Select all, type — then Undo once")
	e.Wrapping = bellui.TextWrapWord
	e.SetText("The quick brown fox jumps over the lazy dog. " +
		"Select all, type one character, then Undo once.")

	setStatus := func(s string) {
		log.Print("BelluiBatch: " + s)
		bellui.Do(func() { status.SetText(s) })
	}

	e.OnChanged = func(string) {
		log.Printf("BelluiBatch: changed len=%d caret=row%d col%d", len(e.Text), e.CursorRow, e.CursorColumn)
	}

	// A short, distinctive preview of the current text so "original vs empty"
	// after a single undo is unmistakable on the small status line.
	preview := func() string {
		t := e.Text
		if len(t) > 48 {
			t = t[:48] + "…"
		}
		return fmt.Sprintf("len=%d text=%q", len(e.Text), t)
	}

	undoOnce := widget.NewButton("Undo once", func() {
		before := e.Text
		e.Undo()
		setStatus(fmt.Sprintf("after 1 undo: %s (was len=%d) — EMPTY⇒two-step bug, ORIGINAL⇒coalesced",
			preview(), len(before)))
	})

	countUndos := widget.NewButton("Count undos", func() {
		steps := 0
		for steps < 1000 {
			prev := e.Text
			e.Undo()
			if e.Text == prev {
				break
			}
			steps++
		}
		setStatus(fmt.Sprintf("undo steps to stable: %d — after select-all+type-1, coalesced=1, bug=2. now %s",
			steps, preview()))
	})

	loadBig := widget.NewButton("Load big doc (~200KB)", func() {
		var b strings.Builder
		for i := 0; b.Len() < 200_000; i++ {
			fmt.Fprintf(&b, "Line %d: the quick brown fox jumps over the lazy dog.\n", i)
		}
		e.SetText(b.String())
		setStatus(fmt.Sprintf("loaded big doc: %s — now type; watch logcat for TransactionTooLargeException", preview()))
	})

	clear := widget.NewButton("Clear", func() {
		e.SetText("")
		setStatus("cleared")
	})

	w.SetContent(container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("A11y batch probe — #1015 undo granularity / #1009 big-doc / #995 layout",
				bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
			container.NewGridWithColumns(4, undoOnce, countUndos, loadBig, clear),
			widget.NewSeparator(),
		),
		status,
		nil, nil,
		e,
	))
	w.Resize(geo.NewSize(720, 520))
	log.Print("a11ybatchprobe: ready — grep logcat for BelluiBatch")
	bellui.ShowAndRun(w)
}
