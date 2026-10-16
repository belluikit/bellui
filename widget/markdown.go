package widget

import (
	"html"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	xhtml "golang.org/x/net/html"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// NewRichTextFromMarkdown configures a RichText widget by parsing the provided markdown content.
//
// Since: 1.0
func NewRichTextFromMarkdown(content string) *RichText {
	return NewRichText(parseMarkdown(content, nil)...)
}

// ParseMarkdown allows setting the content of this RichText widget from a markdown string.
// It will replace the content of this widget similarly to SetText, but with the appropriate formatting.
func (t *RichText) ParseMarkdown(content string) {
	if content == t.markdownCache {
		return // identical content — skip expensive rebuild
	}
	t.markdownCache = content
	t.Segments = parseMarkdown(content, t.namedFonts)
	t.rowBoundsVersion++ // invalidate row bounds cache
	t.visualCache = nil  // new segments — old cache keys are stale
	t.Refresh()
}

// AppendMarkdown parses the given markdown string and appends the
// content to the widget, with the appropriate formatting.
// This API is intended for appending complete markdown documents or
// standalone fragments, and should not be used to parse a single
// markdown document piecewise.
//
// Since: 1.0
func (t *RichText) AppendMarkdown(content string) {
	t.Segments = append(t.Segments, parseMarkdown(content, t.namedFonts)...)
	t.markdownCache = "" // invalidate — content no longer matches a single source
	t.rowBoundsVersion++
	t.Refresh()
}

// markdownRenderer accumulates the parsed segments and carries the per-RichText
// named-font allowlist (#496) through the walk so inline HTML in markdown can
// resolve custom fonts via the shared HTML parser.
type markdownRenderer struct {
	segments []RichTextSegment
	named    map[string]bellui.Resource
}

func (m *markdownRenderer) AddOptions(...renderer.Option) {}

func (m *markdownRenderer) Render(_ io.Writer, source []byte, n ast.Node) error {
	segs, err := renderNode(source, n, false, 0, m.named)
	m.segments = segs
	return err
}

func renderNode(source []byte, n ast.Node, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	switch t := n.(type) {
	case *ast.Document, *ast.TextBlock:
		return renderChildren(source, n, blockquote, listDepth, named)
	case *ast.Paragraph:
		return renderParagraph(source, n, blockquote, listDepth, named)
	case *ast.List:
		return renderList(source, t, blockquote, listDepth, named)
	case *ast.ListItem:
		return renderListItem(source, n, blockquote, listDepth, named)
	case *ast.Heading:
		return renderHeading(source, t, blockquote, listDepth, named)
	case *ast.ThematicBreak:
		return []RichTextSegment{&SeparatorSegment{}}, nil
	case *ast.Link:
		return renderLink(source, t, n), nil
	case *ast.AutoLink:
		return renderAutoLink(source, t), nil
	case *ast.CodeSpan:
		text := forceIntoText(source, n)
		return []RichTextSegment{&TextSegment{Style: RichTextStyleCodeInline, Text: text}}, nil
	case *ast.CodeBlock, *ast.FencedCodeBlock:
		return renderCodeBlock(source, n), nil
	case *ast.Emphasis:
		children, err := renderChildren(source, n, blockquote, listDepth, named)
		applyEmphasisStyle(children, t.Level)
		return children, err
	case *ast.Text:
		return renderText(source, t, n, blockquote), nil
	case *ast.Blockquote:
		return renderChildren(source, n, true, listDepth, named)
	case *ast.Image:
		return parseMarkdownImage(t), nil
	case *ast.RawHTML:
		return renderRawHTML(source, t, named), nil
	case *ast.HTMLBlock:
		return renderHTMLBlock(source, n, named), nil
	case *east.Table:
		return renderTable(source, t), nil
	case *east.TableHeader, *east.TableRow, *east.TableCell:
		// handled inside renderTable
		return nil, nil
	case *east.Strikethrough:
		children, err := renderChildren(source, n, blockquote, listDepth, named)
		applyStrikethroughStyle(children)
		return children, err
	case *east.TaskCheckBox:
		return renderTaskCheckBox(t), nil
	}
	return nil, nil
}

func renderParagraph(source []byte, n ast.Node, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	children, err := renderChildren(source, n, blockquote, listDepth, named)
	if !blockquote {
		linebreak := &TextSegment{Style: RichTextStyleParagraph}
		children = append(children, linebreak)
	}
	return children, err
}

func renderList(source []byte, t *ast.List, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	items, err := renderChildren(source, t, blockquote, listDepth+1, named)
	return []RichTextSegment{
		&ListSegment{startIndex: t.Start - 1, Items: items, Ordered: t.Marker != '*' && t.Marker != '-' && t.Marker != '+', indentationLevel: listDepth},
	}, err
}

func renderListItem(source []byte, n ast.Node, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	children, err := renderChildren(source, n, blockquote, listDepth, named)
	var texts []RichTextSegment
	var sublist RichTextSegment
	for _, child := range children {
		// check if child is a sub-list
		if _, ok := child.(*ListSegment); ok {
			sublist = child
		} else {
			texts = append(texts, child)
		}
	}
	result := []RichTextSegment{&ParagraphSegment{Texts: texts}}
	if sublist != nil {
		result = append(result, sublist)
	}
	return result, err
}

func renderHeading(source []byte, t *ast.Heading, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	children := renderHeadingChildren(source, t, blockquote, listDepth, named)
	if len(children) == 0 {
		children = []RichTextSegment{&TextSegment{Text: ""}}
	}
	return applyHeadingStyle(children, t.Level), nil
}

func renderLink(source []byte, t *ast.Link, n ast.Node) []RichTextSegment {
	link := parsedLinkURL(string(t.Destination))
	text := forceIntoText(source, n)
	return []RichTextSegment{&HyperlinkSegment{Alignment: bellui.TextAlignLeading, Text: decodeText(text), URL: link}}
}

func renderAutoLink(source []byte, t *ast.AutoLink) []RichTextSegment {
	linkText := string(t.URL(source))
	dest := linkText
	if t.AutoLinkType == ast.AutoLinkEmail && !strings.Contains(linkText, ":") {
		dest = "mailto:" + linkText
	}
	u := parsedLinkURL(dest)
	return []RichTextSegment{&HyperlinkSegment{Alignment: bellui.TextAlignLeading, Text: linkText, URL: u}}
}

func renderCodeBlock(source []byte, n ast.Node) []RichTextSegment {
	var data []byte
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		data = append(data, line.Value(source)...)
	}
	if len(data) == 0 {
		return nil
	}
	if data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	// Use CodeSegment with syntax highlighting when a language is specified
	if fenced, ok := n.(*ast.FencedCodeBlock); ok {
		lang := string(fenced.Language(source))
		if lang != "" && codeLexerForLanguage(lang) != nil {
			return []RichTextSegment{&CodeSegment{Text: string(data), Language: lang}}
		}
	}
	return []RichTextSegment{&TextSegment{Style: RichTextStyleCodeBlock, Text: string(data)}}
}

func renderText(source []byte, t *ast.Text, n ast.Node, blockquote bool) []RichTextSegment {
	text := string(t.Value(source))
	if text == "" {
		// These empty text elements indicate single line breaks after non-text elements in goldmark.
		return []RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: " "}}
	}
	text = suffixSpaceIfAppropriate(text, n)
	if blockquote {
		return []RichTextSegment{&TextSegment{Style: RichTextStyleBlockquote, Text: text}}
	}
	return []RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: decodeText(text)}}
}

func renderTaskCheckBox(t *east.TaskCheckBox) []RichTextSegment {
	prefix := "☐ "
	if t.IsChecked {
		prefix = "☑ "
	}
	return []RichTextSegment{&TextSegment{Style: RichTextStyleInline, Text: prefix}}
}

// applyEmphasisStyle applies bold or italic styling to all TextSegments in the slice.
// This allows nested emphasis (e.g. ***bold italic***) to compose correctly.
func applyEmphasisStyle(segs []RichTextSegment, level int) {
	for _, seg := range segs {
		if text, ok := seg.(*TextSegment); ok {
			switch level {
			case 2:
				text.Style.TextStyle.Bold = true
			default:
				text.Style.TextStyle.Italic = true
			}
			text.Style.Inline = true
		}
	}
}

// applyStrikethroughStyle marks all TextSegments in the slice as struck
// through, so nested strikethrough (e.g. **~~bold struck~~**) composes.
func applyStrikethroughStyle(segs []RichTextSegment) {
	for _, seg := range segs {
		if text, ok := seg.(*TextSegment); ok {
			text.Style.TextStyle.Strikethrough = true
			text.Style.Inline = true
		}
	}
}

// renderHeadingChildren renders the children of a heading node.
// Consecutive text nodes are merged into a single TextSegment (without space insertion),
// while non-text children (links, emphasis, etc.) are rendered normally.
func renderHeadingChildren(source []byte, n ast.Node, blockquote bool, listDepth int, named map[string]bellui.Resource) []RichTextSegment {
	var segs []RichTextSegment
	var textBuf strings.Builder
	// Track inline <span>/<font> font state across sibling nodes, mirroring
	// renderChildren, so a bare inline named-font span inside a heading resolves
	// its font (#870). The buffered text is flushed at each span boundary so the
	// font applies to exactly the wrapped run.
	var fonts mdFontStack

	flushText := func() {
		if textBuf.Len() > 0 {
			ts := &TextSegment{Text: decodeText(textBuf.String())}
			if src := fonts.current(); src != nil {
				ts.Style.FontSource = src
			}
			segs = append(segs, ts)
			textBuf.Reset()
		}
	}

	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if raw, ok := child.(*ast.RawHTML); ok {
			if closing, family, isSpan := inlineSpanFont(rawHTMLString(source, raw)); isSpan {
				flushText() // the span boundary splits the buffered text
				if closing {
					fonts.pop()
				} else {
					fonts.push(resolveNamedFont(family, named))
				}
				continue
			}
		}
		if t, ok := child.(*ast.Text); ok {
			textBuf.Write(t.Value(source))
			continue
		}
		flushText()
		childSegs, _ := renderNode(source, child, blockquote, listDepth, named)
		if src := fonts.current(); src != nil {
			applyInlineFont(childSegs, src)
		}
		segs = append(segs, childSegs...)
	}
	flushText()
	return segs
}

// applyHeadingStyle converts rendered children into heading segments.
// TextSegments get the heading style applied; other segments (links, etc.) are kept as-is.
func applyHeadingStyle(segs []RichTextSegment, level int) []RichTextSegment {
	for _, seg := range segs {
		if text, ok := seg.(*TextSegment); ok {
			// Preserve a per-run named font (#870) across the whole-Style
			// assignment below, which would otherwise wipe it.
			src := text.Style.FontSource
			switch level {
			case 1:
				text.Style = RichTextStyleHeading
			case 2:
				text.Style = RichTextStyleSubHeading
			default:
				text.Style = RichTextStyle{
					ColorName: theme.ColorNameOnSurface,
					Inline:    false,
					SizeName:  theme.SizeNameTitleSmall,
					TextStyle: bellui.TextStyle{Bold: true},
				}
			}
			text.Style.HeadingLevel = level
			if src != nil {
				text.Style.FontSource = src
			}
		}
	}
	return segs
}

// renderRawHTML handles inline HTML in markdown by delegating to the HTML parser.
// Recognised inline <span>/<font> open/close tags are intercepted earlier by
// renderChildren's font state machine (#652) and never reach here.
func renderRawHTML(source []byte, n *ast.RawHTML, named map[string]bellui.Resource) []RichTextSegment {
	return parseHTML(rawHTMLString(source, n), named)
}

// rawHTMLString concatenates the literal source spans of an inline RawHTML node.
func rawHTMLString(source []byte, n *ast.RawHTML) string {
	var b strings.Builder
	segs := n.Segments
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

// renderHTMLBlock handles block-level HTML in markdown by delegating to the HTML parser.
func renderHTMLBlock(source []byte, n ast.Node, named map[string]bellui.Resource) []RichTextSegment {
	var data []byte
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		data = append(data, line.Value(source)...)
	}
	if n.HasChildren() {
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if raw, ok := child.(*ast.RawHTML); ok {
				segs := raw.Segments
				for i := 0; i < segs.Len(); i++ {
					seg := segs.At(i)
					data = append(data, seg.Value(source)...)
				}
			}
		}
	}
	return parseHTML(string(data), named)
}

func suffixSpaceIfAppropriate(text string, n ast.Node) string {
	next := n.NextSibling()
	if next != nil && next.Type() == ast.TypeInline && !strings.HasSuffix(text, " ") {
		// Don't add a space when text ends with a potential emphasis delimiter
		// (underscore or asterisk). Goldmark splits text at delimiter boundaries
		// even when no emphasis is created, e.g., "exit_prob" → ["exit_", "prob"].
		if strings.HasSuffix(text, "_") || strings.HasSuffix(text, "*") {
			return text
		}
		return text + " "
	}
	return text
}

func renderChildren(source []byte, n ast.Node, blockquote bool, listDepth int, named map[string]bellui.Resource) ([]RichTextSegment, error) {
	children := make([]RichTextSegment, 0, n.ChildCount())
	// Track open/close inline-HTML <span>/<font> state across sibling nodes so a
	// named font (#496) selected by a bare inline <span style="font-family:…"> …
	// </span> reaches the markdown Text between the tags. goldmark parses inline
	// raw HTML as *separate* RawHTML nodes for the open and close tags with the
	// styled text as an ordinary Text sibling, so per-node delegation to the HTML
	// parser never nests the style — this small stack bridges that gap (#652).
	var fonts mdFontStack
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if raw, ok := child.(*ast.RawHTML); ok {
			if closing, family, isSpan := inlineSpanFont(rawHTMLString(source, raw)); isSpan {
				if closing {
					fonts.pop()
				} else {
					fonts.push(resolveNamedFont(family, named))
				}
				continue // consume the span/font marker; it produces no visible segment
			}
		}
		segs, err := renderNode(source, child, blockquote, listDepth, named)
		if err != nil {
			return children, err
		}
		if src := fonts.current(); src != nil {
			applyInlineFont(segs, src)
		}
		children = append(children, segs...)
	}
	return children, nil
}

// mdFontStack is the inline-HTML <span>/<font> font state for one sibling walk
// (#652). Its top entry is the *effective* font: pushing a span/font tag that
// carries no registered family inherits the enclosing font, so open/close pairs
// stay balanced and nested spans resolve innermost-wins.
type mdFontStack struct{ stack []bellui.Resource }

func (f *mdFontStack) current() bellui.Resource {
	if len(f.stack) == 0 {
		return nil
	}
	return f.stack[len(f.stack)-1]
}

func (f *mdFontStack) push(res bellui.Resource) {
	if res == nil {
		res = f.current() // no family on this tag — inherit the enclosing one
	}
	f.stack = append(f.stack, res)
}

func (f *mdFontStack) pop() {
	if n := len(f.stack); n > 0 {
		f.stack = f.stack[:n-1]
	}
}

// applyInlineFont sets the resolved inline-HTML font on the text-bearing
// segments produced between a <span>/<font> open and its close, leaving any
// segment that already carries an explicit font untouched.
func applyInlineFont(segs []RichTextSegment, src bellui.Resource) {
	for _, seg := range segs {
		switch s := seg.(type) {
		case *TextSegment:
			if s.Style.FontSource == nil {
				s.Style.FontSource = src
			}
		case *HyperlinkSegment: // links carry their own style since #651
			if s.Style.FontSource == nil {
				s.Style.FontSource = src
			}
		}
	}
}

// inlineSpanFont classifies a single inline raw-HTML tag string. It reports
// whether the tag is a <span>/<font> open or close (ok), whether it is the
// closing tag, and — for an opening tag — the font-family it selects (from a
// <span style="font-family:…"> declaration or a <font face="…"> attribute).
// Any other tag returns ok=false so the caller keeps delegating it to the HTML
// parser unchanged.
func inlineSpanFont(raw string) (closing bool, family string, ok bool) {
	z := xhtml.NewTokenizer(strings.NewReader(raw))
	switch z.Next() {
	case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
		name, hasAttr := z.TagName()
		tag := strings.ToLower(string(name))
		if tag != "span" && tag != "font" {
			return false, "", false
		}
		for hasAttr {
			var k, v []byte
			k, v, hasAttr = z.TagAttr()
			switch {
			case tag == "span" && strings.EqualFold(string(k), "style"):
				family = fontFamilyFromStyle(string(v))
			case tag == "font" && strings.EqualFold(string(k), "face"):
				family = string(v)
			}
		}
		return false, family, true
	case xhtml.EndTagToken:
		name, _ := z.TagName()
		tag := strings.ToLower(string(name))
		if tag != "span" && tag != "font" {
			return false, "", false
		}
		return true, "", true
	}
	return false, "", false
}

func forceIntoText(source []byte, n ast.Node) string {
	text := strings.Builder{}
	ast.Walk(n, func(n2 ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch t := n2.(type) {
			case *ast.Text:
				val := t.Value(source)
				// Only add a space between text fragments when the previous
				// text doesn't end with a potential delimiter character
				// (underscore, asterisk). Goldmark splits text at emphasis
				// delimiter boundaries even when no emphasis is created,
				// e.g., "exit_prob" becomes ["exit_", "prob"]. Adding a
				// space between these fragments produces "exit_ prob".
				if text.Len() > 0 {
					prev := text.String()
					lastChar := prev[len(prev)-1]
					if lastChar != '_' && lastChar != '*' && len(val) > 0 && val[0] != '_' && val[0] != '*' {
						text.WriteByte(' ')
					}
				}
				text.Write(val)
			}
		}
		return ast.WalkContinue, nil
	})
	return text.String()
}

func parseMarkdown(content string, named map[string]bellui.Resource) []RichTextSegment {
	r := markdownRenderer{named: named}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithRenderer(&r),
	)
	err := md.Convert([]byte(content), nil)
	if err != nil {
		bellui.LogError("Failed to parse markdown", err)
	}
	return r.segments
}

func renderTable(source []byte, table *east.Table) []RichTextSegment {
	alignments := make([]bellui.TextAlign, len(table.Alignments))
	for i, a := range table.Alignments {
		switch a {
		case east.AlignCenter:
			alignments[i] = bellui.TextAlignCenter
		case east.AlignRight:
			alignments[i] = bellui.TextAlignTrailing
		default:
			alignments[i] = bellui.TextAlignLeading
		}
	}

	var header []string
	var rows [][]string

	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.(type) {
		case *east.TableHeader:
			header = extractTableCells(source, child)
		case *east.TableRow:
			rows = append(rows, extractTableCells(source, child))
		}
	}

	return []RichTextSegment{&TableSegment{
		Header:     header,
		Rows:       rows,
		Alignments: alignments,
	}}
}

func extractTableCells(source []byte, row ast.Node) []string {
	var cells []string
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		cells = append(cells, strings.TrimSpace(forceIntoText(source, cell)))
	}
	return cells
}

func decodeText(text string) string {
	return html.UnescapeString(text)
}
