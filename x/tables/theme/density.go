package theme

// DensityMode controls the visual density of the data grid.
type DensityMode int

const (
	// DensityComfortable is the default density with standard padding.
	DensityComfortable DensityMode = iota
	// DensityCompact uses minimal padding for information-dense displays.
	DensityCompact
	// DensitySpacious uses generous padding for touch-friendly interfaces.
	DensitySpacious
)
