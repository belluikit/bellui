package widget

// Importance represents how prominent the widget should appear
//
// Since: 1.0
type Importance int

const (
	// MediumImportance applies a standard appearance.
	MediumImportance Importance = iota
	// HighImportance applies a prominent appearance.
	HighImportance
	// LowImportance applies a subtle appearance where the widget has one: a
	// Label draws its text in the disabled colour. A Button in the default
	// style draws it like MediumImportance (see Button.Importance).
	LowImportance

	// DangerImportance applies an error theme to the widget.
	//
	// Since: 1.0
	DangerImportance
	// WarningImportance applies a warning theme to the widget.
	//
	// Since: 1.0
	WarningImportance

	// SuccessImportance applies a success theme to the widget.
	//
	// Since: 1.0
	SuccessImportance
)
