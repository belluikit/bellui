package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/internal/pool"
)

// CreateSymbol creates a canvas object for the given symbol type, size, position, and color.
// Returns nil for SymbolNone.
func CreateSymbol(sym SymbolType, size float32, pos geo.Position, c color.Color) core.CanvasObject {
	return CreatePooledSymbol(nil, sym, size, pos, c)
}

// CreatePooledSymbol creates a symbol using the pool when available.
func CreatePooledSymbol(p *pool.ObjectPool, sym SymbolType, size float32, pos geo.Position, c color.Color) core.CanvasObject {
	half := size / 2
	switch sym {
	case SymbolCircle:
		if p != nil {
			return p.Circle(pos, half, c)
		}
		circle := canvas.NewCircle(c)
		circle.Position1 = geo.NewPos(pos.X-half, pos.Y-half)
		circle.Position2 = geo.NewPos(pos.X+half, pos.Y+half)
		return circle
	case SymbolRect, SymbolDiamond, SymbolTriangle:
		if p != nil {
			return p.Rect(geo.NewPos(pos.X-half, pos.Y-half), geo.NewSize(size, size), c)
		}
		rect := canvas.NewRectangle(c)
		rect.Move(geo.NewPos(pos.X-half, pos.Y-half))
		rect.Resize(geo.NewSize(size, size))
		return rect
	default:
		return nil
	}
}
