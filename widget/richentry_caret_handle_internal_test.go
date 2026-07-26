//go:build !mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The RichEntry caret drag-handle is touch-gated like the selection handles
// (#999, parity with Entry's #943): off mobile a touch tap summons no overlay,
// keeping desktop behavior and the shared render masters unchanged.
func TestRichEntry_CaretHandle_NotShownOffMobile(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: "hello world"}})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(200, 100))
	w.Canvas().Focus(e)

	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(10, 10)}})
	e.Tapped(&event.PointEvent{Position: geo.NewPos(10, 10)})
	assert.Nil(t, e.caretHandle, "no caret handle is created on a non-mobile device")
	assert.Empty(t, w.Canvas().Overlays().List())
}
