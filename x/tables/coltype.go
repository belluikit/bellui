package tables

// ColumnType identifies the data type of a column,
// which determines the default renderer, editor, filter, and sort comparator.
type ColumnType int

const (
	// ColumnText is the default column type for string data.
	ColumnText ColumnType = iota
	// ColumnNumber is for numeric data (int, float).
	ColumnNumber
	// ColumnDate is for date/time data.
	ColumnDate
	// ColumnBoolean is for boolean data.
	ColumnBoolean
	// ColumnCustom indicates a column with a user-provided renderer.
	ColumnCustom
)
