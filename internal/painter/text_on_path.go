package painter

import (
	"image"
	"math"

	"github.com/belluikit/render"
	"github.com/belluikit/typesetting/shaping"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// DrawTextOnPath renders text along a path into an NRGBA image.
// Each glyph is positioned at the correct distance along the path and
// rotated to follow the path tangent. Returns nil if there is nothing to draw.
func DrawTextOnPath(top *canvas.TextOnPath, scale float32) *image.NRGBA {
	if top.Text == "" || len(top.Elements()) == 0 {
		return nil
	}

	clr := top.Color
	if clr == nil {
		clr = theme.Color(theme.ColorNameForeground)
	}

	// Flatten and concatenate subpaths.
	subpaths := FlattenPath(top.Elements(), scale)
	points := ConcatenateSubpaths(subpaths)
	if len(points) < 2 {
		return nil
	}
	sampler := NewPathSampler(points)

	fontSize := top.TextSize
	face := CachedFontFace(top.TextStyle, top.FontSource, nil)

	// Collect per-glyph data: advance and glyph output for rendering.
	type glyphInfo struct {
		run     shaping.Output // single-glyph run for rendering
		advance float32        // advance width in scaled pixels
		ascent  float32        // ascent in scaled pixels
	}

	var glyphs []glyphInfo
	advance := float32(0)
	walkString(face.Fonts, top.Text, float32ToFixed266(fontSize), top.TextStyle, &advance, scale,
		func(run shaping.Output, x float32) {
			ascent := fixed266ToFloat32(run.LineBounds.Ascent) * scale
			for i := range run.Glyphs {
				singleRun := run
				singleRun.Glyphs = run.Glyphs[i : i+1]
				glyphAdv := fixed266ToFloat32(run.Glyphs[i].Advance) * scale
				glyphs = append(glyphs, glyphInfo{
					run:     singleRun,
					advance: glyphAdv,
					ascent:  ascent,
				})
			}
		})

	if len(glyphs) == 0 {
		return nil
	}

	// Compute bounding box of all positioned glyphs to determine output image size.
	// Track min/max coordinates considering glyph positions and rotation.
	type glyphPlacement struct {
		x, y, angle float32
		info        glyphInfo
	}

	offset := top.Offset * scale
	baselineShift := top.BaselineShift * scale

	placements := make([]glyphPlacement, 0, len(glyphs))
	cumDist := offset
	for _, g := range glyphs {
		// Position glyph at center of its advance.
		centerDist := cumDist + g.advance/2
		if centerDist > sampler.TotalLength() {
			break // past end of path
		}

		px, py, angle := sampler.Sample(centerDist)

		// Apply baseline shift perpendicular to tangent.
		if baselineShift != 0 {
			perpX := -float32(math.Sin(float64(angle)))
			perpY := float32(math.Cos(float64(angle)))
			px += perpX * baselineShift
			py += perpY * baselineShift
		}

		placements = append(placements, glyphPlacement{
			x: px, y: py, angle: angle, info: g,
		})
		cumDist += g.advance
	}

	if len(placements) == 0 {
		return nil
	}

	// Compute axis-aligned bounding box of all placed glyphs.
	// Each glyph occupies roughly advance x lineHeight, rotated by angle.
	maxAscent := float32(0)
	for _, g := range glyphs {
		if g.ascent > maxAscent {
			maxAscent = g.ascent
		}
	}
	lineHeight := maxAscent * 1.5 // conservative estimate

	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)

	for _, pl := range placements {
		// Glyph rect corners relative to center.
		hw := pl.info.advance / 2
		hh := lineHeight / 2
		corners := [4][2]float32{
			{-hw, -hh}, {hw, -hh}, {hw, hh}, {-hw, hh},
		}
		cos := float32(math.Cos(float64(pl.angle)))
		sin := float32(math.Sin(float64(pl.angle)))
		for _, c := range corners {
			rx := c[0]*cos - c[1]*sin + pl.x
			ry := c[0]*sin + c[1]*cos + pl.y
			if rx < minX {
				minX = rx
			}
			if ry < minY {
				minY = ry
			}
			if rx > maxX {
				maxX = rx
			}
			if ry > maxY {
				maxY = ry
			}
		}
	}

	// Add padding for glyph overshoot.
	pad := maxAscent
	minX -= pad
	minY -= pad
	maxX += pad
	maxY += pad

	imgW := int(math.Ceil(float64(maxX - minX)))
	imgH := int(math.Ceil(float64(maxY - minY)))
	if imgW <= 0 || imgH <= 0 {
		return nil
	}

	output := image.NewNRGBA(image.Rect(0, 0, imgW, imgH))

	// Render each glyph.
	r := render.Renderer{
		FontSize: fontSize,
		PixScale: scale,
		Color:    clr,
	}

	for _, pl := range placements {
		g := pl.info

		// Render glyph into a temp image.
		glyphW := int(math.Ceil(float64(g.advance))) + 4 // +4 for bearing/padding
		glyphH := int(math.Ceil(float64(lineHeight))) + 4
		if glyphW <= 0 || glyphH <= 0 {
			continue
		}
		tmp := image.NewNRGBA(image.Rect(0, 0, glyphW, glyphH))

		baseline := int(math.Ceil(float64(g.ascent)))
		if g.run.Glyphs[0].GlyphID == 0 {
			r.DrawStringAt(string([]rune{0xfffd}), tmp, 0, baseline, face.Fonts.ResolveFace(0xfffd))
		} else {
			r.DrawShapedRunAt(g.run, tmp, 0, baseline)
		}

		// Compute affine transform: rotate around glyph center, then translate to path position.
		// Source glyph center in temp image coordinates.
		gcx := float64(g.advance) / 2
		gcy := float64(lineHeight) / 2

		// Target position in output image coordinates (offset by minX, minY).
		tx := float64(pl.x - minX)
		ty := float64(pl.y - minY)

		cos := math.Cos(float64(pl.angle))
		sin := math.Sin(float64(pl.angle))

		// Affine: translate(-gcx,-gcy) → rotate → translate(tx,ty)
		// [cos -sin tx-gcx*cos+gcy*sin]
		// [sin  cos ty-gcx*sin-gcy*cos]
		aff := f64.Aff3{
			cos, -sin, tx - gcx*cos + gcy*sin,
			sin, cos, ty - gcx*sin - gcy*cos,
		}

		draw.BiLinear.Transform(output, aff, tmp, tmp.Bounds(), draw.Over, nil)
	}

	return output
}

// TextOnPathBounds returns the bounding box size of text on path rendering.
// This is used by MinSize calculations.
func TextOnPathBounds(top *canvas.TextOnPath) geo.Size {
	if top.Text == "" || len(top.Elements()) == 0 {
		return geo.Size{}
	}

	subpaths := FlattenPath(top.Elements(), 1)
	points := ConcatenateSubpaths(subpaths)
	if len(points) < 2 {
		return geo.Size{}
	}

	sampler := NewPathSampler(points)
	face := CachedFontFace(top.TextStyle, top.FontSource, nil)

	advance := float32(0)
	totalAdvance := float32(0)
	walkString(face.Fonts, top.Text, float32ToFixed266(top.TextSize), top.TextStyle, &advance, 1,
		func(run shaping.Output, x float32) {
			for _, g := range run.Glyphs {
				totalAdvance += fixed266ToFloat32(g.Advance)
			}
		})

	// Approximate: path bounds + text height padding.
	textHeight := fixed266ToFloat32(func() shaping.Bounds {
		in := shaping.Input{
			Text:     []rune{' '},
			RunStart: 0, RunEnd: 1,
			Face: face.Fonts.ResolveFace(' '),
			Size: float32ToFixed266(top.TextSize),
		}
		shaper := &shaping.HarfbuzzShaper{}
		return shaper.Shape(in).LineBounds
	}().LineThickness())

	_ = sampler
	_ = totalAdvance

	// Use path control-point bounds + text height padding.
	minX, minY, maxX, maxY := pathBoundsFromPoints(points)
	pad := textHeight + top.BaselineShift
	return geo.NewSize(maxX-minX+pad*2, maxY-minY+pad*2)
}

func pathBoundsFromPoints(points []geo.Position) (minX, minY, maxX, maxY float32) {
	minX, minY = float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY = float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, p := range points {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return
}
