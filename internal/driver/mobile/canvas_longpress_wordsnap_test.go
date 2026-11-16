//go:build (!windows || !ci) && !wasm

package mobile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

const wordSnapText = "hello world quick brown fox jumps"

// wordSnapTargets are the three text widgets with long-press selection, each
// holding wordSnapText on one row and reporting its selected text.
func wordSnapTargets() map[string]func() (core.CanvasObject, func() string) {
	return map[string]func() (core.CanvasObject, func() string){
		"Entry": func() (core.CanvasObject, func() string) {
			e := widget.NewMultiLineEntry()
			e.SetText(wordSnapText)
			return e, e.SelectedText
		},
		"selectable Label": func() (core.CanvasObject, func() string) {
			l := widget.NewLabel(wordSnapText)
			l.Selectable = true
			return l, l.SelectedText
		},
		"RichEntry": func() (core.CanvasObject, func() string) {
			e := widget.NewRichEntry()
			e.SetSegments([]widget.RichTextSegment{&widget.TextSegment{Text: wordSnapText}})
			return e, e.SelectedText
		},
	}
}

func wordSnapMove(c *canvas, pos geo.Position) {
	c.TapMove(pos, 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
}

// hold presses at pos until the long-press has fired and returns the word it
// selected, failing the test if it selected none.
func wordSnapHold(t *testing.T, c *canvas, pos geo.Position, selected func() string) string {
	t.Helper()
	c.TapDown(pos, 0)
	var word string
	waitAndCheck(400, func() { word = selected() })
	require.NotEmpty(t, word, "the long-press selected nothing, so the check would prove nothing")
	require.NotContains(t, word, " ")
	return word
}

// A finger held still on a word creeps a few points once the long-press has
// fired: 8 of 9 holds on an iPad crept 4–5 pt (#1267). The creep must not
// shrink the word the long-press selected, in either direction.
func Test_canvas_LongPressCreepKeepsWord(t *testing.T) {
	for name, build := range wordSnapTargets() {
		for dirName, dir := range map[string]float32{"right": 1, "left": -1} {
			t.Run(name+" "+dirName, func(t *testing.T) {
				obj, selected := build()
				c := newCanvas(bellui.CurrentDevice()).(*canvas)
				c.SetContent(obj)
				c.Resize(geo.NewSize(300, 100))

				pos := geo.NewPos(30, 20)
				word := wordSnapHold(t, c, pos, selected)

				// The iPad's creep: one 4.5 pt step past the tap slop, then
				// half-point steps.
				p := pos.AddXY(4.5*dir, 0)
				wordSnapMove(c, p)
				for range 4 {
					p = p.AddXY(0.5*dir, 0)
					wordSnapMove(c, p)
				}
				assert.Equal(t, word, selected(), "the creep changed the selection while held")
				lprTapUp(c, p)
				waitAndCheck(100, func() {
					assert.Equal(t, word, selected(), "the creep changed the selection by the release")
				})
			})
		}
	}
}

// A drag well past the long-pressed word still extends the selection, a word
// at a time: forward from the word's start, and backward from its end.
func Test_canvas_LongPressDragExtendsByWords(t *testing.T) {
	for name, build := range wordSnapTargets() {
		t.Run(name+" forward", func(t *testing.T) {
			obj, selected := build()
			c := newCanvas(bellui.CurrentDevice()).(*canvas)
			c.SetContent(obj)
			c.Resize(geo.NewSize(300, 100))

			pos := geo.NewPos(30, 20)
			word := wordSnapHold(t, c, pos, selected)
			require.Equal(t, "hello", word)
			for x := pos.X + 10; x <= 120; x += 10 {
				wordSnapMove(c, geo.NewPos(x, 20))
			}
			got := selected()
			assert.True(t, strings.HasPrefix(wordSnapText, got), "%q does not start at the long-pressed word", got)
			assert.Greater(t, len(got), len("hello "), "the drag did not extend past the word")
			if n := len(got); n < len(wordSnapText) {
				assert.Equal(t, byte(' '), wordSnapText[n], "%q ends inside a word", got)
			}
			lprTapUp(c, geo.NewPos(120, 20))
		})
		t.Run(name+" backward", func(t *testing.T) {
			obj, selected := build()
			c := newCanvas(bellui.CurrentDevice()).(*canvas)
			c.SetContent(obj)
			c.Resize(geo.NewSize(300, 100))

			pos := geo.NewPos(110, 20)
			word := wordSnapHold(t, c, pos, selected)
			require.NotEqual(t, "hello", word, "the hold must be past the first word")
			for x := pos.X - 10; x >= 12; x -= 10 {
				wordSnapMove(c, geo.NewPos(x, 20))
			}
			got := selected()
			upTo := wordSnapText[:strings.Index(wordSnapText, word)+len(word)]
			assert.True(t, strings.HasSuffix(upTo, got), "%q does not end at the long-pressed word %q", got, word)
			assert.Greater(t, len(got), len(" "+word), "the drag did not extend before the word")
			if start := len(upTo) - len(got); start > 0 && start < len(upTo) {
				assert.Equal(t, byte(' '), wordSnapText[start-1], "%q starts inside a word", got)
			}
			lprTapUp(c, geo.NewPos(12, 20))
		})
	}
}
