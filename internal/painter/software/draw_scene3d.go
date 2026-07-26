package software

import (
	"image"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"

	"golang.org/x/image/draw"
)

func drawScene3D(c core.Canvas, scene *canvas.Scene3D, pos geo.Position, base *image.NRGBA, clip image.Rectangle) {
	scaledWidth := scale.ToScreenCoordinate(c, scene.Size().Width)
	scaledHeight := scale.ToScreenCoordinate(c, scene.Size().Height)
	scaledX := scale.ToScreenCoordinate(c, pos.X)
	scaledY := scale.ToScreenCoordinate(c, pos.Y)

	if scaledWidth <= 0 || scaledHeight <= 0 {
		return
	}

	// Render the 3D scene to an image
	rendered := scene.Render(scaledWidth, scaledHeight)

	bounds := clip.Intersect(image.Rect(scaledX, scaledY, scaledX+scaledWidth, scaledY+scaledHeight))
	offX, offY := 0, 0
	if scaledX < 0 {
		offX = -scaledX
	}
	if scaledY < 0 {
		offY = -scaledY
	}
	draw.Draw(base, bounds, rendered, image.Point{offX, offY}, draw.Over)
}
