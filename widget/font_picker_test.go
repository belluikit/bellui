package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
)

func TestFontPicker_Creation(t *testing.T) {
	families := []FontFamily{{Name: "Arial"}, {Name: "Roboto"}}
	config := FontPickerConfig{Family: "Roboto", Size: 16, Weight: bellui.WeightNormal}
	fp := NewFontPicker(config, families, nil)

	assert.NotNil(t, fp)
	assert.Equal(t, "Roboto", fp.Config.Family)
	assert.Equal(t, 16.0, fp.Config.Size)
	assert.Equal(t, bellui.WeightNormal, fp.Config.Weight)
	assert.NotNil(t, fp.familySelect)
	assert.NotNil(t, fp.sizeSpinBox)
	assert.NotNil(t, fp.weightSelect)
	assert.NotNil(t, fp.boldBtn)
	assert.NotNil(t, fp.italicBtn)
	assert.NotNil(t, fp.underlineBtn)
	assert.NotNil(t, fp.previewLines[0])
}

func TestFontPicker_DefaultSize(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{}, nil, nil)
	assert.Equal(t, 14.0, fp.Config.Size, "Default size should be 14")
}

func TestFontPicker_SetConfig(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 12}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.SetConfig(FontPickerConfig{
		Family: "Arial",
		Size:   24,
		Weight: bellui.WeightBold,
		Bold:   true,
		Italic: true,
	})

	assert.Equal(t, "Arial", fp.Config.Family)
	assert.Equal(t, 24.0, fp.Config.Size)
	assert.Equal(t, bellui.WeightBold, fp.Config.Weight)
	assert.True(t, fp.Config.Bold)
	assert.True(t, fp.Config.Italic)
}

func TestFontPicker_FamilySelect(t *testing.T) {
	families := []FontFamily{{Name: "Arial"}, {Name: "Roboto"}, {Name: "Courier"}}
	var received FontPickerConfig
	fp := NewFontPicker(FontPickerConfig{Size: 14}, families, func(c FontPickerConfig) {
		received = c
	})
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.familySelect.SetSelected("Courier")
	assert.Equal(t, "Courier", fp.Config.Family)
	assert.Equal(t, "Courier", received.Family)
}

func TestFontPicker_SizeChange(t *testing.T) {
	var received FontPickerConfig
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, func(c FontPickerConfig) {
		received = c
	})
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.sizeSpinBox.SetValue(24)
	assert.Equal(t, 24.0, fp.Config.Size)
	assert.Equal(t, 24.0, received.Size)
}

func TestFontPicker_WeightSelect(t *testing.T) {
	var received FontPickerConfig
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, func(c FontPickerConfig) {
		received = c
	})
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.weightSelect.SetSelected("Bold")
	assert.Equal(t, bellui.WeightBold, fp.Config.Weight)
	assert.True(t, fp.Config.Bold, "Bold should auto-activate when weight >= SemiBold")
	assert.Equal(t, bellui.WeightBold, received.Weight)
}

func TestFontPicker_WeightSelectLight(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14, Weight: bellui.WeightBold, Bold: true}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.weightSelect.SetSelected("Light")
	assert.Equal(t, bellui.WeightLight, fp.Config.Weight)
	assert.False(t, fp.Config.Bold, "Bold should deactivate when weight < SemiBold")
}

func TestFontPicker_BoldToggle(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	assert.False(t, fp.Config.Bold)
	test.Tap(fp.boldBtn)
	assert.True(t, fp.Config.Bold)
	assert.Equal(t, bellui.WeightBold, fp.Config.Weight)

	test.Tap(fp.boldBtn)
	assert.False(t, fp.Config.Bold)
	assert.Equal(t, bellui.WeightNormal, fp.Config.Weight)
}

func TestFontPicker_ItalicToggle(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	assert.False(t, fp.Config.Italic)
	test.Tap(fp.italicBtn)
	assert.True(t, fp.Config.Italic)

	test.Tap(fp.italicBtn)
	assert.False(t, fp.Config.Italic)
}

func TestFontPicker_UnderlineToggle(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	assert.False(t, fp.Config.Underline)
	test.Tap(fp.underlineBtn)
	assert.True(t, fp.Config.Underline)

	test.Tap(fp.underlineBtn)
	assert.False(t, fp.Config.Underline)
}

func TestFontPicker_BoldWeightSync(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	// Selecting SemiBold should activate bold
	fp.weightSelect.SetSelected("SemiBold")
	assert.True(t, fp.Config.Bold)
	assert.Equal(t, ButtonStyleFilledTonal, fp.boldBtn.Style)

	// Selecting Normal should deactivate bold
	fp.weightSelect.SetSelected("Normal")
	assert.False(t, fp.Config.Bold)
	assert.Equal(t, ButtonStyleOutlined, fp.boldBtn.Style)
}

func TestFontPicker_OnChanged(t *testing.T) {
	callCount := 0
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, func(c FontPickerConfig) {
		callCount++
	})
	w := test.NewTempWindow(t, fp)
	_ = w

	test.Tap(fp.boldBtn)
	assert.Equal(t, 1, callCount)

	test.Tap(fp.italicBtn)
	assert.Equal(t, 2, callCount)

	fp.sizeSpinBox.SetValue(20)
	assert.Equal(t, 3, callCount)
}

func TestFontPickerConfig_TextStyle(t *testing.T) {
	config := FontPickerConfig{
		Bold:      true,
		Italic:    true,
		Underline: true,
		Weight:    bellui.WeightBold,
	}

	style := config.TextStyle()
	assert.True(t, style.Bold)
	assert.True(t, style.Italic)
	assert.True(t, style.Underline)
	assert.Equal(t, bellui.WeightBold, style.ResolvedWeight())
}

func TestFontPicker_Disable(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.Disable()
	assert.True(t, fp.Disabled())
	assert.True(t, fp.familySelect.Disabled())
	assert.True(t, fp.sizeSpinBox.Disabled())
	assert.True(t, fp.weightSelect.Disabled())
	assert.True(t, fp.boldBtn.Disabled())

	fp.Enable()
	assert.False(t, fp.Disabled())
	assert.False(t, fp.familySelect.Disabled())
}

func TestFontPicker_SetFamilies(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14, Family: "Roboto"}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	fp.SetFamilies([]FontFamily{{Name: "Arial"}, {Name: "Helvetica"}, {Name: "Roboto"}})
	assert.Equal(t, 3, len(fp.familySelect.Options))
	assert.Equal(t, "Roboto", fp.familySelect.Selected)
}

func TestFontPicker_PreviewUpdates(t *testing.T) {
	fp := NewFontPicker(FontPickerConfig{Size: 14}, nil, nil)
	w := test.NewTempWindow(t, fp)
	_ = w

	test.Tap(fp.boldBtn)
	assert.True(t, fp.previewLines[0].TextStyle.Bold)
	assert.Equal(t, float32(14), fp.previewLines[0].TextSize)

	test.Tap(fp.italicBtn)
	assert.True(t, fp.previewLines[0].TextStyle.Italic)

	fp.sizeSpinBox.SetValue(32)
	assert.Equal(t, float32(32), fp.previewLines[0].TextSize)
}

func TestShowFontPickerPopup(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel(""))
	var received FontPickerConfig
	called := false

	ShowFontPickerPopup(
		FontPickerConfig{Size: 16, Family: "Test"},
		[]FontFamily{{Name: "Test"}, {Name: "Other"}},
		func(c FontPickerConfig) {
			called = true
			received = c
		},
		w.Canvas(),
	)

	// Popup should be visible on the canvas overlay
	assert.NotNil(t, w.Canvas().Overlays().Top())

	// Find and tap OK button by searching the overlay
	pop := w.Canvas().Overlays().Top().(*PopUp)
	assert.NotNil(t, pop)

	// The popup content is a container with [picker, buttons]
	// We can verify it's showing by checking the overlay
	pop.Hide()
	assert.Nil(t, w.Canvas().Overlays().Top())

	_ = received
	_ = called
}
