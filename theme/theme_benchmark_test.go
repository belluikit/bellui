package theme

import (
	"testing"

	"github.com/belluikit/bellui"
)

func BenchmarkTheme_current(b *testing.B) {
	bellui.CurrentApp().Settings().SetTheme(LightTheme())

	for n := 0; n < b.N; n++ {
		Current()
	}
}
