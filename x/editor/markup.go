package editor

import (
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// naturalRichSize returns a RichText's natural (unwrapped) size — the extent it
// wants before any word-wrapping — via fyne's [widget.RichText.IntrinsicSize].
// A word-wrapping RichText reports MinSize().Width as ~0, so the hover and
// signature cards size themselves from this and then flow the wrapped height
// within the capped width (#103; the API landed in fyne v2.10.1 / fyne#192,
// replacing fycoder's earlier refresh-toggle workaround).
func naturalRichSize(rt *widget.RichText) geo.Size {
	return rt.IntrinsicSize()
}

// MarkupKind says how a documentation string should be rendered. It is the
// single representation shared by the hover card, the completion documentation
// panel, and signature-help docs, so plaintext-vs-markdown is expressed the same
// way across all three surfaces (#87).
type MarkupKind int

const (
	// MarkupPlainText renders the value verbatim. It is the zero value, so a doc
	// string with no declared kind is shown as-is rather than parsed as markdown.
	MarkupPlainText MarkupKind = iota
	// MarkupMarkdown renders the value as markdown.
	MarkupMarkdown
)

// boolMarkupKind maps the convenience "is markdown?" booleans carried by
// [CompletionItem], [SignatureInformation], and [ParameterInformation] to a
// [MarkupKind].
func boolMarkupKind(markdown bool) MarkupKind {
	if markdown {
		return MarkupMarkdown
	}
	return MarkupPlainText
}

// markupSegments turns a documentation string into rich-text segments — parsed
// as markdown or shown verbatim — so a value can be embedded among other
// segments (e.g. beneath a signature label).
func markupSegments(value string, kind MarkupKind) []widget.RichTextSegment {
	if kind == MarkupMarkdown {
		return widget.NewRichTextFromMarkdown(value).Segments
	}
	return []widget.RichTextSegment{&widget.TextSegment{Text: value}}
}

// setMarkup replaces a RichText's whole content with value rendered per kind.
func setMarkup(rt *widget.RichText, value string, kind MarkupKind) {
	rt.Segments = markupSegments(value, kind)
	rt.Refresh()
}
