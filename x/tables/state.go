package tables

// GridState represents the serializable state of a DataGrid for save/restore.
type GridState struct {
	Columns    []ColumnState `json:"columns"`
	SortModel  []SortItem    `json:"sortModel,omitempty"`
	PageNumber int           `json:"pageNumber,omitempty"`
}

// ColumnState stores the state of a single column.
type ColumnState struct {
	ColID  string  `json:"colId"`
	Width  float32 `json:"width,omitempty"`
	Pinned PinSide `json:"pinned,omitempty"`
	Hidden bool    `json:"hidden,omitempty"`
	Index  int     `json:"index"`
}
