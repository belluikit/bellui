package widget

import (
	"net/url"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func richTextRenderTexts(rich core.Widget) []*canvas.Text {
	renderer := cache.Renderer(rich).(*textRenderer)
	texts := make([]*canvas.Text, len(renderer.Objects()))
	for i, obj := range renderer.Objects() {
		texts[i] = obj.(*canvas.Text)
	}
	return texts
}

func trailingBoldErrorSegment() *TextSegment {
	return &TextSegment{Style: RichTextStyle{
		Alignment: bellui.TextAlignTrailing,
		ColorName: theme.ColorNameError,
		TextStyle: bellui.TextStyle{Bold: true},
	}}
}

func TestRichText_Hyperlink_Endline(t *testing.T) {
	u, _ := url.Parse("https://github.com/belluikit/bellui")
	r := NewRichText(
		&TextSegment{Text: "Text", Style: RichTextStyleInline},
		&HyperlinkSegment{Text: "Link", URL: u},
	)
	r.Resize(r.MinSize())
	view := cache.Renderer(r)

	assert.Len(t, view.Objects(), 2)
	assert.Equal(t, view.Objects()[0].Position().Y, view.Objects()[1].Position().Y)   // same baseline
	assert.Greater(t, view.Objects()[1].Position().X, view.Objects()[0].Position().X) // to the right
}

func TestText_Alignment(t *testing.T) {
	seg := trailingBoldErrorSegment()
	seg.Text = "Test"
	text := NewRichText(seg)
	assert.Equal(t, bellui.TextAlignTrailing, test.TempWidgetRenderer(t, text).Objects()[0].(*canvas.Text).Alignment)
}

func TestText_AlignmentInline(t *testing.T) {
	seg1 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignTrailing,
		},
		Text: "Thing 1",
	}
	seg2 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignTrailing,
			Inline:    true,
		},
		Text: "Thing 2",
	}
	seg3 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignTrailing,
			Inline:    true,
		},
		Text: "Thing 3",
	}
	text := NewRichText(seg1, seg2, seg3)
	text.Resize(geo.NewSize(150, 50))

	r := test.TempWidgetRenderer(t, text)
	obj1 := r.Objects()[0].(*canvas.Text)
	right1 := obj1.Position().X + obj1.Size().Width
	assert.Equal(t, bellui.TextAlignTrailing, obj1.Alignment)
	obj3 := r.Objects()[2].(*canvas.Text)
	right3 := obj3.Position().X + obj3.Size().Width
	assert.Equal(t, bellui.TextAlignTrailing, obj3.Alignment)

	assert.Equal(t, right1, right3)
}

func TestText_AlignmentInlineCenter(t *testing.T) {
	seg1 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignCenter,
		},
		Text: "MMM",
	}
	seg2 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	seg3 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	seg4 := &TextSegment{
		Style: RichTextStyle{
			Alignment: bellui.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	text := NewRichText(seg1, seg2, seg3, seg4)
	text.Resize(geo.NewSize(150, 50))

	r := test.TempWidgetRenderer(t, text)
	obj1 := r.Objects()[0].(*canvas.Text)
	left1 := obj1.Position().X
	assert.Equal(t, bellui.TextAlignCenter, obj1.Alignment)
	obj3 := r.Objects()[2].(*canvas.Text)
	left3 := obj3.Position().X
	assert.Equal(t, bellui.TextAlignCenter, obj3.Alignment)

	assert.Equal(t, left1, left3)
}

func TestText_Row(t *testing.T) {
	text := NewRichTextWithText("")
	text.Segments[0].(*TextSegment).Text = "test"
	text.Refresh()

	assert.Nil(t, text.row(-1))
	assert.Nil(t, text.row(1))

	assert.Equal(t, []rune("test"), text.row(0))
}

func TestText_Rows(t *testing.T) {
	text := NewRichTextWithText("test")
	assert.Equal(t, 1, text.rows())
	textSeg := text.Segments[0].(*TextSegment)

	textSeg.Text = "test\ntest"
	text.Refresh()
	assert.Equal(t, 2, text.rows())

	textSeg.Text = "test\ntest\ntest"
	text.Refresh()
	assert.Equal(t, 3, text.rows())

	textSeg.Text = "test\ntest\ntest\n"
	text.Refresh()
	assert.Equal(t, 4, text.rows())

	textSeg.Text = "\n"
	text.Refresh()
	assert.Equal(t, 2, text.rows())
}

func TestText_RowLength(t *testing.T) {
	text := NewRichTextWithText("test")

	rl := text.rowLength(0)
	assert.Equal(t, 4, rl)
	textSeg := text.Segments[0].(*TextSegment)

	textSeg.Text = "test\ntèsts"
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 4, rl)

	rl = text.rowLength(1)
	assert.Equal(t, 5, rl)

	textSeg.Text = ""
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 0, rl)

	textSeg.Text = "\nhello"
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 0, rl)

	rl = text.rowLength(1)
	assert.Equal(t, 5, rl)
}

func TestText_Scroll(t *testing.T) {
	text1 := NewRichTextWithText("test1\ntest2")
	text2 := NewRichTextWithText("test1\ntest2")
	text2.Scroll = widget.ScrollBoth

	assert.Less(t, text2.MinSize().Width, text1.MinSize().Width)
	assert.Less(t, text2.MinSize().Height, text1.MinSize().Height)

	text3 := NewRichTextWithText("test1\ntest2")
	text3.Scroll = widget.ScrollVerticalOnly
	assert.Equal(t, text3.MinSize().Width, text1.MinSize().Width)
	assert.Less(t, text3.MinSize().Height, text1.MinSize().Height)

	text4 := NewRichTextWithText("test1\ntest2")
	text4.Scroll = widget.ScrollVerticalOnly
	text4.Wrapping = bellui.TextWrapBreak

	text3.Resize(geo.NewSize(32, 32))
	text4.Resize(geo.NewSize(32, 32))
	assert.Less(t, text4.MinSize().Width, text3.MinSize().Width)
	assert.Equal(t, text4.MinSize().Height, text3.MinSize().Height)

	content3 := test.TempWidgetRenderer(t, text3).Objects()[0].(*widget.Scroll).Content
	content4 := test.TempWidgetRenderer(t, text4).Objects()[0].(*widget.Scroll).Content
	assert.Less(t, content4.MinSize().Width, content3.MinSize().Width)
	assert.Greater(t, content4.MinSize().Height, content3.MinSize().Height)
}

func TestText_InsertAt(t *testing.T) {
	type fields struct {
		buffer string
	}
	type args struct {
		pos   int
		runes string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantBuffer string
	}{
		{
			name:   "case_1",
			fields: fields{buffer: "A\n1"},
			args: args{
				pos:   0,
				runes: "\n",
			},
			wantBuffer: "\nA\n1",
		},
		{
			name:   "case_2",
			fields: fields{buffer: "hello\nèé+^#"},
			args: args{
				pos:   5,
				runes: "\naddme",
			},
			wantBuffer: "hello\naddme\nèé+^#",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichTextWithText(tt.fields.buffer)
			text.insertAt(tt.args.pos, []rune(tt.args.runes))
			assert.Equal(t, tt.wantBuffer, text.String())
		})
	}
}

func TestText_Insert(t *testing.T) {
	text := NewRichTextWithText("")
	text.insertAt(0, []rune("a"))
	assert.Equal(t, "a", text.String())
	text.insertAt(1, []rune("\n"))
	assert.Equal(t, "a\n", text.String())
	text.insertAt(2, []rune("b"))
	assert.Equal(t, "a\nb", text.String())
}

func TestText_DeleteFromTo(t *testing.T) {
	type fields struct {
		buffer string
	}
	type args struct {
		lowBound  int
		highBound int
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       string
		wantBuffer string
	}{
		{
			name:   "case_1",
			fields: fields{buffer: "A\n1"},
			args: args{
				lowBound:  0,
				highBound: 1,
			},
			want:       "A",
			wantBuffer: "\n1",
		},
		{
			name:   "case_2",
			fields: fields{buffer: "A\n1"},
			args: args{
				lowBound:  1,
				highBound: 2,
			},
			want:       "\n",
			wantBuffer: "A1",
		},
		{
			name:   "case_3",
			fields: fields{buffer: "A\nè1"},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want:       "\nè",
			wantBuffer: "A1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichTextWithText(tt.fields.buffer)
			got := text.deleteFromTo(tt.args.lowBound, tt.args.highBound)
			assert.Equal(t, tt.want, string(got))
			assert.Equal(t, tt.wantBuffer, text.String())
		})
	}
}

func TestText_DeleteFromTo_Segments(t *testing.T) {
	type args struct {
		lowBound  int
		highBound int
	}
	tests := []struct {
		name         string
		segments     []RichTextSegment
		args         args
		want         string
		wantSegments []RichTextSegment
	}{
		{
			name: "remove begin",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&TextSegment{Text: "1"},
			},
			args: args{
				lowBound:  0,
				highBound: 1,
			},
			want: "A",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "\n"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove end",
			segments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "\n1"},
			},
			args: args{
				lowBound:  1,
				highBound: 2,
			},
			want: "\n",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove both",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&TextSegment{Text: "è1"},
			},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want: "\nè",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove nontext",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&SeparatorSegment{},
				&TextSegment{Text: "B1"},
			},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want: "\nB",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichText(tt.segments...)
			got := text.deleteFromTo(tt.args.lowBound, tt.args.highBound)
			assert.Equal(t, tt.want, string(got))
			for _, s := range tt.wantSegments {
				if txt, ok := s.(*TextSegment); ok {
					txt.parent = text
				}
			}
			assert.Equal(t, tt.wantSegments, text.Segments)
		})
	}
}

func TestText_Multiline(t *testing.T) {
	text := NewRichText(
		&TextSegment{Text: "line1\nli", Style: RichTextStyleStrong},
		&TextSegment{Text: "ne2\nline3", Style: RichTextStyleInline},
	)

	w := test.NewTempWindow(t, text)
	w.Resize(geo.NewSize(64, 90))
	test.AssertImageMatches(t, "richtext/richtext_multiline.png", w.Canvas().Capture())
}

func TestText_Color(t *testing.T) {
	text := NewRichText(trailingBoldErrorSegment())

	assert.Equal(t, theme.Color(theme.ColorNameError), richTextRenderTexts(text)[0].Color)
}

func TestTextRenderer_ApplyTheme(t *testing.T) {
	label := NewLabel("Test\nLine2")
	texts := labelTextRenderTexts(label)

	text1 := texts[0]
	text2 := texts[1]
	textSize1 := text1.TextSize
	textSize2 := text2.TextSize
	customTextSize1 := textSize1
	customTextSize2 := textSize2
	test.WithTestTheme(t, func() {
		label.Refresh()
		text1 := texts[0]
		text2 := texts[1]
		customTextSize1 = text1.TextSize
		customTextSize2 = text2.TextSize
	})

	assert.NotEqual(t, textSize1, customTextSize1)
	assert.NotEqual(t, textSize2, customTextSize2)
}

func TestTextProvider_LineSizeToColumn(t *testing.T) {
	label := NewLabel("Test")
	label.CreateRenderer() // TODO make this a simple refresh call once it's in
	provider := label.provider

	inPad := theme.InnerPadding()
	textSize := theme.TextSize()
	fullSize := provider.lineSizeToColumn(4, 0, textSize, inPad, true)
	assert.Equal(t, fullSize, provider.lineSizeToColumn(10, 0, textSize, inPad, true))
	assert.Greater(t, fullSize.Width, provider.lineSizeToColumn(2, 0, textSize, inPad, true).Width)

	out := provider.lineSizeToColumn(-1, -1, textSize, inPad, true)
	assert.Equal(t, out, provider.lineSizeToColumn(0, 0, textSize, inPad, true))
}

func TestText_splitLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [][2]int
	}{
		{
			name: "Empty",
			text: "",
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Single",
			text: "foo",
			want: [][2]int{
				{0, 3},
			},
		},
		{
			name: "Multiple",
			text: "foo\nbar",
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Trailing",
			text: "foo\nbar\n",
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(&TextSegment{Text: tt.text})
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)
			}
		})
	}
}

func TestText_lineBounds(t *testing.T) {
	measurer := func(text []rune) geo.Size {
		return bellui.MeasureText(string(text), 14, bellui.TextStyle{})
	}
	tests := []struct {
		name     string
		text     string
		wrap     bellui.TextWrap
		trunc    bellui.TextTruncation
		want     [][2]int
		ellipses int
	}{
		{
			name: "Empty_WrapOff",
			text: "",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_Truncate",
			text: "",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name:  "Empty_TruncateClip",
			text:  "",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_WrapBreak",
			text: "",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_WrapWord",
			text: "",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Single_Short_WrapOff",
			text: "foobar",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Short_Truncate",
			text: "foobar",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name:  "Single_Short_TruncateClip",
			text:  "foobar",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name:  "Single_Short_TruncateEllipsis",
			text:  "foobar",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
			},
			ellipses: 0, // too short to cut
		},
		{
			name: "Single_Short_WrapBreak",
			text: "foobar",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Short_WrapWord",
			text: "foobar",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Long_WrapOff",
			text: "foobar foobar",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 13},
			},
		},
		{
			name: "Single_Long_Truncate",
			text: "foobar foobar",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 10},
			},
		},
		{
			name:  "Single_Long_TruncateClip",
			text:  "foobar foobar",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 10},
			},
		},
		{
			name:  "Single_Long_TruncateEllipsis",
			text:  "foobar foobar",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 8},
			},
			ellipses: 1,
		},
		{
			name: "Single_Long_WrapBreak",
			text: "foobar foobar",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 10},
				{10, 13},
			},
		},
		{
			name: "Single_Long_WrapWord",
			text: "foobar foobar",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
			},
		},
		{
			name: "Multiple_Short_WrapOff",
			text: "foo\nbar",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Short_Truncate",
			text: "foo\nbar",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Short_TruncateClip",
			text:  "foo\nbar",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Short_TruncateEllipsis",
			text:  "foo\nbar",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
			ellipses: 0, // too wide
		},
		{
			name: "Multiple_Short_WrapBreak",
			text: "foo\nbar",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Short_WrapWord",
			text: "foo\nbar",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Long_WrapOff",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 27},
				{28, 41},
			},
		},
		{
			name: "Multiple_Long_Truncate",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
			},
		},
		{
			name:  "Multiple_Long_TruncateClip",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
			},
		},
		{
			name:  "Multiple_Long_TruncateEllipsis",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 15},
				{28, 36},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Long_WrapBreak",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 27},
				{28, 38},
				{38, 41},
			},
		},
		{
			name: "Multiple_Long_WrapWord",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
				{14, 20},
				{21, 27},
				{28, 34},
				{35, 41},
			},
		},
		{
			name: "Multiple_Contiguous_Long_WrapOff",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 25},
				{26, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Contiguous_Long_Truncate",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 16},
				{26, 35},
				{39, 39},
			},
		},
		{
			name:  "Multiple_Contiguous_Long_TruncateClip",
			text:  "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 16},
				{26, 35},
				{39, 39},
			},
		},
		{
			name:  "Multiple_Contiguous_Long_TruncateEllipsis",
			text:  "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 15},
				{26, 34},
				{39, 39},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Contiguous_Long_WrapBreak",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 16},
				{16, 25},
				{26, 35},
				{35, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Contiguous_Long_WrapWord",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 16},
				{16, 25},
				{26, 35},
				{35, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapOff",
			text: "foo\nbar\n",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_Truncate",
			text: "foo\nbar\n",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_TruncateClip",
			text:  "foo\nbar\n",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_TruncateEllipsis",
			text:  "foo\nbar\n",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapBreak",
			text: "foo\nbar\n",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapWord",
			text: "foo\nbar\n",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_WrapTruncateClip",
			text:  "foo\nbar\nbaz",
			wrap:  bellui.TextWrapBreak,
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Trailing_Short_WrapTruncateEllipsis",
			text:  "foo\nbar\nbaz",
			wrap:  bellui.TextWrapBreak,
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
			ellipses: 0,
		},
		{
			name: "Multiple_Trailing_Long_WrapOff",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 27},
				{28, 41},
				{42, 42},
			},
		},
		{
			name: "Multiple_Trailing_Long_Truncate",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_TruncateClip",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_TruncateEllipsis",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 15},
				{28, 36},
				{42, 42},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Trailing_Long_WrapBreak",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 27},
				{28, 38},
				{38, 41},
				{42, 42},
			},
		},
		{
			name: "Multiple_Trailing_Long_WrapWord",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
				{14, 20},
				{21, 27},
				{28, 34},
				{35, 41},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_WrapTruncateClip",
			text:  "foo\nfoobar foobar foobar\nbaz",
			wrap:  bellui.TextWrapBreak,
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 14},
			},
		},
		{
			name:  "Multiple_Trailing_Long_WrapTruncateEllipsis",
			text:  "foo\nfoobar foobar foobar\nbazzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			wrap:  bellui.TextWrapBreak,
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 14},
				{14, 22},
			},
			ellipses: 1,
		},
		{
			name:  "Multiple_Trailing_Long_WrapWordTruncateEllipsis",
			text:  "foo\nfoobar foobar foobar\nbaz",
			wrap:  bellui.TextWrapWord,
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 10},
				{11, 17},
			},
			ellipses: 1,
		},
		{
			name:  "Multi_byte_ellipsis_not_truncated",
			text:  "🪃 234",
			trunc: bellui.TextTruncateEllipsis,
			wrap:  bellui.TextWrapOff,
			want: [][2]int{
				{0, 5},
			},
			ellipses: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ellipses := 0
			// Use a width that fits exactly 10 chars of "foobar foobar" at the
			// measurer's font size (14px). This keeps the expected bounds
			// independent of font-metric changes.
			const w = 66
			got, _ := lineBounds(&TextSegment{Text: tt.text}, tt.wrap, tt.trunc, w, geo.NewSize(w, 64), measurer)
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)

				if got[i].ellipsis {
					ellipses++
				}
			}
			assert.Equal(t, tt.ellipses, ellipses)
		})
	}
}

func TestText_lineBounds_variable_char_width(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		wrap  bellui.TextWrap
		trunc bellui.TextTruncation
		want  [][2]int
	}{
		{
			name: "IM_WrapOff",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: bellui.TextWrapOff,
			want: [][2]int{
				{0, 20},
			},
		},
		{
			name: "IM_Truncate",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: [][2]int{
				{0, 12},
			},
		},
		{
			name:  "IM_TruncateClip",
			text:  "iiiiiiiiiimmmmmmmmmm",
			trunc: bellui.TextTruncateClip,
			want: [][2]int{
				{0, 12},
			},
		},
		{
			name:  "IM_TruncateEllipsis",
			text:  "iiiiiiiiiimmmmmmmmmm",
			trunc: bellui.TextTruncateEllipsis,
			want: [][2]int{
				{0, 9},
			},
		},
		{
			name: "IM_WrapBreak",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: bellui.TextWrapBreak,
			want: [][2]int{
				{0, 12},
				{12, 16},
				{16, 20},
			},
		},
		{
			name: "IM_WrapWord",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: bellui.TextWrapWord,
			want: [][2]int{
				{0, 12},
				{12, 16},
				{16, 20},
			},
		},
	}
	textSize := float32(10)
	textStyle := bellui.TextStyle{}
	measurer := func(text []rune) geo.Size {
		return bellui.MeasureText(string(text), textSize, textStyle)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a width that fits exactly 12 chars (10 i's + 2 m's) at 10px.
			const w = 42
			got, _ := lineBounds(&TextSegment{Text: tt.text}, tt.wrap, tt.trunc, w, geo.NewSize(w, 184), measurer)
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)
			}
		})
	}
}

func TestText_binarySearch(t *testing.T) {
	maxWidth := float32(46)
	textSize := float32(10)
	textStyle := bellui.TextStyle{}
	measurer := func(text []rune) float32 {
		return bellui.MeasureText(string(text), textSize, textStyle).Width
	}
	for name, tt := range map[string]struct {
		text string
		want int
	}{
		"IM": {
			text: "iiiiiiiiiimmmmmmmmmm",
			want: 12,
		},
		"Single_Line": {
			text: "foobar foobar",
			want: 9,
		},
		"WH": {
			text: "wwwww hhhhhh",
			want: 7,
		},
		"DS": {
			text: "dddddd sssssss",
			want: 8,
		},
		"DI": {
			text: "dididi dididd",
			want: 11,
		},
		"XW": {
			text: "xwxwxwxw xwxw",
			want: 7,
		},
		"W": {
			text: "WWWWW",
			want: 5,
		},
		"Empty": {
			text: "",
			want: 0,
		},
	} {
		checker := func(low int, high int) bool {
			return measurer([]rune(tt.text[low:high])) <= maxWidth
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, binarySearch(checker, 0, len(tt.text)))
		})
	}
}

func TestText_findSpaceIndex(t *testing.T) {
	for name, tt := range map[string]struct {
		text string
		want int
	}{
		"no_space_fallback": {
			text: "iiiiiiiiiimmmmmmmmmm",
			want: 19,
		},
		"single_space": {
			text: "foobar foobar",
			want: 6,
		},
		"double_space": {
			text: "ww wwww www",
			want: 7,
		},
		"many_spaces": {
			text: "ww wwww www wwwww",
			want: 11,
		},
		"space beginning": {
			text: " ww",
			want: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, findSpaceIndex([]rune(tt.text), len(tt.text)-1))
		})
	}
}

// Regression test: when the first-line width is very small (e.g. previous inline
// segment consumed most of the line), text should wrap to the next full-width line
// rather than producing one character per line.
func TestText_lineBounds_narrowFirstLine(t *testing.T) {
	measurer := func(text []rune) geo.Size {
		return bellui.MeasureText(string(text), 14, bellui.TextStyle{})
	}
	seg := &TextSegment{Text: "Hello World, this is a test"}
	maxWidth := float32(500)

	// Simulate a previous segment consuming almost all of the first line,
	// leaving only 10px for the current segment.
	firstWidth := float32(10)

	for name, wrap := range map[string]bellui.TextWrap{"WrapBreak": bellui.TextWrapBreak, "WrapWord": bellui.TextWrapWord} {
		t.Run(name, func(t *testing.T) {
			bounds, _ := lineBounds(seg, wrap, bellui.TextTruncateOff, firstWidth, geo.NewSize(maxWidth, 1000), measurer)
			// The text is short enough to fit in ~1-2 full-width lines plus a
			// possible empty first-line wrap. It must NOT degenerate into
			// one-character-per-line output (which would be 27 lines for 27 chars).
			assert.LessOrEqual(t, len(bounds), 5,
				"expected a small number of lines, got %d – likely one-char-per-line regression", len(bounds))
		})
	}
}

// Regression test (#1156): a long run whose first line starts mid-row (a
// previous inline segment consumed part of the width) must wrap its
// continuation rows at the full width. Before the fix the binary-search width
// checker stayed frozen at the narrow first-line width, so every later row was
// clamped to the leftover width — one word (or a mid-word fragment) per line.
func TestText_lineBounds_narrowFirstLine_longRun(t *testing.T) {
	measurer := func(text []rune) geo.Size {
		return bellui.MeasureText(string(text), 14, bellui.TextStyle{})
	}
	seg := &TextSegment{Text: " version) are exported as data — the same export that drives editor completion, compiled-mode code generation, and bundle compatibility validation."}
	maxWidth := float32(300)
	maxSize := geo.NewSize(maxWidth, 10000)

	baseline, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, maxWidth, maxSize, measurer)
	require.Greater(t, len(baseline), 2, "test needs text long enough to wrap more than once")

	// Leave room for a couple of words on the first row, like a trailing run
	// after an inline code segment.
	narrow, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, 90, maxSize, measurer)

	assert.LessOrEqual(t, len(narrow), len(baseline)+1,
		"continuation rows must wrap at full width: got %d rows vs %d at full width", len(narrow), len(baseline))

	text := []rune(seg.Text)
	for i, b := range narrow {
		if i == len(narrow)-1 || b.begin == b.end {
			continue
		}
		assert.True(t, unicode.IsSpace(text[b.end]),
			"row %d must break at a word boundary, broke mid-word after %q", i, string(text[b.begin:b.end]))
	}
}

// Widget-level regression for #1156: an inline text run, an inline code
// segment, then a long trailing run. The trailing run's continuation rows must
// wrap at the paragraph width, not at the width left over after the code
// segment on the shared first row.
func TestRichText_WrapWordAfterInlineCode(t *testing.T) {
	test.NewTempApp(t)

	trailing := &TextSegment{Style: RichTextStyleInline, Text: " version) are exported as data — the same export that drives editor completion, compiled-mode code generation, and bundle compatibility validation."}
	rt := NewRichText(
		&TextSegment{Style: RichTextStyleInline, Text: "…that introduced each (the "},
		&TextSegment{Style: RichTextStyleCodeInline, Text: "since"},
		trailing,
	)
	rt.Wrapping = bellui.TextWrapWord
	rt.Resize(geo.NewSize(400, 400))
	test.TempWidgetRenderer(t, rt).Refresh()

	text := []rune(trailing.Text)
	var contRows []rowBoundary // rows holding only the trailing run (continuation rows)
	for _, b := range rt.rowBounds {
		if len(b.segments) != 1 || b.segments[0] != RichTextSegment(trailing) {
			continue
		}
		contRows = append(contRows, b)
		if b.end < len(text) && b.begin < b.end {
			assert.True(t, unicode.IsSpace(text[b.end]),
				"row must break at a word boundary, broke mid-word after %q", string(text[b.begin:b.end]))
		}
	}
	require.Greater(t, len(contRows), 1, "test needs the trailing run to wrap more than once")
	// Every continuation row except the last must be wrapped near the full
	// paragraph width; the pre-fix degenerate wrap clamped them to the width
	// left over beside the code segment (well under half the paragraph).
	for i, b := range contRows[:len(contRows)-1] {
		measured := bellui.MeasureText(string(text[b.begin:b.end]), theme.TextSize(), bellui.TextStyle{})
		assert.Greater(t, measured.Width, float32(200),
			"continuation row %d wrapped at the leftover first-line width: %q", i, string(text[b.begin:b.end]))
	}
}

// TestText_lineBoundsForRange_isolatedMatchesFull verifies the lazy/windowed
// measurement invariant (#157): wrapping a single logical line in isolation (a
// fresh lineWrapState, full-width first row) produces the same visual sub-row
// begin/end offsets as that line's slice of a full-document lineBounds pass. Only
// begin/end/ellipsis are compared — firstSegmentReuse is a document-wide counter
// the lazy path rebases to the absolute visual-row index.
func TestText_lineBoundsForRange_isolatedMatchesFull(t *testing.T) {
	measurer := func(text []rune) geo.Size {
		return bellui.MeasureText(string(text), 14, bellui.TextStyle{})
	}
	const maxWidth = float32(120)
	maxSize := geo.NewSize(maxWidth, 100000)

	for _, text := range []string{
		"hello world this is a reasonably long single line that wraps several times",
		"short\nlines\nhere",
		"a long first paragraph that needs wrapping\n\ntrailing empty line above\nlast",
		"line ending in newline so trailing empty row\n",
	} {
		for name, wrap := range map[string]bellui.TextWrap{"WrapWord": bellui.TextWrapWord, "WrapBreak": bellui.TextWrapBreak} {
			t.Run(name+"/"+text[:min(len(text), 12)], func(t *testing.T) {
				seg := &TextSegment{Text: text}
				// firstWidth == maxWidth so the full pass's first row uses the same
				// width the isolated per-line measurement does.
				full, _ := lineBounds(seg, wrap, bellui.TextTruncateOff, maxWidth, maxSize, measurer)

				runes := []rune(text)
				widthChecker := func(low, high int) bool {
					return measurer(runes[low:high]).Width <= maxWidth
				}
				var isolated []rowBoundary
				low := 0
				for i := 0; i <= len(runes); i++ {
					if i < len(runes) && runes[i] != '\n' {
						continue
					}
					st := lineWrapState{measureWidth: maxWidth}
					lineBoundsForRange(seg, runes, low, i, wrap, bellui.TextTruncateOff, maxSize, measurer, widthChecker, &st)
					isolated = append(isolated, st.bounds...)
					low = i + 1
				}

				require.Equal(t, len(full), len(isolated), "row count mismatch")
				for i := range full {
					assert.Equal(t, full[i].begin, isolated[i].begin, "row %d begin", i)
					assert.Equal(t, full[i].end, isolated[i].end, "row %d end", i)
					assert.Equal(t, full[i].ellipsis, isolated[i].ellipsis, "row %d ellipsis", i)
				}
			})
		}
	}
}

func TestRichText_RowBounds_NoSpuriousInvalidation(t *testing.T) {
	test.NewTempApp(t)

	seg := &TextSegment{Style: RichTextStyleInline, Text: "hello world"}
	rt := NewRichText(seg)
	rt.Resize(geo.NewSize(200, 100))

	r := test.TempWidgetRenderer(t, rt)
	r.Refresh()

	// After the first refresh, rowBoundsVersion and rowBoundsComputed should match.
	versionAfterFirst := rt.rowBoundsVersion
	computedAfterFirst := rt.rowBoundsComputed
	assert.Equal(t, versionAfterFirst, computedAfterFirst,
		"version and computed should match after first refresh")

	// A second Refresh without any segment change should NOT bump the version.
	r.Refresh()
	assert.Equal(t, versionAfterFirst, rt.rowBoundsVersion,
		"rowBoundsVersion should not change on redundant Refresh")
}

func TestRichText_CleanVisualCache_SameCount(t *testing.T) {
	test.NewTempApp(t)

	seg1 := &TextSegment{Style: RichTextStyleInline, Text: "hello"}
	seg2 := &TextSegment{Style: RichTextStyleInline, Text: "world"}
	rt := NewRichText(seg1, seg2)
	rt.Resize(geo.NewSize(200, 100))

	// Force renderer creation and cache population.
	r := test.TempWidgetRenderer(t, rt)
	r.Refresh()
	assert.Equal(t, 2, len(rt.visualCache))

	// Replace segments 1:1 — same count, different objects.
	newSeg1 := &TextSegment{Style: RichTextStyleInline, Text: "foo"}
	newSeg2 := &TextSegment{Style: RichTextStyleInline, Text: "bar"}
	rt.Segments = []RichTextSegment{newSeg1, newSeg2}
	r.Refresh()

	// Stale entries for seg1/seg2 must be cleaned even though count == count.
	for seg := range rt.visualCache {
		assert.True(t, seg == newSeg1 || seg == newSeg2,
			"stale segment %v should have been cleaned from visual cache", seg)
	}
}

// The heading virtual nodes report widget-relative bounds even when the
// RichText virtualizes behind its internal scroller: the scroll offset is
// subtracted and rows outside the viewport are dropped (#545).
func TestRichText_AccessibleVirtualNodes_InternalScroll(t *testing.T) {
	test.NewTempApp(t)
	rt := NewRichTextFromMarkdown("# Title\n\nbody")
	rt.Resize(geo.NewSize(300, 200))
	rt.Refresh()

	nodes := rt.AccessibleVirtualNodes()
	require.Len(t, nodes, 1)
	unscrolledY := nodes[0].Pos.Y

	// Fabricate the internal scroller state the virtualized path maintains.
	rt.scr = widget.NewScroll(&bellui.Container{})
	rt.scr.Resize(geo.NewSize(300, 200))
	rt.scr.Offset = geo.NewPos(0, 10)

	nodes = rt.AccessibleVirtualNodes()
	require.Len(t, nodes, 1)
	assert.Equal(t, unscrolledY-10, nodes[0].Pos.Y, "scroll offset must be subtracted")

	// Scrolled far past the heading: the node is culled.
	rt.scr.Offset = geo.NewPos(0, 5000)
	assert.Empty(t, rt.AccessibleVirtualNodes())
}

// TestRichText_MixedHeightRowKeepsSegmentHeight is the regression guard for the
// layout bug where a shorter run that is the last (grown-to-fill) segment of a
// row containing a taller run (e.g. a normal run beside a monospace one) was
// resized to the row's max height instead of its own — so the software/GPU
// painter rendered its glyphs low in the over-tall box, misaligning the text.
func TestRichText_MixedHeightRowKeepsSegmentHeight(t *testing.T) {
	test.NewTempApp(t)

	rt := NewRichText(
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Monospace: true}}, Text: "mono"},
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: " normal"}, // shorter, and last → grown to fill spare width
	)
	rt.Resize(geo.NewSize(600, 100)) // wide enough that the last run grows to absorb spare width
	r := rt.CreateRenderer()
	r.Layout(geo.NewSize(600, 100))

	var mono, normal *canvas.Text
	for _, o := range r.Objects() {
		if tx, ok := o.(*canvas.Text); ok {
			switch tx.Text {
			case "mono":
				mono = tx
			case " normal":
				normal = tx
			}
		}
	}
	if !assert.NotNil(t, mono) || !assert.NotNil(t, normal) {
		return
	}

	// Sanity: the monospace run really is the taller one (otherwise the test is vacuous).
	assert.Greater(t, mono.Size().Height, normal.Size().Height)

	// The grown last run must keep its own text height, not the row's max height.
	want, _ := bellui.CurrentApp().Driver().RenderedTextSize(normal.Text, normal.TextSize, normal.TextStyle, normal.FontSource)
	assert.InDelta(t, want.Height, normal.Size().Height, 0.01,
		"a grown-to-fill run must keep its own height, not the taller row height")
}
