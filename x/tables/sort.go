package tables

// SortDirection specifies ascending, descending, or no sort.
type SortDirection int

const (
	// SortNone indicates the column is not sorted.
	SortNone SortDirection = iota
	// SortAscending sorts the column in ascending order.
	SortAscending
	// SortDescending sorts the column in descending order.
	SortDescending
)

// SortItem represents a single column in a multi-column sort.
type SortItem struct {
	ColID     string
	Direction SortDirection
}
