package pipeline

// PaginateTransform slices the row indices to the current page.
type PaginateTransform struct {
	PageSize    int
	CurrentPage int // 0-indexed
}

// NewPaginateTransform creates a pagination transform.
func NewPaginateTransform(pageSize int) *PaginateTransform {
	return &PaginateTransform{
		PageSize:    pageSize,
		CurrentPage: 0,
	}
}

func (t *PaginateTransform) Apply(indices []int, source DataSource) []int {
	if t.PageSize <= 0 {
		return indices
	}
	total := len(indices)
	start := t.CurrentPage * t.PageSize
	if start >= total {
		// Clamp to last page
		t.CurrentPage = (total - 1) / t.PageSize
		if t.CurrentPage < 0 {
			t.CurrentPage = 0
		}
		start = t.CurrentPage * t.PageSize
	}
	end := start + t.PageSize
	if end > total {
		end = total
	}
	return indices[start:end]
}

// SetPage sets the current page (0-indexed).
func (t *PaginateTransform) SetPage(page int) {
	if page < 0 {
		page = 0
	}
	t.CurrentPage = page
}

// PageCount returns the total number of pages for the given total row count.
func (t *PaginateTransform) PageCount(totalRows int) int {
	if t.PageSize <= 0 {
		return 1
	}
	pages := totalRows / t.PageSize
	if totalRows%t.PageSize != 0 {
		pages++
	}
	return pages
}
