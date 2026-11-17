package widget

import (
	"testing"

	"github.com/belluikit/bellui/internal/lookcheck"
)

// fallbackDimmed are the disableable widgets that draw no disabled look of
// their own, so the paint walk dims them (#1223).
var fallbackDimmed = map[string]bool{
	"DisableableWidget": true, // the base an app's widget embeds
	"SwipeArea":         true,
	"WheelPicker":       true,
	"ToolbarAction":     true, // not drawn: its button is, and Button draws its look
}

// Every disableable widget in the package has made the choice core.DisabledLook
// asks for: it draws its own disabled look (disabled_look.go), or it is on the
// fallback list above. A new widget with a Disable method fails here until it
// is added to one, so it is never dimmed twice, or not at all, by accident.
func TestDisabledLook_EveryDisableableWidgetChose(t *testing.T) {
	missing, err := lookcheck.Unchosen(".", "widget", fallbackDimmed)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("disableable widgets that neither draw their own disabled look (DrawsDisabledLook in disabled_look.go) nor are on fallbackDimmed: %v", missing)
	}
}
