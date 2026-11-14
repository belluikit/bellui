package layout

import "github.com/belluikit/bellui/core"

// fitGive is how much wider than base a box would like to make o, a child it
// does not stretch: what its FitWidth asks beyond base, capped at maxW when
// maxW is set (#1195).
func fitGive(o core.CanvasObject, base, maxW float32) float32 {
	f, ok := o.(core.FitWidther)
	if !ok {
		return 0
	}
	w := f.FitWidth()
	if maxW > 0 && w > maxW {
		w = maxW
	}
	return max(w-base, 0)
}

// fitKeep is the share of their give the children of a row keep: all of it
// when the row has room, less as the row overflows by over, none once giving
// all of it back is not enough.
func fitKeep(over, give float32) float32 {
	if over <= 0 || give <= 0 {
		return 1
	}
	return max(1-over/give, 0)
}
