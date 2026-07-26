package widget

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/theme"
	"github.com/stretchr/testify/assert"
)

type extendedIcon struct {
	Icon
}

func newExtendedIcon() *extendedIcon {
	icon := &extendedIcon{}
	icon.ExtendBaseWidget(icon)
	return icon
}

func TestIcon_Extended_SetResource(t *testing.T) {
	icon := newExtendedIcon()
	icon.SetResource(theme.ComputerIcon())

	objs := cache.Renderer(icon).Objects()
	assert.Len(t, objs, 1)
	assert.Equal(t, theme.ComputerIcon(), objs[0].(*canvas.Image).Resource)
}
