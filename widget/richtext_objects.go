package widget

import (
	"image"
	"image/color"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/theme"
)

var (
	// RichTextStyleBlockquote represents a quote presented in an indented block.
	//
	// Since: 1.0
	RichTextStyleBlockquote = RichTextStyle{
		ColorName: theme.ColorNameOnSurfaceVariant,
		Inline:    false,
		SizeName:  theme.SizeNameBodyMedium,
		TextStyle: bellui.TextStyle{Italic: true},
	}
	// RichTextStyleCodeBlock represents a code blog segment.
	//
	// Since: 1.0
	RichTextStyleCodeBlock = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    false,
		SizeName:  theme.SizeNameBodyMedium,
		TextStyle: bellui.TextStyle{Monospace: true},
	}
	// RichTextStyleCodeInline represents an inline code segment.
	//
	// Since: 1.0
	RichTextStyleCodeInline = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    true,
		SizeName:  theme.SizeNameBodyMedium,
		TextStyle: bellui.TextStyle{Monospace: true},
	}
	// RichTextStyleEmphasis represents regular text with emphasis.
	//
	// Since: 1.0
	RichTextStyleEmphasis = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    true,
		SizeName:  theme.SizeNameBodyMedium,
		TextStyle: bellui.TextStyle{Italic: true},
	}
	// RichTextStyleHeading represents a heading text that stands on its own line.
	//
	// Since: 1.0
	RichTextStyleHeading = RichTextStyle{
		ColorName:    theme.ColorNameOnSurface,
		Inline:       false,
		SizeName:     theme.SizeNameTitleLarge,
		TextStyle:    bellui.TextStyle{Bold: true},
		HeadingLevel: 1,
	}
	// RichTextStyleInline represents standard text that can be surrounded by other elements.
	//
	// Since: 1.0
	RichTextStyleInline = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    true,
		SizeName:  theme.SizeNameBodyMedium,
	}
	// RichTextStyleParagraph represents standard text that should appear separate from other text.
	//
	// Since: 1.0
	RichTextStyleParagraph = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    false,
		SizeName:  theme.SizeNameBodyMedium,
	}
	// RichTextStylePassword represents standard sized text where the characters are obscured.
	//
	// Since: 1.0
	RichTextStylePassword = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    true,
		SizeName:  theme.SizeNameBodyMedium,
		concealed: true,
	}
	// RichTextStyleStrong represents regular text with a strong emphasis.
	//
	// Since: 1.0
	RichTextStyleStrong = RichTextStyle{
		ColorName: theme.ColorNameOnSurface,
		Inline:    true,
		SizeName:  theme.SizeNameBodyMedium,
		TextStyle: bellui.TextStyle{Bold: true},
	}
	// RichTextStyleSubHeading represents a sub-heading text that stands on its own line.
	//
	// Since: 1.0
	RichTextStyleSubHeading = RichTextStyle{
		ColorName:    theme.ColorNameOnSurface,
		Inline:       false,
		SizeName:     theme.SizeNameTitleMedium,
		TextStyle:    bellui.TextStyle{Bold: true},
		HeadingLevel: 2,
	}
)

// HyperlinkSegment represents a hyperlink within a rich text widget.
//
// Since: 1.0
type HyperlinkSegment struct {
	Alignment bellui.TextAlign
	Text      string
	URL       *url.URL

	// OnTapped overrides the default `bellui.OpenURL` call when the link is tapped
	//
	// Since: 1.0
	OnTapped func() `json:"-"`

	// Style carries optional per-segment styling for the link text — FontSource
	// (pre-rebrand tracker #494/#651), ColorName, SizeName and TextStyle — mirroring TextSegment.Style
	// so a link can render in a custom font, color or size rather than always the
	// theme font/primary color. The zero value is byte-identical to before (theme
	// font, primary color, theme text size). The segment's own Alignment field
	// (not Style.Alignment) controls alignment; Style.Inline, HeadingLevel and the
	// internal concealed flag are not consulted for links.
	//
	// Since: 1.0
	Style RichTextStyle
}

// Inline returns true as hyperlinks are inside other elements.
func (h *HyperlinkSegment) Inline() bool {
	return true
}

// Textual returns the content of this segment rendered to plain text.
func (h *HyperlinkSegment) Textual() string {
	return h.Text
}

// Visual returns a new instance of a hyperlink widget required to render this segment.
func (h *HyperlinkSegment) Visual() core.CanvasObject {
	link := NewHyperlink(h.Text, h.URL)
	link.Alignment = h.Alignment
	link.OnTapped = h.OnTapped
	h.applyStyle(link)
	return &bellui.Container{Layout: &unpadTextWidgetLayout{parent: link}, Objects: []core.CanvasObject{link}}
}

// applyStyle threads the segment's per-segment style onto the backing Hyperlink
// widget. Zero-value fields leave the widget at its defaults (theme font/size,
// primary color), keeping unstyled links byte-identical.
func (h *HyperlinkSegment) applyStyle(link *Hyperlink) {
	link.TextStyle = h.Style.TextStyle
	link.SizeName = h.Style.SizeName
	link.ColorName = h.Style.ColorName
	link.FontSource = h.Style.FontSource
}

// Update applies the current state of this hyperlink segment to an existing visual.
func (h *HyperlinkSegment) Update(o core.CanvasObject) {
	c, ok := o.(*bellui.Container)
	if !ok || len(c.Objects) == 0 {
		return
	}
	link, ok := c.Objects[0].(*Hyperlink)
	if !ok {
		return
	}
	link.Text = h.Text
	link.URL = h.URL
	link.Alignment = h.Alignment
	link.OnTapped = h.OnTapped
	h.applyStyle(link)
	link.Refresh()
}

// Select tells the segment that the user is selecting the content between the two positions.
func (h *HyperlinkSegment) Select(begin, end geo.Position) {
	// no-op: this will be added when we progress to editor
}

// SelectedText should return the text representation of any content currently selected through the Select call.
func (h *HyperlinkSegment) SelectedText() string {
	// no-op: this will be added when we progress to editor
	return ""
}

// Unselect tells the segment that the user is has cancelled the previous selection.
func (h *HyperlinkSegment) Unselect() {
	// no-op: this will be added when we progress to editor
}

// parsedLinkURL parses a link target taken from untrusted markup (HTML/Markdown)
// and only returns it when its scheme is safe to hand to bellui.OpenURL. Anything
// outside the http/https/mailto allowlist (or a relative reference) — e.g.
// file://, smb://, javascript:, or Windows handler schemes such as search-ms:
// and ms-msdt: — is dropped (nil), so tapping such a link does nothing rather
// than invoking a dangerous external handler (pre-rebrand tracker #285). Relative references (empty
// scheme) are preserved as they are inert for OpenURL.
func parsedLinkURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil
	}
	switch strings.ToLower(u.Scheme) {
	case "", "http", "https", "mailto":
		return u
	}
	return nil
}

// ImageSegment represents an image within a rich text widget.
//
// Since: 1.0
type ImageSegment struct {
	Source bellui.URI
	Title  string

	// Image is an in-memory image source. When set it takes precedence over
	// Source and is rendered without any network or disk access — so it is not
	// subject to the AllowRemoteContent policy. Use it to embed decoded image
	// data (e.g. a pasted bitmap) directly.
	//
	// Since: 1.0
	Image image.Image

	// Resource is an in-memory resource source (e.g. bundled image bytes). Like
	// Image it is rendered without network or disk access. Source precedence is
	// Image, then Resource, then Source.
	//
	// Since: 1.0
	Resource bellui.Resource

	// Alignment specifies the horizontal alignment of this image segment
	// Since: 1.0
	Alignment bellui.TextAlign

	// InlineImage lays the image out *within* a text row — flowing between the
	// surrounding text like a large glyph — instead of on its own block row. The
	// image still occupies exactly one caret position. Zero value (false) keeps
	// the historical block layout, so existing content is unchanged (pre-rebrand tracker #665).
	//
	// Since: 1.0
	InlineImage bool

	// allowRemote is set by the embedding RichText from its AllowRemoteContent
	// policy. When false (the default) the image is not fetched and a
	// broken-image placeholder is rendered instead, so rendering untrusted
	// markup cannot trigger network requests or local-file reads (pre-rebrand tracker #285).
	allowRemote bool
}

// Inline reports whether this image flows within a text row (see InlineImage).
// Block images (the default) return false and lay out on their own row.
func (i *ImageSegment) Inline() bool {
	return i.InlineImage
}

// Textual returns the content of this segment rendered to plain text.
func (i *ImageSegment) Textual() string {
	return "Image " + i.Title
}

// Visual returns a new instance of an image widget required to render this segment.
func (i *ImageSegment) Visual() core.CanvasObject {
	return newRichImage(i.Source, i.Image, i.Resource, i.Alignment, i.allowRemote)
}

// Update applies the current state of this image segment to an existing visual.
func (i *ImageSegment) Update(o core.CanvasObject) {
	img := o.(*richImage)
	img.source = i.Source
	img.image = i.Image
	img.resource = i.Resource
	img.allowRemote = i.allowRemote
	newer := img.loadImage()

	// one of the following will be used
	img.img.File = newer.File
	img.img.Resource = newer.Resource
	img.img.Image = newer.Image
	img.setAlign(i.Alignment)

	img.Refresh()
}

// Select tells the segment that the user is selecting the content between the two positions.
func (i *ImageSegment) Select(begin, end geo.Position) {
	// no-op: this will be added when we progress to editor
}

// SelectedText should return the text representation of any content currently selected through the Select call.
func (i *ImageSegment) SelectedText() string {
	// no-op: images have no text rendering
	return ""
}

// Unselect tells the segment that the user is has cancelled the previous selection.
func (i *ImageSegment) Unselect() {
	// no-op: this will be added when we progress to editor
}

// ListSegment includes an itemised list with the content set using the Items field.
//
// Since: 1.0
type ListSegment struct {
	Items   []RichTextSegment
	Ordered bool

	// startIndex is the starting number - 1 (If it is ordered). Unordered lists
	// ignore startIndex.
	//
	// startIndex is set to start - 1 to allow the empty value of ListSegment to have a starting
	// number of 1, while also allowing the caller to override the starting
	// number to any int, including 0.
	startIndex       int
	indentationLevel int
}

// SetStartNumber sets the starting number for an ordered list.
// Unordered lists are not affected.
//
// Since: 1.0
func (l *ListSegment) SetStartNumber(s int) {
	l.startIndex = s - 1
}

// StartNumber return the starting number for an ordered list.
//
// Since: 1.0
func (l *ListSegment) StartNumber() int {
	return l.startIndex + 1
}

// Inline returns false as a list should be in a block.
func (l *ListSegment) Inline() bool {
	return false
}

// Segments returns the segments required to draw bullets before each item
func (l *ListSegment) Segments() []RichTextSegment {
	out := make([]RichTextSegment, len(l.Items))
	j := l.StartNumber()
	for i, in := range l.Items {
		var texts []RichTextSegment
		if _, ok := in.(*ListSegment); !ok {
			txt := "• "
			if l.Ordered {
				txt = strconv.Itoa(j) + "."
				j++
			}
			indentation := strings.Repeat(" ", l.indentationLevel*4)
			bullet := &TextSegment{Text: indentation + txt + " ", Style: RichTextStyleStrong}
			texts = append(texts, bullet)
		}
		texts = append(texts, in)
		out[i] = &ParagraphSegment{Texts: texts}
	}
	return out
}

// Textual returns no content for a list as the content is in sub-segments.
func (l *ListSegment) Textual() string {
	return ""
}

// Visual returns no additional elements for this segment.
func (l *ListSegment) Visual() core.CanvasObject {
	return nil
}

// Update doesn't need to change a list visual.
func (l *ListSegment) Update(core.CanvasObject) {
}

// Select does nothing for a list container.
func (l *ListSegment) Select(_, _ geo.Position) {
}

// SelectedText returns the empty string for this list.
func (l *ListSegment) SelectedText() string {
	return ""
}

// Unselect does nothing for a list container.
func (l *ListSegment) Unselect() {
}

// ParagraphSegment wraps a number of text elements in a paragraph.
// It is similar to using a list of text elements when the final style is RichTextStyleParagraph.
//
// Since: 1.0
type ParagraphSegment struct {
	Texts []RichTextSegment
}

// Inline returns false as a paragraph should be in a block.
func (p *ParagraphSegment) Inline() bool {
	return false
}

// Segments returns the list of text elements in this paragraph.
func (p *ParagraphSegment) Segments() []RichTextSegment {
	return p.Texts
}

// Textual returns no content for a paragraph container.
func (p *ParagraphSegment) Textual() string {
	return ""
}

// Visual returns the no extra elements.
func (p *ParagraphSegment) Visual() core.CanvasObject {
	return nil
}

// Update doesn't need to change a paragraph container.
func (p *ParagraphSegment) Update(core.CanvasObject) {
}

// Select does nothing for a paragraph container.
func (p *ParagraphSegment) Select(_, _ geo.Position) {
}

// SelectedText returns the empty string for this paragraph container.
func (p *ParagraphSegment) SelectedText() string {
	return ""
}

// Unselect does nothing for a paragraph container.
func (p *ParagraphSegment) Unselect() {
}

// TableSegment includes a table parsed from markdown in a rich text widget.
//
// Since: 1.0
type TableSegment struct {
	Header     []string
	Rows       [][]string
	Alignments []bellui.TextAlign
}

// Inline returns false as a table should be in a block.
func (t *TableSegment) Inline() bool {
	return false
}

// Textual returns the text content of the table.
func (t *TableSegment) Textual() string {
	b := strings.Builder{}
	b.WriteString(strings.Join(t.Header, "\t"))
	b.WriteByte('\n')
	for _, row := range t.Rows {
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

// Visual returns a new table widget to render this segment.
func (t *TableSegment) Visual() core.CanvasObject {
	return newRichTable(t)
}

// Update applies the current state of this table segment to an existing visual.
func (t *TableSegment) Update(o core.CanvasObject) {
	rt := o.(*richTable)
	rt.updateData(t)
}

// Select does nothing for a table.
func (t *TableSegment) Select(_, _ geo.Position) {
}

// SelectedText returns the empty string for this table.
func (t *TableSegment) SelectedText() string {
	return ""
}

// Unselect does nothing for a table.
func (t *TableSegment) Unselect() {
}

type richTable struct {
	BaseWidget
	header     []string
	rows       [][]string
	alignments []bellui.TextAlign

	// Measurements populated by renderer during buildObjects
	naturalColWidths []float32
	cellNatWidths    []float32
	headerWidths     []float32 // per-column header text width (headers don't wrap)
	refLineHeight    float32

	// Cached actual height from last Layout (more accurate than formula)
	lastLayoutWidth  float32
	lastLayoutHeight float32
	minColWidth      float32
	sepHeight        float32
}

// computeLayout computes column widths and row heights for a given available width.
func (rt *richTable) computeLayout(availableWidth float32) (colWidths []float32, rowHeights []float32, totalHeight float32) {
	numCols := len(rt.header)
	if numCols == 0 || len(rt.naturalColWidths) == 0 {
		return
	}

	totalNatural := float32(0)
	for _, w := range rt.naturalColWidths {
		totalNatural += w
	}

	colWidths = make([]float32, numCols)
	if totalNatural > 0 {
		for i := range colWidths {
			colWidths[i] = availableWidth * (rt.naturalColWidths[i] / totalNatural)
		}
	} else {
		share := availableWidth / float32(numCols)
		for i := range colWidths {
			colWidths[i] = share
		}
	}

	// Enforce per-column minimums: header width (headers don't wrap) and 5-char floor.
	// Then redistribute any excess so total doesn't exceed availableWidth.
	totalUsed := float32(0)
	for i := range colWidths {
		minW := rt.minColWidth
		if i < len(rt.headerWidths) && rt.headerWidths[i] > minW {
			minW = rt.headerWidths[i]
		}
		if colWidths[i] < minW {
			colWidths[i] = minW
		}
		totalUsed += colWidths[i]
	}
	if totalUsed > availableWidth && availableWidth > 0 {
		scale := availableWidth / totalUsed
		for i := range colWidths {
			colWidths[i] *= scale
		}
	}

	// Estimate row heights (used by HeightForWidth; Layout uses actual Label heights)
	numRows := 1 + len(rt.rows)
	rowHeights = make([]float32, numRows)
	for row := 0; row < numRows; row++ {
		rowHeights[row] = rt.refLineHeight
		for col := 0; col < numCols; col++ {
			idx := row*numCols + col
			if idx < len(rt.cellNatWidths) {
				cellW := rt.cellNatWidths[idx]
				h := rt.refLineHeight
				if colWidths[col] > 0 && cellW > colWidths[col] {
					lines := float32(math.Ceil(float64(cellW / colWidths[col])))
					h = rt.refLineHeight * lines
				}
				if h > rowHeights[row] {
					rowHeights[row] = h
				}
			}
		}
	}

	for _, h := range rowHeights {
		totalHeight += h
	}
	totalHeight += rt.sepHeight

	return
}

// HeightForWidth returns the table height when laid out at the given width.
// Uses the actual measured height from the last Layout when available (more
// accurate than the formula estimate, which doesn't account for word wrapping).
func (rt *richTable) HeightForWidth(width float32) float32 {
	if rt.lastLayoutHeight > 0 && rt.lastLayoutWidth == width {
		return rt.lastLayoutHeight
	}
	_, _, h := rt.computeLayout(width)
	return h
}

func newRichTable(seg *TableSegment) *richTable {
	r := &richTable{
		header:     seg.Header,
		rows:       seg.Rows,
		alignments: seg.Alignments,
	}
	r.ExtendBaseWidget(r)
	return r
}

func (rt *richTable) updateData(seg *TableSegment) {
	rt.header = seg.Header
	rt.rows = seg.Rows
	rt.alignments = seg.Alignments
	rt.Refresh()
}

func (rt *richTable) CreateRenderer() core.WidgetRenderer {
	rr := &richTableRenderer{table: rt}
	rr.buildObjects()
	return rr
}

type richTableRenderer struct {
	table   *richTable
	objects []core.CanvasObject
}

func (r *richTableRenderer) buildObjects() {
	numCols := len(r.table.header)
	numRows := len(r.table.rows)
	total := numCols * (1 + numRows)

	if len(r.objects) != total+1 { // +1 for separator
		r.objects = make([]core.CanvasObject, 0, total+1)
		for i, h := range r.table.header {
			l := NewLabel(h)
			l.TextStyle.Bold = true
			if i < len(r.table.alignments) {
				l.Alignment = r.table.alignments[i]
			}
			r.objects = append(r.objects, l)
		}
		r.objects = append(r.objects, NewSeparator())
		for _, row := range r.table.rows {
			for i := 0; i < numCols; i++ {
				text := ""
				if i < len(row) {
					text = row[i]
				}
				l := NewLabel(text)
				if i < len(r.table.alignments) {
					l.Alignment = r.table.alignments[i]
				}
				r.objects = append(r.objects, l)
			}
		}
	} else {
		idx := 0
		for i, h := range r.table.header {
			l := r.objects[idx].(*Label)
			l.SetText(h)
			l.TextStyle.Bold = true
			if i < len(r.table.alignments) {
				l.Alignment = r.table.alignments[i]
			}
			idx++
		}
		idx++ // skip separator
		for _, row := range r.table.rows {
			for i := 0; i < numCols; i++ {
				l := r.objects[idx].(*Label)
				text := ""
				if i < len(row) {
					text = row[i]
				}
				l.SetText(text)
				if i < len(r.table.alignments) {
					l.Alignment = r.table.alignments[i]
				}
				idx++
			}
		}
	}

	// Measure natural widths using Label MinSize for accuracy.
	r.table.naturalColWidths = make([]float32, numCols)
	r.table.cellNatWidths = make([]float32, 0, total)

	r.table.headerWidths = make([]float32, numCols)
	// Use Label MinSize for accurate measurement — this includes internal
	// padding and matches the actual rendering exactly.
	for i, h := range r.table.header {
		ref := NewLabel(h)
		ref.TextStyle.Bold = true
		w := ref.MinSize().Width
		r.table.cellNatWidths = append(r.table.cellNatWidths, w)
		r.table.headerWidths[i] = w
		if w > r.table.naturalColWidths[i] {
			r.table.naturalColWidths[i] = w
		}
	}
	for _, row := range r.table.rows {
		for i := 0; i < numCols; i++ {
			text := ""
			if i < len(row) {
				text = row[i]
			}
			ref := NewLabel(text)
			w := ref.MinSize().Width
			r.table.cellNatWidths = append(r.table.cellNatWidths, w)
			if w > r.table.naturalColWidths[i] {
				r.table.naturalColWidths[i] = w
			}
		}
	}

	// Reference line height, min column width, separator height
	ref := NewLabel("X")
	r.table.refLineHeight = ref.MinSize().Height
	r.table.minColWidth = NewLabel("XXXXX").MinSize().Width

	r.table.sepHeight = NewSeparator().MinSize().Height

	// Enforce minimum on natural column widths
	for i := range r.table.naturalColWidths {
		if r.table.naturalColWidths[i] < r.table.minColWidth {
			r.table.naturalColWidths[i] = r.table.minColWidth
		}
	}

	// Enable wrapping on data cells only (headers stay unwrapped)
	pastSep := false
	for _, obj := range r.objects {
		if _, ok := obj.(*Separator); ok {
			pastSep = true
			continue
		}
		if l, ok := obj.(*Label); ok && pastSep {
			l.Wrapping = bellui.TextWrapWord
		}
	}
}

func (r *richTableRenderer) Layout(size geo.Size) {
	numCols := len(r.table.header)
	if numCols == 0 {
		return
	}

	var cells []core.CanvasObject
	var sep *Separator
	for _, obj := range r.objects {
		if s, ok := obj.(*Separator); ok {
			sep = s
		} else {
			cells = append(cells, obj)
		}
	}

	if len(cells) == 0 || len(cells)%numCols != 0 {
		return
	}

	colWidths, _, _ := r.table.computeLayout(size.Width)
	if len(colWidths) == 0 {
		return
	}

	// Compute actual row heights by resizing each cell to its column width
	// and measuring the Label's MinSize height. This is more accurate than
	// the formula-based estimation in computeLayout because it accounts for
	// word-boundary wrapping and font metrics.
	numRows := len(cells) / numCols
	rowHeights := make([]float32, numRows)
	for row := 0; row < numRows; row++ {
		rowHeights[row] = r.table.refLineHeight
		for col := 0; col < numCols; col++ {
			cell := cells[row*numCols+col]
			cell.Resize(geo.NewSize(colWidths[col], cell.MinSize().Height))
			h := cell.MinSize().Height
			if h > rowHeights[row] {
				rowHeights[row] = h
			}
		}
	}

	y := float32(0)
	for row := 0; row < numRows; row++ {
		if row == 1 && sep != nil {
			sep.Move(geo.NewPos(0, y))
			sep.Resize(geo.NewSize(size.Width, r.table.sepHeight))
			y += r.table.sepHeight
		}

		rh := rowHeights[row]
		x := float32(0)
		for col := 0; col < numCols; col++ {
			cell := cells[row*numCols+col]
			cell.Move(geo.NewPos(x, y))
			cell.Resize(geo.NewSize(colWidths[col], rh))
			x += colWidths[col]
		}
		y += rh
	}

	// Cache actual height so HeightForWidth returns accurate values.
	r.table.lastLayoutWidth = size.Width
	r.table.lastLayoutHeight = y
}

func (r *richTableRenderer) MinSize() geo.Size {
	numCols := len(r.table.header)
	if numCols == 0 || len(r.table.naturalColWidths) == 0 {
		return geo.Size{}
	}

	totalWidth := float32(0)
	for _, w := range r.table.naturalColWidths {
		totalWidth += w
	}

	// Use HeightForWidth at current widget width if available, otherwise at natural width
	w := r.table.Size().Width
	if w <= 0 {
		w = totalWidth
	}
	totalHeight := r.table.HeightForWidth(w)

	return geo.NewSize(totalWidth, totalHeight)
}

func (r *richTableRenderer) Refresh() {
	r.buildObjects()
}

func (r *richTableRenderer) Objects() []core.CanvasObject {
	if len(r.objects) == 0 {
		r.buildObjects()
	}
	return r.objects
}

func (r *richTableRenderer) Destroy() {
}

// SeparatorSegment includes a horizontal separator in a rich text widget.
//
// Since: 1.0
type SeparatorSegment struct {
	_ bool // Without this a pointer to SeparatorSegment will always be the same.
}

// Inline returns false as a separator should be full width.
func (s *SeparatorSegment) Inline() bool {
	return false
}

// Textual returns no content for a separator element.
func (s *SeparatorSegment) Textual() string {
	return ""
}

// Visual returns a new instance of a separator widget for this segment.
func (s *SeparatorSegment) Visual() core.CanvasObject {
	return NewSeparator()
}

// Update doesn't need to change a separator visual.
func (s *SeparatorSegment) Update(core.CanvasObject) {
}

// Select does nothing for a separator.
func (s *SeparatorSegment) Select(_, _ geo.Position) {
}

// SelectedText returns the empty string for this separator.
func (s *SeparatorSegment) SelectedText() string {
	return ""
}

// Unselect does nothing for a separator.
func (s *SeparatorSegment) Unselect() {
}

// RichTextStyle describes the details of a text object inside a RichText widget.
//
// Since: 1.0
type RichTextStyle struct {
	Alignment bellui.TextAlign
	ColorName bellui.ThemeColorName
	Inline    bool
	SizeName  bellui.ThemeSizeName // The theme name of the text size to use, if blank will be the standard text size
	TextStyle bellui.TextStyle

	// FontSource is an optional custom font resource used to render this
	// segment's text, matching canvas.Text.FontSource / canvas.TextRun.FontSource
	// at the canvas level. When nil (the zero value) the segment renders with the
	// theme font, so existing content is byte-identical to before. The chosen font
	// is honoured consistently across measurement, wrapping, alignment, selection
	// and truncation, so a custom-font segment lays out exactly as it renders.
	//
	// Since: 1.0
	FontSource bellui.Resource

	// HeadingLevel marks the segment as a document heading of the given
	// 1-based level (1 = H1). Zero means not a heading. It carries no
	// visual meaning of its own — the markdown/HTML parsers set it
	// alongside the heading text styles — but it feeds the accessibility
	// tree, where headings are exposed as navigable RoleHeading nodes.
	HeadingLevel int

	// an internal detail where we obscure password fields
	concealed bool
}

// isBlockquote reports whether this style is the blockquote preset, ignoring any
// per-segment FontSource. FontSource (pre-rebrand tracker #494) composes with any preset, so a
// blockquote that also carries a custom font must still be laid out as a
// blockquote (indented) rather than falling back to a plain paragraph — a whole
// struct comparison against RichTextStyleBlockquote would miss it.
func (s RichTextStyle) isBlockquote() bool {
	s.FontSource = nil
	return s == RichTextStyleBlockquote
}

// RichTextSegment describes any element that can be rendered in a RichText widget.
//
// Since: 1.0
type RichTextSegment interface {
	Inline() bool
	Textual() string
	Update(core.CanvasObject)
	Visual() core.CanvasObject

	Select(pos1, pos2 geo.Position)
	SelectedText() string
	Unselect()
}

// TextSegment represents the styling for a segment of rich text.
//
// Since: 1.0
type TextSegment struct {
	Style RichTextStyle
	Text  string

	parent *RichText
}

// Inline should return true if this text can be included within other elements, or false if it creates a new block.
func (t *TextSegment) Inline() bool {
	return t.Style.Inline
}

// Textual returns the content of this segment rendered to plain text.
func (t *TextSegment) Textual() string {
	return t.Text
}

// Visual returns a new instance of a graphical element required to render this segment.
func (t *TextSegment) Visual() core.CanvasObject {
	obj := canvas.NewText(t.Text, t.color())

	t.Update(obj)
	return obj
}

// Update applies the current state of this text segment to an existing visual.
func (t *TextSegment) Update(o core.CanvasObject) {
	obj := o.(*canvas.Text)
	obj.Text = t.Text
	obj.Color = t.color()
	obj.Alignment = t.Style.Alignment
	obj.TextStyle = t.Style.TextStyle
	obj.FontSource = t.Style.FontSource
	obj.TextSize = t.size()
	obj.Refresh()
}

// Select tells the segment that the user is selecting the content between the two positions.
func (t *TextSegment) Select(begin, end geo.Position) {
	// no-op: this will be added when we progress to editor
}

// SelectedText should return the text representation of any content currently selected through the Select call.
func (t *TextSegment) SelectedText() string {
	// no-op: this will be added when we progress to editor
	return ""
}

// Unselect tells the segment that the user is has cancelled the previous selection.
func (t *TextSegment) Unselect() {
	// no-op: this will be added when we progress to editor
}

func (t *TextSegment) color() color.Color {
	if t.Style.ColorName != "" {
		return theme.ColorForWidget(t.Style.ColorName, t.parent)
	}

	return theme.ColorForWidget(theme.ColorNameOnSurface, t.parent)
}

func (t *TextSegment) size() float32 {
	if t.Style.SizeName != "" {
		i := theme.SizeForWidget(t.Style.SizeName, t.parent)
		return i
	}

	i := theme.SizeForWidget(theme.SizeNameText, t.parent)
	return i
}

type richImage struct {
	BaseWidget
	align       bellui.TextAlign
	source      bellui.URI
	image       image.Image
	resource    bellui.Resource
	allowRemote bool
	img         *canvas.Image
	oldMin      geo.Size
	layout      *bellui.Container
	min         geo.Size
}

func newRichImage(u bellui.URI, img image.Image, res bellui.Resource, align bellui.TextAlign, allowRemote bool) *richImage {
	i := &richImage{source: u, image: img, resource: res, align: align, allowRemote: allowRemote}
	i.img = i.loadImage()
	i.img.FillMode = canvas.ImageFillOriginal
	i.ExtendBaseWidget(i)
	return i
}

// loadImage returns the canvas image for this segment's source. In-memory
// sources (Image, Resource) are rendered directly. A remote/local URI source is
// only fetched when explicitly allowed; otherwise a broken-image placeholder is
// rendered so nothing is read from the network or disk (pre-rebrand tracker #285).
func (r *richImage) loadImage() *canvas.Image {
	if r.image != nil {
		return canvas.NewImageFromImage(r.image)
	}
	if r.resource != nil {
		return canvas.NewImageFromResource(r.resource)
	}
	if r.source != nil && r.allowRemote {
		return canvas.NewImageFromURI(r.source)
	}
	return canvas.NewImageFromResource(theme.BrokenImageIcon())
}

func (r *richImage) CreateRenderer() core.WidgetRenderer {
	r.layout = &bellui.Container{Layout: &richImageLayout{r}, Objects: []core.CanvasObject{r.img}}
	return NewSimpleRenderer(r.layout)
}

func (r *richImage) MinSize() geo.Size {
	orig := r.img.MinSize()
	c := bellui.CurrentApp().Driver().CanvasForObject(r)
	if c == nil {
		return r.oldMin // not yet rendered
	}

	// unscale the image so it is not varying based on canvas
	w := scale.ToScreenCoordinate(c, orig.Width)
	h := scale.ToScreenCoordinate(c, orig.Height)
	// we return size / 2 as this assumes a HiDPI / 2x image scaling
	r.min = geo.NewSize(float32(w)/2, float32(h)/2)
	return r.min
}

func (r *richImage) setAlign(a bellui.TextAlign) {
	if r.layout != nil {
		r.layout.Refresh()
	}
	r.align = a
}

type richImageLayout struct {
	r *richImage
}

func (r *richImageLayout) Layout(_ []core.CanvasObject, s geo.Size) {
	r.r.img.Resize(r.r.min)
	gap := float32(0)

	switch r.r.align {
	case bellui.TextAlignCenter:
		gap = (s.Width - r.r.min.Width) / 2
	case bellui.TextAlignTrailing:
		gap = s.Width - r.r.min.Width
	}

	r.r.img.Move(geo.NewPos(gap, 0))
}

func (r *richImageLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return r.r.min
}

type unpadTextWidgetLayout struct {
	parent core.Widget
}

func (u *unpadTextWidgetLayout) Layout(o []core.CanvasObject, s geo.Size) {
	innerPad := theme.SizeForWidget(theme.SizeNameInnerPadding, u.parent)
	pad := innerPad * -1
	pad2 := pad * -2

	o[0].Move(geo.NewPos(pad, pad))
	o[0].Resize(s.Add(geo.NewSize(pad2, pad2)))
}

func (u *unpadTextWidgetLayout) MinSize(o []core.CanvasObject) geo.Size {
	innerPad := theme.SizeForWidget(theme.SizeNameInnerPadding, u.parent)
	pad := innerPad * 2
	return o[0].MinSize().Subtract(geo.NewSize(pad, pad))
}
