package app_test

import (
	"testing"

	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/test"
)

func TestApplySettings_BeforeContentSet(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("NoContent")
	defer w.Close()

	app.ApplySettings(a.Settings(), a)
}
