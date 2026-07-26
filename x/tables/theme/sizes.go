package theme

import "github.com/belluikit/bellui"

// Grid-specific theme size names.
const (
	SizeNameRowHeight         bellui.ThemeSizeName = "fytables:rowHeight"
	SizeNameHeaderHeight      bellui.ThemeSizeName = "fytables:headerHeight"
	SizeNameCellPaddingH      bellui.ThemeSizeName = "fytables:cellPaddingH"
	SizeNameCellPaddingV      bellui.ThemeSizeName = "fytables:cellPaddingV"
	SizeNameResizeHandleWidth bellui.ThemeSizeName = "fytables:resizeHandleWidth"
	SizeNameSortIndicator     bellui.ThemeSizeName = "fytables:sortIndicatorSize"
	SizeNameMinColumnWidth    bellui.ThemeSizeName = "fytables:minColumnWidth"
)

// DefaultSize returns the default value for a grid-specific size name.
func DefaultSize(name bellui.ThemeSizeName, density DensityMode) float32 {
	switch name {
	case SizeNameRowHeight:
		switch density {
		case DensityCompact:
			return 28
		case DensitySpacious:
			return 48
		default:
			return 36
		}
	case SizeNameHeaderHeight:
		switch density {
		case DensityCompact:
			return 32
		case DensitySpacious:
			return 52
		default:
			return 40
		}
	case SizeNameCellPaddingH:
		switch density {
		case DensityCompact:
			return 4
		case DensitySpacious:
			return 12
		default:
			return 8
		}
	case SizeNameCellPaddingV:
		switch density {
		case DensityCompact:
			return 2
		case DensitySpacious:
			return 8
		default:
			return 4
		}
	case SizeNameResizeHandleWidth:
		return 6
	case SizeNameSortIndicator:
		return 10
	case SizeNameMinColumnWidth:
		return 40
	}
	return 0
}
