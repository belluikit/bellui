//go:build !mobile

package widget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func TestTable_Hovered(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 2, 2 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			c.(*Label).SetText(fmt.Sprintf("Cell %d, %d", id.Row, id.Col))
		},
	)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))

	test.MoveMouse(w.Canvas(), geo.NewPos(35, 50))
	test.MoveMouse(w.Canvas(), geo.NewPos(35, 100))

	assert.Nil(t, table.hoveredCell)

	test.AssertRendersToMarkup(t, "table/desktop/hovered_out.xml", w.Canvas())

	table.Length = func() (int, int) { return 3, 5 }
	table.Refresh()

	w.SetContent(table)
	w.Resize(geo.NewSize(180, 180))

	// Invoke Table.HoverIn directly because Label cells now implement
	// Hoverable (for tooltip support) and steal the hover event
	// from test.MoveMouse's FindObjectAtPositionMatching walk.
	// The position is relative to the table widget.
	padding := table.Theme().Size("padding")
	cellH := table.cellSize.Height
	hoverY := cellH + padding + cellH/2 // middle of row 1
	table.HoverIn(&event.HoverEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(31, hoverY)},
	})

	assert.NotNil(t, table.hoveredCell)
	assert.Equal(t, 0, table.hoveredCell.Col)
	assert.Equal(t, 1, table.hoveredCell.Row)

	test.AssertRendersToMarkup(t, "table/desktop/hovered.xml", w.Canvas())
}
