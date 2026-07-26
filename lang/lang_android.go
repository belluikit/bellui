package lang

import (
	"github.com/belluikit/bellui/internal/driver/mobile/app"

	"github.com/jeandeaual/go-locale"
)

func initRuntime() {
	locale.SetRunOnJVM(app.RunOnJVM)
}
