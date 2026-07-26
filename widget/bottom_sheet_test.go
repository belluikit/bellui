package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestBottomSheet_New(t *testing.T) {
	label := widget.NewLabel("Content")
	sheet := widget.NewBottomSheet(label)
	assert.NotNil(t, sheet)
	assert.Equal(t, label, sheet.Content)
	assert.False(t, sheet.Modal)
}

func TestBottomSheet_Show(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 600))

	sheet := widget.NewBottomSheet(widget.NewLabel("Hello"))
	assert.NotPanics(t, func() { sheet.Show(w.Canvas()) })
	sheet.Hide()
}

func TestBottomSheet_Hide(t *testing.T) {
	sheet := widget.NewBottomSheet(widget.NewLabel("Test"))
	assert.NotPanics(t, func() { sheet.Hide() })
}
