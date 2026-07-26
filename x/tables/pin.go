package tables

// PinSide indicates which side a column is pinned to.
type PinSide int

const (
	// PinNone means the column is not pinned.
	PinNone PinSide = iota
	// PinLeft pins the column to the left side of the grid.
	PinLeft
	// PinRight pins the column to the right side of the grid.
	PinRight
)
