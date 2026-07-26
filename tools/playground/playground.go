// Package playground provides tooling for running bellui applications inside the Go playground.
package playground

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/theme"
)

func encodeImage(img image.Image) (string, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func imageToPlayground(img image.Image) {
	enc, err := encodeImage(img)
	if err != nil {
		bellui.LogError("Failed to encode image", err)
		return
	}

	fmt.Println("IMAGE:" + enc)
}

// RenderCanvas takes a canvas and converts it into an inline image for showing in the playground
func RenderCanvas(c core.Canvas) {
	imageToPlayground(software.RenderCanvas(c, test.DarkTheme(theme.DefaultTheme())))
}

// RenderWindow takes a window and converts its canvas into an inline image for showing in the playground
func RenderWindow(w bellui.Window) {
	RenderCanvas(w.Canvas())
}

// Render takes a canvasobject and converts it into an inline image for showing in the playground
func Render(obj core.CanvasObject) {
	imageToPlayground(software.Render(obj, test.DarkTheme(theme.DefaultTheme())))
}
