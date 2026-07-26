package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

func TestCreateSymbol_Circle(t *testing.T) {
	pos := geo.NewPos(100, 100)
	c := color.NRGBA{R: 255, A: 255}
	obj := CreateSymbol(SymbolCircle, 10, pos, c)
	if obj == nil {
		t.Fatal("expected non-nil object for SymbolCircle")
	}
	circle, ok := obj.(*canvas.Circle)
	if !ok {
		t.Fatal("expected *canvas.Circle")
	}
	if circle.Position1.X != 95 || circle.Position1.Y != 95 {
		t.Errorf("circle Position1 = %v, want (95, 95)", circle.Position1)
	}
	if circle.Position2.X != 105 || circle.Position2.Y != 105 {
		t.Errorf("circle Position2 = %v, want (105, 105)", circle.Position2)
	}
}

func TestCreateSymbol_Rect(t *testing.T) {
	obj := CreateSymbol(SymbolRect, 8, geo.NewPos(50, 50), color.White)
	if obj == nil {
		t.Fatal("expected non-nil object for SymbolRect")
	}
	if _, ok := obj.(*canvas.Rectangle); !ok {
		t.Fatal("expected *canvas.Rectangle")
	}
}

func TestCreateSymbol_None(t *testing.T) {
	obj := CreateSymbol(SymbolNone, 8, geo.NewPos(0, 0), color.White)
	if obj != nil {
		t.Error("SymbolNone should return nil")
	}
}

func TestCreateSymbol_Diamond(t *testing.T) {
	obj := CreateSymbol(SymbolDiamond, 8, geo.NewPos(0, 0), color.White)
	if obj == nil {
		t.Fatal("expected non-nil for SymbolDiamond")
	}
}

func TestCreateSymbol_Triangle(t *testing.T) {
	obj := CreateSymbol(SymbolTriangle, 8, geo.NewPos(0, 0), color.White)
	if obj == nil {
		t.Fatal("expected non-nil for SymbolTriangle")
	}
}
