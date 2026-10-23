package editor

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
)

// MaxAutoGrowRows' zero value means the default cap of 50 rows, not
// "unlimited" as its comment used to say (#339). The cap has been 50 since
// AutoGrow was added; this pins it.
func TestAutoGrow_ZeroMaxMeansFifty(t *testing.T) {
	test.NewTempApp(t)
	text := strings.Repeat("line\n", 79) + "line" // 80 lines

	height := func(maxRows int) float32 {
		e := NewEditor()
		e.AutoGrow = true
		e.MaxAutoGrowRows = maxRows
		e.SetText(text)
		return e.entry.MinSize().Height
	}

	zero, fifty, eighty := height(0), height(50), height(80)
	if zero != fifty {
		t.Errorf("MaxAutoGrowRows 0 grew to height %v, want the 50-row cap's %v", zero, fifty)
	}
	if eighty <= fifty {
		t.Errorf("MaxAutoGrowRows 80 grew to height %v, want more than the 50-row cap's %v", eighty, fifty)
	}
}
