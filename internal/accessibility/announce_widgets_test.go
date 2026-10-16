//go:build accessibility

package accessibility_test

import (
	"testing"

	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

// The transient status widgets must push their text through the announcement
// seam when shown (pre-rebrand tracker #525): a screen reader only discovers tree nodes the user
// navigates to, and these auto-dismiss first.

type announcement struct {
	text      string
	assertive bool
}

func TestToast_AnnouncesOnShow(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	var got []announcement
	remove := accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		got = append(got, announcement{text, assertive})
	})
	defer remove()

	toast := widget.ShowToast("saved", w.Canvas())
	defer toast.Dismiss()
	assert.Equal(t, []announcement{{"saved", false}}, got)

	danger := widget.ShowToastWithImportance("disk full", widget.DangerImportance, w.Canvas())
	defer danger.Dismiss()
	assert.Equal(t, announcement{"disk full", true}, got[len(got)-1])
}

func TestSnackbar_AnnouncesOnShow(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	var got []announcement
	remove := accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		got = append(got, announcement{text, assertive})
	})
	defer remove()

	s := widget.NewSnackbar("message sent", "Undo", func() {})
	s.Show(w.Canvas())
	defer s.Hide()

	// Only the message text is announced, politely; the action button stays
	// an ordinary tree node.
	assert.Equal(t, []announcement{{"message sent", false}}, got)
}
