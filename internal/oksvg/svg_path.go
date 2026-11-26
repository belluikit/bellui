// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/internal/rasterx"
	"golang.org/x/image/math/fixed"
)

// SvgPath binds a style to a path.
type SvgPath struct {
	PathStyle
	Path rasterx.Path
}

// Draw the compiled SvgPath into the Dasher.
func (svgp *SvgPath) Draw(r *rasterx.Dasher, opacity float64) {
	svgp.DrawTransformed(r, opacity, rasterx.Identity)
}

// DrawTransformed draws the compiled SvgPath into the Dasher while applying transform t.
func (svgp *SvgPath) DrawTransformed(r *rasterx.Dasher, opacity float64, t rasterx.Matrix2D) {
	m := svgp.mAdder.M
	svgp.mAdder.M = t.Mult(m)
	defer func() { svgp.mAdder.M = m }() // Restore untransformed matrix
	if svgp.fillerColor != nil {
		r.Clear()
		rf := &r.Filler
		rf.SetWinding(svgp.UseNonZeroWinding)
		svgp.mAdder.Adder = rf // This allows transformations to be applied
		svgp.Path.AddTo(&svgp.mAdder)

		switch fillerColor := svgp.fillerColor.(type) {
		case color.Color:
			rf.SetColor(rasterx.ApplyOpacity(fillerColor, svgp.FillOpacity*opacity))
		case rasterx.Gradient:
			if fillerColor.Units == rasterx.ObjectBoundingBox {
				fRect := rf.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				fillerColor.Bounds.X, fillerColor.Bounds.Y = mnx, mny
				fillerColor.Bounds.W, fillerColor.Bounds.H = mxx-mnx, mxy-mny
			}
			rf.SetColor(fillerColor.GetColorFunction(svgp.FillOpacity * opacity))
		}
		rf.Draw()
		// default is true
		rf.SetWinding(true)
	}
	if svgp.linerColor != nil {
		r.Clear()
		svgp.mAdder.Adder = r
		lineGap := svgp.LineGap
		if lineGap == nil {
			lineGap = DefaultStyle.LineGap
		}
		lineCap := svgp.LineCap
		if lineCap == nil {
			lineCap = DefaultStyle.LineCap
		}
		leadLineCap := lineCap
		if svgp.LeadLineCap != nil {
			leadLineCap = svgp.LeadLineCap
		}
		// The path's points reach the dasher already transformed, so its
		// stroke width, dashes and dash offset must be in that space too.
		// Upstream passed them in user units, so a stroke kept its width in
		// pixels whatever the scale: a 7-unit stroke on a 120-unit viewBox
		// was 7 px at 40 px (a filled disc) and at 240 px alike (#1292).
		//
		// A non-uniform transform has no single width to scale by: a browser
		// strokes in user space and then transforms the outline, so the pen
		// stretches with the shape (#1295, srwiley/oksvg#53). Here the path
		// is stroked under a uniform scale by the transform's larger axis
		// scale, which keeps curves at least as finely flattened as the
		// final pixels need, and the rest of the transform is applied to the
		// outline on its way to the scanner.
		scale, rest, stretched := splitStrokeTransform(svgp.mAdder.M)
		if stretched {
			svgp.mAdder.M = rasterx.Identity.Scale(scale, scale)
			inner := r.Filler.Scanner
			r.Filler.Scanner = &transformingScanner{Scanner: inner, m: rest}
			defer func() { r.Filler.Scanner = inner }()
		}
		dash := svgp.Dash
		if scale != 1 && len(dash) > 0 {
			dash = make([]float64, len(svgp.Dash))
			for i, v := range svgp.Dash {
				dash[i] = v * scale
			}
		}
		r.SetStroke(fixed.Int26_6(svgp.LineWidth*scale*64),
			fixed.Int26_6(svgp.MiterLimit*64), leadLineCap, lineCap,
			lineGap, svgp.LineJoin, dash, svgp.DashOffset*scale)
		svgp.Path.AddTo(&svgp.mAdder)
		switch linerColor := svgp.linerColor.(type) {
		case color.Color:
			r.SetColor(rasterx.ApplyOpacity(linerColor, svgp.LineOpacity*opacity))
		case rasterx.Gradient:
			if linerColor.Units == rasterx.ObjectBoundingBox {
				fRect := r.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				linerColor.Bounds.X, linerColor.Bounds.Y = mnx, mny
				linerColor.Bounds.W, linerColor.Bounds.H = mxx-mnx, mxy-mny
			}
			r.SetColor(linerColor.GetColorFunction(svgp.LineOpacity * opacity))
		}
		r.Draw()
	}
}

// splitStrokeTransform splits m for stroking. For a uniform scale with or
// without rotation (stretched false), scale is the factor m applies to every
// length and the path is stroked under m itself (#1292). Otherwise scale is
// m's larger axis scale (its largest singular value) and m = rest · scale(s):
// the path is stroked under the uniform scale and rest, which shrinks or
// keeps lengths but never grows them, is applied to the outline (#1295).
func splitStrokeTransform(m rasterx.Matrix2D) (scale float64, rest rasterx.Matrix2D, stretched bool) {
	det := math.Abs(m.A*m.D - m.B*m.C)
	sum := m.A*m.A + m.B*m.B + m.C*m.C + m.D*m.D
	disc := math.Sqrt(math.Max(sum*sum-4*det*det, 0))
	larger := math.Sqrt((sum + disc) / 2)
	smaller := 0.0
	if larger > 0 {
		smaller = det / larger
	}
	if larger-smaller <= 1e-9*larger {
		return math.Sqrt(det), m, false
	}
	return larger, rasterx.Matrix2D{
		A: m.A / larger, B: m.B / larger,
		C: m.C / larger, D: m.D / larger,
		E: m.E, F: m.F,
	}, true
}

// transformingScanner applies m to every point on its way to the scanner it
// wraps. The dasher flattens curves into lines before the scanner, and an
// affine transform keeps a line a line, so the outline is transformed
// exactly.
type transformingScanner struct {
	rasterx.Scanner
	m rasterx.Matrix2D
}

func (s *transformingScanner) Start(a fixed.Point26_6) { s.Scanner.Start(s.apply(a)) }

func (s *transformingScanner) Line(b fixed.Point26_6) { s.Scanner.Line(s.apply(b)) }

func (s *transformingScanner) apply(p fixed.Point26_6) fixed.Point26_6 {
	x, y := float64(p.X)/64, float64(p.Y)/64
	return fixed.Point26_6{
		X: fixed.Int26_6(math.Round((s.m.A*x + s.m.C*y + s.m.E) * 64)),
		Y: fixed.Int26_6(math.Round((s.m.B*x + s.m.D*y + s.m.F) * 64)),
	}
}

// GetFillColor returns the fill color of the SvgPath if one is defined and otherwise returns colornames.Black
func (svgp *SvgPath) GetFillColor() color.Color {
	return getColor(svgp.fillerColor)
}

// GetLineColor returns the stroke color of the SvgPath if one is defined and otherwise returns colornames.Black
func (svgp *SvgPath) GetLineColor() color.Color {
	return getColor(svgp.linerColor)
}

// SetFillColor sets the fill color of the SvgPath
func (svgp *SvgPath) SetFillColor(clr color.Color) {
	svgp.fillerColor = clr
}

// SetLineColor sets the line color of the SvgPath
func (svgp *SvgPath) SetLineColor(clr color.Color) {
	svgp.linerColor = clr
}
