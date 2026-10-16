// Command repro960 drives the pre-rebrand tracker #960 scene (Transform-scaled canvas.Text vs
// untransformed and Transform-scaled Rectangles) on a real window. On the
// OnePlus Pad 2 (Adreno 750, WebGPU default) the transformed text is reported
// to render as mispositioned fragments / vanish while the rectangles are fine;
// under the software painter the same scene (hack test) is correct.
//
// A repeat-forever animation nudges the whole scene horizontally so cross-frame
// fragment/flicker behaviour is visible, not just a static frame.
package main

import (
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

func main() {
	a := app.New()
	w := a.NewWindow("repro960")

	scene := BuildScene()
	w.SetContent(scene)
	w.Resize(geo.NewSize(400, 700))

	// Slide the scene left/right forever to exercise the dirty-region / per-frame
	// path (the "fragments across frames" symptom). Opt-in via REPRO960_ANIM=1 so
	// a static screenshot stays deterministic by default.
	if os.Getenv("REPRO960_ANIM") == "1" {
		anim := canvas.NewPositionAnimation(
			geo.NewPos(0, 0), geo.NewPos(40, 0), canvas.DurationStandard*4,
			func(p geo.Position) {
				scene.Move(p)
				canvas.Refresh(scene)
			},
		)
		anim.RepeatCount = bellui.AnimationRepeatForever
		anim.AutoReverse = true
		anim.Start()
	}

	bellui.ShowAndRun(w)
}
