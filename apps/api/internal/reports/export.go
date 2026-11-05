package reports

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/xuri/excelize/v2"

	"licoreria/api/internal/platform/pdfx"
)

// display formats a cell the way people read it: "1,250.50", "05/11/2026 10:21 a. m.".
func display(v any, kind ColumnKind) string {
	switch val := v.(type) {
	case nil:
		return "—"
	case time.Time:
		if kind == ColDate {
			return val.In(pdfx.Store).Format("02/01/2006")
		}
		return pdfx.DateTime(val)
	case string:
		switch kind {
		case ColMoney:
			return thousands(val)
		case ColPercent:
			return val + " %"
		case ColDate:
			if len(val) == 10 {
				return val[8:10] + "/" + val[5:7] + "/" + val[0:4]
			}
		}
		return val
	default:
		return fmt.Sprint(val)
	}
}

func thousands(amount string) string {
	c := cents(amount)
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	whole := strconv.FormatInt(c/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), c%100)
}

// visibleColumns drops the grouping column; its value becomes a group header instead.
func visibleColumns(def Definition) []Column {
	if def.GroupBy == "" {
		return def.Columns
	}
	out := make([]Column, 0, len(def.Columns))
	for _, c := range def.Columns {
		if c.Key != def.GroupBy {
			out = append(out, c)
		}
	}
	return out
}

func numeric(kind ColumnKind) bool {
	return kind == ColMoney || kind == ColNumber || kind == ColPercent
}

// Excel writes the result as a sheet with real numbers so totals and filters work in Excel.
func Excel(def Definition, res Result, storeName string) ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	sheet := "Reporte"
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	cols := visibleColumns(def)
	title, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	muted, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "5F564F"}})
	header, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F3EEE6"}}})
	group, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FBF9F5"}}})
	money, _ := book.NewStyle(&excelize.Style{NumFmt: 4})
	moneyBold, _ := book.NewStyle(&excelize.Style{NumFmt: 4, Font: &excelize.Font{Bold: true}})
	bold, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	cell := func(c, r int) string { name, _ := excelize.CoordinatesToCellName(c, r); return name }
	_ = book.SetCellValue(sheet, "A1", res.Title)
	_ = book.SetCellStyle(sheet, "A1", "A1", title)
	meta := append([]string{storeName, "Generado: " + pdfx.DateTime(time.Now())}, res.Filters...)
	r := 2
	for _, line := range meta {
		_ = book.SetCellValue(sheet, cell(1, r), line)
		_ = book.SetCellStyle(sheet, cell(1, r), cell(1, r), muted)
		r++
	}
	r++
	for i, c := range cols {
		_ = book.SetCellValue(sheet, cell(i+1, r), c.Label)
		width := float64(max(len(c.Label), c.Width*4) + 2)
		colName, _ := excelize.ColumnNumberToName(i + 1)
		_ = book.SetColWidth(sheet, colName, colName, width)
	}
	_ = book.SetCellStyle(sheet, cell(1, r), cell(len(cols), r), header)
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: r, TopLeftCell: cell(1, r+1), ActivePane: "bottomLeft"})
	r++

	write := func(c Column, i, r int, v any, isTotal bool) {
		ref := cell(i+1, r)
		switch val := v.(type) {
		case string:
			if numeric(c.Kind) {
				f, _ := strconv.ParseFloat(val, 64)
				_ = book.SetCellValue(sheet, ref, f)
				if c.Kind == ColMoney {
					_ = book.SetCellStyle(sheet, ref, ref, map[bool]int{true: moneyBold, false: money}[isTotal])
				}
				return
			}
		case time.Time:
			_ = book.SetCellValue(sheet, ref, display(val, c.Kind))
			return
		}
		if v == nil {
			return
		}
		_ = book.SetCellValue(sheet, ref, v)
		if isTotal {
			_ = book.SetCellStyle(sheet, ref, ref, bold)
		}
	}

	lastGroup := ""
	for _, row := range res.Rows {
		if def.GroupBy != "" {
			if g := display(row[def.GroupBy], ColText); g != lastGroup {
				_ = book.SetCellValue(sheet, cell(1, r), g)
				_ = book.SetCellStyle(sheet, cell(1, r), cell(len(cols), r), group)
				lastGroup = g
				r++
			}
		}
		for i, c := range cols {
			write(c, i, r, row[c.Key], false)
		}
		r++
	}
	if res.Totals != nil {
		_ = book.SetCellValue(sheet, cell(1, r), "Total")
		_ = book.SetCellStyle(sheet, cell(1, r), cell(1, r), bold)
		for i, c := range cols {
			if v, ok := res.Totals[c.Key]; ok && i > 0 {
				write(c, i, r, v, true)
			}
		}
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		return nil, fmt.Errorf("write report xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// PDF renders the result as a paginated table; wide reports go landscape.
func PDF(def Definition, res Result, storeName string) ([]byte, error) {
	cols := visibleColumns(def)
	grid := 0
	for _, c := range cols {
		grid += c.Width
	}
	m, err := pdfx.NewDocument(res.Title, grid > 26, grid)
	if err != nil {
		return nil, err
	}
	muted := pdfx.Text(8, false, false, align.Left, pdfx.Muted)
	m.AddRows(text.NewRow(8, res.Title, pdfx.Text(14, true, false, align.Left, nil)))
	meta := strings.Join(append([]string{storeName, "Generado: " + pdfx.DateTime(time.Now())}, res.Filters...), " · ")
	m.AddRows(text.NewRow(6, meta, muted), row.New(3))

	pad := func(t props.Text) props.Text { t.Top, t.Bottom, t.Left, t.Right = 1.6, 1.6, 1, 1; return t }
	cellText := func(c Column, bold bool) props.Text {
		a := align.Left
		if numeric(c.Kind) {
			a = align.Right
		}
		return pad(pdfx.Text(7.5, bold, c.Kind == ColMono || numeric(c.Kind), a, nil))
	}
	head := row.New()
	for _, c := range cols {
		t := cellText(c, true)
		t.Family, t.Color = pdfx.Sans, pdfx.Muted
		head.Add(text.NewCol(c.Width, c.Label, t))
	}
	m.AddRows(head.WithStyle(&props.Cell{BackgroundColor: pdfx.Sunken}))

	lastGroup := ""
	for _, r := range res.Rows {
		if def.GroupBy != "" {
			if g := display(r[def.GroupBy], ColText); g != lastGroup {
				m.AddRows(text.NewRow(7, g, pad(pdfx.Text(8.5, true, false, align.Left, nil))))
				lastGroup = g
			}
		}
		line := row.New()
		for _, c := range cols {
			line.Add(text.NewCol(c.Width, display(r[c.Key], c.Kind), cellText(c, false)))
		}
		m.AddAutoRow(line.GetColumns()...).WithStyle(&props.Cell{BorderColor: pdfx.Line, BorderType: border.Bottom})
	}
	if res.Totals != nil {
		line := row.New()
		for i, c := range cols {
			v, ok := res.Totals[c.Key]
			switch {
			case i == 0:
				line.Add(text.NewCol(c.Width, "Total", cellText(c, true)))
			case ok:
				line.Add(text.NewCol(c.Width, display(v, c.Kind), cellText(c, true)))
			default:
				line.Add(col.New(c.Width))
			}
		}
		m.AddAutoRow(line.GetColumns()...)
	}
	if res.Truncated {
		m.AddRows(text.NewRow(8, fmt.Sprintf("Se muestran las primeras %d filas. Acota los filtros para ver el resto.", len(res.Rows)), muted))
	}
	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate report pdf: %w", err)
	}
	return doc.GetBytes(), nil
}
