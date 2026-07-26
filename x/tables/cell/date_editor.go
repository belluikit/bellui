package cell

import (
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// DateEditor is a cell editor for date values using a text Entry.
// The user types a date string which is parsed on commit.
type DateEditor struct {
	// Format is the expected input format. Defaults to "2006-01-02".
	Format string
}

// NewDateEditor creates a date editor factory.
func NewDateEditor() *DateEditor {
	return &DateEditor{Format: "2006-01-02"}
}

func (e *DateEditor) CreateEditor() core.CanvasObject {
	entry := widget.NewEntry()
	entry.PlaceHolder = "YYYY-MM-DD"
	return entry
}

func (e *DateEditor) BeginEdit(editor core.CanvasObject, value any, params Params) {
	entry := editor.(*widget.Entry)
	format := e.Format
	if format == "" {
		format = "2006-01-02"
	}
	switch t := value.(type) {
	case time.Time:
		entry.SetText(t.Format(format))
	case *time.Time:
		if t != nil {
			entry.SetText(t.Format(format))
		} else {
			entry.SetText("")
		}
	case string:
		entry.SetText(t)
	default:
		entry.SetText("")
	}
}

func (e *DateEditor) EndEdit(editor core.CanvasObject) (any, bool) {
	entry := editor.(*widget.Entry)
	text := entry.Text
	format := e.Format
	if format == "" {
		format = "2006-01-02"
	}
	t, err := time.Parse(format, text)
	if err != nil {
		return text, false // return as string if parse fails
	}
	return t, false
}
