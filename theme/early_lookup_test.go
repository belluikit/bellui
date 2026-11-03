package theme_test

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/apphook"
	"github.com/belluikit/bellui/theme"
)

// TestLookupBeforeTheAppStarts pins #861: a theme icon or colour looked up
// before any app exists, as an app's metadata can be, gets the built-in theme
// and one log line that says so, not "Attempt to access current Bellui app".
func TestLookupBeforeTheAppStarts(t *testing.T) {
	if apphook.Started() {
		prevApp := bellui.CurrentApp()
		t.Cleanup(func() { bellui.SetCurrentApp(prevApp) })
	}
	bellui.SetCurrentApp(nil)
	var out bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(prev) })

	if theme.ColorPaletteIcon() == nil {
		t.Error("no icon before the app started")
	}
	_ = theme.Color(theme.ColorNameForeground)
	_ = theme.Current()

	got := out.String()
	if strings.Contains(got, "none is started") {
		t.Errorf("a lookup before the app logged the generic error:\n%s", got)
	}
	if n := strings.Count(got, "before the app started"); n != 1 {
		t.Errorf("logged %d lines naming the early lookup; want one:\n%s", n, got)
	}
}
