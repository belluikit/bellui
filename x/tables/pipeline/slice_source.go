package pipeline

// SliceSource wraps a slice of any type as a DataSource.
type SliceSource struct {
	data []any
}

// NewSliceSource creates a DataSource backed by a slice.
func NewSliceSource(data []any) *SliceSource {
	return &SliceSource{data: data}
}

func (s *SliceSource) Length() int       { return len(s.data) }
func (s *SliceSource) Row(index int) any { return s.data[index] }

// SetData replaces the underlying data.
func (s *SliceSource) SetData(data []any) {
	s.data = data
}

// CallbackSource wraps length/row callbacks as a DataSource.
type CallbackSource struct {
	LengthFunc func() int
	RowFunc    func(int) any
}

// NewCallbackSource creates a DataSource from callbacks.
func NewCallbackSource(length func() int, row func(int) any) *CallbackSource {
	return &CallbackSource{LengthFunc: length, RowFunc: row}
}

func (s *CallbackSource) Length() int       { return s.LengthFunc() }
func (s *CallbackSource) Row(index int) any { return s.RowFunc(index) }
