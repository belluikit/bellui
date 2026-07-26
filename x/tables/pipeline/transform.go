package pipeline

// Transform takes a set of row indices and returns a transformed set.
// Transforms can filter, reorder, or group rows.
type Transform interface {
	// Apply transforms the given indices using data from the source.
	// Returns the new set of indices in display order.
	Apply(indices []int, source DataSource) []int
}
