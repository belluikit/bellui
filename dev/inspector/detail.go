package inspector

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// detailRow is one labelled property shown in the inspector's detail pane.
type detailRow struct {
	Label string
	Value string
}

// describe extracts the properties of a snapshot node for the detail pane. It
// reads the live object's current geometry (so values are fresh even between
// tree refreshes) and probes the fork's optional capabilities: scroll offset,
// direction scope, theme override and the accessibility snapshot. It is a pure
// function of the node so it can be unit-tested without a window.
func describe(n *node) []detailRow {
	if n == nil {
		return nil
	}
	obj := n.obj
	pos := obj.Position()
	size := obj.Size()
	min := obj.MinSize()

	rows := []detailRow{
		{"Type", n.typeName},
		{"Kind", n.kind.String()},
		{"Visible", fmt.Sprintf("%t", obj.Visible())},
		{"Position", fmtPos(pos)},
		{"Size", fmtSize(size)},
		{"MinSize", fmtSize(min)},
		{"Abs bounds", fmt.Sprintf("(%g,%g) %g×%g", n.absPos.X, n.absPos.Y, size.Width, size.Height)},
	}

	if n.hasClip {
		rows = append(rows, detailRow{"Clip", fmt.Sprintf("(%g,%g) %g×%g",
			n.clip.Pos.X, n.clip.Pos.Y, n.clip.Size.Width, n.clip.Size.Height)})
	}

	if sc, ok := obj.(*container.Scroll); ok {
		rows = append(rows,
			detailRow{"Scroll offset", fmtPos(sc.Offset)},
			detailRow{"Scroll content", fmtSize(sc.Content.Size())})
	}

	if ds, ok := obj.(*container.DirectionScope); ok {
		rows = append(rows, detailRow{"Direction scope", directionName(ds.ScopedDirection())})
	}

	if _, ok := obj.(*container.ThemeOverride); ok {
		rows = append(rows, detailRow{"Theme override", "yes"})
	}

	if snap, ok := core.SnapshotAccessible(obj); ok {
		rows = append(rows, detailRow{"A11y role", snap.Role.String()})
		if snap.Label != "" {
			rows = append(rows, detailRow{"A11y label", snap.Label})
		}
		if snap.Value != "" {
			rows = append(rows, detailRow{"A11y value", snap.Value})
		}
	}

	return rows
}

func fmtPos(p geo.Position) string { return fmt.Sprintf("(%g, %g)", p.X, p.Y) }
func fmtSize(s geo.Size) string    { return fmt.Sprintf("%g × %g", s.Width, s.Height) }

func directionName(d bellui.Direction) string {
	switch d {
	case bellui.DirectionLTR:
		return "LTR"
	case bellui.DirectionRTL:
		return "RTL"
	default:
		return "Auto"
	}
}
