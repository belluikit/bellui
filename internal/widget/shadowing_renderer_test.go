package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	w "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/widget"
)

func TestShadowingRenderer_Objects(t *testing.T) {
	tests := map[string]struct {
		level                w.ElevationLevel
		wantPrependedObjects []core.CanvasObject
	}{
		"with shadow": {
			12,
			[]core.CanvasObject{},
		},
		"without shadow": {
			0,
			[]core.CanvasObject{},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shadowIndex := 0
			if tt.level > 0 {
				shadowIndex = 1 // Shadow pointers are not the same. Avoid comparing.
			}

			objects := []core.CanvasObject{widget.NewLabel("A"), widget.NewLabel("B")}
			r := w.NewShadowingRenderer(objects, tt.level)
			assert.Equal(t, append(tt.wantPrependedObjects, objects...), r.Objects()[shadowIndex:])

			otherObjects := []core.CanvasObject{widget.NewLabel("X"), widget.NewLabel("Y")}
			r.SetObjects(otherObjects)
			assert.Equal(t, append(tt.wantPrependedObjects, otherObjects...), r.Objects()[shadowIndex:])
		})
	}
}
