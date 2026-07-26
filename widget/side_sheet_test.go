package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSideSheet_New(t *testing.T) {
	content := widget.NewLabel("Details")
	sheet := widget.NewSideSheet(content)
	assert.NotNil(t, sheet)
	assert.Equal(t, content, sheet.Content)
	assert.False(t, sheet.Modal)
}

func TestSideSheet_HideWithoutShow(t *testing.T) {
	sheet := widget.NewSideSheet(widget.NewLabel("Test"))
	assert.NotPanics(t, func() { sheet.Hide() })
}

func TestSideSheet_Modal(t *testing.T) {
	sheet := widget.NewSideSheet(widget.NewLabel("Modal"))
	sheet.Modal = true
	assert.True(t, sheet.Modal)
}
