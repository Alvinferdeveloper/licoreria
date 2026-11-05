package products

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/textx"
)

var ErrImportNothingValid = apperr.New(apperr.KindValidation, "import_nothing_valid", "Ninguna fila es válida. Corrige el archivo y vuelve a subirlo.")

// ImportRow is the preview of one file row; Errors is empty when the row can be imported.
type ImportRow struct {
	Line     int                 `json:"line"`
	SKU      string              `json:"sku"`
	Name     string              `json:"name"`
	Category string              `json:"category"`
	Brand    string              `json:"brand"`
	Price    string              `json:"price"`
	NewBrand bool                `json:"newBrand"`
	Errors   []apperr.FieldError `json:"errors"`

	input    Input
	brandKey string
}

type ImportReport struct {
	Total     int         `json:"total"`
	Valid     int         `json:"valid"`
	Invalid   int         `json:"invalid"`
	Created   int         `json:"created"`
	NewBrands []string    `json:"newBrands"`
	Rows      []ImportRow `json:"rows"`
}

// importRecord holds a row's cleaned cells; json names match the column keys so
// validation errors point at the column the user sees.
type importRecord struct {
	SKU          string `json:"sku" validate:"required,min=2,max=40"`
	Barcode      string `json:"codigo_barras" validate:"omitempty,max=32"`
	Name         string `json:"nombre" validate:"required,min=2,max=160"`
	Description  string `json:"descripcion" validate:"max=1000"`
	Category     string `json:"categoria" validate:"required"`
	Brand        string `json:"marca" validate:"max=80"`
	Unit         string `json:"unidad" validate:"required"`
	Alcohol      string `json:"grado_alcohol" validate:"omitempty,percent"`
	Volume       string `json:"volumen_ml" validate:"omitempty,number"`
	Price        string `json:"precio" validate:"required,money"`
	Cost         string `json:"costo" validate:"omitempty,money"`
	Tax          string `json:"iva" validate:"omitempty,percent"`
	MinStock     string `json:"stock_minimo" validate:"omitempty,number"`
	ReorderPoint string `json:"punto_reorden" validate:"omitempty,number"`
}

func newImportRecord(v map[string]string) importRecord {
	return importRecord{
		SKU: strings.ToUpper(cleanCode(v[colSKU])), Barcode: cleanCode(v[colBarcode]),
		Name: strings.Join(strings.Fields(v[colName]), " "), Description: v[colDescription],
		Category: v[colCategory], Brand: strings.Join(strings.Fields(v[colBrand]), " "), Unit: v[colUnit],
		Alcohol: cleanNumber(v[colAlcohol]), Volume: cleanInt(v[colVolume]),
		Price: cleanNumber(v[colPrice]), Cost: cleanNumber(v[colCost]), Tax: cleanNumber(v[colTax]),
		MinStock: cleanInt(v[colMinStock]), ReorderPoint: cleanInt(v[colReorderPoint]),
	}
}

func cleanInt(v string) string {
	v = cleanNumber(v)
	if whole, frac, ok := strings.Cut(v, "."); ok && strings.Trim(frac, "0") == "" {
		return whole
	}
	return v
}

func atoiOrZero(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

// Import validates every row and, unless dryRun, inserts the valid ones in a single
// transaction (rows with errors are skipped). Unknown brands are created.
func (s *Service) Import(ctx context.Context, actor auth.Principal, data []byte, dryRun bool) (ImportReport, error) {
	if !canSeeCosts(actor) {
		return ImportReport{}, ErrPriceChangeForbidden
	}
	rows, err := readSheet(data)
	if err != nil {
		return ImportReport{}, err
	}
	if dryRun {
		return s.analyzeImport(ctx, s.pool, rows)
	}

	var report ImportReport
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if report, err = s.analyzeImport(ctx, tx, rows); err != nil {
			return err
		}
		if report.Valid == 0 {
			return ErrImportNothingValid
		}
		brandIDs := map[string]uuid.UUID{}
		for _, name := range report.NewBrands {
			id, err := s.repo.ensureBrand(ctx, tx, name)
			if err != nil {
				return err
			}
			brandIDs[textx.Fold(name)] = id
		}
		skus := make([]string, 0, report.Valid)
		for _, row := range report.Rows {
			if len(row.Errors) > 0 {
				continue
			}
			in := row.input
			if row.NewBrand {
				id := brandIDs[row.brandKey]
				in.BrandID = &id
			}
			if _, err := s.repo.insert(ctx, tx, in); err != nil {
				if appErr, ok := apperr.As(err); ok {
					return appErr.WithDetails(map[string]any{"line": row.Line})
				}
				return err
			}
			skus = append(skus, in.SKU)
		}
		report.Created = len(skus)
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.imported", Entity: "products",
			After: map[string]any{"created": report.Created, "skipped": report.Invalid, "newBrands": report.NewBrands, "skus": skus}})
	})
	return report, err
}

func (s *Service) analyzeImport(ctx context.Context, db database.DBTX, rows []sheetRow) (ImportReport, error) {
	records := make([]importRecord, len(rows))
	var skus, codes []string
	for i, row := range rows {
		records[i] = newImportRecord(row.values)
		skus = append(skus, records[i].SKU)
		if records[i].Barcode != "" {
			codes = append(codes, records[i].Barcode)
		}
	}
	refs, err := s.repo.importRefs(ctx, db, skus, codes)
	if err != nil {
		return ImportReport{}, err
	}

	report := ImportReport{Total: len(rows), NewBrands: []string{}, Rows: make([]ImportRow, len(rows))}
	seenSKU, seenCode, newBrands := map[string]int{}, map[string]int{}, map[string]bool{}
	for i, rec := range records {
		row := ImportRow{Line: rows[i].line, SKU: rec.SKU, Name: rec.Name, Category: rec.Category, Brand: rec.Brand,
			Price: rec.Price, Errors: []apperr.FieldError{}}
		fail := func(field, code, message string) {
			row.Errors = append(row.Errors, apperr.FieldError{Field: field, Code: code, Message: message})
		}
		if err := httpx.Validate(rec); err != nil {
			appErr, ok := apperr.As(err)
			if !ok {
				return ImportReport{}, err
			}
			row.Errors = append(row.Errors, appErr.Fields...)
		}

		in := Input{SKU: rec.SKU, Name: rec.Name, Description: rec.Description, Price: rec.Price,
			AverageCost: rec.Cost, TaxPercent: rec.Tax,
			MinStock: atoiOrZero(rec.MinStock), ReorderPoint: atoiOrZero(rec.ReorderPoint)}
		if rec.Barcode != "" {
			in.Barcode = &rec.Barcode
		}
		if rec.Alcohol != "" {
			in.AlcoholPercent = &rec.Alcohol
		}
		if rec.Volume != "" {
			volume := atoiOrZero(rec.Volume)
			in.VolumeML = &volume
		}
		// "number" validation already rejected non-digits; this bounds the digit-only values.
		for _, c := range []struct {
			field, value string
			min          int
		}{{colVolume, rec.Volume, 1}, {colMinStock, rec.MinStock, 0}, {colReorderPoint, rec.ReorderPoint, 0}} {
			if c.value == "" || strings.Trim(c.value, "0123456789") != "" {
				continue
			}
			if n, err := strconv.Atoi(c.value); err != nil || n < c.min || n > 100000 {
				fail(c.field, "range", fmt.Sprintf("Escribe un número entero entre %d y 100000.", c.min))
			}
		}

		if rec.SKU != "" {
			if refs.takenSKUs[rec.SKU] {
				fail(colSKU, "sku_taken", fmt.Sprintf("Ya existe un producto con el SKU %s.", rec.SKU))
			} else if line, dup := seenSKU[rec.SKU]; dup {
				fail(colSKU, "duplicate_in_file", fmt.Sprintf("El SKU se repite en la fila %d.", line))
			}
			seenSKU[rec.SKU] = row.Line
		}
		if rec.Barcode != "" {
			if refs.takenCodes[rec.Barcode] {
				fail(colBarcode, "barcode_taken", fmt.Sprintf("El código %s ya está asignado a otro producto o presentación.", rec.Barcode))
			} else if line, dup := seenCode[rec.Barcode]; dup {
				fail(colBarcode, "duplicate_in_file", fmt.Sprintf("El código se repite en la fila %d.", line))
			}
			seenCode[rec.Barcode] = row.Line
		}
		if rec.Category != "" {
			switch ids := refs.categories[textx.Fold(rec.Category)]; len(ids) {
			case 0:
				fail(colCategory, "not_found", fmt.Sprintf("No existe una categoría activa llamada «%s».", rec.Category))
			case 1:
				in.CategoryID = ids[0]
			default:
				fail(colCategory, "ambiguous", fmt.Sprintf("Hay varias categorías llamadas «%s». Renombra una antes de importar.", rec.Category))
			}
		}
		if rec.Unit != "" {
			if id, ok := refs.units[textx.Fold(rec.Unit)]; ok {
				in.UnitID = id
			} else {
				fail(colUnit, "not_found", fmt.Sprintf("No existe la unidad «%s».", rec.Unit))
			}
		}
		if rec.Brand != "" {
			key := textx.Fold(rec.Brand)
			if id, ok := refs.brands[key]; ok {
				in.BrandID = &id
			} else {
				row.NewBrand, row.brandKey = true, key
			}
		}

		if len(row.Errors) == 0 {
			row.input = normalize(in)
			report.Valid++
			if row.NewBrand && !newBrands[row.brandKey] {
				newBrands[row.brandKey] = true
				report.NewBrands = append(report.NewBrands, rec.Brand)
			}
		} else {
			report.Invalid++
		}
		report.Rows[i] = row
	}
	return report, nil
}

// ImportTemplate builds the .xlsx template with an example row and a sheet listing the
// categories and units the file may reference.
func (s *Service) ImportTemplate(ctx context.Context) ([]byte, error) {
	categories, units, err := s.repo.masterNames(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	book := excelize.NewFile()
	defer book.Close()

	const sheet, refSheet = "Productos", "Catálogo"
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	example := map[string]any{colSKU: "FDC-7-750", colBarcode: "7430011001073", colName: "Flor de Caña 7 años 750 ml",
		colDescription: "Ron añejo", colCategory: firstOr(categories, "Ron"), colBrand: "Flor de Caña", colUnit: "bot",
		colAlcohol: 40, colVolume: 750, colPrice: 421.75, colCost: 300, colTax: 15, colMinStock: 6, colReorderPoint: 12}
	bold, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F3EEE6"}}})
	if err != nil {
		return nil, err
	}
	text, err := book.NewStyle(&excelize.Style{NumFmt: 49})
	if err != nil {
		return nil, err
	}
	for i, key := range importColumns {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = book.SetCellValue(sheet, col+"1", key)
		_ = book.SetCellValue(sheet, col+"2", example[key])
		_ = book.SetColWidth(sheet, col, col, float64(max(len(key), 12)+4))
	}
	_ = book.SetCellStyle(sheet, "A1", "N1", bold)
	// SKU and barcode as text so long codes keep every digit.
	_ = book.SetColStyle(sheet, "A:B", text)
	_ = book.SetCellValue(sheet, "B2", "7430011001073")
	_ = book.SetColWidth(sheet, "C", "C", 34)
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})

	if _, err := book.NewSheet(refSheet); err != nil {
		return nil, err
	}
	_ = book.SetSheetRow(refSheet, "A1", &[]string{"categoria", "", "unidad", "abreviatura"})
	_ = book.SetCellStyle(refSheet, "A1", "D1", bold)
	_ = book.SetColWidth(refSheet, "A", "D", 24)
	for i, name := range categories {
		_ = book.SetCellValue(refSheet, fmt.Sprintf("A%d", i+2), name)
	}
	for i, unit := range units {
		_ = book.SetSheetRow(refSheet, fmt.Sprintf("C%d", i+2), &[]string{unit[0], unit[1]})
	}

	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		return nil, fmt.Errorf("write import template: %w", err)
	}
	return buf.Bytes(), nil
}

func firstOr(values []string, fallback string) string {
	if len(values) > 0 {
		return values[0]
	}
	return fallback
}
