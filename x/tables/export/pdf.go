package export

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// PageSize selects the paper size for PDF export.
type PageSize int

const (
	PageA4 PageSize = iota
	PageLetter
)

// DefaultPDFMaxRows is the row cap applied when PDFOptions.MaxRows is
// zero. Grids can hold millions of rows; a paginated report of that
// size is rarely intentional, so the cap is the safe default and the
// report ends with an explicit truncation notice when it bites.
const DefaultPDFMaxRows = 10000

// PDFOptions configures the paginated PDF report.
type PDFOptions struct {
	// Title is rendered as a heading block on the first page. Empty
	// means no title block.
	Title string
	// PageSize selects the paper size (default PageA4).
	PageSize PageSize
	// Landscape rotates the page 90°.
	Landscape bool
	// FontSize is the body text size in points (default 9).
	FontSize float64
	// MaxRows caps the number of data rows rendered. 0 applies
	// DefaultPDFMaxRows; a negative value removes the cap.
	MaxRows int
}

// PDF writes the grid data as a paginated PDF report: column headers
// repeated on every page, columns scaled to fit the page width, a
// page-number footer, and an optional title block on the first page.
//
// The writer streams page by page — working memory is bounded by a
// single page regardless of row count, so large grids export without
// being materialized. Rows beyond the MaxRows cap are omitted and the
// report ends with a truncation notice.
//
// The document uses the built-in Helvetica fonts with WinAnsi
// (cp1252) encoding; characters outside that repertoire render as '?'.
func PDF(w io.Writer, columns []Column, data DataAccess, opts PDFOptions) error {
	if len(columns) == 0 {
		return fmt.Errorf("export: PDF requires at least one column")
	}
	if opts.FontSize <= 0 {
		opts.FontSize = 9
	}
	maxRows := opts.MaxRows
	if maxRows == 0 {
		maxRows = DefaultPDFMaxRows
	}

	pageW, pageH := pageDims(opts.PageSize, opts.Landscape)
	d := &pdfDoc{w: &countingWriter{w: w}, pageW: pageW, pageH: pageH}

	const (
		margin   = 36.0
		footerH  = 24.0
		cellPadX = 3.0
	)
	fontSize := opts.FontSize
	lineH := fontSize * 1.5
	contentW := pageW - 2*margin

	colWidths := fitColumnWidths(columns, data, fontSize, cellPadX, contentW)

	d.begin()

	top := pageH - margin
	bottom := margin + footerH
	pageNum := 0
	var page *pdfPage

	newPage := func() {
		pageNum++
		page = d.newPage()
		y := top
		if pageNum == 1 && opts.Title != "" {
			titleSize := fontSize + 4
			page.text(margin, y-titleSize, fontBold, titleSize, opts.Title)
			y -= titleSize * 1.6
			page.rule(margin, y, pageW-margin, y)
			y -= lineH * 0.75
		}
		// Column header row, repeated on every page.
		x := margin
		for c, col := range columns {
			page.text(x+cellPadX, y-fontSize, fontBold, fontSize,
				fitText(col.Header, fontBold, fontSize, colWidths[c]-2*cellPadX))
			x += colWidths[c]
		}
		y -= fontSize * 1.3
		page.rule(margin, y, pageW-margin, y)
		y -= lineH * 0.4
		page.y = y
		// Footer.
		label := fmt.Sprintf("Page %d", pageNum)
		lw := textWidth(label, fontRegular, fontSize-1)
		page.text(pageW-margin-lw, margin*0.5, fontRegular, fontSize-1, label)
	}

	writeLine := func(cells func(c int) (string, pdfFont)) {
		if page == nil || page.y-lineH < bottom {
			if page != nil {
				d.endPage(page)
			}
			newPage()
		}
		x := margin
		for c := range columns {
			s, f := cells(c)
			page.text(x+cellPadX, page.y-fontSize, f, fontSize,
				fitText(s, f, fontSize, colWidths[c]-2*cellPadX))
			x += colWidths[c]
		}
		page.y -= lineH
	}

	newPage()

	rowCount := data.RowCount()
	emitted := 0
	truncated := false
	for r := 0; r < rowCount; r++ {
		if maxRows >= 0 && emitted >= maxRows {
			truncated = true
			break
		}
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		writeLine(func(c int) (string, pdfFont) {
			return cellString(columns[c], rowData), fontRegular
		})
		emitted++
	}

	if truncated {
		notice := fmt.Sprintf("Output truncated at %d rows (%d rows in grid). Raise MaxRows to include more.",
			emitted, rowCount)
		writeLine(func(c int) (string, pdfFont) {
			if c == 0 {
				return notice, fontBold
			}
			return "", fontRegular
		})
	}

	d.endPage(page)
	return d.finish()
}

func pageDims(size PageSize, landscape bool) (w, h float64) {
	switch size {
	case PageLetter:
		w, h = 612, 792
	default: // PageA4
		w, h = 595.28, 841.89
	}
	if landscape {
		w, h = h, w
	}
	return w, h
}

// fitColumnWidths measures header and sampled cell text, then scales
// every column proportionally so the table exactly fills the content
// width (fit-to-width). Sampling is capped so measurement stays cheap
// on huge grids.
func fitColumnWidths(columns []Column, data DataAccess, fontSize, cellPadX, contentW float64) []float64 {
	const sampleRows = 512
	widths := make([]float64, len(columns))
	for c, col := range columns {
		widths[c] = textWidth(col.Header, fontBold, fontSize) + 2*cellPadX
	}
	n := data.RowCount()
	if n > sampleRows {
		n = sampleRows
	}
	for r := 0; r < n; r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		for c, col := range columns {
			w := textWidth(cellString(col, rowData), fontRegular, fontSize) + 2*cellPadX
			if w > widths[c] {
				widths[c] = w
			}
		}
	}
	var total float64
	for _, w := range widths {
		total += w
	}
	if total <= 0 {
		total = 1
	}
	scale := contentW / total
	for c := range widths {
		widths[c] *= scale
	}
	return widths
}

// --- text measurement / encoding (Helvetica core metrics) ---

type pdfFont int

const (
	fontRegular pdfFont = iota
	fontBold
)

// textWidth returns the rendered width in points of s after WinAnsi
// encoding.
func textWidth(s string, f pdfFont, size float64) float64 {
	cw := &helvWidths
	if f == fontBold {
		cw = &helvBoldWidths
	}
	var units int
	for _, b := range encodeWinAnsi(s) {
		units += int(cw[b-32])
	}
	return float64(units) * size / 1000
}

// fitText truncates s with an ellipsis so it renders within maxW.
func fitText(s string, f pdfFont, size, maxW float64) string {
	if textWidth(s, f, size) <= maxW {
		return s
	}
	ellipsis := "…"
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if textWidth(string(runes[:mid])+ellipsis, f, size) <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return string(runes[:lo]) + ellipsis
}

// encodeWinAnsi maps s to WinAnsi (cp1252) bytes; unmappable runes
// and control characters become '?'. Only bytes >= 32 are produced,
// so width lookups can index tables offset by 32.
func encodeWinAnsi(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 32 && r < 127:
			out = append(out, byte(r))
		case r >= 0xA0 && r <= 0xFF:
			out = append(out, byte(r))
		default:
			if b, ok := winAnsiSpecials[r]; ok {
				out = append(out, b)
			} else {
				out = append(out, '?')
			}
		}
	}
	return out
}

// winAnsiSpecials maps the cp1252 0x80–0x9F block.
var winAnsiSpecials = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84,
	'…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88,
	'‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C,
	'Ž': 0x8E, '‘': 0x91, '’': 0x92, '“': 0x93,
	'”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
	'˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B,
	'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// Helvetica / Helvetica-Bold advance widths (1/1000 em) for WinAnsi
// codes 32–255, from the Adobe core-14 AFM metrics. Index = code - 32.
var helvWidths = [224]uint16{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584, 350,
	556, 350, 222, 556, 333, 1000, 556, 556, 333, 1000, 667, 333, 1000, 350, 611, 350,
	350, 222, 222, 333, 333, 350, 556, 1000, 333, 1000, 500, 333, 944, 350, 500, 667,
	278, 333, 556, 556, 556, 556, 260, 556, 333, 737, 370, 556, 584, 333, 737, 333,
	400, 584, 333, 333, 333, 556, 537, 278, 333, 333, 365, 556, 834, 834, 834, 611,
	667, 667, 667, 667, 667, 667, 1000, 722, 667, 667, 667, 667, 278, 278, 278, 278,
	722, 722, 778, 778, 778, 778, 778, 584, 778, 722, 722, 722, 722, 667, 667, 611,
	556, 556, 556, 556, 556, 556, 889, 500, 556, 556, 556, 556, 278, 278, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 584, 611, 556, 556, 556, 556, 500, 556, 500,
}

var helvBoldWidths = [224]uint16{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584, 350,
	556, 350, 278, 556, 500, 1000, 556, 556, 333, 1000, 667, 333, 1000, 350, 611, 350,
	350, 278, 278, 500, 500, 350, 556, 1000, 333, 1000, 556, 333, 944, 350, 500, 667,
	278, 333, 556, 556, 556, 556, 280, 556, 333, 737, 370, 556, 584, 333, 737, 333,
	400, 584, 333, 333, 333, 611, 556, 278, 333, 333, 365, 556, 834, 834, 834, 611,
	722, 722, 722, 722, 722, 722, 1000, 722, 667, 667, 667, 667, 278, 278, 278, 278,
	722, 722, 778, 778, 778, 778, 778, 584, 778, 722, 722, 722, 722, 667, 667, 611,
	556, 556, 556, 556, 556, 556, 889, 556, 556, 556, 556, 556, 278, 278, 278, 278,
	611, 611, 611, 611, 611, 611, 611, 584, 611, 611, 611, 611, 611, 556, 611, 556,
}

// --- minimal streaming PDF writer ---

// pdfDoc emits a PDF incrementally: fonts first, then a content-stream
// and page object per page, and finally the page tree, catalog, and
// xref table. Object offsets are recorded as they are written, so
// nothing but the current page's content is buffered.
type pdfDoc struct {
	w       *countingWriter
	pageW   float64
	pageH   float64
	offsets []int64 // offsets[n] = byte offset of object n
	pageIDs []int
	nextID  int
}

type pdfPage struct {
	buf bytes.Buffer
	y   float64
}

const (
	objCatalog  = 1
	objPages    = 2
	objFontReg  = 3
	objFontBold = 4
	objFirst    = 5
)

func (d *pdfDoc) begin() {
	d.nextID = objFirst
	d.offsets = make([]int64, objFirst)
	// Binary comment line marks the file as 8-bit data.
	fmt.Fprintf(d.w, "%%PDF-1.4\n%%\xe2\xe3\xcf\xd3\n")
	d.startObj(objFontReg)
	fmt.Fprintf(d.w, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")
	d.startObj(objFontBold)
	fmt.Fprintf(d.w, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\nendobj\n")
}

func (d *pdfDoc) startObj(id int) {
	for len(d.offsets) <= id {
		d.offsets = append(d.offsets, 0)
	}
	d.offsets[id] = d.w.n
	fmt.Fprintf(d.w, "%d 0 obj\n", id)
}

func (d *pdfDoc) allocID() int {
	id := d.nextID
	d.nextID++
	return id
}

func (d *pdfDoc) newPage() *pdfPage {
	return &pdfPage{}
}

// endPage flushes the page's content stream and page object.
func (d *pdfDoc) endPage(p *pdfPage) {
	contentID := d.allocID()
	pageID := d.allocID()
	d.pageIDs = append(d.pageIDs, pageID)

	d.startObj(contentID)
	fmt.Fprintf(d.w, "<< /Length %d >>\nstream\n", p.buf.Len())
	d.w.Write(p.buf.Bytes())
	fmt.Fprintf(d.w, "endstream\nendobj\n")

	d.startObj(pageID)
	fmt.Fprintf(d.w,
		"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>\nendobj\n",
		objPages, fnum(d.pageW), fnum(d.pageH), objFontReg, objFontBold, contentID)
}

func (d *pdfDoc) finish() error {
	d.startObj(objPages)
	fmt.Fprintf(d.w, "<< /Type /Pages /Count %d /Kids [", len(d.pageIDs))
	for i, id := range d.pageIDs {
		if i > 0 {
			d.w.Write([]byte(" "))
		}
		fmt.Fprintf(d.w, "%d 0 R", id)
	}
	fmt.Fprintf(d.w, "] >>\nendobj\n")

	d.startObj(objCatalog)
	fmt.Fprintf(d.w, "<< /Type /Catalog /Pages %d 0 R >>\nendobj\n", objPages)

	xrefStart := d.w.n
	size := len(d.offsets)
	fmt.Fprintf(d.w, "xref\n0 %d\n0000000000 65535 f \n", size)
	for id := 1; id < size; id++ {
		fmt.Fprintf(d.w, "%010d 00000 n \n", d.offsets[id])
	}
	fmt.Fprintf(d.w, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		size, objCatalog, xrefStart)
	return d.w.err
}

// text draws s at (x, y) — y is the text baseline.
func (p *pdfPage) text(x, y float64, f pdfFont, size float64, s string) {
	if s == "" {
		return
	}
	name := "/F1"
	if f == fontBold {
		name = "/F2"
	}
	fmt.Fprintf(&p.buf, "BT %s %s Tf %s %s Td (", name, fnum(size), fnum(x), fnum(y))
	for _, b := range encodeWinAnsi(s) {
		switch b {
		case '(', ')', '\\':
			p.buf.WriteByte('\\')
		}
		p.buf.WriteByte(b)
	}
	p.buf.WriteString(") Tj ET\n")
}

// rule draws a thin horizontal line.
func (p *pdfPage) rule(x1, y1, x2, y2 float64) {
	fmt.Fprintf(&p.buf, "0.5 w 0.6 G %s %s m %s %s l S\n",
		fnum(x1), fnum(y1), fnum(x2), fnum(y2))
}

func fnum(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

type countingWriter struct {
	w   io.Writer
	n   int64
	err error
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	if cw.err != nil {
		return 0, cw.err
	}
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	cw.err = err
	return n, err
}
