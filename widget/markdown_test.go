package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

func TestRichTextMarkdown_Blockquote(t *testing.T) {
	r := NewRichTextFromMarkdown("p1\n\n> quote\n\np2")

	assert.Len(t, r.Segments, 5)
	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, "quote", text.Text)
		assert.Equal(t, RichTextStyleBlockquote, text.Style)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextMarkdown_Code(t *testing.T) {
	r := NewRichTextFromMarkdown("a `code` inline")

	assert.Len(t, r.Segments, 4)
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "code", text.Text)
		assert.Equal(t, RichTextStyleCodeInline, text.Style)
	} else {
		t.Error("Segment should be Text")
	}

	r.ParseMarkdown("``` go\ncode\nblock\n```")
	assert.Len(t, r.Segments, 1)
	if code, ok := r.Segments[0].(*CodeSegment); ok {
		assert.Equal(t, "code\nblock", code.Text)
		assert.Equal(t, "go", code.Language)
	} else {
		t.Error("Segment should be CodeSegment")
	}

	// Unrecognized languages still produce TextSegment
	r.ParseMarkdown("```\nplain\n```")
	assert.Len(t, r.Segments, 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "plain", text.Text)
		assert.Equal(t, RichTextStyleCodeBlock, text.Style)
	} else {
		t.Error("Segment should be Text for unlabeled code block")
	}
}

func TestRichTextMarkdown_Code_Incomplete(t *testing.T) {
	r := NewRichTextFromMarkdown("` ")

	assert.Len(t, r.Segments, 2)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "`", text.Text)
		assert.Equal(t, RichTextStyleInline, text.Style)
	} else {
		t.Error("Segment should be Text")
	}

	r.ParseMarkdown("``` ")
	assert.Empty(t, r.Segments)

	r.ParseMarkdown("~~~ ")
	assert.Empty(t, r.Segments)
}

func TestRichTextMarkdown_Emphasis(t *testing.T) {
	r := NewRichTextFromMarkdown("*a*")

	assert.Len(t, r.Segments, 2)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "a", text.Text)
		assert.True(t, text.Style.TextStyle.Italic)
	} else {
		t.Error("Segment should be text")
	}

	r.ParseMarkdown("**b**.")

	assert.Len(t, r.Segments, 3)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "b", text.Text)
		assert.True(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("Segment should be text")
	}
}

func TestRichTextMarkdown_Heading(t *testing.T) {
	r := NewRichTextFromMarkdown("# Head1\n\n## Head2!\n### Head3\n")

	assert.Len(t, r.Segments, 3)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "Head1", text.Text)
		assert.Equal(t, RichTextStyleHeading, text.Style)
	} else {
		t.Error("Segment should be Heading")
	}
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "Head2!", text.Text)
		assert.Equal(t, RichTextStyleSubHeading, text.Style)
	} else {
		t.Error("Segment should be SubHeading")
	}

	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, "Head3", text.Text)
		assert.True(t, text.Style.TextStyle.Bold) // we don't have 6 levels of heading so just bold others
	} else {
		t.Error("Segment should be Strong")
	}
}

func TestRichTextMarkdown_Heading_Blank(t *testing.T) {
	r := NewRichTextFromMarkdown("#")

	assert.Len(t, r.Segments, 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "", text.Text)
		assert.Equal(t, RichTextStyleHeading, text.Style)
	} else {
		t.Error("Segment should be Text")
	}

	r = NewRichTextFromMarkdown("# ")

	assert.Len(t, r.Segments, 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "", text.Text)
		assert.Equal(t, RichTextStyleHeading, text.Style)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextMarkdown_Hyperlink(t *testing.T) {
	r := NewRichTextFromMarkdown("[title](https://bellui.io/)")

	assert.Len(t, r.Segments, 2)
	if link, ok := r.Segments[0].(*HyperlinkSegment); ok {
		assert.Equal(t, "title", link.Text)
		assert.Equal(t, "bellui.io", link.URL.Host)
	} else {
		t.Error("Segment should be a Hyperlink")
	}
}

func TestRichTextMarkdown_Image(t *testing.T) {
	// A schemeless (relative or absolute path) source must NOT be turned into a
	// file:// URI — that fallback let untrusted markup read local files (#285).
	r := NewRichTextFromMarkdown("![title](../../theme/testdata/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Nil(t, img.Source)
	} else {
		t.Error("Segment should be a Image")
	}

	// An explicit file:// URI is still honoured.
	r = NewRichTextFromMarkdown("![](file:///tmp/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Equal(t, storage.NewFileURI("/tmp/bellui.png"), img.Source)
	} else {
		t.Error("Segment should be a Image")
	}

	// A remote URI is preserved on the segment (fetching is gated separately by
	// RichText.AllowRemoteContent).
	r = NewRichTextFromMarkdown("![](https://example.com/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Equal(t, "https", img.Source.Scheme())
	} else {
		t.Error("Segment should be a Image")
	}
}

func TestRichTextMarkdown_Lines(t *testing.T) {
	r := NewRichTextFromMarkdown("line1\nline2\n") // a single newline is not a new paragraph

	assert.Len(t, r.Segments, 3)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "line1 ", text.Text)
		assert.True(t, text.Inline())
	} else {
		t.Error("Segment should be Text")
	}
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "line2", text.Text)
	} else {
		t.Error("Segment should be Text")
	}

	r.ParseMarkdown("line1\n\nline2\n")

	assert.Len(t, r.Segments, 4)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "line1", text.Text)
		assert.True(t, text.Inline())
	} else {
		t.Error("Segment should be Text")
	}
	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, "line2", text.Text)
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextMarkdown_List(t *testing.T) {
	r := NewRichTextFromMarkdown("* line1 in _three_ segments\n* line2")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.Len(t, list.Items, 2)
		assert.Len(t, list.Items[0].(*ParagraphSegment).Texts, 3)
		assert.Equal(t, "line1 in ", list.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
	} else {
		t.Error("Segment should be a List")
	}

	r.ParseMarkdown("1. line1\n2. line2")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.True(t, list.Ordered)
		assert.Len(t, list.Items, 2)
		assert.Len(t, list.Items[1].(*ParagraphSegment).Texts, 1)
		assert.Equal(t, "line2", list.Items[1].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextMarkdown_ListWithDifferentStartingIndex(t *testing.T) {
	for name, tt := range map[string]struct {
		text       string
		Start      int
		startIndex int
	}{
		"Start at 2": {"2. line1\n3. line2", 2, 1},
		"Start at 1": {"1. line1\n2. line2", 1, 0},
		"Start at 0": {"0. line1\n1. line2", 0, -1},
	} {
		t.Run(name, func(t *testing.T) {
			r := NewRichTextFromMarkdown(tt.text)

			assert.Len(t, r.Segments, 1)
			if list, ok := r.Segments[0].(*ListSegment); ok {
				assert.Equal(t, tt.Start, list.StartNumber())
				assert.Equal(t, tt.startIndex, list.startIndex)
			} else {
				t.Error("Segment should be a List")
			}
		})
	}
}

func TestRichTextMarkdown_NestedList(t *testing.T) {
	r := NewRichTextFromMarkdown("* item 1\n  * item 1.1\n* item 2")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.Len(t, list.Items, 3)
		assert.Len(t, list.Items[0].(*ParagraphSegment).Texts, 1)
		assert.Equal(t, "item 1", list.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		if sublist, ok := list.Items[1].(*ListSegment); ok {
			assert.Len(t, sublist.Items, 1)
			assert.False(t, sublist.Ordered)
			assert.Equal(t, "item 1.1", sublist.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		} else {
			t.Error("Segment should be a List")
		}
		assert.Len(t, list.Items[1].(*ListSegment).Items, 1)
	} else {
		t.Error("Segment should be a List")
	}

	r.ParseMarkdown("1. item 1\n   - item 1.1\n2. item 2")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.True(t, list.Ordered)
		assert.Len(t, list.Items, 3)
		assert.Len(t, list.Items[0].(*ParagraphSegment).Texts, 1)
		if sublist, ok := list.Items[1].(*ListSegment); ok {
			assert.Len(t, sublist.Items, 1)
			assert.False(t, sublist.Ordered)
			assert.Equal(t, "item 1.1", sublist.Items[0].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
		} else {
			t.Error("Segment should be a List")
		}
		assert.Equal(t, "item 2", list.Items[2].(*ParagraphSegment).Texts[0].(*TextSegment).Text)
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextMarkdown_Separator(t *testing.T) {
	r := NewRichTextFromMarkdown("---\n")

	assert.Len(t, r.Segments, 1)
	if _, ok := r.Segments[0].(*SeparatorSegment); !ok {
		t.Error("Segment should be a separator")
	}
}

func TestRichTextMarkdown_TextEncode(t *testing.T) {
	r := NewRichTextFromMarkdown("a&mdash;b")

	assert.GreaterOrEqual(t, len(r.Segments), 1)
	if _, ok := r.Segments[0].(*TextSegment); !ok {
		t.Error("Segment should be a text segment")
	}

	assert.Equal(t, "a—b", r.Segments[0].(*TextSegment).Text)
}

func TestRichTextMarkdown_Paragraph(t *testing.T) {
	r := NewRichTextFromMarkdown("foo")

	assert.Len(t, r.Segments, 2)
	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "", text.Text)
		assert.Equal(t, RichTextStyleParagraph, text.Style)
	} else {
		t.Error("Last segment of paragraph should be empty text")
	}
}

func TestRichTextMarkdown_NewlinesAroundEmphasis(t *testing.T) {
	r := NewRichTextFromMarkdown("foo\n*bar*\nbaz")
	assert.Equal(t, "foo bar baz", r.String())
}

func TestRichTextMarkdown_NewlinesAroundStrong(t *testing.T) {
	r := NewRichTextFromMarkdown("foo\n**bar**\nbaz")
	assert.Equal(t, "foo bar baz", r.String())
}

func TestRichTextMarkdown_NewlinesAroundHyperlink(t *testing.T) {
	r := NewRichTextFromMarkdown("foo\n[bar](https://bellui.io/)\nbaz")
	assert.Equal(t, "foo bar baz", r.String())
}

func TestRichTextMarkdown_SpacesAroundHyperlink(t *testing.T) {
	r := NewRichTextFromMarkdown("foo [bar](https://bellui.io/) baz")
	assert.Equal(t, "foo bar baz", r.String())
}

func TestRichTextMarkdown_NewlineInHyperlink(t *testing.T) {
	r := NewRichTextFromMarkdown("[foo\nbar](https://bellui.io/)")
	assert.Equal(t, "foo bar", r.String())
}

func TestRichTextMarkdown_Table(t *testing.T) {
	r := NewRichTextFromMarkdown("| Name | Age |\n| --- | --- |\n| Alice | 30 |\n| Bob | 25 |")

	assert.Len(t, r.Segments, 1)
	tbl, ok := r.Segments[0].(*TableSegment)
	if !ok {
		t.Fatal("Segment should be a TableSegment")
	}
	assert.Equal(t, []string{"Name", "Age"}, tbl.Header)
	assert.Len(t, tbl.Rows, 2)
	assert.Equal(t, []string{"Alice", "30"}, tbl.Rows[0])
	assert.Equal(t, []string{"Bob", "25"}, tbl.Rows[1])
}

func TestRichTextMarkdown_Table_Alignment(t *testing.T) {
	r := NewRichTextFromMarkdown("| Left | Center | Right |\n| :--- | :---: | ---: |\n| a | b | c |")

	assert.Len(t, r.Segments, 1)
	tbl, ok := r.Segments[0].(*TableSegment)
	if !ok {
		t.Fatal("Segment should be a TableSegment")
	}
	assert.Equal(t, bellui.TextAlignLeading, tbl.Alignments[0])
	assert.Equal(t, bellui.TextAlignCenter, tbl.Alignments[1])
	assert.Equal(t, bellui.TextAlignTrailing, tbl.Alignments[2])
}

func TestRichTextMarkdown_Table_Visual(t *testing.T) {
	r := NewRichTextFromMarkdown("| A | B |\n| --- | --- |\n| 1 | 2 |")

	tbl := r.Segments[0].(*TableSegment)
	vis := tbl.Visual()
	assert.NotNil(t, vis)
	assert.False(t, tbl.Inline())

	rt := vis.(*richTable)
	assert.Equal(t, []string{"A", "B"}, rt.header)
}

// Regression test: mid-word underscores (e.g. "exit_prob") must not produce
// spurious spaces. Goldmark splits text at delimiter boundaries even when no
// emphasis is created, and we must avoid inserting a space between those fragments.
func TestRichTextMarkdown_UnderscoreInWord(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		want     string
	}{
		{
			name:     "simple_underscore",
			markdown: "exit_prob",
			want:     "exit_prob",
		},
		{
			name:     "multiple_underscores",
			markdown: "some_parameter_name",
			want:     "some_parameter_name",
		},
		{
			name:     "underscore_in_sentence",
			markdown: "The exit_prob value is 0.5",
			want:     "The exit_prob value is 0.5",
		},
		{
			name:     "table_with_underscore",
			markdown: "| Key | Value |\n| --- | --- |\n| exit_prob | 0.5 |",
			want:     "exit_prob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRichTextFromMarkdown(tt.markdown)
			if tt.name == "table_with_underscore" {
				tbl, ok := r.Segments[0].(*TableSegment)
				if !ok {
					t.Fatal("Segment should be a TableSegment")
				}
				assert.Equal(t, tt.want, tbl.Rows[0][0])
			} else {
				text := r.String()
				assert.Contains(t, text, tt.want,
					"expected %q to appear without spurious spaces in %q", tt.want, text)
			}
		})
	}
}

func TestRichTextMarkdown_BoldItalic(t *testing.T) {
	r := NewRichTextFromMarkdown("***bold italic***")

	assert.GreaterOrEqual(t, len(r.Segments), 1)
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "bold italic", text.Text)
		assert.True(t, text.Style.TextStyle.Bold, "should be bold")
		assert.True(t, text.Style.TextStyle.Italic, "should be italic")
	} else {
		t.Error("Segment should be Text")
	}
}

func TestRichTextMarkdown_BoldInsideItalic(t *testing.T) {
	r := NewRichTextFromMarkdown("*italic **bold italic** italic*")

	assert.GreaterOrEqual(t, len(r.Segments), 3)

	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "italic ", text.Text)
		assert.True(t, text.Style.TextStyle.Italic)
		assert.False(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("First segment should be italic text")
	}

	if text, ok := r.Segments[1].(*TextSegment); ok {
		assert.Equal(t, "bold italic", text.Text)
		assert.True(t, text.Style.TextStyle.Bold, "should be bold")
		assert.True(t, text.Style.TextStyle.Italic, "should be italic")
	} else {
		t.Error("Second segment should be bold+italic text")
	}

	if text, ok := r.Segments[2].(*TextSegment); ok {
		assert.Equal(t, " italic", text.Text)
		assert.True(t, text.Style.TextStyle.Italic)
		assert.False(t, text.Style.TextStyle.Bold)
	} else {
		t.Error("Third segment should be italic text")
	}
}

func TestRichTextMarkdown_HeadingWithLink(t *testing.T) {
	r := NewRichTextFromMarkdown("# Heading with [link](https://bellui.io/)")

	found := false
	for _, seg := range r.Segments {
		if link, ok := seg.(*HyperlinkSegment); ok {
			assert.Equal(t, "link", link.Text)
			assert.Equal(t, "bellui.io", link.URL.Host)
			found = true
			break
		}
	}
	assert.True(t, found, "Heading should preserve link")

	// The text part should have heading style
	if text, ok := r.Segments[0].(*TextSegment); ok {
		assert.Equal(t, "Heading with ", text.Text)
		assert.Equal(t, RichTextStyleHeading, text.Style)
	} else {
		t.Error("First segment should be heading text")
	}
}

func TestRichTextMarkdown_Strikethrough(t *testing.T) {
	r := NewRichTextFromMarkdown("~~deleted~~")

	// Strikethrough text is preserved (markers dropped) and carries the
	// Strikethrough text style so it renders struck through.
	text := r.String()
	assert.Contains(t, text, "deleted")
	assert.NotContains(t, text, "~~")

	if assert.GreaterOrEqual(t, len(r.Segments), 1) {
		if seg, ok := r.Segments[0].(*TextSegment); assert.True(t, ok, "first segment should be text") {
			assert.Equal(t, "deleted", seg.Text)
			assert.True(t, seg.Style.TextStyle.Strikethrough, "should be struck through")
		}
	}
}

func TestRichTextMarkdown_StrikethroughComposesWithBold(t *testing.T) {
	r := NewRichTextFromMarkdown("**~~gone~~**")

	if assert.GreaterOrEqual(t, len(r.Segments), 1) {
		if seg, ok := r.Segments[0].(*TextSegment); assert.True(t, ok, "first segment should be text") {
			assert.Equal(t, "gone", seg.Text)
			assert.True(t, seg.Style.TextStyle.Strikethrough, "should be struck through")
			assert.True(t, seg.Style.TextStyle.Bold, "should also be bold")
		}
	}
}

func TestRichTextMarkdown_TaskList(t *testing.T) {
	r := NewRichTextFromMarkdown("- [x] done\n- [ ] todo")

	assert.Len(t, r.Segments, 1)
	if list, ok := r.Segments[0].(*ListSegment); ok {
		assert.Len(t, list.Items, 2)
		// First item should have checkbox prefix
		para := list.Items[0].(*ParagraphSegment)
		assert.GreaterOrEqual(t, len(para.Texts), 2)
		assert.Equal(t, "☑ ", para.Texts[0].(*TextSegment).Text)

		// Second item
		para2 := list.Items[1].(*ParagraphSegment)
		assert.GreaterOrEqual(t, len(para2.Texts), 2)
		assert.Equal(t, "☐ ", para2.Texts[0].(*TextSegment).Text)
	} else {
		t.Error("Segment should be a List")
	}
}

func TestRichTextMarkdown_AutoLink(t *testing.T) {
	r := NewRichTextFromMarkdown("<https://bellui.io/>")

	found := false
	for _, seg := range r.Segments {
		if link, ok := seg.(*HyperlinkSegment); ok {
			assert.Equal(t, "https://bellui.io/", link.Text)
			assert.Equal(t, "bellui.io", link.URL.Host)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find auto-link")
}

func TestRichTextMarkdown_HTMLInline(t *testing.T) {
	r := NewRichTextFromMarkdown("text <strong>bold</strong> more")

	text := r.String()
	assert.Contains(t, text, "bold")
	assert.Contains(t, text, "text")
}

func TestRichTextMarkdown_HTMLBlock(t *testing.T) {
	r := NewRichTextFromMarkdown("<div>\n<p>hello</p>\n</div>")

	text := r.String()
	assert.Contains(t, text, "hello")
}

func BenchmarkMarkdownParsing(b *testing.B) {
	md := `# Test heading
This is some test markdown. It contains some different markdown
elements in an effort to put a realistic load onto the parser.

> Here is a quote.
> It stretches over two lines and contains some ` + "`inline code`" + `.

Moreover, we've got a list:
- foo
- bar
- baz
`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewRichTextFromMarkdown(md)
	}
}
