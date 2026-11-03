package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Header falls back to Field when HeaderName is unset (#918).
func TestColumnDef_HeaderFallsBackToField(t *testing.T) {
	assert.Equal(t, "play", (&ColumnDef{Field: "play"}).Header())
	assert.Equal(t, "Play", (&ColumnDef{Field: "play", HeaderName: "Play"}).Header())
	assert.Equal(t, "play", (&ColumnDef{Field: "play"}).headerText())
}

// A column with HideHeaderText draws no header text, but its header cell
// still names it to screen readers and exports still title it (#918).
func TestDataGrid_HideHeaderText(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"play": "x", "name": "Song"}}, []*ColumnDef{
		{Field: "play", Width: 56, HideHeaderText: true},
		{Field: "name", HeaderName: "Name"},
	})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 200))

	assert.Equal(t, "", g.ColumnHeaderText(0))
	assert.Equal(t, "play", g.ColumnHeader(0))
	assert.Equal(t, "Name", g.ColumnHeaderText(1))

	var texts []string
	headings := map[string]bool{}
	var walk func(o core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleHeading {
			headings[a.AccessibleLabel()] = true
		}
		switch v := o.(type) {
		case *canvas.Text:
			if v.Text != "" {
				texts = append(texts, v.Text)
			}
		case *bellui.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Clip:
			walk(v.Content)
		case core.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c)
			}
		}
	}
	walk(g.headerWidget)
	assert.Contains(t, texts, "Name", "the walk reached the header labels")
	assert.NotContains(t, texts, "play", "no header text drawn for the column")
	assert.True(t, headings["play"], "the column still has an accessible name; have %v", headings)

	cols := g.API().exportColumns()
	assert.Equal(t, "play", cols[0].Header, "exports still title the column")
}
