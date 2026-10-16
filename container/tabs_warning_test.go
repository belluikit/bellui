package container

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// assertNoExtendBaseWidgetWarning builds and renders a widget while capturing the
// global logger, failing if the "used before ExtendBaseWidget was called" warning
// (widget/widget.go super()) fires. Guards pre-rebrand tracker #388: the tab renderers' overflow/all
// action button was built as a bare literal and Hide()/Show()/Refresh()/SetIcon()'d
// before ExtendBaseWidget, logging a spurious warning on every app that uses tabs.
//
// Note: widget.warnedWidgets dedups by call-site for the whole process, so this is
// most decisive run in isolation; the fix makes the warning never fire regardless,
// so the assertion holds in-suite too.
func assertNoExtendBaseWidgetWarning(t *testing.T, render func()) {
	t.Helper()
	// Clear the process-global warn-once de-dup so the assertion observes the
	// warning fresh regardless of what earlier tests in this package tripped (pre-rebrand tracker #394);
	// otherwise a reintroduced bug could be masked in-suite.
	widget.ResetExtendWarningsForTests(t)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	render()

	if strings.Contains(buf.String(), "used before ExtendBaseWidget") {
		t.Errorf("spurious ExtendBaseWidget warning logged during tab render:\n%s", buf.String())
	}
}

func TestAppTabs_NoExtendBaseWidgetWarning(t *testing.T) {
	test.NewTempApp(t)
	assertNoExtendBaseWidgetWarning(t, func() {
		// One tab: updateTabs takes the tabCount<=max branch → r.action.Hide(), the
		// path that triggered the warning (the multi-tab overflow branch calls Show(),
		// which early-returns on an already-visible button and hides the bug).
		tabs := NewAppTabs(NewTabItem("Files", widget.NewLabel("x")))
		test.TempWidgetRenderer(t, tabs) // forces CreateRenderer → updateTabs/applyTheme
	})
}

func TestDocTabs_NoExtendBaseWidgetWarning(t *testing.T) {
	test.NewTempApp(t)
	assertNoExtendBaseWidgetWarning(t, func() {
		// Empty DocTabs: updateAllTabs takes the no-items branch → r.action.Hide().
		tabs := NewDocTabs()
		test.TempWidgetRenderer(t, tabs) // forces CreateRenderer → updateAllTabs/applyTheme
	})
}
