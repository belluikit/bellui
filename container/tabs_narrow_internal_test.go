package container

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// barWidths sweeps from "narrower than a single tab" up to "everything fits".
// The side-panel shape that prompted these tests sits at the low end, which is
// where a fit calculation is most likely to be wrong — and where the existing
// coverage, three markup goldens at one fixed size, says nothing (#322).
var barWidths = []float32{40, 80, 120, 200, 320, 600, 1000}

func makeTabItems(n int) []*TabItem {
	items := make([]*TabItem, 0, n)
	for i := range n {
		items = append(items, &TabItem{
			Text:    fmt.Sprintf("Tab%d", i),
			Content: widget.NewLabel(fmt.Sprintf("c%d", i)),
		})
	}
	return items
}

// buttonsIn returns the tab buttons laid out in a bar container, with their
// positions and sizes as the renderer left them.
func buttonsIn(group *bellui.Container) []core.CanvasObject {
	out := make([]core.CanvasObject, 0, len(group.Objects))
	for _, o := range group.Objects {
		if _, ok := o.(*tabButton); ok {
			out = append(out, o)
		}
	}
	return out
}

// assertNoOverlap fails naming both offenders. Overlap is the symptom a user
// sees; reporting only "N overlaps" would leave the next reader measuring by
// hand.
func assertNoOverlap(t *testing.T, buttons []core.CanvasObject, what string) {
	t.Helper()
	for i := range buttons {
		for j := i + 1; j < len(buttons); j++ {
			a, b := buttons[i], buttons[j]
			ax2, ay2 := a.Position().X+a.Size().Width, a.Position().Y+a.Size().Height
			bx2, by2 := b.Position().X+b.Size().Width, b.Position().Y+b.Size().Height
			if a.Position().X < bx2 && b.Position().X < ax2 &&
				a.Position().Y < by2 && b.Position().Y < ay2 {
				t.Errorf("%s: buttons %d [x %.1f..%.1f] and %d [x %.1f..%.1f] overlap",
					what, i, a.Position().X, ax2, j, b.Position().X, bx2)
			}
		}
	}
}

// TestAppTabs_NarrowBar_EveryTabStaysReachable is the invariant with a
// user-visible consequence: AppTabs renders what fits and puts the rest behind
// the overflow button, so the fitted buttons plus the overflow entries must
// account for every tab. Dropping tabs without an overflow button would make
// them unreachable, and no fixed-size golden would notice.
func TestAppTabs_NarrowBar_EveryTabStaysReachable(t *testing.T) {
	const tabCount = 8

	for _, width := range barWidths {
		t.Run(fmt.Sprintf("w%.0f", width), func(t *testing.T) {
			test.NewTempApp(t)

			tabs := NewAppTabs(makeTabItems(tabCount)...)
			c := test.NewCanvas()
			c.SetPadded(false)
			c.SetContent(tabs)
			c.Resize(geo.NewSize(width, 300))

			r := cache.Renderer(tabs).(*appTabsRenderer)
			group, ok := r.bar.Objects[0].(*bellui.Container)
			require.True(t, ok, "the AppTabs bar holds its buttons in a plain container")

			fitted := buttonsIn(group)
			hidden := tabCount - len(fitted)
			require.LessOrEqual(t, len(fitted), tabCount, "more buttons than tabs")

			// The overflow button is r.action, appended as bar.Objects[1] and
			// shown only when some tabs did not fit — NOT a member of the
			// button group. Reading it from the group was this test's first
			// mistake and made a working widget look broken.
			require.NotNil(t, r.action, "the renderer always builds an overflow button")

			if hidden > 0 {
				assert.True(t, r.action.Visible(),
					"%d of %d tabs do not fit at width %.0f, so the overflow button must be visible — otherwise those tabs are unreachable",
					hidden, tabCount, width)
			} else {
				assert.False(t, r.action.Visible(),
					"every tab fits at width %.0f, so the overflow button should be hidden", width)
			}

			// Reachability: fitted + overflow entries == every tab. The menu is
			// built from Items[len(fitted):], so this is the count that matters.
			assert.Equal(t, tabCount, len(fitted)+hidden,
				"fitted buttons plus overflow entries must account for every tab")

			assertNoOverlap(t, fitted, fmt.Sprintf("AppTabs w=%.0f", width))

			// AppTabs bounds its bar: what it chose to render must fit in it.
			// Except at the degenerate end — the renderer always keeps at least
			// one tab, so a bar narrower than a single button legitimately
			// overhangs and there is nothing narrower it could draw.
			if len(fitted) > 1 {
				for i, b := range fitted {
					assert.LessOrEqualf(t, b.Position().X+b.Size().Width, width+0.01,
						"AppTabs w=%.0f: fitted button %d runs to %.1f, past the bar",
						width, i, b.Position().X+b.Size().Width)
				}
			}
		})
	}
}

// TestDocTabs_NarrowBar_ButtonsNeverOverlap covers the other strategy. DocTabs
// lays out every button and lets its Scroll clip the excess, so buttons MAY
// extend past the bar — that is the design, and asserting otherwise would be
// wrong. They must still never overlap each other.
func TestDocTabs_NarrowBar_ButtonsNeverOverlap(t *testing.T) {
	const tabCount = 8

	for _, width := range barWidths {
		t.Run(fmt.Sprintf("w%.0f", width), func(t *testing.T) {
			test.NewTempApp(t)

			tabs := NewDocTabs(makeTabItems(tabCount)...)
			c := test.NewCanvas()
			c.SetPadded(false)
			c.SetContent(tabs)
			c.Resize(geo.NewSize(width, 300))

			r := cache.Renderer(tabs).(*docTabsRenderer)
			group, ok := r.scroller.Content.(*bellui.Container)
			require.True(t, ok, "the DocTabs bar scrolls a plain container of buttons")

			buttons := buttonsIn(group)
			assert.Len(t, buttons, tabCount, "DocTabs lays out every tab and scrolls; none should be dropped")
			assertNoOverlap(t, buttons, fmt.Sprintf("DocTabs w=%.0f", width))
		})
	}
}
