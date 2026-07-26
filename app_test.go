package bellui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type dummyApp struct{}

func (dummyApp) NewWindow(title string) Window {
	return nil
}

func (dummyApp) Icon() Resource {
	return nil
}

func (dummyApp) SetIcon(Resource) {
}

func (dummyApp) Run() {
}

func (dummyApp) Quit() {
}

func (dummyApp) Driver() Driver {
	return nil
}

func (dummyApp) UniqueID() string {
	return "dummy"
}

func (dummyApp) Settings() Settings {
	return nil
}

func (dummyApp) Storage() Storage {
	return nil
}

func (dummyApp) Preferences() Preferences {
	return nil
}

func (dummyApp) Lifecycle() Lifecycle {
	return nil
}

func (dummyApp) Metadata() AppMetadata {
	return AppMetadata{}
}

func (dummyApp) Clipboard() Clipboard {
	return nil
}

func TestSetCurrentApp(t *testing.T) {
	a := &dummyApp{}
	SetCurrentApp(a)

	assert.Equal(t, a, CurrentApp())
}
