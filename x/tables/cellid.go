package tables

// CellID identifies a specific cell in the data grid by its row and column index.
type CellID struct {
	Row int
	Col int
}

// RowID is a type alias for row indices.
type RowID = int

// ColID is a type alias for column indices.
type ColID = int
