package animation

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
)

func BenchmarkRunnerAllocs(b *testing.B) {
	r := Runner{}
	var fl float32
	// setup some animations
	for i := 0; i < 10; i++ {
		r.pendingAnimations = append(r.pendingAnimations, newAnim(
			bellui.NewAnimation(1000*time.Second, func(f float32) {
				fl = f
			}),
		))
	}
	for n := 0; n < b.N; n++ {
		r.runOneFrame()
	}

	b.ReportAllocs()
	fl = fl + 1 // dummy use of variable
}

// TestRunnerActive checks Active, which keeps the desktop loop ticking every
// frame while an animation runs (#470): true from Start until the last
// animation finishes.
func TestRunnerActive(t *testing.T) {
	r := Runner{}
	if r.Active() {
		t.Fatal("a new runner is active")
	}
	r.Start(bellui.NewAnimation(time.Millisecond, func(float32) {}))
	if !r.Active() {
		t.Fatal("a runner with a started animation is not active")
	}
	time.Sleep(2 * time.Millisecond)
	r.TickAnimations() // ends the animation
	if r.Active() {
		t.Fatal("a runner whose animations finished is still active")
	}
}
