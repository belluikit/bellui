package tables

import (
	"fmt"
	"image"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// A group's disclosure arrows are the theme's own icons, MenuDropDownIcon when
// the group is expanded and MenuExpandIcon when it is collapsed. They have the
// same style and colour as every other icon an app shows. A text arrow went
// through font fallback, where bellui's bundled colour emoji font drew ▶ as a
// blue play button on every platform (#1256). The render check catches any
// colour cast: a grey icon on a grey row has none, and the emoji has plenty.
func TestDataGrid_GroupArrowsAreThemeIcons(t *testing.T) {
	var data []map[string]any
	for i := range 6 {
		data = append(data, map[string]any{"dept": fmt.Sprintf("D%d", i%2), "n": i})
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "dept"}, {Field: "n"}})
	g.API().SetRowGroupColumn("dept")
	g.API().ToggleGroup("D1") // D0 expanded, D1 collapsed

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	img := w.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width

	want := map[string]bool{
		theme.MenuDropDownIcon().Name(): true,
		theme.MenuExpandIcon().Name():   true,
	}
	seen := map[string]bool{}
	origin := g.groupRowOverlay.Position()
	walkCanvasObjects(g.groupRowOverlay, func(o core.CanvasObject) {
		icon, ok := o.(*canvas.Image)
		if !ok || icon.Resource == nil || !icon.Visible() {
			return
		}
		name := icon.Resource.Name()
		if !want[name] {
			t.Errorf("group arrow shows %q, want a theme disclosure icon", name)
			return
		}
		seen[name] = true
		pos := origin.Add(icon.Position())
		r := image.Rect(int(pos.X*scale), int(pos.Y*scale),
			int((pos.X+icon.Size().Width)*scale), int((pos.Y+icon.Size().Height)*scale))
		if worst, at := maxChroma(img, r); worst > 24 {
			t.Errorf("arrow %q has a coloured pixel at %v (chroma %d)", name, at, worst)
		}
	})
	if len(seen) != 2 {
		t.Errorf("group arrows showed %v, want both MenuDropDownIcon (expanded) and MenuExpandIcon (collapsed)", seen)
	}
}

// maxChroma returns the largest spread between a pixel's colour channels inside
// r, and where it is. Greys have none; the emoji's blue has a large one.
func maxChroma(img image.Image, r image.Rectangle) (worst int, at image.Point) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			hi := max(cr, cg, cb) >> 8
			lo := min(cr, cg, cb) >> 8
			if c := int(hi - lo); c > worst {
				worst, at = c, image.Pt(x, y)
			}
		}
	}
	return worst, at
}
