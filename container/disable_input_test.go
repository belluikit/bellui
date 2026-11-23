package container_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A widget that cannot be disabled itself, inside a disabled container, takes
// no tap: the drivers drop it as if it had landed on a disabled Button
// (#1378). Enabling the container brings the taps back.
func TestDisabledContainer_DropsTaps(t *testing.T) {
	test.NewTempApp(t)
	abs := func(o core.CanvasObject) geo.Position {
		return bellui.CurrentApp().Driver().AbsolutePositionForObject(o)
	}

	t.Run("Hyperlink in AppTabs", func(t *testing.T) {
		taps := 0
		link := widget.NewHyperlink("link", nil)
		link.OnTapped = func() { taps++ }
		tabs := container.NewAppTabs(container.NewTabItem("a", container.NewVBox(link)))
		w := test.NewTempWindow(t, tabs)
		w.Resize(geo.NewSize(300, 200))
		at := abs(link).Add(geo.NewPos(6, link.Size().Height/2)) // on the text

		test.TapCanvas(w.Canvas(), at)
		require.Equal(t, 1, taps, "the probe tap missed the link")
		tabs.Disable()
		test.TapCanvas(w.Canvas(), at)
		assert.Equal(t, 1, taps, "a tap opened a link inside disabled tabs")
		tabs.Enable()
		test.TapCanvas(w.Canvas(), at)
		assert.Equal(t, 2, taps)
	})

	t.Run("Hyperlink in InnerWindow", func(t *testing.T) {
		taps := 0
		link := widget.NewHyperlink("link", nil)
		link.OnTapped = func() { taps++ }
		win := container.NewInnerWindow("w", container.NewVBox(link))
		w := test.NewTempWindow(t, container.NewMultipleWindows(win))
		w.Resize(geo.NewSize(400, 300))
		at := abs(link).Add(geo.NewPos(6, link.Size().Height/2))

		test.TapCanvas(w.Canvas(), at)
		require.Equal(t, 1, taps, "the probe tap missed the link")
		win.Disable()
		test.TapCanvas(w.Canvas(), at)
		assert.Equal(t, 1, taps, "a tap opened a link inside a disabled window")
	})

	t.Run("Table in AppTabs", func(t *testing.T) {
		selected := 0
		tbl := widget.NewTable(func() (int, int) { return 3, 2 },
			func() core.CanvasObject { return widget.NewLabel("cell") },
			func(widget.TableCellID, core.CanvasObject) {})
		tbl.OnSelected = func(widget.TableCellID) { selected++ }
		tabs := container.NewAppTabs(container.NewTabItem("a", tbl))
		w := test.NewTempWindow(t, tabs)
		w.Resize(geo.NewSize(300, 300))

		test.TapCanvas(w.Canvas(), abs(tbl).Add(geo.NewPos(20, 15)))
		require.Equal(t, 1, selected, "the probe tap missed the table")
		tabs.Disable()
		test.TapCanvas(w.Canvas(), abs(tbl).Add(geo.NewPos(20, 60)))
		assert.Equal(t, 1, selected, "a tap selected a cell inside disabled tabs")
	})
}

// Hover goes nowhere inside a disabled container either.
func TestDisabledContainer_DropsHover(t *testing.T) {
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("no hover on mobile")
	}
	test.NewTempApp(t)
	h := &hoverCounter{}
	h.ExtendBaseWidget(h)
	tabs := container.NewAppTabs(container.NewTabItem("a", h))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(300, 200))
	at := bellui.CurrentApp().Driver().AbsolutePositionForObject(h).Add(geo.NewPos(10, 10))

	test.MoveMouse(w.Canvas(), at)
	require.Equal(t, 1, h.in, "the probe hover missed")
	test.MoveMouse(w.Canvas(), geo.NewPos(0, 0))
	tabs.Disable()
	test.MoveMouse(w.Canvas(), at)
	assert.Equal(t, 1, h.in, "a widget inside disabled tabs took the hover")
}

// hoverCounter is an app widget that counts the hovers it gets and cannot be
// disabled.
type hoverCounter struct {
	widget.BaseWidget
	in int
}

func (h *hoverCounter) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
func (h *hoverCounter) MinSize() geo.Size            { return geo.NewSize(50, 50) }
func (h *hoverCounter) HoverIn(*event.HoverEvent)    { h.in++ }
func (h *hoverCounter) HoverMoved(*event.HoverEvent) {}
func (h *hoverCounter) HoverOut()                    {}

// A disabled Entry keeps its text selectable (#1251), inside a disabled
// container too: the drag that selects reaches it.
func TestDisabledContainer_EntryStillSelects(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("hello world")
	tabs := container.NewAppTabs(container.NewTabItem("a", entry))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(300, 200))
	tabs.Disable()
	require.True(t, entry.Disabled())

	at := bellui.CurrentApp().Driver().AbsolutePositionForObject(entry).Add(geo.NewPos(12, entry.Size().Height/2))
	test.DragSteps(w.Canvas(), at, 60, 0, 4, nil)
	assert.NotEmpty(t, entry.SelectedText(), "the drag did not select the disabled Entry's text")
}
