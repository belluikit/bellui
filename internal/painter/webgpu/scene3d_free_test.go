//go:build !gl

package webgpu

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
)

// Free drops a 3D scene's cache entry, so the scene is rendered again on its
// next draw. The painter checks the object's type before touching the map
// (#543); this keeps that check from skipping the scenes themselves.
func TestFreeDropsSceneEntry(t *testing.T) {
	scene := canvas.NewScene3D()
	p := &painter{scenes: map[core.CanvasObject]*sceneState{scene: {}}}

	p.Free(canvas.NewRectangle(nil))
	if _, ok := p.scenes[scene]; !ok {
		t.Fatal("freeing another object dropped the scene's entry")
	}
	p.Free(scene)
	if _, ok := p.scenes[scene]; ok {
		t.Fatal("freeing the scene kept its entry")
	}
}
