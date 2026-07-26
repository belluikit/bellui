package widget

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	_ = color.RGBA{}
	return img
}

func imgSeg() *ImageSegment { return &ImageSegment{Image: testImage()} }

func docSegKinds(segs []RichTextSegment) []string {
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		switch v := s.(type) {
		case *TextSegment:
			out = append(out, "T:"+v.Text)
		case *ImageSegment:
			out = append(out, "IMG")
		default:
			out = append(out, "?")
		}
	}
	return out
}

// --- doc model: atoms count as one position ---

func TestRichEntryDoc_ImageIsOnePosition(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		imgSeg(),
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	assert.Equal(t, 5, d.len()) // a,b,IMG,c,d
	assert.Equal(t, "abcd", d.String())
	assert.Equal(t, "abcd", d.textBetween(0, 5)) // image contributes no text

	// delete the image (position 2)
	d.deleteRange(2, 3)
	assert.Equal(t, []string{"T:abcd"}, docSegKinds(d.segments()))
	assert.Equal(t, 4, d.len())
}

func TestRichEntryDoc_InsertAtomSplitsText(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcd"}})
	pos := d.insertAtom(2, imgSeg())
	assert.Equal(t, 3, pos)
	assert.Equal(t, []string{"T:ab", "IMG", "T:cd"}, docSegKinds(d.segments()))
}

func TestRichEntryDoc_SegmentsBetweenIncludesAtom(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		imgSeg(),
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	got := d.segmentsBetween(1, 4) // "b", IMG, "c"
	assert.Equal(t, []string{"T:b", "IMG", "T:c"}, docSegKinds(got))
	// cloned: the returned image is a distinct pointer
	assert.NotSame(t, d.segments()[1], got[1])
}

// --- the linchpin: my row model must match RichText's actual layout ---

func TestRichEntryDoc_RowModelMatchesProvider(t *testing.T) {
	test.NewTempApp(t)
	cases := [][]RichTextSegment{
		{&TextSegment{Text: "hello"}},
		{&TextSegment{Text: "ab\ncd"}},
		{&TextSegment{Text: "ab\n"}},
		{imgSeg()},
		{&TextSegment{Text: "ab"}, imgSeg()},
		{&TextSegment{Text: "ab"}, imgSeg(), &TextSegment{Text: "cd"}},
		{imgSeg(), &TextSegment{Text: "ef"}},
		{imgSeg(), imgSeg()},
		{&TextSegment{Text: "ab\n"}, imgSeg(), &TextSegment{Text: "cd"}},
		{&TextSegment{Text: "x\ny\nz"}, imgSeg()},
	}
	for i, segs := range cases {
		e := NewRichEntry()
		e.SetSegments(segs)
		w := test.NewWindow(e)
		w.Resize(geo.NewSize(200, 200))
		e.Refresh()
		assert.Equal(t, e.provider.rows(), e.doc.rowCount(), "case %d row count", i)
		w.Close()
	}
}

// --- widget: insert / caret / delete / undo of images ---

func TestRichEntry_InsertImageAtCaret(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcd"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	e.caret, e.anchor = 2, 2
	e.InsertImage(testImage())
	assert.Equal(t, []string{"T:ab", "IMG", "T:cd"}, docSegKinds(e.Segments()))
	assert.Equal(t, 3, e.caret) // after the image

	// backspace removes the image
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, []string{"T:abcd"}, docSegKinds(e.Segments()))
	assert.Equal(t, 2, e.caret)
}

func TestRichEntry_UndoRedoImage(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcd"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	e.caret, e.anchor = 2, 2
	e.InsertImage(testImage())
	assert.Equal(t, []string{"T:ab", "IMG", "T:cd"}, docSegKinds(e.Segments()))

	e.Undo()
	assert.Equal(t, []string{"T:abcd"}, docSegKinds(e.Segments()))
	assert.Equal(t, 2, e.caret)

	e.Redo()
	assert.Equal(t, []string{"T:ab", "IMG", "T:cd"}, docSegKinds(e.Segments()))
}

func TestRichEntry_CaretOnImageRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab\n"},
		imgSeg(),
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))
	e.FocusGained()

	// caret just before the image (position 3 = the image atom)
	e.caret, e.anchor = 3, 3
	row, _ := e.doc.offsetToRowCol(e.caret)
	assert.True(t, e.doc.rowIsImage(row), "caret should resolve to the image row")
	pos, size := e.caretRect()
	// image row caret sits at the left edge and has positive height
	assert.Less(t, pos.X, float32(12), "caret x near the left edge on an image row")
	assert.Greater(t, size.Height, float32(0))
}

func TestRichEntry_RenderWithImage(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "top\n"},
		imgSeg(),
		&TextSegment{Style: normalStyle(), Text: "end"},
	})
	e.Resize(geo.NewSize(120, 90))
	test.AssertObjectRendersToMarkup(t, "richentry/image.xml", e)
}

// --- in-memory ImageSegment source renders without a URI (RichText path) ---

func TestImageSegment_InMemorySourceRenders(t *testing.T) {
	test.NewTempApp(t)
	seg := &ImageSegment{Image: testImage()}
	vis := seg.Visual().(*richImage)
	assert.NotNil(t, vis.img.Image, "in-memory image.Image is used directly")
	assert.Nil(t, vis.img.Resource)

	res := bellui.NewStaticResource("x.png", []byte{0x89, 'P', 'N', 'G'})
	seg2 := &ImageSegment{Resource: res}
	vis2 := seg2.Visual().(*richImage)
	assert.Equal(t, res, vis2.img.Resource)
}
