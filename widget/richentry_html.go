package widget

import (
	"html"
	"strings"

	"github.com/belluikit/bellui"
)

// segsToHTML serializes a document fragment to HTML for the clipboard (pre-rebrand tracker #666).
// Text runs carry their inline styling (bold/italic/underline/strikethrough) and
// newlines become <br>; non-text atoms (images) are omitted — a single-image
// selection is written to the image flavor instead, and mixed image round-trip
// through HTML is a follow-up.
func segsToHTML(segs []RichTextSegment) string {
	var b strings.Builder
	for _, seg := range segs {
		ts, ok := seg.(*TextSegment)
		if !ok {
			continue
		}
		text := html.EscapeString(ts.Text)
		text = strings.ReplaceAll(text, "\n", "<br>")
		st := ts.Style.TextStyle
		if st.Strikethrough {
			text = "<s>" + text + "</s>"
		}
		if st.Underline {
			text = "<u>" + text + "</u>"
		}
		if st.Italic {
			text = "<i>" + text + "</i>"
		}
		if styleIsBold(st) {
			text = "<b>" + text + "</b>"
		}
		b.WriteString(text)
	}
	return b.String()
}

func styleIsBold(st bellui.TextStyle) bool {
	return st.Bold || (st.Weight != nil && *st.Weight >= bellui.WeightBold)
}

// flattenHTMLToInline converts the segments produced by [NewRichTextFromHTML]
// into RichEntry's inline model (pre-rebrand tracker #657): inline text runs keep their style, block
// boundaries (empty non-inline separator segments) become '\n', images pass
// through as atoms, and any other segment contributes its plain text. A single
// trailing newline is trimmed so a wrapped paste doesn't add a blank line.
func flattenHTMLToInline(segs []RichTextSegment) []RichTextSegment {
	var out []RichTextSegment
	for _, seg := range segs {
		switch v := seg.(type) {
		case *TextSegment:
			st := v.Style
			st.Inline = true
			if v.Text != "" {
				out = append(out, &TextSegment{Style: st, Text: v.Text})
			}
			if !v.Style.Inline { // a block segment ends a paragraph
				out = append(out, newlineSegment())
			}
		case *ImageSegment:
			out = append(out, cloneSegment(v))
		default:
			if t := seg.Textual(); t != "" {
				out = append(out, &TextSegment{Style: RichTextStyle{Inline: true}, Text: t})
			}
		}
	}
	// trim one trailing paragraph break
	if n := len(out); n > 0 {
		if ts, ok := out[n-1].(*TextSegment); ok && ts.Text == "\n" {
			out = out[:n-1]
		}
	}
	return out
}

func newlineSegment() RichTextSegment {
	return &TextSegment{Style: RichTextStyle{Inline: true}, Text: "\n"}
}
