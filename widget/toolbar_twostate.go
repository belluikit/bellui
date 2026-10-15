package widget

// TwoStateToolbarAction follows the API of the fyne-x project's widget of the
// same name (fyne.io/x/fyne, widget/twostatetoolbaraction.go, Copyright (C)
// 2020 Fyne.io developers and community, BSD-3-Clause license), written fresh
// against this fork's Toolbar.

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

var _ ToolbarItem = (*TwoStateToolbarAction)(nil)

// TwoStateToolbarAction is a push button style of ToolbarItem that displays a
// different icon depending on its state, e.g. play/pause for a media player
// or show/hide for a side panel. Tapping toggles the state.
//
// Since: 1.0
type TwoStateToolbarAction struct {
	// OnActivated is called with the new state whenever it changes, by tap
	// or by SetOn.
	OnActivated func(on bool) `json:"-"`

	on      bool
	offIcon bellui.Resource
	onIcon  bellui.Resource

	button Button
}

// NewTwoStateToolbarAction returns a new two-state ToolbarItem showing
// offIcon while the state is false and onIcon while it is true.
//
// Since: 1.0
func NewTwoStateToolbarAction(offIcon, onIcon bellui.Resource, onActivated func(on bool)) *TwoStateToolbarAction {
	t := &TwoStateToolbarAction{OnActivated: onActivated, offIcon: offIcon, onIcon: onIcon}
	t.button.SetIcon(t.offIcon)
	t.button.OnTapped = t.activated
	return t
}

// On returns the current state.
func (t *TwoStateToolbarAction) On() bool {
	return t.on
}

// SetOn updates the state, the icon shown, and notifies OnActivated when the
// state actually changed.
func (t *TwoStateToolbarAction) SetOn(on bool) {
	if t.on == on {
		return
	}
	t.on = on
	t.syncIcon()
	if f := t.OnActivated; f != nil {
		f(t.on)
	}
}

// SetOffIcon sets the icon that is displayed when the state is false.
func (t *TwoStateToolbarAction) SetOffIcon(icon bellui.Resource) {
	t.offIcon = icon
	t.syncIcon()
}

// SetOnIcon sets the icon that is displayed when the state is true.
func (t *TwoStateToolbarAction) SetOnIcon(icon bellui.Resource) {
	t.onIcon = icon
	t.syncIcon()
}

// ToolbarObject gets a button to render this ToolbarAction.
func (t *TwoStateToolbarAction) ToolbarObject() core.CanvasObject {
	t.button.Importance = LowImportance

	// synchronize properties
	t.syncIcon()
	t.button.OnTapped = t.activated
	return &t.button
}

func (t *TwoStateToolbarAction) activated() {
	t.SetOn(!t.on)
}

func (t *TwoStateToolbarAction) syncIcon() {
	if t.on {
		t.button.SetIcon(t.onIcon)
	} else {
		t.button.SetIcon(t.offIcon)
	}
}
