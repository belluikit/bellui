//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/theme"
)

const floatEqualityThreshold = 1e-9

var noTexture = Texture(cache.NoTexture)

// Texture represents an uploaded GL texture
type Texture cache.TextureType

func (p *painter) freeTexture(obj core.CanvasObject) {
	texture, ok := cache.GetTexture(obj)
	if !ok {
		return
	}

	p.ctx.DeleteTexture(Texture(texture))
	p.logError()
	cache.DeleteTexture(obj)
}

func (p *painter) getTexture(object core.CanvasObject, creator func(canvasObject core.CanvasObject) Texture) (Texture, error) {
	if t, ok := object.(*canvas.Text); ok {
		custom := ""
		if t.FontSource != nil {
			custom = t.FontSource.Name()
		}
		ent := cache.NewFontCacheEntry(t.Text, t.TextSize, t.TextStyle, custom, p.canvas, t.Color)

		texture, ok := cache.GetTextTexture(ent)

		if !ok {
			tex := creator(object)
			texture = cache.TextureType(tex)
			cache.SetTextTexture(ent, texture, p.canvas, func() {
				p.ctx.DeleteTexture(tex)
			})
		}

		return Texture(texture), nil
	}

	texture, ok := cache.GetTexture(object)

	if !ok {
		texture = cache.TextureType(creator(object))
		cache.SetTexture(object, texture, p.canvas)
	}
	if !cache.IsValid(texture) {
		return noTexture, errors.New("no texture available")
	}
	return Texture(texture), nil
}

func (p *painter) imgToTexture(img image.Image, textureFilter canvas.ImageScale) Texture {
	switch i := img.(type) {
	case *image.Uniform:
		texture := p.newTexture(textureFilter)
		r, g, b, a := i.RGBA()
		r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
		data := []uint8{r8, g8, b8, a8}
		p.ctx.TexImage2D(
			texture2D,
			0,
			1,
			1,
			colorFormatRGBA,
			unsignedByte,
			data,
		)
		p.logError()
		return texture
	case *image.RGBA:
		if len(i.Pix) == 0 { // image is empty
			return noTexture
		}

		texture := p.newTexture(textureFilter)
		p.ctx.TexImage2D(
			texture2D,
			0,
			i.Rect.Size().X,
			i.Rect.Size().Y,
			colorFormatRGBA,
			unsignedByte,
			i.Pix,
		)
		p.logError()
		return texture
	default:
		rgba := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(rgba, rgba.Rect, img, image.Point{}, draw.Over)
		return p.imgToTexture(rgba, textureFilter)
	}
}

func (p *painter) newGlImageTexture(obj core.CanvasObject) Texture {
	img := obj.(*canvas.Image)

	width := p.textureScale(img.Size().Width)
	height := p.textureScale(img.Size().Height)

	tex := paint.PaintImage(img, p.canvas, int(width), int(height))
	if tex == nil {
		return noTexture
	}

	return p.imgToTexture(tex, img.ScaleMode)
}

func (p *painter) newGlLinearGradientTexture(obj core.CanvasObject) Texture {
	gradient := obj.(*canvas.LinearGradient)

	w := gradient.Size().Width
	h := gradient.Size().Height
	switch a := gradient.Angle; {
	case almostEqual(a, 90), almostEqual(a, 270):
		h = 1
	case almostEqual(a, 0), almostEqual(a, 180):
		w = 1
	}
	width := p.textureScale(w)
	height := p.textureScale(h)

	return p.imgToTexture(gradient.Generate(int(width), int(height)), canvas.ImageScaleSmooth)
}

func (p *painter) newGlRadialGradientTexture(obj core.CanvasObject) Texture {
	gradient := obj.(*canvas.RadialGradient)

	width := p.textureScale(gradient.Size().Width)
	height := p.textureScale(gradient.Size().Height)

	return p.imgToTexture(gradient.Generate(int(width), int(height)), canvas.ImageScaleSmooth)
}

func (p *painter) newGlRasterTexture(obj core.CanvasObject) Texture {
	rast := obj.(*canvas.Raster)

	width := p.textureScale(rast.Size().Width)
	height := p.textureScale(rast.Size().Height)

	tex := p.imgToTexture(rast.Generator(int(width), int(height)), rast.ScaleMode)
	if rast.Streaming {
		rast.StreamMu.Lock()
		rast.StreamTexW = int(width)
		rast.StreamTexH = int(height)
		rast.StreamMu.Unlock()
	}
	return tex
}

func (p *painter) updateStreamingTexture(obj *canvas.Raster) {
	texture, ok := cache.GetTexture(obj)
	if !ok {
		return // no existing texture; will be created by getTexture
	}

	width := int(p.textureScale(obj.Size().Width))
	height := int(p.textureScale(obj.Size().Height))

	obj.StreamMu.Lock()
	var pix []uint8
	if obj.StreamPix != nil {
		pix = obj.StreamPix
		width = obj.StreamPixW
		height = obj.StreamPixH
	}

	if pix == nil {
		obj.StreamMu.Unlock()
		// Generator path: produces a fresh image, no race with SetPixels.
		if obj.Generator == nil {
			return
		}
		img := obj.Generator(width, height)
		switch i := img.(type) {
		case *image.RGBA:
			pix = i.Pix
		default:
			rgba := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
			draw.Draw(rgba, rgba.Rect, img, image.Point{}, draw.Over)
			pix = rgba.Pix
		}
		if len(pix) == 0 {
			return
		}
		// Re-lock for size check and texture upload.
		obj.StreamMu.Lock()
	}
	defer obj.StreamMu.Unlock()

	if len(pix) == 0 {
		return
	}

	// Size changed — delete old texture and let getTexture create a new one
	if obj.StreamTexW != width || obj.StreamTexH != height {
		p.ctx.DeleteTexture(Texture(texture))
		p.logError()
		cache.DeleteTexture(obj)
		obj.StreamTexW = width
		obj.StreamTexH = height
		return
	}

	// Same size — in-place update (lock held to prevent SetPixels race)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, Texture(texture))
	p.ctx.TexSubImage2D(
		texture2D, 0, 0, 0,
		width, height,
		colorFormatRGBA, unsignedByte,
		pix,
	)
	p.logError()
}

func (p *painter) newGlTextTexture(obj core.CanvasObject) Texture {
	text := obj.(*canvas.Text)
	color := text.Color
	if color == nil {
		color = theme.Color(theme.ColorNameForeground)
	}

	bounds := text.MinSize()
	width := int(math.Ceil(float64(p.textureScale(bounds.Width) + paint.VectorPad(text)))) // potentially italic overspill
	height := int(math.Ceil(float64(p.textureScale(bounds.Height))))
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	face := paint.CachedFontFace(text.TextStyle, text.FontSource, text)
	paint.DrawString(img, text.Text, color, face.Fonts, text.TextSize, p.pixScale, text.TextStyle)
	return p.imgToTexture(img, canvas.ImageScaleSmooth)
}

func (p *painter) newGlStyledTextTexture(obj core.CanvasObject) Texture {
	st := obj.(*canvas.StyledText)
	clr := st.Color
	if clr == nil {
		clr = theme.Color(theme.ColorNameForeground)
	}

	bounds := st.MinSize()
	width := int(math.Ceil(float64(p.textureScale(bounds.Width) + paint.VectorPad(st))))
	height := int(math.Ceil(float64(p.textureScale(bounds.Height))))
	if width <= 0 || height <= 0 {
		return noTexture
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	paint.DrawStyledRuns(img, st.Runs, clr, st.TextSize, p.pixScale)
	return p.imgToTexture(img, canvas.ImageScaleSmooth)
}

func (p *painter) newGlTextOnPathTexture(obj core.CanvasObject) Texture {
	top := obj.(*canvas.TextOnPath)
	img := paint.DrawTextOnPath(top, p.pixScale)
	if img == nil {
		return noTexture
	}
	return p.imgToTexture(img, canvas.ImageScaleSmooth)
}

func (p *painter) newTexture(textureFilter canvas.ImageScale) Texture {
	if int(textureFilter) >= len(textureFilterToGL) {
		bellui.LogError(fmt.Sprintf("Invalid canvas.ImageScale value (%d), using canvas.ImageScaleSmooth as default value", textureFilter), nil)
		textureFilter = canvas.ImageScaleSmooth
	}

	texture := p.ctx.CreateTexture()
	p.logError()
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, texture)
	p.logError()
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[textureFilter])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[textureFilter])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.logError()

	return texture
}

func (p *painter) textureScale(v float32) float32 {
	if p.pixScale == 1.0 {
		return float32(math.Round(float64(v)))
	}

	return float32(math.Round(float64(v * p.pixScale)))
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatEqualityThreshold
}
