package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// TreeNode represents a node in a tree layout.
type TreeNode struct {
	Name     string
	Value    float64
	Color    color.Color
	Children []TreeNode
}

// TreeSeries configures a tree layout chart.
type TreeSeries struct {
	SeriesName string
	Data       TreeNode // root node
	NodeSize   float32  // radius (default 6)
	// Orient: "TB" (top-bottom, default), "LR" (left-right)
	Orient string
}

func (s *TreeSeries) SeriesType() Type { return TypeTree }
func (s *TreeSeries) Name() string     { return s.SeriesName }
func (s *TreeSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeTree, func() Renderer { return &treeRenderer{} })
}

type treeRenderer struct{}

func (r *treeRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ts, ok := ctx.Series.(*TreeSeries)
	if !ok {
		return nil
	}

	vp := ctx.Viewport
	nodeSize := ResolveFloat32(ts.NodeSize, DefaultTreeNodeSize)

	palette := DefaultPalette

	// Count total leaves and depth
	depth := treeDepth(ts.Data)
	if depth == 0 {
		depth = 1
	}
	leaves := treeLeafCount(ts.Data)
	if leaves == 0 {
		leaves = 1
	}

	lr := ts.Orient == "LR"
	margin := float32(20)

	var objects []core.CanvasObject
	leafIdx := 0
	labelColor := backgroundLabelColor(ctx, color.NRGBA{R: 220, G: 220, B: 220, A: 255})
	r.layout(ts.Data, 0, depth, leaves, &leafIdx, vp, margin, nodeSize, lr, palette, labelColor, &objects)
	return objects
}

func (r *treeRenderer) layout(node TreeNode, level, maxDepth, totalLeaves int, leafIdx *int,
	vp coord.Rect, margin, nodeSize float32, lr bool, palette []color.Color, labelColor color.Color, objects *[]core.CanvasObject,
) geo.Position {
	var px, py float32

	if len(node.Children) == 0 {
		// Leaf node
		if lr {
			px = vp.X + margin + float32(level)/float32(maxDepth)*(vp.W-2*margin)
			py = vp.Y + margin + float32(*leafIdx)/float32(totalLeaves)*(vp.H-2*margin)
		} else {
			px = vp.X + margin + float32(*leafIdx)/float32(totalLeaves)*(vp.W-2*margin)
			py = vp.Y + margin + float32(level)/float32(maxDepth)*(vp.H-2*margin)
		}
		*leafIdx++
	} else {
		// Branch node: position at average of children
		childPositions := make([]geo.Position, len(node.Children))
		for i, child := range node.Children {
			childPositions[i] = r.layout(child, level+1, maxDepth, totalLeaves, leafIdx, vp, margin, nodeSize, lr, palette, labelColor, objects)
		}
		sumX, sumY := float32(0), float32(0)
		for _, cp := range childPositions {
			sumX += cp.X
			sumY += cp.Y
		}
		px = sumX / float32(len(childPositions))
		py = sumY / float32(len(childPositions))
		if lr {
			px = vp.X + margin + float32(level)/float32(maxDepth)*(vp.W-2*margin)
		} else {
			py = vp.Y + margin + float32(level)/float32(maxDepth)*(vp.H-2*margin)
		}

		// Draw edges to children
		for _, cp := range childPositions {
			edge := &canvas.Line{
				Position1:   geo.NewPos(px, py),
				Position2:   cp,
				StrokeColor: color.NRGBA{R: 120, G: 120, B: 120, A: 150},
				StrokeWidth: 1,
			}
			*objects = append(*objects, edge)
		}
	}

	// Draw node
	c := palette[level%len(palette)]
	if node.Color != nil {
		c = node.Color
	}
	circle := canvas.NewCircle(c)
	circle.Position1 = geo.NewPos(px-nodeSize, py-nodeSize)
	circle.Position2 = geo.NewPos(px+nodeSize, py+nodeSize)
	*objects = append(*objects, circle)

	// Label
	if node.Name != "" {
		label := canvas.NewText(node.Name, labelColor)
		label.TextSize = 9
		labelW := float32(len(node.Name))*6 + 4
		label.Move(geo.NewPos(px-labelW/2, py+nodeSize+1))
		label.Resize(geo.NewSize(labelW, 11))
		*objects = append(*objects, label)
	}

	return geo.NewPos(px, py)
}

func treeDepth(n TreeNode) int {
	if len(n.Children) == 0 {
		return 1
	}
	max := 0
	for _, c := range n.Children {
		d := treeDepth(c)
		if d > max {
			max = d
		}
	}
	return max + 1
}

func treeLeafCount(n TreeNode) int {
	if len(n.Children) == 0 {
		return 1
	}
	sum := 0
	for _, c := range n.Children {
		sum += treeLeafCount(c)
	}
	return sum
}
