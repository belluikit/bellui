//go:build tamago || noos || tinygo

package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/embedded"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
	"github.com/belluikit/bellui/theme"
)

func NewWithID(id string) bellui.App {
	return newAppWithDriver(nil, nil, id)
}

// SetDriverDetails provides the required information to our app to run without a standard
// driver. This is useful for embedded devices like GOOS=tamago, tinygo or noos.
//
// Since: 1.0
func SetDriverDetails(a bellui.App, d embedded.Driver) {
	a.(*belluiApp).Settings().SetTheme(theme.DefaultTheme())
	a.(*belluiApp).driver = intNoos.NewNoOSDriver(d.Render, d.Run, d.Queue(), d.ScreenSize)
}
