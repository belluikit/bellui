//go:build !gl

package webgpu

import (
	"image"
	"image/color"
	"math"
	"os"

	"github.com/belluikit/typesetting/font"
	"github.com/belluikit/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/render"
	"github.com/belluikit/bellui/theme"
)

// Batched drawing prototype (#475). With BELLUI_BATCH set, rectangles,
// rounded rectangles and single-line text are drawn as instances of one quad
// pipeline (shaders/quad.wgsl), which the Device draws in batches, and text
// comes from a glyph atlas instead of a texture per text object. Everything
// else, and anything under a scale, rotation or skew, takes the usual path.

var batchEnabled = os.Getenv("BELLUI_BATCH") != ""

const atlasSize = 2048

// atlasKey names one rendered glyph: a face's glyph at a pixel size, at one
// of four sub-pixel horizontal offsets.
type atlasKey struct {
	face *font.Face
	gid  font.GID
	size float32 // font size × pixel scale
	sub  uint8   // horizontal offset in quarter pixels
}

// atlasEntry is where a glyph's bitmap sits in the atlas, and the bitmap's
// offset from the glyph's pen position.
type atlasEntry struct {
	x, y, w, h   int
	left, top    int
	empty, color bool // nothing to draw; not an outline (falls back)
}

// glyphAtlas packs glyph bitmaps into one texture in shelves.
type glyphAtlas struct {
	tex            *Texture
	entries        map[atlasKey]*atlasEntry
	shelfX, shelfY int
	shelfH         int
	full           bool
}

// placedGlyph is one glyph of a laid-out text, relative to the text's origin.
type placedGlyph struct {
	x, y float32 // pen position, pixels
	key  atlasKey
}

// layoutKey is everything that decides a text's shaped glyphs. Layouts are
// shared by content, as text textures are: equal texts shape once.
type layoutKey struct {
	text  string
	size  float32
	scale float32
	fonts shaping.Fontmap
	style bellui.TextStyle
}

// textLayout is a text's shaped glyphs.
type textLayout struct {
	glyphs []placedGlyph
	used   uint64
}

type batchState struct {
	atlas   glyphAtlas
	layouts map[layoutKey]*textLayout
	entries []*atlasEntry // scratch for batchText
	xs      []float32

	uni        []byte // the quad uniform for uniW×uniH
	uniW, uniH float32
}

func (p *painter) batchUniform(frame geo.Size) []byte {
	fw, fh := p.scaleFrameSize(frame)
	if fw != p.batch.uniW || fh != p.batch.uniH || p.batch.uni == nil {
		u := [4]float32{fw, fh}
		p.batch.uni = append(p.batch.uni[:0], uniformBytes(&u)...)
		p.batch.uniW, p.batch.uniH = fw, fh
	}
	return p.batch.uni
}

func (p *painter) quadKey() PipelineKey {
	return PipelineKey{Shader: "quad", Fragment: FragmentEntry, Blend: BlendOver, Stencil: p.stencilMode, Format: p.dev.Format()}
}

// ensureAtlas creates the atlas texture on first use.
func (p *painter) ensureAtlas() bool {
	a := &p.batch.atlas
	if a.tex != nil {
		return true
	}
	tex, err := p.dev.CreateAtlasTexture(atlasSize, atlasSize)
	if err != nil {
		return false
	}
	a.tex = tex
	a.entries = map[atlasKey]*atlasEntry{}
	return true
}

// batchOblong draws a rectangle or rounded rectangle as one instance. It
// reports false when the shape needs the usual path.
func (p *painter) batchOblong(obj interface{ Size() geo.Size }, fill, stroke color.Color, strokeWidth, trR, tlR, brR, blR, aspect float32, pos geo.Position, frame geo.Size, bounds [4]float32) bool {
	if !batchEnabled || p.hasTransform() || !p.ensureAtlas() {
		return false
	}
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	strokeWidthScaled := roundToPixel(strokeWidth*p.pixScale, 1.0)
	size := geo.NewSize(bounds[2]-bounds[0], bounds[3]-bounds[1])
	var radius [4]float32
	if trR != 0 || tlR != 0 || brR != 0 || blR != 0 {
		radius = [4]float32{
			roundToPixel(paint.GetMaximumCornerRadius(trR, tlR, brR, size)*p.pixScale, 1.0),
			roundToPixel(paint.GetMaximumCornerRadius(brR, blR, trR, size)*p.pixScale, 1.0),
			roundToPixel(paint.GetMaximumCornerRadius(tlR, trR, blR, size)*p.pixScale, 1.0),
			roundToPixel(paint.GetMaximumCornerRadius(blR, brR, tlR, size)*p.pixScale, 1.0),
		}
	}
	p.appendShape(frame, x1, y1, x2, y2, radius, strokeWidthScaled*0.5, p.fragColor(fill), p.fragColor(orTransparent(stroke)))
	return true
}

// batchCircle draws a circle as one instance: a rounded rectangle whose
// corners are half its size.
func (p *painter) batchCircle(circle *canvas.Circle, pos geo.Position, frame geo.Size) bool {
	if !batchEnabled || p.hasTransform() || !p.ensureAtlas() {
		return false
	}
	bounds, _ := p.vecSquareCoords(pos, circle, frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	r := roundToPixel(paint.GetMaximumRadius(circle.Size())*p.pixScale, 1.0)
	p.appendShape(frame, x1, y1, x2, y2, [4]float32{r, r, r, r}, roundToPixel(circle.StrokeWidth*p.pixScale, 1.0)*0.5,
		p.fragColor(circle.FillColor), p.fragColor(orTransparent(circle.StrokeColor)))
	return true
}

// appendShape adds one rounded-rectangle instance (pixels, top-left origin).
func (p *painter) appendShape(frame geo.Size, x1, y1, x2, y2 float32, radius [4]float32, strokeHalf float32, fill, stroke [4]float32) {
	softness := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	inst := [QuadInstanceFloats]float32{
		x1, y1, x2, y2,
		radius[0], radius[1], radius[2], radius[3],
		fill[0], fill[1], fill[2], fill[3],
		stroke[0], stroke[1], stroke[2], stroke[3],
		strokeHalf, softness, 0, 0,
	}
	grow := softness + 1
	p.dev.AppendInstance(p.quadKey(), p.batchUniform(frame), p.batch.atlas.tex.View(), inst[:],
		int(x1-grow), int(y1-grow), int(x2-x1+2*grow)+1, int(y2-y1+2*grow)+1)
}

// batchText draws a single-line text from the glyph atlas. It reports false
// when the text needs the usual path.
func (p *painter) batchText(text *canvas.Text, pos geo.Position, frame geo.Size) bool {
	if !batchEnabled || p.hasTransform() || text.TextStyle.Underline || text.TextStyle.Strikethrough || !p.ensureAtlas() {
		return false
	}
	l := p.layoutText(text)
	if l == nil {
		return false
	}
	clr := text.Color
	if clr == nil {
		clr = theme.Color(theme.ColorNameForeground)
	}
	fillC := p.fragColor(clr)
	ox, oy := pos.X*p.pixScale, pos.Y*p.pixScale
	key, uni, view := p.quadKey(), p.batchUniform(frame), p.batch.atlas.tex.View()
	// Resolve every glyph first: a text the atlas cannot hold, or with a
	// glyph that is not an outline (colour emoji), takes the usual path whole.
	entries := p.batch.entries[:0]
	xs := p.batch.xs[:0]
	for _, g := range l.glyphs {
		gx := ox + g.x
		ix := float32(math.Floor(float64(gx)))
		sub := uint8(math.Round(float64(gx-ix) * 4))
		if sub == 4 {
			ix, sub = ix+1, 0
		}
		k := g.key
		k.sub = sub
		e := p.glyphEntry(k)
		if e == nil || e.color {
			return false
		}
		entries = append(entries, e)
		xs = append(xs, ix)
	}
	p.batch.entries, p.batch.xs = entries, xs
	for i, g := range l.glyphs {
		e, ix := entries[i], xs[i]
		if e.empty {
			continue
		}
		iy := float32(math.Round(float64(oy + g.y)))
		x0, y0 := ix+float32(e.left), iy+float32(e.top)
		inst := [QuadInstanceFloats]float32{
			x0, y0, x0 + float32(e.w), y0 + float32(e.h),
			0, 0, 0, 0,
			fillC[0], fillC[1], fillC[2], fillC[3],
			0, 0, 0, 0,
			0, 0, 1, 0,
			float32(e.x) / atlasSize, float32(e.y) / atlasSize,
			float32(e.x+e.w) / atlasSize, float32(e.y+e.h) / atlasSize,
		}
		p.dev.AppendInstance(key, uni, view, inst[:], int(x0), int(y0), e.w, e.h)
	}
	return true
}

// layoutText returns the text's shaped glyphs, shaping a text only the first
// time its content, size, style, scale or fonts are seen.
func (p *painter) layoutText(text *canvas.Text) *textLayout {
	if p.batch.layouts == nil {
		p.batch.layouts = map[layoutKey]*textLayout{}
	}
	face := paint.CachedFontFace(text.TextStyle, text.FontSource, text)
	k := layoutKey{text: text.Text, size: text.TextSize, scale: p.pixScale, fonts: face.Fonts, style: text.TextStyle}
	if l := p.batch.layouts[k]; l != nil {
		l.used = p.frame
		return l
	}
	l := &textLayout{used: p.frame}
	pixSize := text.TextSize * p.pixScale
	baseline := paint.ShapeString(face.Fonts, text.Text, text.TextSize, p.pixScale, text.TextStyle, func(run shaping.Output, x float32) {
		pen := float32(int(x)) // DrawShapedRunAt starts a run at a whole pixel
		for _, g := range run.Glyphs {
			l.glyphs = append(l.glyphs, placedGlyph{
				x:   pen + fixedToFloat(g.XOffset)*p.pixScale,
				y:   -fixedToFloat(g.YOffset) * p.pixScale,
				key: atlasKey{face: run.Face, gid: g.GlyphID, size: pixSize},
			})
			pen += fixedToFloat(g.Advance) * p.pixScale
		}
	})
	for i := range l.glyphs {
		l.glyphs[i].y += float32(baseline)
	}
	p.batch.layouts[k] = l
	return l
}

// glyphEntry returns the glyph's place in the atlas, rendering it there the
// first time. It returns nil when the atlas is full.
func (p *painter) glyphEntry(k atlasKey) *atlasEntry {
	a := &p.batch.atlas
	if e, ok := a.entries[k]; ok {
		return e
	}
	sub := float32(k.sub) / 4
	ps := p.pixScale
	ext, ok := k.face.GlyphExtents(k.gid)
	if !ok || ext.Width == 0 || ext.Height == 0 {
		e := &atlasEntry{empty: true}
		a.entries[k] = e
		return e
	}
	// Font units, y up, to pixels at the key's size.
	unit := k.size / float32(k.face.Upem())
	left := int(math.Floor(float64(ext.XBearing*unit+sub))) - 1
	right := int(math.Ceil(float64((ext.XBearing+ext.Width)*unit+sub))) + 1
	top := int(math.Floor(float64(-ext.YBearing*unit))) - 1
	bottom := int(math.Ceil(float64(-(ext.YBearing+ext.Height)*unit))) + 1
	w, h := right-left, bottom-top
	if w <= 0 || h <= 0 {
		e := &atlasEntry{empty: true}
		a.entries[k] = e
		return e
	}
	if _, ok := k.face.GlyphData(k.gid).(font.GlyphOutline); !ok {
		e := &atlasEntry{color: true}
		a.entries[k] = e
		return e
	}
	if a.shelfX+w > atlasSize {
		a.shelfX, a.shelfY, a.shelfH = 0, a.shelfY+a.shelfH, 0
	}
	if a.shelfY+h > atlasSize {
		a.full = true
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r := render.Renderer{FontSize: k.size / ps, PixScale: ps, Color: color.White}
	g := shaping.Glyph{GlyphID: k.gid, XOffset: floatToFixed(sub / ps)}
	r.DrawShapedRunAt(shaping.Output{Face: k.face, Glyphs: []shaping.Glyph{g}}, img, -left, -top)
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		a := img.Pix[i*4+3]
		pix[i*4], pix[i*4+1], pix[i*4+2], pix[i*4+3] = a, a, a, a
	}
	p.dev.WriteTextureRegion(a.tex, a.shelfX, a.shelfY, w, h, pix)
	e := &atlasEntry{x: a.shelfX, y: a.shelfY, w: w, h: h, left: left, top: top}
	a.shelfX += w + 1
	a.shelfH = max(a.shelfH, h+1)
	a.entries[k] = e
	return e
}

// evictLayouts forgets the layouts of texts not drawn for a while.
func (p *painter) evictLayouts() {
	if p.frame%120 != 0 {
		return
	}
	for t, l := range p.batch.layouts {
		if p.frame-l.used > 120 {
			delete(p.batch.layouts, t)
		}
	}
}

func fixedToFloat(v fixed.Int26_6) float32 { return float32(v) / 64 }
func floatToFixed(v float32) fixed.Int26_6 { return fixed.Int26_6(math.Round(float64(v) * 64)) }
