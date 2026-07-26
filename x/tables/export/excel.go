package export

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// Excel writes the grid data as a minimal .xlsx file.
// This produces a valid Excel file without external dependencies by writing
// the minimum required XML parts of the OOXML format.
func Excel(w io.Writer, columns []Column, data DataAccess) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// [Content_Types].xml
	if err := writeZipFile(zw, "[Content_Types].xml", contentTypesXML); err != nil {
		return err
	}

	// _rels/.rels
	if err := writeZipFile(zw, "_rels/.rels", relsXML); err != nil {
		return err
	}

	// xl/_rels/workbook.xml.rels
	if err := writeZipFile(zw, "xl/_rels/workbook.xml.rels", workbookRelsXML); err != nil {
		return err
	}

	// xl/workbook.xml
	if err := writeZipFile(zw, "xl/workbook.xml", workbookXML); err != nil {
		return err
	}

	// xl/styles.xml
	if err := writeZipFile(zw, "xl/styles.xml", stylesXML); err != nil {
		return err
	}

	// xl/sharedStrings.xml + xl/worksheets/sheet1.xml
	strings, sharedStringsXML := buildSharedStrings(columns, data)
	if err := writeZipFile(zw, "xl/sharedStrings.xml", sharedStringsXML); err != nil {
		return err
	}

	sheetXML := buildSheetXML(columns, data, strings)
	if err := writeZipFile(zw, "xl/worksheets/sheet1.xml", sheetXML); err != nil {
		return err
	}

	return nil
}

func writeZipFile(zw *zip.Writer, name, content string) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

func buildSharedStrings(columns []Column, data DataAccess) (map[string]int, string) {
	idx := make(map[string]int)
	var strs []string

	addStr := func(s string) {
		if _, ok := idx[s]; !ok {
			idx[s] = len(strs)
			strs = append(strs, s)
		}
	}

	for _, col := range columns {
		addStr(col.Header)
	}

	for r := 0; r < data.RowCount(); r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		for _, col := range columns {
			var value any
			if col.ValueGetter != nil {
				value = col.ValueGetter(rowData)
			} else if m, ok := rowData.(map[string]any); ok {
				value = m[col.Field]
			}
			if col.Formatter != nil {
				addStr(col.Formatter(value))
			} else {
				addStr(formatAny(value))
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(fmt.Sprintf(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, len(strs), len(strs)))
	for _, s := range strs {
		sb.WriteString("<si><t>")
		sb.WriteString(escapeXML(s))
		sb.WriteString("</t></si>")
	}
	sb.WriteString("</sst>")

	return idx, sb.String()
}

func buildSheetXML(columns []Column, data DataAccess, strIdx map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	sb.WriteString("<sheetData>")

	// Header row
	sb.WriteString(`<row r="1">`)
	for c, col := range columns {
		ref := cellRef(0, c)
		sb.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, strIdx[col.Header]))
	}
	sb.WriteString("</row>")

	// Data rows
	for r := 0; r < data.RowCount(); r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, r+2))
		for c, col := range columns {
			var value any
			if col.ValueGetter != nil {
				value = col.ValueGetter(rowData)
			} else if m, ok := rowData.(map[string]any); ok {
				value = m[col.Field]
			}
			var s string
			if col.Formatter != nil {
				s = col.Formatter(value)
			} else {
				s = formatAny(value)
			}
			ref := cellRef(r+1, c)
			sb.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, strIdx[s]))
		}
		sb.WriteString("</row>")
	}

	sb.WriteString("</sheetData></worksheet>")
	return sb.String()
}

func cellRef(row, col int) string {
	colStr := ""
	c := col
	for {
		colStr = string(rune('A'+c%26)) + colStr
		c = c/26 - 1
		if c < 0 {
			break
		}
	}
	return fmt.Sprintf("%s%d", colStr, row+1)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const workbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="1"><fill><patternFill patternType="none"/></fill></fills>
  <borders count="1"><border/></borders>
  <cellStyleXfs count="1"><xf/></cellStyleXfs>
  <cellXfs count="1"><xf/></cellXfs>
</styleSheet>`
