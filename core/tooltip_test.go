package core

import "testing"

type mockToolTipable struct {
	mockNonAccessible
	text string
}

func (m *mockToolTipable) ToolTip() string { return m.text }

var _ ToolTipable = (*mockToolTipable)(nil)

func TestNotifyToolTip_NilByDefault(t *testing.T) {
	// Without the widget package installed the seam is nil, so a driver pays a
	// single nil check and NotifyToolTip must not panic in either direction.
	if ShowToolTip != nil || HideToolTip != nil {
		t.Fatal("tooltip seam should be nil unless a controller is installed")
	}
	NotifyToolTip(nil, nil)
	NotifyToolTip(&mockToolTipable{text: "hi"}, nil)
}

func TestNotifyToolTip_Dispatch(t *testing.T) {
	var shown CanvasObject
	hidden := 0
	ShowToolTip = func(o CanvasObject, _ Canvas) { shown = o }
	HideToolTip = func() { hidden++ }
	t.Cleanup(func() { ShowToolTip = nil; HideToolTip = nil })

	obj := &mockToolTipable{text: "New file"}
	NotifyToolTip(obj, nil)
	if shown != obj {
		t.Fatal("a non-nil target must route to ShowToolTip")
	}
	if hidden != 0 {
		t.Fatal("showing a target must not hide")
	}

	NotifyToolTip(nil, nil)
	if hidden != 1 {
		t.Fatalf("a nil target must route to HideToolTip once, got %d", hidden)
	}
}
