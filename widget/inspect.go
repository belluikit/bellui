package widget

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// InspectLayout returns a human-readable tree showing each object's type, size,
// position, and MinSize. Useful for debugging why a widget gets a particular size.
//
//	fmt.Println(widget.InspectLayout(myWindow.Content()))
//
// Output example:
//
//	*widget.Button  size=(200,40)  pos=(10,10)  min=(80,36)
//	  *canvas.Text  size=(180,20)  pos=(10,10)  min=(60,16)
func InspectLayout(obj core.CanvasObject) string {
	var b strings.Builder
	inspectObject(&b, obj, 0)
	return b.String()
}

func inspectObject(b *strings.Builder, obj core.CanvasObject, depth int) {
	if obj == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	typeName := fmt.Sprintf("%T", obj)

	size := obj.Size()
	pos := obj.Position()
	min := obj.MinSize()
	visible := obj.Visible()

	fmt.Fprintf(b, "%s%s  size=(%g,%g)  pos=(%g,%g)  min=(%g,%g)",
		indent, typeName,
		size.Width, size.Height,
		pos.X, pos.Y,
		min.Width, min.Height)

	if !visible {
		b.WriteString("  HIDDEN")
	}
	b.WriteByte('\n')

	// Descend into children.
	switch c := obj.(type) {
	case *bellui.Container:
		for _, child := range c.Objects {
			inspectObject(b, child, depth+1)
		}
	case core.Widget:
		r := cache.Renderer(c)
		if r != nil {
			for _, child := range r.Objects() {
				inspectObject(b, child, depth+1)
			}
		}
	}
}
