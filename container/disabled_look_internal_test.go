package container

import (
	"testing"

	"github.com/belluikit/bellui/internal/lookcheck"
)

// fallbackDimmed are the types here with a Disable method that the paint walk
// may dim (#1223).
var fallbackDimmed = map[string]bool{
	// Not core.Disableable (it has no Disabled method), so the walk never
	// dims it; the widget.Button it forwards to draws the disabled look.
	"borderButton": true,
}

// Every disableable widget in the package has made the choice core.DisabledLook
// asks for, as widget's guard test checks for that package (#1223): it draws
// its own disabled look (disable.go), or it is on the fallback list above.
func TestDisabledLook_EveryDisableableContainerChose(t *testing.T) {
	missing, err := lookcheck.Unchosen(".", "container", fallbackDimmed)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("disableable widgets that neither draw their own disabled look (DrawsDisabledLook in disable.go) nor are on fallbackDimmed: %v", missing)
	}
}
