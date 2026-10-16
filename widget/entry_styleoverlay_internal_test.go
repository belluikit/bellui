package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

func TestMergeStyleRun(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}

	// Foreground replaces as a unit: an overlay ColorName clears a base direct
	// color that would otherwise shadow it at render time.
	dst := StyleRun{ForegroundColor: red, Bold: true}
	mergeStyleRun(&dst, StyleRun{ColorName: "primary"})
	assert.Nil(t, dst.ForegroundColor)
	assert.Equal(t, "primary", string(dst.ColorName))
	assert.True(t, dst.Bold, "unset overlay fields keep base values")

	// ...and an overlay direct color replaces a base ColorName.
	dst = StyleRun{ColorName: "error", Underline: true, UnderlineStyle: UnderlineWavy, UnderlineColor: red}
	mergeStyleRun(&dst, StyleRun{ForegroundColor: blue})
	assert.Equal(t, blue, dst.ForegroundColor)
	assert.Empty(t, string(dst.ColorName))
	assert.True(t, dst.Underline, "squiggle survives a foreground-only overlay")
	assert.Equal(t, UnderlineWavy, dst.UnderlineStyle)

	// A zero overlay leaves everything intact.
	dst = StyleRun{ColorName: "error", Bold: true, BackgroundColor: red}
	mergeStyleRun(&dst, StyleRun{Text: "  "})
	assert.Equal(t, StyleRun{ColorName: "error", Bold: true, BackgroundColor: red}, dst)

	// Bold/Italic only switch on; background replaces; a set Underline carries
	// its style and color with it.
	dst = StyleRun{Bold: true}
	mergeStyleRun(&dst, StyleRun{Italic: true, BackgroundColor: blue, Underline: true, UnderlineStyle: UnderlineDotted, UnderlineColor: red})
	assert.True(t, dst.Bold)
	assert.True(t, dst.Italic)
	assert.Equal(t, blue, dst.BackgroundColor)
	assert.True(t, dst.Underline)
	assert.Equal(t, UnderlineDotted, dst.UnderlineStyle)
	assert.Equal(t, red, dst.UnderlineColor)

	// A set StrikeThrough carries its color and switches on additively without
	// clearing base bold/italic/underline.
	dst = StyleRun{Bold: true, Italic: true, Underline: true, UnderlineStyle: UnderlineWavy}
	mergeStyleRun(&dst, StyleRun{StrikeThrough: true, StrikeColor: red})
	assert.True(t, dst.StrikeThrough)
	assert.Equal(t, red, dst.StrikeColor)
	assert.True(t, dst.Bold, "strike overlay keeps base bold")
	assert.True(t, dst.Italic, "strike overlay keeps base italic")
	assert.True(t, dst.Underline, "strike overlay keeps base underline")
	assert.Equal(t, UnderlineWavy, dst.UnderlineStyle)

	// A zero overlay never clears an existing strike.
	dst = StyleRun{StrikeThrough: true, StrikeColor: blue}
	mergeStyleRun(&dst, StyleRun{Text: "  "})
	assert.True(t, dst.StrikeThrough)
	assert.Equal(t, blue, dst.StrikeColor)
}

func TestOverlaySubrange(t *testing.T) {
	blue := color.NRGBA{B: 255, A: 255}

	// Overlay [1,3) over "ab"(red)|"cd"(green) → "a" | "b"(blue) | "c"(blue) | "d".
	runs := overlaySubrange(
		[]StyleRun{{Text: "ab", ColorName: "red"}, {Text: "cd", ColorName: "green"}},
		1, 3, StyleRun{ForegroundColor: blue},
	)
	require.Len(t, runs, 4)
	assert.Equal(t, "a", runs[0].Text)
	assert.Equal(t, "red", string(runs[0].ColorName))
	assert.Nil(t, runs[0].ForegroundColor)
	assert.Equal(t, "b", runs[1].Text)
	assert.Equal(t, blue, runs[1].ForegroundColor)
	assert.Equal(t, "c", runs[2].Text)
	assert.Equal(t, blue, runs[2].ForegroundColor)
	assert.Equal(t, "d", runs[3].Text)
	assert.Equal(t, "green", string(runs[3].ColorName))

	// A range past the run end styles only up to the end (implicit clamp).
	runs = overlaySubrange([]StyleRun{{Text: "abc"}}, 2, 99, StyleRun{Bold: true})
	require.Len(t, runs, 2)
	assert.Equal(t, "ab", runs[0].Text)
	assert.False(t, runs[0].Bold)
	assert.Equal(t, "c", runs[1].Text)
	assert.True(t, runs[1].Bold)
}

// overlayEntry builds a multi-line Entry with per-line "primary" highlighting and
// the given overlay, mirroring an LSP-semantic-token host (pre-rebrand tracker #1079).
func overlayEntry(t *testing.T, text string, overlay func(row int) []StyleRun) *Entry {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		return []StyleRun{{Text: line, ColorName: "primary"}}, state
	}
	e.StyleOverlayForRow = overlay
	e.SetText(text)
	return e
}

func TestEntry_StyleOverlay_OverHighlighter(t *testing.T) {
	blue := color.NRGBA{B: 255, A: 255}
	rows := make(map[int]bool)
	e := overlayEntry(t, "hello world\nsecond line", func(row int) []StyleRun {
		rows[row] = true
		if row != 0 {
			return nil
		}
		// Spacer over "hello ", token over "world".
		return []StyleRun{{Text: "hello "}, {Text: "world", ForegroundColor: blue, Bold: true}}
	})

	runs := e.styleRunsForRow(0)
	require.Len(t, runs, 2)
	assert.Equal(t, "hello ", runs[0].Text)
	assert.Equal(t, "primary", string(runs[0].ColorName), "spacer keeps the lexical color")
	assert.Equal(t, "world", runs[1].Text)
	assert.Equal(t, blue, runs[1].ForegroundColor)
	assert.Empty(t, string(runs[1].ColorName), "overlay foreground replaces the lexical color as a unit")
	assert.True(t, runs[1].Bold)

	// Row 1 has no overlay → untouched highlighter output.
	runs = e.styleRunsForRow(1)
	require.Len(t, runs, 1)
	assert.Equal(t, "second line", runs[0].Text)
	assert.Equal(t, "primary", string(runs[0].ColorName))
	assert.True(t, rows[0] && rows[1], "overlay is consulted with the row index")
}

func TestEntry_StyleOverlay_StrikeThrough(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	e := overlayEntry(t, "deprecated call", func(row int) []StyleRun {
		if row != 0 {
			return nil
		}
		// Spacer over "deprecated", strike over " call".
		return []StyleRun{{Text: "deprecated"}, {Text: " call", StrikeThrough: true, StrikeColor: red}}
	})

	runs := e.styleRunsForRow(0)
	require.Len(t, runs, 2)
	assert.Equal(t, "deprecated", runs[0].Text)
	assert.False(t, runs[0].StrikeThrough, "spacer leaves its range unstruck")
	assert.Equal(t, "primary", string(runs[0].ColorName), "spacer keeps the lexical color")
	assert.Equal(t, " call", runs[1].Text)
	assert.True(t, runs[1].StrikeThrough)
	assert.Equal(t, red, runs[1].StrikeColor)
	assert.Equal(t, "primary", string(runs[1].ColorName), "strike is additive: lexical foreground survives")
}

func TestEntry_StyleOverlay_NoHighlighter(t *testing.T) {
	test.NewTempApp(t)
	blue := color.NRGBA{B: 255, A: 255}
	e := NewMultiLineEntry()
	e.StyleOverlayForRow = func(row int) []StyleRun {
		return []StyleRun{{Text: "he"}, {Text: "ll", ForegroundColor: blue}}
	}
	e.SetText("hello")

	// No LineHighlighter → a base run is synthesized so the runs still tile
	// the whole row (the renderer requires it).
	runs := e.styleRunsForRow(0)
	require.Len(t, runs, 3)
	assert.Equal(t, "he", runs[0].Text)
	assert.Nil(t, runs[0].ForegroundColor)
	assert.Equal(t, "ll", runs[1].Text)
	assert.Equal(t, blue, runs[1].ForegroundColor)
	assert.Equal(t, "o", runs[2].Text)
	assert.Nil(t, runs[2].ForegroundColor)
}

func TestEntry_StyleOverlay_SpacersOnly(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.StyleOverlayForRow = func(row int) []StyleRun {
		return []StyleRun{{Text: "hello"}}
	}
	e.SetText("hello")

	// An overlay of only spacers must not synthesize runs: nil means the
	// renderer skips the fragment pass entirely for the row.
	assert.Nil(t, e.styleRunsForRow(0))
}

func TestEntry_StyleOverlay_OutOfRangeRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.StyleOverlayForRow = func(row int) []StyleRun {
		return []StyleRun{{Text: "x", Bold: true}}
	}
	e.SetText("hello")

	assert.Nil(t, e.styleRunsForRow(7), "row past the document → no overlay")
	assert.Nil(t, e.styleRunsForRow(-1))
}

func TestEntry_StyleOverlay_ClampedToRow(t *testing.T) {
	test.NewTempApp(t)
	blue := color.NRGBA{B: 255, A: 255}
	e := overlayEntry(t, "ab", func(row int) []StyleRun {
		// Overlay tiling runs past the row end — styled to the end, no panic.
		return []StyleRun{{Text: "a"}, {Text: "bcdef", ForegroundColor: blue}}
	})

	runs := e.styleRunsForRow(0)
	require.Len(t, runs, 2)
	assert.Equal(t, "a", runs[0].Text)
	assert.Equal(t, "b", runs[1].Text)
	assert.Equal(t, blue, runs[1].ForegroundColor)
}

func TestEntry_StyleOverlay_PreeditOverOverlay(t *testing.T) {
	test.NewTempApp(t)
	blue := color.NRGBA{B: 255, A: 255}
	e := NewMultiLineEntry()
	e.StyleOverlayForRow = func(row int) []StyleRun {
		return []StyleRun{{Text: "hello", ForegroundColor: blue}}
	}
	e.SetText("hello")
	e.FocusGained()
	e.CursorRow, e.CursorColumn = 0, 5
	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})

	// Display row "helloni" — runs must tile it, the preedit range gains the
	// underline, and the overlay's foreground survives underneath on "hello".
	runs := e.styleRunsForRow(0)
	var total, underlined string
	for _, r := range runs {
		total += r.Text
		if r.Underline {
			underlined += r.Text
		}
	}
	assert.Equal(t, "helloni", total, "runs tile the whole display row")
	assert.Equal(t, "ni", underlined, "only the preedit is underlined")
	assert.Equal(t, blue, runs[0].ForegroundColor, "overlay foreground kept under the preedit pass")
}

func TestEntry_StyleOverlay_DoesNotRelex(t *testing.T) {
	test.NewTempApp(t)
	calls := 0
	e := NewMultiLineEntry()
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		calls++
		return []StyleRun{{Text: line, ColorName: "primary"}}, state
	}
	e.SetText("one\ntwo")
	e.styleRunsForRow(0)
	e.styleRunsForRow(1)
	before := calls
	require.Positive(t, before)

	// Installing an overlay and re-reading rows (a host Refresh on new tokens)
	// must serve the cached highlight — pure composition, no re-lex.
	e.StyleOverlayForRow = func(row int) []StyleRun {
		return []StyleRun{{Text: "o", Bold: true}}
	}
	e.styleRunsForRow(0)
	e.styleRunsForRow(1)
	assert.Equal(t, before, calls, "overlay composition never re-runs the LineHighlighter")
}
