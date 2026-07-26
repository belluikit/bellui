package editor_test

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/editor"
)

func TestEditor_InsideVScroll(t *testing.T) {
	e := editor.NewEditor()
	e.SetText("line1\nline2\nline3\nline4\nline5")

	vbox := container.NewVBox(e)
	scroll := container.NewVScroll(vbox)

	w := test.NewWindow(scroll)
	w.Resize(geo.NewSize(400, 300))

	// The editor should render with proper size
	size := e.Size()
	t.Logf("Editor size inside VScroll: %v", size)

	if size.Width <= 0 {
		t.Errorf("Editor width=%f inside VScroll, expected > 0", size.Width)
	}
	if size.Height <= 0 {
		t.Errorf("Editor height=%f inside VScroll, expected > 0", size.Height)
	}
}
