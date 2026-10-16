package painter

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/internal/render"
	"github.com/belluikit/typesetting/di"
	"github.com/belluikit/typesetting/font"
	"github.com/belluikit/typesetting/fontscan"
	"github.com/belluikit/typesetting/language"
	"github.com/belluikit/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

func init() {
	bellui.RegisterTextMeasure(RenderedTextSize)
	cache.CleanHooks = append(cache.CleanHooks, func() {
		SweepFontCache(cache.ValidDuration)
	})
}

const (
	// DefaultTabWidth is the default width in spaces
	DefaultTabWidth = 4

	fontTabSpaceSize = 10
)

var (
	fm           *fontscan.FontMap
	fontScanLock sync.Mutex
	loaded       bool
)

func loadMap() {
	loaded = true

	fm = fontscan.NewFontMap(noopLogger{})
	err := loadSystemFonts(fm)
	if err != nil {
		fm = nil // just don't fallback
	}
}

func lookupLangFont(family string, aspect font.Aspect) *font.Face {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()

	if !loaded {
		loadMap()
	}
	if fm == nil {
		return nil
	}

	fm.SetQuery(fontscan.Query{Families: []string{family}, Aspect: aspect})
	l, _ := language.NewLangID(language.Language(lang.SystemLocale().LanguageString()))
	return fm.ResolveFaceForLang(l)
}

func lookupRuneFont(r rune, family string, aspect font.Aspect) *font.Face {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()

	if !loaded {
		loadMap()
	}
	if fm == nil {
		return nil
	}

	fm.SetQuery(fontscan.Query{Families: []string{family}, Aspect: aspect})
	return fm.ResolveFace(r)
}

func lookupFaces(theme, fallback, emoji bellui.Resource, family string, style bellui.TextStyle) (faces *dynamicFontMap) {
	f1 := loadMeasureFont(theme)
	if theme == fallback {
		faces = &dynamicFontMap{family: family, faces: []*font.Face{f1}}
	} else {
		f2 := loadMeasureFont(fallback)
		faces = &dynamicFontMap{family: family, faces: []*font.Face{f1, f2}}
	}

	aspect := font.Aspect{Style: font.StyleNormal}
	if style.Italic {
		aspect.Style = font.StyleItalic
	}
	if style.Weight != nil {
		aspect.Weight = font.Weight(*style.Weight)
	} else if style.Bold {
		aspect.Weight = font.WeightBold
	}

	if emoji != nil {
		faces.addFace(loadMeasureFont(emoji))
	}
	// Bundled UI-symbol fallbacks (✓, ☐, arrows, ⌘, geometric shapes, …) sit
	// before the host locale face so these glyphs keep identical metrics on
	// every platform instead of resolving through a host system font (pre-rebrand tracker #686).
	addSymbolsFallbackFaces(faces)

	local := lookupLangFont(family, aspect)
	if local != nil {
		faces.systemStart = len(faces.faces)
		faces.addFace(local)
	}

	return faces
}

// CachedFontFace returns a Font face held in memory. These are loaded from the current theme.
func CachedFontFace(style bellui.TextStyle, source bellui.Resource, o core.CanvasObject) *FontCacheItem {
	now := time.Now().Unix()
	if source != nil {
		val, ok := fontCustomCache.Load(source)
		if !ok {
			face := loadMeasureFont(source)
			if face == nil {
				face = loadMeasureFont(theme.TextFont())
			}
			faces := &dynamicFontMap{family: source.Name(), faces: []*font.Face{face}}
			// A custom source covers only its own glyphs; keep emoji and the
			// bundled symbol fallbacks working, and rely on ResolveFace for
			// app-registered (e.g. CJK) and system fallback.
			if emoji := theme.DefaultEmojiFont(); emoji != nil {
				faces.addFace(loadMeasureFont(emoji))
			}
			addSymbolsFallbackFaces(faces)

			val = &FontCacheItem{Fonts: faces}
			val.lastAccess.Store(now)
			fontCustomCache.Store(source, val)
		} else {
			val.lastAccess.Store(now)
		}
		return val
	}

	scope := ""
	if o != nil { // for overridden themes get the cache key right
		scope = cache.WidgetScopeID(o)
	}

	resolvedWeight := style.ResolvedWeight()
	key := cacheID{
		bold: style.Bold, italic: style.Italic,
		monospace: style.Monospace, symbol: style.Symbol,
		weight: resolvedWeight, scope: scope,
	}
	val, ok := fontCache.Load(key)
	if !ok {
		var faces *dynamicFontMap

		th := theme.CurrentForWidget(o)
		// Build a clean style for theme font lookup (themes only use Bold/Italic/Monospace/Symbol).
		fontStyle := bellui.TextStyle{
			Bold:      style.Bold || resolvedWeight >= bellui.WeightSemiBold,
			Italic:    style.Italic,
			Monospace: style.Monospace,
			Symbol:    style.Symbol,
		}
		font1 := th.Font(fontStyle)

		isBold := fontStyle.Bold
		emoji := theme.DefaultEmojiFont()
		switch {
		case style.Monospace:
			faces = lookupFaces(font1, theme.DefaultTextMonospaceFont(), emoji, fontscan.Monospace, style)
		case isBold:
			if style.Italic {
				faces = lookupFaces(font1, theme.DefaultTextBoldItalicFont(), emoji, fontscan.SansSerif, style)
			} else {
				faces = lookupFaces(font1, theme.DefaultTextBoldFont(), emoji, fontscan.SansSerif, style)
			}
		case style.Italic:
			faces = lookupFaces(font1, theme.DefaultTextItalicFont(), emoji, fontscan.SansSerif, style)
		case style.Symbol:
			th := theme.SymbolFont()
			fallback := theme.DefaultSymbolFont()
			f1 := loadMeasureFont(th)

			if th == fallback {
				faces = &dynamicFontMap{family: fontscan.SansSerif, faces: []*font.Face{f1}}
			} else {
				f2 := loadMeasureFont(fallback)
				faces = &dynamicFontMap{family: fontscan.SansSerif, faces: []*font.Face{f1, f2}}
			}
			// Symbol faces carry no regular characters (not even space); fall
			// back to the bundled text font so mixed symbol/text strings keep
			// host-independent metrics.
			faces.addFace(loadMeasureFont(theme.DefaultTextFont()))
			addSymbolsFallbackFaces(faces)
		default:
			faces = lookupFaces(font1, theme.DefaultTextFont(), emoji, fontscan.SansSerif, style)
		}

		val = &FontCacheItem{Fonts: faces}
		val.lastAccess.Store(now)
		fontCache.Store(key, val)
	} else {
		val.lastAccess.Store(now)
	}

	return val
}

// ClearFontCache is used to remove cached fonts in the case that we wish to re-load Font faces
func ClearFontCache() {
	fontCache.Clear()
	fontCustomCache.Clear()
}

// SweepFontCache removes font cache entries that have not been accessed for the given duration.
// This is called periodically from the cache clean cycle to bound memory usage.
func SweepFontCache(expiry time.Duration) {
	cutoff := time.Now().Add(-expiry).Unix()
	fontCache.Range(func(key cacheID, val *FontCacheItem) bool {
		if val.lastAccess.Load() < cutoff {
			fontCache.Delete(key)
		}
		return true
	})
	fontCustomCache.Range(func(key bellui.Resource, val *FontCacheItem) bool {
		if val.lastAccess.Load() < cutoff {
			fontCustomCache.Delete(key)
		}
		return true
	})
}

// DrawString draws a string into an image.
func DrawString(dst draw.Image, s string, clr color.Color, f shaping.Fontmap, fontSize, scale float32, style bellui.TextStyle) {
	r := render.Renderer{
		FontSize: fontSize,
		PixScale: scale,
		Color:    clr,
	}

	advance := float32(0)
	y := math.MinInt
	var lastRun shaping.Output
	walkString(f, s, float32ToFixed266(fontSize), style, &advance, scale, func(run shaping.Output, x float32) {
		if y == math.MinInt {
			y = int(math.Ceil(float64(fixed266ToFloat32(run.LineBounds.Ascent) * r.PixScale)))
		}
		lastRun = run
		if len(run.Glyphs) == 1 {
			if run.Glyphs[0].GlyphID == 0 {
				r.DrawStringAt(string([]rune{0xfffd}), dst, int(x), y, f.ResolveFace(0xfffd))
				return
			}
		}

		r.DrawShapedRunAt(run, dst, int(x), y)
	})

	if style.Underline && y != math.MinInt {
		drawUnderline(dst, clr, y, lastRun.LineBounds, 0, int(advance), scale)
	}
	if style.Strikethrough && y != math.MinInt {
		drawStrikethrough(dst, clr, y, lastRun.LineBounds, 0, int(advance), scale)
	}
}

// drawUnderline draws a horizontal line below the text baseline, spanning the
// glyph advance from x0 to x1.
func drawUnderline(dst draw.Image, clr color.Color, baseline int, bounds shaping.Bounds, x0, x1 int, scale float32) {
	// Position the underline slightly below the baseline (2/5 of descent).
	descent := fixed266ToFloat32(-bounds.Descent) * scale
	ulY := baseline + int(math.Ceil(float64(descent*0.4)))
	drawDecorationLine(dst, clr, ulY, x0, x1, scale)
}

// drawStrikethrough draws a horizontal line through the middle of the text,
// spanning the glyph advance from x0 to x1.
func drawStrikethrough(dst draw.Image, clr color.Color, baseline int, bounds shaping.Bounds, x0, x1 int, scale float32) {
	// Position the line near the middle of the x-height, roughly a third of
	// the ascent above the baseline.
	ascent := fixed266ToFloat32(bounds.Ascent) * scale
	stY := baseline - int(math.Round(float64(ascent*0.3)))
	drawDecorationLine(dst, clr, stY, x0, x1, scale)
}

// drawDecorationLine fills a horizontal line (underline/strikethrough) of a
// scale-derived thickness across [x0, x1) at the given y.
func drawDecorationLine(dst draw.Image, clr color.Color, y, x0, x1 int, scale float32) {
	if x1 <= x0 {
		return
	}
	thickness := int(math.Max(1, math.Ceil(float64(scale))))
	src := image.NewUniform(clr)
	for dy := 0; dy < thickness; dy++ {
		draw.Draw(dst, image.Rect(x0, y+dy, x1, y+dy+1), src, image.Point{}, draw.Over)
	}
}

func loadMeasureFont(data bellui.Resource) *font.Face {
	// Try single font first, then collection (TTC).
	loaded, err := font.ParseTTF(bytes.NewReader(data.Content()))
	if err != nil {
		faces, err2 := font.ParseTTC(bytes.NewReader(data.Content()))
		if err2 != nil || len(faces) == 0 {
			bellui.LogError("font load error", err)
			return nil
		}
		return faces[0]
	}

	return loaded
}

// MeasureString returns how far dot would advance by drawing s with f.
// Tabs are translated into a dot location change.
func MeasureString(f shaping.Fontmap, s string, textSize float32, style bellui.TextStyle) (size geo.Size, advance float32) {
	return walkString(f, s, float32ToFixed266(textSize), style, &advance, 1, func(shaping.Output, float32) {})
}

// MeasureStyledRuns measures the combined size of multiple styled text runs.
// It returns the total size (sum of widths, max height) and per-run advances.
func MeasureStyledRuns(runs []canvas.TextRun, defaultSize float32) (size geo.Size, advances []float32) {
	advances = make([]float32, len(runs))
	totalWidth := float32(0)
	maxHeight := float32(0)

	for i, run := range runs {
		if run.Text == "" {
			continue
		}
		sz := run.TextSize
		if sz == 0 {
			sz = defaultSize
		}
		face := CachedFontFace(run.TextStyle, run.FontSource, nil)
		s, adv := MeasureString(face.Fonts, run.Text, sz, run.TextStyle)
		advances[i] = adv
		totalWidth += adv
		if s.Height > maxHeight {
			maxHeight = s.Height
		}
	}
	return geo.NewSize(totalWidth, maxHeight), advances
}

// DrawStyledRuns draws multiple styled text runs into a single image.
// Each run is rendered at successive x positions with its own color, size,
// style, and font. The runs share a common baseline derived from the
// tallest run's ascent.
func DrawStyledRuns(dst draw.Image, runs []canvas.TextRun, defaultColor color.Color,
	defaultSize float32, scale float32,
) {
	// First pass: find max ascent across all runs for shared baseline.
	maxAscent := float32(0)
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		sz := run.TextSize
		if sz == 0 {
			sz = defaultSize
		}
		face := CachedFontFace(run.TextStyle, run.FontSource, nil)
		// Shape a single space to get line metrics for this font/size.
		in := shaping.Input{
			Text:      []rune{' '},
			RunStart:  0,
			RunEnd:    1,
			Direction: resolveBaseDirection(run.TextStyle),
			Face:      face.Fonts.ResolveFace(' '),
			Size:      float32ToFixed266(sz),
		}
		shaper := &shaping.HarfbuzzShaper{}
		out := shaper.Shape(in)
		ascent := fixed266ToFloat32(out.LineBounds.Ascent) * scale
		if ascent > maxAscent {
			maxAscent = ascent
		}
	}

	// Second pass: draw groups of adjacent runs at the shared baseline, in visual
	// (bidi) order so that runs spanning both directions are placed correctly
	// rather than in raw array order. Adjacent runs that share a face (size,
	// style, font) and may contain RTL text are shaped together so cursive
	// joining is preserved across the run boundary (pre-rebrand issue #26); each shaped glyph
	// is then drawn in its originating run's colour. Runs that differ in face, or
	// pure-LTR runs, keep the legacy one-run-at-a-time shaping.
	groups := groupStyledRunsForDraw(runs, defaultColor, defaultSize)
	groupRuns := make([]canvas.TextRun, len(groups))
	for i := range groups {
		groupRuns[i] = canvas.TextRun{
			Text:       string(groups[i].runes),
			TextSize:   groups[i].size,
			TextStyle:  groups[i].style,
			FontSource: groups[i].font,
		}
	}

	x := float32(0)
	y := int(math.Ceil(float64(maxAscent)))
	for _, gi := range styledRunVisualOrder(groupRuns) {
		g := groups[gi]
		face := CachedFontFace(g.style, g.font, nil)
		r := render.Renderer{FontSize: g.size, PixScale: scale}

		advance := float32(0)
		var lastRun shaping.Output
		walkString(face.Fonts, string(g.runes), float32ToFixed266(g.size), g.style, &advance, scale,
			func(out shaping.Output, runX float32) {
				lastRun = out
				drawColoredOutput(&r, out, dst, int(x+runX), y, g.colors, defaultColor, face.Fonts)
			})

		if g.style.Underline && y > 0 {
			drawUnderline(dst, g.colors[0], y, lastRun.LineBounds, int(x), int(x+advance), scale)
		}
		if g.style.Strikethrough && y > 0 {
			drawStrikethrough(dst, g.colors[0], y, lastRun.LineBounds, int(x), int(x+advance), scale)
		}
		x += advance
	}
}

// styledDrawGroup is a maximal span of adjacent canvas.TextRuns that share a
// face (size, style, font) and may join cursively, concatenated for single-pass
// shaping. colors holds the resolved draw colour of each rune so the joined
// glyphs can still be drawn in their originating run's colour. See pre-rebrand issue #26.
type styledDrawGroup struct {
	runes  []rune
	colors []color.Color
	size   float32
	style  bellui.TextStyle
	font   bellui.Resource
}

// groupStyledRunsForDraw partitions runs (dropping empties) into draw groups.
// Two adjacent runs are merged only when they share size, style and font AND
// both may contain RTL text — the cursive-joining case. Pure-LTR neighbours and
// face changes start a new group, so the common case keeps the exact legacy
// one-run-per-group shaping. Each rune carries its run's resolved draw colour.
func groupStyledRunsForDraw(runs []canvas.TextRun, defaultColor color.Color, defaultSize float32) []styledDrawGroup {
	var groups []styledDrawGroup
	for i := range runs {
		if runs[i].Text == "" {
			continue
		}
		sz := runs[i].TextSize
		if sz == 0 {
			sz = defaultSize
		}
		clr := runs[i].Color
		if clr == nil {
			clr = defaultColor
		}
		rs := []rune(runs[i].Text)
		if n := len(groups); n > 0 {
			g := &groups[n-1]
			if g.size == sz && g.style == runs[i].TextStyle && g.font == runs[i].FontSource &&
				MayContainRTL(g.runes) && MayContainRTL(rs) {
				g.runes = append(g.runes, rs...)
				for range rs {
					g.colors = append(g.colors, clr)
				}
				continue
			}
		}
		colors := make([]color.Color, len(rs))
		for k := range colors {
			colors[k] = clr
		}
		groups = append(groups, styledDrawGroup{
			runes: rs, colors: colors, size: sz, style: runs[i].TextStyle, font: runs[i].FontSource,
		})
	}
	return groups
}

// drawColoredOutput draws one shaped run, splitting it into maximal slices of
// glyphs that share a draw colour (looked up per glyph by its source rune) so a
// joined run assembled from several differently-coloured TextRuns renders each
// part in its own colour. The slices are drawn left to right, each starting
// where the previous ended, so spacing matches drawing the run as a whole. A
// lone missing glyph is drawn as the replacement character, as before.
func drawColoredOutput(r *render.Renderer, out shaping.Output, dst draw.Image, startX, y int, colors []color.Color, defaultColor color.Color, faces shaping.Fontmap) {
	glyphs := out.Glyphs
	if len(glyphs) == 0 {
		return
	}
	colorAt := func(g shaping.Glyph) color.Color {
		if idx := g.TextIndex(); idx >= 0 && idx < len(colors) && colors[idx] != nil {
			return colors[idx]
		}
		return defaultColor
	}
	if len(glyphs) == 1 && glyphs[0].GlyphID == 0 {
		r.Color = colorAt(glyphs[0])
		r.DrawStringAt(string([]rune{0xfffd}), dst, startX, y, faces.ResolveFace(0xfffd))
		return
	}
	x := startX
	for i := 0; i < len(glyphs); {
		clr := colorAt(glyphs[i])
		j := i + 1
		for j < len(glyphs) && colorAt(glyphs[j]) == clr {
			j++
		}
		slice := out
		slice.Glyphs = glyphs[i:j]
		r.Color = clr
		x = r.DrawShapedRunAt(slice, dst, x, y)
		i = j
	}
}

// styledRunVisualOrder returns the indices of the non-empty runs in the visual
// (left-to-right) order they should be drawn, accounting for bidi reordering of
// runs that span both directions. For pure-LTR content (or fewer than two runs)
// it returns the runs in their original logical order.
//
// Ordering is computed by running the bidi segmenter once over the concatenated
// run text and ranking each run by the lowest visual position of its runes. This
// is exact for single-direction runs (the normal case). A run that is itself
// internally mixed is ordered atomically by its leftmost rune. Cross-run cursive
// joining is handled before this by grouping adjacent same-face runs (see
// groupStyledRunsForDraw / pre-rebrand issue #26); this function only orders the resulting
// groups, so a group's runes are already shaped together.
func styledRunVisualOrder(runs []canvas.TextRun) []int {
	idx := make([]int, 0, len(runs))
	for i := range runs {
		if runs[i].Text != "" {
			idx = append(idx, i)
		}
	}
	if len(idx) < 2 {
		return idx
	}

	var all []rune
	starts := make([]int, len(idx))
	ends := make([]int, len(idx))
	for k, i := range idx {
		starts[k] = len(all)
		all = append(all, []rune(runs[i].Text)...)
		ends[k] = len(all)
	}

	first := runs[idx[0]]
	visualSeq, anyRTL := lineVisualSeq(all, first.TextStyle, first.FontSource)
	if !anyRTL {
		return idx // logical order already equals visual order
	}

	key := make([]int, len(idx))
	for k := range idx {
		min := visualSeq[starts[k]]
		for r := starts[k] + 1; r < ends[k]; r++ {
			if visualSeq[r] < min {
				min = visualSeq[r]
			}
		}
		key[k] = min
	}
	order := make([]int, len(idx))
	for k := range order {
		order[k] = k
	}
	sort.SliceStable(order, func(a, b int) bool { return key[order[a]] < key[order[b]] })

	result := make([]int, len(idx))
	for k, p := range order {
		result[k] = idx[p]
	}
	return result
}

// RenderedTextSize looks up how big a string would be if drawn on screen.
// It also returns the distance from top to the text baseline.
func RenderedTextSize(text string, fontSize float32, style bellui.TextStyle, source bellui.Resource) (size geo.Size, baseline float32) {
	size, base := cache.GetFontMetrics(text, fontSize, style, source)
	if base != 0 {
		return size, base
	}

	size, base = measureText(text, fontSize, style, source)
	cache.SetFontMetrics(text, fontSize, style, source, size, base)
	return size, base
}

func fixed266ToFloat32(i fixed.Int26_6) float32 {
	return float32(float64(i) / (1 << 6))
}

func float32ToFixed266(f float32) fixed.Int26_6 {
	return fixed.Int26_6(float64(f) * (1 << 6))
}

func measureText(text string, fontSize float32, style bellui.TextStyle, source bellui.Resource) (geo.Size, float32) {
	face := CachedFontFace(style, source, nil)
	return MeasureString(face.Fonts, text, fontSize, style)
}

func tabStop(spacew, x float32, tabWidth int) float32 {
	if tabWidth <= 0 {
		tabWidth = DefaultTabWidth
	}

	tabw := spacew * float32(tabWidth)
	tabs, _ := math.Modf(float64((x + tabw) / tabw))
	return tabw * float32(tabs)
}

// resolveBaseDirection maps a bellui.TextStyle's Direction to the bidi paragraph
// base direction used by the typesetting Segmenter. TextDirectionAuto derives
// the direction from the active locale; explicit LTR/RTL override the locale.
//
// This only sets the base (paragraph) direction. Per-run direction, script and
// language are still detected from the text content by the Segmenter, so Arabic
// joining and Hebrew glyph forms work regardless of this value.
func resolveBaseDirection(style bellui.TextStyle) di.Direction {
	switch style.Direction {
	case bellui.TextDirectionLTR:
		return di.DirectionLTR
	case bellui.TextDirectionRTL:
		return di.DirectionRTL
	default: // DirectionAuto follows the ambient UI direction (override + locale)
		if lang.IsRTLDirection() {
			return di.DirectionRTL
		}
		return di.DirectionLTR
	}
}

// resolveLanguage maps a bellui.TextStyle's Language to the language tag fed to
// HarfBuzz for script-specific OpenType feature selection. An empty Language
// (the zero value) follows the active locale; a non-empty value overrides it,
// e.g. "sr-Cyrl" to force Serbian Cyrillic glyph forms regardless of the app
// locale.
//
// The locale fallback uses lang.CurrentLocale (the override-aware active locale),
// the same source resolveBaseDirection consults via lang.IsRTLDirection, so a
// locale override drives shaping language and base direction together.
//
// Like resolveBaseDirection, this only influences shaping of the text content
// (contextual alternates, localized forms); it never changes which font face is
// selected or the bidi ordering of runs.
func resolveLanguage(style bellui.TextStyle) language.Language {
	if style.Language != "" {
		return language.NewLanguage(style.Language)
	}
	return language.NewLanguage(lang.CurrentLocale().LanguageString())
}

func walkString(faces shaping.Fontmap, s string, textSize fixed.Int26_6, style bellui.TextStyle, advance *float32, scale float32,
	cb func(run shaping.Output, x float32),
) (size geo.Size, base float32) {
	s = strings.ReplaceAll(s, "\r", "")
	// walkString shapes a single visual line; multi-line widgets split on '\n'
	// before calling it (see splitLines). Any '\n' that reaches here belongs to a
	// single-line primitive (canvas.Text, TextOnPath, StyledText), so collapse it
	// to a space: it stays on one line and keeps a sensible advance. Upstream
	// typesetting pre-rebrand tracker #257 stopped mapping such control chars to .notdef, so without
	// this they would otherwise shape to near-zero width.
	s = strings.ReplaceAll(s, "\n", " ")

	runes := []rune(s)
	in := shaping.Input{
		Text:      []rune{' '},
		RunStart:  0,
		RunEnd:    1,
		Direction: resolveBaseDirection(style),
		Language:  resolveLanguage(style),
		Face:      faces.ResolveFace(' '),
		Size:      textSize,
	}
	shaper := &shaping.HarfbuzzShaper{}
	segmenter := &shaping.Segmenter{}
	out := shaper.Shape(in)

	in.Text = runes
	in.RunStart = 0
	in.RunEnd = len(runes)

	x := float32(0)
	spacew := scale * fontTabSpaceSize
	if style.Monospace {
		spacew = scale * fixed266ToFloat32(out.Advance)
	}
	letterSpacing := style.ResolvedLetterSpacing()
	if style.Monospace {
		letterSpacing = 0
	}

	// Line metrics start from the primary face (via the shaped space) as a floor,
	// then grow to cover every run on the line. Fallback faces — notably the
	// bundled CJK font (pre-rebrand tracker #314) — have taller ascent/descent than the Latin default;
	// measuring the line box from the primary face alone left it too short, so CJK
	// glyphs clipped at the bottom of an Entry and mixed-script baselines shifted.
	// Union each run's LineBounds so the box fits the tallest glyph on the line,
	// mirroring the shared-baseline max-ascent pass in DrawStyledRuns. Latin-only
	// content shapes in the same face as the space, so this is a no-op there.
	lineBounds := out.LineBounds

	ins := segmenter.Split(in, faces)
	for _, in := range ins {
		inEnd := in.RunEnd

		pending := false
		for i, r := range in.Text[in.RunStart:in.RunEnd] {
			if r == '\t' {
				if pending {
					in.RunEnd = i
					var b shaping.Bounds
					x, b = shapeCallback(shaper, in, x, scale, letterSpacing, cb)
					lineBounds = unionLineBounds(lineBounds, b)
				}
				x = tabStop(spacew, x, style.TabWidth)

				in.RunStart = i + 1
				in.RunEnd = inEnd
				pending = false
			} else {
				pending = true
			}
		}

		var b shaping.Bounds
		x, b = shapeCallback(shaper, in, x, scale, letterSpacing, cb)
		lineBounds = unionLineBounds(lineBounds, b)
	}

	*advance = x

	lineHeight := fixed266ToFloat32(lineBounds.LineThickness())
	if lhs := style.ResolvedLineHeightScale(); lhs != 1.0 {
		lineHeight *= lhs
	}
	return geo.NewSize(*advance, lineHeight),
		fixed266ToFloat32(lineBounds.Ascent)
}

// unionLineBounds returns the tightest line bounds that contain both a and b:
// the larger ascent, the deeper (more negative) descent and the larger gap. It
// grows a single line's metrics to fit every font face shaped onto it.
func unionLineBounds(a, b shaping.Bounds) shaping.Bounds {
	if b.Ascent > a.Ascent {
		a.Ascent = b.Ascent
	}
	if b.Descent < a.Descent {
		a.Descent = b.Descent
	}
	if b.Gap > a.Gap {
		a.Gap = b.Gap
	}
	return a
}

func shapeCallback(shaper shaping.Shaper, in shaping.Input, x, scale, letterSpacing float32, cb func(shaping.Output, float32)) (float32, shaping.Bounds) {
	out := shaper.Shape(in)
	bounds := out.LineBounds
	glyphs := out.Glyphs
	start := 0
	pending := false
	adv := fixed.I(0)

	// Count total glyphs for letter spacing calculation.
	totalGlyphs := len(glyphs)
	glyphIndex := 0

	for i, g := range out.Glyphs {
		if g.GlyphID == 0 {
			if pending {
				out.Glyphs = glyphs[start:i]
				cb(out, x)
				x += fixed266ToFloat32(adv) * scale
				adv = 0
			}

			out.Glyphs = glyphs[i : i+1]
			cb(out, x)
			x += fixed266ToFloat32(glyphs[i].Advance) * scale
			adv = 0

			start = i + 1
			pending = false
		} else {
			pending = true
		}
		adv += g.Advance
		glyphIndex++
		if letterSpacing != 0 && glyphIndex < totalGlyphs {
			x += letterSpacing * scale
		}
	}

	if pending {
		out.Glyphs = glyphs[start:]
		cb(out, x)
		x += fixed266ToFloat32(adv) * scale
		adv = 0
	}
	return x + fixed266ToFloat32(adv)*scale, bounds
}

// LineLayout is the visual layout of one logical line of a single text style.
// It maps logical rune offsets to visual x positions, accounting for the bidi
// reordering of mixed LTR/RTL runs within the line. It is the shared primitive
// behind widget-level bidi: caret placement, click hit-testing, selection
// rectangles and visual arrow navigation.
//
// Positions are in unscaled (device-independent) coordinates, matching
// MeasureString, so callers that already work in DP can use them directly.
type LineLayout struct {
	// Carets has len(text)+1 entries. Carets[i] is the visual x of the leading
	// edge of logical rune i; Carets[len] is the trailing edge of the line. For
	// RTL or mixed-direction text the sequence is not monotonic. Boundary carets
	// carry forward affinity, so they are insertion points, not glyph edges —
	// use extents for glyph geometry (e.g. selection rectangles).
	Carets []float32
	// extents has one entry per logical rune: the unambiguous visual [left,right]
	// of that rune's glyph cluster. Unlike consecutive carets, these never bridge
	// across a direction boundary.
	extents [][2]float32
	// caretsBwd mirrors Carets but holds the upstream (backward-affinity) x of each
	// offset: caretsBwd[i] is the trailing edge of logical rune i-1. It equals
	// Carets[i] everywhere except at a direction boundary, where the two affinities
	// resolve to opposite sides. nil for the degenerate empty line. See CaretXAt.
	caretsBwd []float32
	// Width is the total visual advance of the line.
	Width float32
	// rtl reports whether any run in the line is right-to-left. Pure-LTR lines
	// can take a fast path that skips bidi-aware logic (and preserves the exact
	// legacy measurement path, including tab expansion).
	rtl bool
	// boxes holds the opaque inline boxes (non-text segments) interleaved into the
	// line's visual order; nil for a line of pure text. SelectionSpans uses them
	// to fill a box that falls inside a selection (pre-rebrand issue #41).
	boxes []boxExtent
}

// HasRTL reports whether the line contains any right-to-left run. When false,
// logical order equals visual order and Carets is monotonically increasing.
func (l LineLayout) HasRTL() bool { return l.rtl }

// CaretX returns the visual x of the leading edge of logical rune offset, using
// forward affinity at direction boundaries. Out-of-range offsets are clamped.
func (l LineLayout) CaretX(logical int) float32 {
	if logical < 0 {
		logical = 0
	}
	if logical >= len(l.Carets) {
		logical = len(l.Carets) - 1
	}
	return l.Carets[logical]
}

// CaretAffinity disambiguates the two visual positions a single logical caret
// offset can have at an LTR↔RTL direction boundary, where one offset maps to two
// valid x positions. Away from a boundary both resolve to the same x.
type CaretAffinity bool

const (
	// AffinityDownstream attaches the caret to the character that logically
	// follows it (its leading edge). This is the zero value and matches CaretX.
	AffinityDownstream CaretAffinity = false
	// AffinityUpstream attaches the caret to the character that logically
	// precedes it (the trailing edge of offset-1) — e.g. so the caret sticks to a
	// just-typed character rather than jumping across a direction boundary.
	AffinityUpstream CaretAffinity = true
)

// CaretXAt returns the visual x of logical offset under the given affinity. With
// AffinityDownstream it is identical to CaretX (forward affinity). With
// AffinityUpstream it returns the trailing edge of the preceding rune, which
// differs from the downstream x only when offset sits exactly at a direction
// boundary. Out-of-range offsets are clamped.
func (l LineLayout) CaretXAt(logical int, aff CaretAffinity) float32 {
	if logical < 0 {
		logical = 0
	}
	if logical >= len(l.Carets) {
		logical = len(l.Carets) - 1
	}
	if aff == AffinityUpstream && logical > 0 && l.caretsBwd != nil {
		return l.caretsBwd[logical]
	}
	return l.Carets[logical]
}

// HitTest maps a visual x to the nearest logical rune offset, i.e. the insertion
// point a click at x should select. Because Carets is non-monotonic for mixed
// text, this picks the logical offset whose visual caret is closest to x.
func (l LineLayout) HitTest(x float32) int {
	best := 0
	bestDist := float32(math.MaxFloat32)
	for i, cx := range l.Carets {
		d := cx - x
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

// HitTestAffinity is HitTest extended with affinity: at a direction boundary the
// same logical offset has a downstream and an upstream visual position, and a
// click should resolve to whichever side it landed on. It returns the logical
// offset and the affinity of the nearest caret position, considering both the
// downstream (Carets) and upstream (caretsBwd) candidate of every offset.
func (l LineLayout) HitTestAffinity(x float32) (int, CaretAffinity) {
	bestOff := 0
	bestAff := AffinityDownstream
	bestDist := float32(math.MaxFloat32)
	consider := func(off int, aff CaretAffinity, cx float32) {
		d := cx - x
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist, bestOff, bestAff = d, off, aff
		}
	}
	for i, cx := range l.Carets {
		consider(i, AffinityDownstream, cx)
	}
	if l.caretsBwd != nil {
		for i := 1; i < len(l.Carets); i++ {
			// Only the upstream candidate that differs (a real boundary) adds a stop.
			if l.caretsBwd[i] != l.Carets[i] {
				consider(i, AffinityUpstream, l.caretsBwd[i])
			}
		}
	}
	return bestOff, bestAff
}

// SelectionSpans returns the disjoint visual x-intervals [x1,x2] that cover the
// logical column range [start,end). In LTR text this is a single interval; in
// RTL/mixed text a logically-contiguous selection can map to several disjoint
// visual rectangles, which this merges and returns left-to-right.
//
// Opaque boxes ride their owning rune: a box is filled when the text rune it
// logically trails lies in the selection, exactly as the legacy left-to-right
// single-rectangle path folds a non-text segment's width into the run before
// it. Callers whose selection model gives an atom its own selectable position
// should use SelectionSpansBoxes instead.
func (l LineLayout) SelectionSpans(start, end int) [][2]float32 {
	return l.selectionSpans(start, end, func(b boxExtent) bool {
		return b.owner >= start && b.owner < end
	})
}

// SelectionSpansBoxes is SelectionSpans with the box-inclusion decision
// supplied by the caller: boxSelected is asked once per opaque box, indexed in
// logical (run-supply) order. It serves selection models where an inline atom
// occupies its own selectable position (e.g. RichEntry's document space, pre-rebrand tracker #913)
// rather than riding its preceding rune — including an atom-only selection
// with an empty text range.
func (l LineLayout) SelectionSpansBoxes(start, end int, boxSelected func(int) bool) [][2]float32 {
	return l.selectionSpans(start, end, func(b boxExtent) bool {
		return boxSelected(b.logical)
	})
}

func (l LineLayout) selectionSpans(start, end int, includeBox func(boxExtent) bool) [][2]float32 {
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if max := len(l.Carets) - 1; end > max {
		end = max
	}
	if end > len(l.extents) || start > end {
		return nil
	}

	ivals := make([][2]float32, 0, end-start+len(l.boxes))
	for i := start; i < end; i++ {
		ivals = append(ivals, l.extents[i])
	}
	// Opaque boxes carry no rune extent; include each the caller selects so it
	// is filled.
	for _, b := range l.boxes {
		if includeBox(b) {
			ivals = append(ivals, [2]float32{b.left, b.right})
		}
	}
	if len(ivals) == 0 {
		return nil
	}
	sort.Slice(ivals, func(i, j int) bool { return ivals[i][0] < ivals[j][0] })

	merged := make([][2]float32, 0, len(ivals))
	cur := ivals[0]
	for _, iv := range ivals[1:] {
		if iv[0] <= cur[1]+0.5 { // overlapping or touching (sub-pixel gap)
			if iv[1] > cur[1] {
				cur[1] = iv[1]
			}
		} else {
			merged = append(merged, cur)
			cur = iv
		}
	}
	return append(merged, cur)
}

// BoxCount returns the number of opaque inline boxes in the line.
func (l LineLayout) BoxCount() int { return len(l.boxes) }

// BoxCaretX returns the visual caret x for the insertion point logically
// before (after=false) or after (after=true) the line's box-th opaque box, in
// logical (run-supply) order. The "before" side is the edge facing the
// logically-preceding content — the right edge when the bidi flow around the
// box runs right-to-left (pre-rebrand tracker #913). Out-of-range indices clamp to the line edges.
func (l LineLayout) BoxCaretX(box int, after bool) float32 {
	for _, b := range l.boxes {
		if b.logical != box {
			continue
		}
		// before-side = right when beforeRight, else left; after = the opposite.
		if after == b.beforeRight {
			return b.left
		}
		return b.right
	}
	if len(l.Carets) > 0 {
		return l.Carets[len(l.Carets)-1]
	}
	return 0
}

// HitTestBoxes maps a visual x to the nearest insertion point, considering both
// text carets and the edges of opaque inline boxes (pre-rebrand tracker #983) — the box-aware
// inverse of BoxCaretX, extending HitTestAffinity so a click on or beside an
// inline atom resolves to the atom's own boundary rather than snapping to the
// nearest text caret (which HitTestAffinity does, being box-blind).
//
// When the result is a box edge, isBox is true with the box's logical
// (run-supply) index and which side (after) the insertion point sits — a click
// inside a box resolves to the nearer edge, before/after following the bidi
// flow around the box (before = the edge facing logically-preceding content).
// Otherwise isBox is false and off/aff are HitTestAffinity's text hit.
func (l LineLayout) HitTestBoxes(x float32) (off int, aff CaretAffinity, box int, after bool, isBox bool) {
	// A click strictly inside a box's visual extent resolves to that box; the
	// nearer edge decides the side (the edges may tie with adjacent text carets
	// in LTR flow, where a box abuts the surrounding runs, so this wins first).
	for _, b := range l.boxes {
		if x > b.left && x < b.right {
			beforeIsLeft := !b.beforeRight
			leftClose := x < (b.left+b.right)/2
			return 0, AffinityDownstream, b.logical, leftClose != beforeIsLeft, true
		}
	}

	bestDist := float32(math.MaxFloat32)
	consider := func(o int, a CaretAffinity, cx float32) {
		d := cx - x
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist, off, aff, isBox = d, o, a, false
		}
	}
	considerBox(&bestDist, x, l.boxes, func(bi int, aft bool) {
		box, after, isBox = bi, aft, true
	})
	for i, cx := range l.Carets {
		consider(i, AffinityDownstream, cx)
	}
	if l.caretsBwd != nil {
		for i := 1; i < len(l.Carets); i++ {
			if l.caretsBwd[i] != l.Carets[i] {
				consider(i, AffinityUpstream, l.caretsBwd[i])
			}
		}
	}
	return
}

// considerBox folds every box edge into the running nearest-hit search: for each
// box it tests the before- and after-side edge distance against bestDist and, on
// a win, records the box index and side via set. It is the box half of
// HitTestBoxes' nearest scan (text carets are the other half).
func considerBox(bestDist *float32, x float32, boxes []boxExtent, set func(box int, after bool)) {
	try := func(cx float32, boxIdx int, after bool) {
		d := cx - x
		if d < 0 {
			d = -d
		}
		if d < *bestDist {
			*bestDist = d
			set(boxIdx, after)
		}
	}
	for _, b := range boxes {
		beforeX, afterX := b.left, b.right
		if b.beforeRight {
			beforeX, afterX = b.right, b.left
		}
		try(beforeX, b.logical, false)
		try(afterX, b.logical, true)
	}
}

// VisualNeighbor returns the logical offset of the caret position immediately to
// the visual right (dir > 0) or left (dir < 0) of the given logical offset, for
// visual arrow-key navigation. It returns -1 when there is no caret in that
// direction (the cursor is at the visual edge of the line and the caller should
// cross to an adjacent row).
func (l LineLayout) VisualNeighbor(logical, dir int) int {
	if logical < 0 || logical >= len(l.Carets) {
		return -1
	}
	// Order carets by (visual x, logical offset) so that boundary carets sharing
	// an exact x are still distinct, ordered stops. Stepping moves to the
	// immediately greater (dir > 0) or lesser (dir < 0) caret in that order.
	curX := l.Carets[logical]
	greaterThanCur := func(i int, x float32) bool {
		return x > curX || (x == curX && i > logical)
	}
	best := -1
	var bestX float32
	for i, cx := range l.Carets {
		if i == logical {
			continue
		}
		if dir > 0 {
			if greaterThanCur(i, cx) && (best < 0 || cx < bestX || (cx == bestX && i < best)) {
				best, bestX = i, cx
			}
		} else {
			if !greaterThanCur(i, cx) && (best < 0 || cx > bestX || (cx == bestX && i > best)) {
				best, bestX = i, cx
			}
		}
	}
	return best
}

// MayContainRTL reports whether the runes include any character from a
// right-to-left script (Hebrew, Arabic, Syriac, Thaana, NKo, Samaritan,
// Mandaic, their presentation forms, and assorted historic/maths RTL blocks).
// It is a cheap pre-filter: callers use it to skip the bidi visual-layout path
// for pure-LTR text. False positives (e.g. a stray RTL punctuation mark) merely
// trigger the full layout, which then reports HasRTL accurately.
func MayContainRTL(runes []rune) bool {
	for _, r := range runes {
		switch {
		case r >= 0x0590 && r <= 0x08FF, // Hebrew, Arabic, Syriac, Thaana, NKo, Samaritan, Mandaic, Arabic Extended
			r >= 0xFB1D && r <= 0xFDFF,   // Hebrew & Arabic presentation forms A
			r >= 0xFE70 && r <= 0xFEFF,   // Arabic presentation forms B
			r >= 0x10800 && r <= 0x10FFF, // historic RTL scripts
			r >= 0x1E800 && r <= 0x1EFFF: // Adlam, Arabic mathematical
			return true
		}
	}
	return false
}

// LineVisualLayout shapes a single logical line of one text style and returns
// its visual layout: a logical-rune → visual-x caret table plus the total
// width. It reuses the same Segmenter/HarfBuzz pipeline as walkString, so the
// caret positions agree with how the line is measured and drawn.
//
// The line must not contain newlines (callers split on '\n' first). Tab
// expansion is not applied here; callers should gate caret usage on HasRTL and
// keep the legacy logical-prefix path for pure-LTR text, which handles tabs.
func LineVisualLayout(faces shaping.Fontmap, runes []rune, textSize float32, style bellui.TextStyle) LineLayout {
	carets := make([]float32, len(runes)+1)
	caretsBwd := make([]float32, len(runes)+1)
	extents := make([][2]float32, len(runes))
	if len(runes) == 0 {
		return LineLayout{Carets: carets}
	}

	size := float32ToFixed266(textSize)
	in := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: resolveBaseDirection(style),
		Language:  resolveLanguage(style),
		Face:      faces.ResolveFace(' '),
		Size:      size,
	}
	shaper := &shaping.HarfbuzzShaper{}
	segmenter := &shaping.Segmenter{}

	letterSpacing := style.ResolvedLetterSpacing()
	if style.Monospace {
		letterSpacing = 0
	}

	rtl := false
	x := float32(0)
	// Segmenter.Split returns runs in visual (left-to-right) order; placing each
	// at the running x reproduces the paragraph's bidi visual order.
	for _, run := range segmenter.Split(in, faces) {
		if run.Direction.Progression() == di.TowardTopLeft {
			rtl = true
		}
		out := shaper.Shape(run)
		x = recordRunCarets(out, x, 1, letterSpacing, carets, caretsBwd, extents, 0)
	}

	// The line-end offset has no following rune (downstream) and the start offset
	// no preceding rune (upstream); fold each onto its sole defined position.
	carets[len(carets)-1] = caretsBwd[len(caretsBwd)-1]
	caretsBwd[0] = carets[0]
	return LineLayout{Carets: carets, caretsBwd: caretsBwd, extents: extents, Width: x, rtl: rtl}
}

// StyledRun is one contiguous logical span of runes sharing a single text style,
// the unit StyledLineVisualLayout lays out. Each run carries its own size, style
// and font, so a line assembled from several differently-styled spans (e.g. the
// TextSegments of one RichText row) shapes each span correctly while a single
// bidi resolution orders the whole line.
type StyledRun struct {
	Runes      []rune
	TextSize   float32
	Style      bellui.TextStyle
	FontSource bellui.Resource

	// Box marks this run as an opaque inline box (a non-text RichText segment such
	// as a HyperlinkSegment or image) of width BoxWidth. It occupies one slot in
	// the line's bidi visual order but contributes no caret columns; its Runes are
	// ignored. This lets a bidi row interleave non-text segments with RTL text
	// while keeping the text-only column model (pre-rebrand issue #41).
	Box      bool
	BoxWidth float32
}

// objectReplacement (U+FFFC) stands in for an opaque box during the line's single
// bidi pass: it is a neutral (bidi class ON) character, so the box adopts the
// surrounding/base direction's position in the visual order, like a real inline
// object. It never reaches shaping — boxes are placed by measured width.
const objectReplacement = '￼'

// boxExtent records an opaque inline box placed in the line's visual order. owner
// is the logical text rune the box trails: the box is part of any selection that
// includes that rune, mirroring how the legacy left-to-right layout folds a
// non-text segment's width into the run before it. owner == -1 means the box
// leads the line (no preceding text rune) and is never part of a selection.
// left/right are the box's visual edges. logical is the box's index among the
// line's boxes in logical (run-supply) order, and beforeRight reports which
// visual edge faces the logically-preceding content: true when the bidi flow
// around the box runs right-to-left, so an insertion point "before" the box
// sits at its right edge (pre-rebrand tracker #913).
type boxExtent struct {
	owner       int
	left, right float32
	logical     int
	beforeRight bool
}

// joinAdjacentStyledRuns merges neighbouring text runs that share a face (size,
// style, font) and both may contain RTL text into a single run, so the line is
// shaped with cursive joining preserved across the original run boundary (issue
// pre-rebrand tracker #26). Boxes, face changes and pure-LTR neighbours are left as separate runs,
// so the text-only caret model (boxes keep their own slot) and the legacy
// pure-LTR path are unchanged. The concatenation order and total rune count are
// preserved, so the caller's global text-offset indexing is unaffected.
func joinAdjacentStyledRuns(runs []StyledRun) []StyledRun {
	merged := make([]StyledRun, 0, len(runs))
	for _, run := range runs {
		if n := len(merged); n > 0 {
			p := &merged[n-1]
			if !p.Box && !run.Box && p.TextSize == run.TextSize && p.Style == run.Style &&
				p.FontSource == run.FontSource && MayContainRTL(p.Runes) && MayContainRTL(run.Runes) {
				joined := make([]rune, 0, len(p.Runes)+len(run.Runes))
				joined = append(joined, p.Runes...)
				joined = append(joined, run.Runes...)
				p.Runes = joined
				continue
			}
		}
		merged = append(merged, run)
	}
	return merged
}

// StyledLineVisualLayout is the multi-style analogue of LineVisualLayout. It lays
// out one logical line whose runes are partitioned into style runs, resolving the
// bidi visual order across the entire line while shaping each run with its own
// font and size. Runs flagged Box are opaque inline boxes (non-text segments):
// they take a slot in the bidi order and consume width but contribute no caret
// columns. The returned LineLayout's Carets/extents are indexed by global logical
// rune offset over the concatenation of all the non-box runs' Runes.
//
// Adjacent text runs that share a face (size, style, font) and both may contain
// RTL text are merged before shaping (joinAdjacentStyledRuns), so cursive
// joining is preserved across an Arabic word split into two same-font segments
// (pre-rebrand issue #26). Runs differing in face, boxes, and pure-LTR neighbours are still
// shaped separately. The line must not contain newlines, and tabs are not
// expanded — callers gate on HasRTL and keep the legacy path for pure-LTR text,
// exactly as for LineVisualLayout.
func StyledLineVisualLayout(runs []StyledRun) LineLayout {
	runs = joinAdjacentStyledRuns(runs)
	// Two parallel indexings. bidiRunes drives the single bidi ordering pass and
	// carries one placeholder rune per box, so a box gets a position in the visual
	// order. The caret/extent tables stay indexed by text-only logical offset
	// (boxes add width but no column), matching the legacy left-to-right layout.
	bidiOffsets := make([]int, len(runs))
	textOffsets := make([]int, len(runs))
	var bidiRunes []rune
	textCount := 0
	for i := range runs {
		bidiOffsets[i] = len(bidiRunes)
		textOffsets[i] = textCount
		if runs[i].Box {
			bidiRunes = append(bidiRunes, objectReplacement)
			continue
		}
		bidiRunes = append(bidiRunes, runs[i].Runes...)
		textCount += len(runs[i].Runes)
	}
	carets := make([]float32, textCount+1)
	caretsBwd := make([]float32, textCount+1)
	extents := make([][2]float32, textCount)
	if len(bidiRunes) == 0 {
		return LineLayout{Carets: carets}
	}

	// Resolve the base direction (and the ordering pass's font) from the first
	// actual text run; a leading box carries no style of its own.
	baseStyle := runs[0].Style
	baseSource := runs[0].FontSource
	for i := range runs {
		if !runs[i].Box {
			baseStyle, baseSource = runs[i].Style, runs[i].FontSource
			break
		}
	}
	baseDir := resolveBaseDirection(baseStyle)
	visualSeq, anyRTL := lineVisualSeq(bidiRunes, baseStyle, baseSource)

	type fragment struct {
		out            shaping.Output
		base           int
		letterSpacing  float32
		rank           int
		isBox          bool
		boxWidth       float32
		boxOwner       int
		boxLogical     int
		boxBeforeRight bool
	}
	var frags []fragment

	shaper := &shaping.HarfbuzzShaper{}
	segmenter := &shaping.Segmenter{}
	boxLogical := 0
	for i := range runs {
		if runs[i].Box {
			// One atomic fragment ranked by its placeholder's place in the visual
			// order; owner is the text rune it trails (-1 if it leads the line).
			// The side facing the logically-preceding content follows the bidi
			// flow around the placeholder: when a logical neighbour sits on the
			// visually opposite side of what left-to-right order would give, the
			// flow there is right-to-left and "before" is the right edge (pre-rebrand tracker #913).
			p := bidiOffsets[i]
			beforeRight := baseDir.Progression() == di.TowardTopLeft
			if p > 0 {
				beforeRight = visualSeq[p-1] > visualSeq[p]
			} else if p+1 < len(visualSeq) {
				beforeRight = visualSeq[p+1] < visualSeq[p]
			}
			frags = append(frags, fragment{
				isBox:          true,
				boxWidth:       runs[i].BoxWidth,
				boxOwner:       textOffsets[i] - 1,
				boxLogical:     boxLogical,
				boxBeforeRight: beforeRight,
				rank:           visualSeq[p],
			})
			boxLogical++
			continue
		}
		if len(runs[i].Runes) == 0 {
			continue
		}
		style := runs[i].Style
		faces := CachedFontFace(style, runs[i].FontSource, nil).Fonts
		letterSpacing := style.ResolvedLetterSpacing()
		if style.Monospace {
			letterSpacing = 0
		}
		in := shaping.Input{
			Text:      runs[i].Runes,
			RunStart:  0,
			RunEnd:    len(runs[i].Runes),
			Direction: baseDir,
			Language:  resolveLanguage(style),
			Face:      faces.ResolveFace(' '),
			Size:      float32ToFixed266(runs[i].TextSize),
		}
		bidiOff, textOff := bidiOffsets[i], textOffsets[i]
		// Segmenter.Split orders this run's own directional sub-runs visually; each
		// becomes an atomic fragment ranked by its leftmost rune in the whole-line
		// visual order, so fragments from different runs interleave correctly.
		for _, sub := range segmenter.Split(in, faces) {
			out := shaper.Shape(sub)
			rank := visualSeq[bidiOff+sub.RunStart]
			for r := sub.RunStart + 1; r < sub.RunEnd; r++ {
				if visualSeq[bidiOff+r] < rank {
					rank = visualSeq[bidiOff+r]
				}
			}
			frags = append(frags, fragment{out: out, base: textOff, letterSpacing: letterSpacing, rank: rank})
		}
	}

	sort.SliceStable(frags, func(a, b int) bool { return frags[a].rank < frags[b].rank })

	var boxes []boxExtent
	x := float32(0)
	for i := range frags {
		if frags[i].isBox {
			left := x
			x += frags[i].boxWidth
			boxes = append(boxes, boxExtent{
				owner: frags[i].boxOwner, left: left, right: x,
				logical: frags[i].boxLogical, beforeRight: frags[i].boxBeforeRight,
			})
			continue
		}
		x = recordRunCarets(frags[i].out, x, 1, frags[i].letterSpacing, carets, caretsBwd, extents, frags[i].base)
	}
	carets[textCount] = caretsBwd[textCount]
	caretsBwd[0] = carets[0]
	return LineLayout{Carets: carets, caretsBwd: caretsBwd, extents: extents, Width: x, rtl: anyRTL, boxes: boxes}
}

// lineVisualSeq runs a single bidi segmentation over runes and returns, per
// logical rune, its visual rank (0 = leftmost) plus whether any run is RTL. It is
// the shared ordering primitive behind both styledRunVisualOrder (which ranks
// whole runs) and StyledLineVisualLayout (which ranks shaped fragments).
func lineVisualSeq(runes []rune, style bellui.TextStyle, source bellui.Resource) (seq []int, anyRTL bool) {
	seq = make([]int, len(runes))
	if len(runes) == 0 {
		return seq, false
	}
	faces := CachedFontFace(style, source, nil).Fonts
	in := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: resolveBaseDirection(style),
		Language:  resolveLanguage(style),
		Face:      faces.ResolveFace(' '),
		Size:      float32ToFixed266(16), // segmentation is size-independent
	}
	segmenter := &shaping.Segmenter{}
	s := 0
	for _, run := range segmenter.Split(in, faces) {
		if run.Direction.Progression() == di.TowardTopLeft {
			anyRTL = true
			for r := run.RunEnd - 1; r >= run.RunStart; r-- {
				seq[r] = s
				s++
			}
		} else {
			for r := run.RunStart; r < run.RunEnd; r++ {
				seq[r] = s
				s++
			}
		}
	}
	return seq, anyRTL
}

// recordRunCarets walks one shaped run's glyph clusters left-to-right from
// visual x, recording the visual edges of each logical rune into carets. For an
// RTL run the leftmost glyph corresponds to the highest logical rune, so the
// cluster's left edge maps to its trailing rune offset and vice versa.
//
// base is added to each glyph's TextIndex, so a run shaped from a sub-slice of
// the line (StyledLineVisualLayout shapes each style run separately) maps into
// the line's global caret/extent tables. Callers shaping the whole line at once
// pass base 0.
func recordRunCarets(out shaping.Output, x, scale, letterSpacing float32, carets, caretsBwd []float32, extents [][2]float32, base int) float32 {
	rtl := out.Direction.Progression() == di.TowardTopLeft
	glyphs := out.Glyphs
	n := len(glyphs)
	i := 0
	for i < n {
		cluster := glyphs[i].TextIndex()
		runeCount := glyphs[i].RunesCount()
		if runeCount < 1 {
			runeCount = 1
		}
		adv := float32(0)
		j := i
		for j < n && glyphs[j].TextIndex() == cluster {
			adv += fixed266ToFloat32(glyphs[j].Advance) * scale
			j++
			if letterSpacing != 0 && j < n {
				adv += letterSpacing * scale
			}
		}
		left, right := x, x+adv
		// base shifts the cluster into the line's global tables; the grouping
		// comparison above stays on the run-local TextIndex.
		assignCluster(carets, caretsBwd, extents, cluster+base, runeCount, left, right, rtl)
		x = right
		i = j
	}
	return x
}

// assignCluster records the visual edges of a single glyph cluster that spans
// runeCount logical runes starting at cluster. carets receives each rune's
// leading (downstream) edge — the forward-affinity insertion point — and
// caretsBwd receives each rune's trailing (upstream) edge at the next offset, so
// a logical offset at a direction boundary has both its candidate x positions
// available. extents records the unambiguous per-rune glyph [left,right]. Interior
// runes (multi-rune clusters such as ligatures) are interpolated linearly across
// the cluster. The two caret tables coincide everywhere except at a direction
// boundary, where the leading edge of rune i and the trailing edge of rune i-1
// fall on opposite sides (see CaretXAt).
func assignCluster(carets, caretsBwd []float32, extents [][2]float32, cluster, runeCount int, left, right float32, rtl bool) {
	if cluster < 0 || cluster+runeCount >= len(carets) {
		return
	}
	width := right - left
	if rtl {
		// Rune cluster+k is the (k+1)-th from the visual right: its leading (right)
		// edge is the downstream caret, its trailing (left) edge the upstream caret
		// of the following offset.
		for k := 0; k < runeCount; k++ {
			carets[cluster+k] = right - width*float32(k)/float32(runeCount)
			caretsBwd[cluster+k+1] = right - width*float32(k+1)/float32(runeCount)
			extents[cluster+k] = [2]float32{
				right - width*float32(k+1)/float32(runeCount),
				right - width*float32(k)/float32(runeCount),
			}
		}
	} else {
		for k := 0; k < runeCount; k++ {
			carets[cluster+k] = left + width*float32(k)/float32(runeCount)
			caretsBwd[cluster+k+1] = left + width*float32(k+1)/float32(runeCount)
			extents[cluster+k] = [2]float32{
				left + width*float32(k)/float32(runeCount),
				left + width*float32(k+1)/float32(runeCount),
			}
		}
	}
}

// FontCacheItem holds a cached font face set with a last-access timestamp
// for expiry-based eviction.
type FontCacheItem struct {
	Fonts      shaping.Fontmap
	lastAccess atomic.Int64 // unix timestamp (seconds) of last access
}

// cacheID is a comparable cache key for font face lookups.
// Only fields that affect font face selection are included.
type cacheID struct {
	bold      bool
	italic    bool
	monospace bool
	symbol    bool
	weight    bellui.TextWeight
	scope     string
}

var (
	fontCache       async.Map[cacheID, *FontCacheItem]
	fontCustomCache async.Map[bellui.Resource, *FontCacheItem] // for custom resources
)

type noopLogger struct{}

func (n noopLogger) Printf(string, ...any) {}

type dynamicFontMap struct {
	faces       []*font.Face
	family      string
	systemStart int // index of the first host-system face in faces; 0 = none
}

// debugFontEscape, when BELLUI_DEBUG_FONTESCAPE is set, logs every rune whose
// resolution escaped the chain's compiled-in faces — to the host locale face,
// an app-registered fallback or the host system-font scan. The first and last
// are host-dependent: text containing such runes measures differently per OS
// (the pre-rebrand tracker #686 diagnostic).
var debugFontEscape = os.Getenv("BELLUI_DEBUG_FONTESCAPE") != ""

func (d *dynamicFontMap) ResolveFace(r rune) *font.Face {
	for i, f := range d.faces {
		if _, ok := f.NominalGlyph(r); ok {
			if debugFontEscape && d.systemStart > 0 && i >= d.systemStart {
				fmt.Fprintf(os.Stderr, "FONTESCAPE locale-face rune=%U %q family=%s faceIdx=%d\n", r, string(r), d.family, i)
			}
			return f
		}
	}

	// App-registered fallbacks are consulted before the system scan: they are
	// portable (they work where there are no scannable system fonts, e.g. wasm)
	// and give consistent cross-platform glyphs for the app's chosen face.
	if f := resolveFallback(r); f != nil {
		if debugFontEscape {
			fmt.Fprintf(os.Stderr, "FONTESCAPE registered-fallback rune=%U %q family=%s\n", r, string(r), d.family)
		}
		d.addFace(f)
		return f
	}

	toAdd := lookupRuneFont(r, d.family, font.Aspect{})
	if toAdd != nil {
		if debugFontEscape {
			fmt.Fprintf(os.Stderr, "FONTESCAPE system-scan rune=%U %q family=%s\n", r, string(r), d.family)
		}
		d.addFace(toAdd)
		return toAdd
	}

	return d.faces[0]
}

func (d *dynamicFontMap) addFace(f *font.Face) {
	d.faces = append(d.faces, f)
}
