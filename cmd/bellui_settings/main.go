package main

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/cmd/bellui_settings/settings"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
)

func main() {
	s := settings.NewSettings()

	a := app.New()
	w := a.NewWindow("Bellui Settings")

	appearance := s.LoadAppearanceScreen(w)
	tabs := container.NewAppTabs(
		&container.TabItem{Text: "Appearance", Icon: s.AppearanceIcon(), Content: appearance},
	)
	tabs.SetTabLocation(container.TabLocationLeading)
	w.SetContent(tabs)

	w.Resize(geo.NewSize(520, 520))
	bellui.ShowAndRun(w)
}
