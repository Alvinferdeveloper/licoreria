package products

import (
	"bytes"
	"encoding/csv"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/platform/textx"
)

const (
	MaxImportBytes = 5 << 20
	maxImportRows  = 2000
)

var (
	ErrImportFile = apperr.Validation(apperr.FieldError{Field: "file", Code: "invalid_file",
		Message: "Sube un archivo Excel (.xlsx) o CSV de hasta 5 MB."})
	ErrImportEmpty    = apperr.New(apperr.KindValidation, "import_empty", "El archivo no tiene filas de productos.")
	ErrImportTooLarge = apperr.New(apperr.KindValidation, "import_too_many_rows", "El archivo supera las 2000 filas. Divídelo en partes.")
)

// Column keys double as the field names reported in row errors.
const (
	colSKU          = "sku"
	colBarcode      = "codigo_barras"
	colName         = "nombre"
	colDescription  = "descripcion"
	colCategory     = "categoria"
	colBrand        = "marca"
	colUnit         = "unidad"
	colAlcohol      = "grado_alcohol"
	colVolume       = "volumen_ml"
	colPrice        = "precio"
	colCost         = "costo"
	colTax          = "iva"
	colMinStock     = "stock_minimo"
	colReorderPoint = "punto_reorden"
)

// importColumns is the template order; headers in the file may come in any order.
var importColumns = []string{colSKU, colBarcode, colName, colDescription, colCategory, colBrand, colUnit,
	colAlcohol, colVolume, colPrice, colCost, colTax, colMinStock, colReorderPoint}

var requiredColumns = []string{colSKU, colName, colCategory, colUnit, colPrice}

// columnAliases maps a folded header (no accents, spaces or symbols) to its column key.
var columnAliases = map[string]string{
	"sku": colSKU, "codigo": colSKU, "codigointerno": colSKU,
	"codigobarras": colBarcode, "codigodebarras": colBarcode, "barcode": colBarcode, "ean": colBarcode,
	"nombre": colName, "producto": colName, "descripcioncorta": colName,
	"descripcion": colDescription,
	"categoria":   colCategory,
	"marca":       colBrand,
	"unidad":      colUnit, "unidaddemedida": colUnit,
	"gradoalcohol": colAlcohol, "gradoalcoholico": colAlcohol, "alcohol": colAlcohol, "grado": colAlcohol,
	"volumenml": colVolume, "volumen": colVolume, "ml": colVolume,
	"precio": colPrice, "precioventa": colPrice, "preciodeventa": colPrice,
	"costo": colCost, "costopromedio": colCost, "costounitario": colCost,
	"iva": colTax, "impuesto": colTax,
	"stockminimo": colMinStock, "minimo": colMinStock,
	"puntoreorden": colReorderPoint, "puntodereorden": colReorderPoint, "reorden": colReorderPoint,
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func headerKey(header string) string {
	return columnAliases[nonAlnum.ReplaceAllString(textx.Fold(header), "")]
}

// sheetRow is one data row keyed by column, with its 1-based line in the file.
type sheetRow struct {
	line   int
	values map[string]string
}

// readSheet parses the first worksheet of an .xlsx file or a CSV (comma or semicolon).
func readSheet(data []byte) ([]sheetRow, error) {
	var rows [][]string
	var err error
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		rows, err = readXLSX(data)
	} else {
		rows, err = readCSV(data)
	}
	if err != nil {
		return nil, ErrImportFile.WithCause(err)
	}
	return mapRows(rows)
}

func readXLSX(data []byte) ([][]string, error) {
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer book.Close()
	return book.GetRows(book.GetSheetName(0))
}

func readCSV(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, apperr.ErrInvalid
	}
	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	reader := csv.NewReader(bytes.NewReader(data))
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		reader.Comma = ';'
	}
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

func mapRows(rows [][]string) ([]sheetRow, error) {
	if len(rows) == 0 {
		return nil, ErrImportEmpty
	}
	index := map[string]int{}
	for i, header := range rows[0] {
		if key := headerKey(header); key != "" {
			if _, dup := index[key]; !dup {
				index[key] = i
			}
		}
	}
	var missing []string
	for _, key := range requiredColumns {
		if _, ok := index[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, apperr.New(apperr.KindValidation, "import_missing_columns",
			"Faltan columnas obligatorias: "+strings.Join(missing, ", ")+". Usa la plantilla.").
			WithDetails(map[string]any{"columns": missing})
	}

	var out []sheetRow
	for i, raw := range rows[1:] {
		values := map[string]string{}
		blank := true
		for key, col := range index {
			if col < len(raw) {
				if v := strings.TrimSpace(raw[col]); v != "" {
					values[key] = v
					blank = false
				}
			}
		}
		if blank {
			continue
		}
		out = append(out, sheetRow{line: i + 2, values: values})
	}
	if len(out) == 0 {
		return nil, ErrImportEmpty
	}
	if len(out) > maxImportRows {
		return nil, ErrImportTooLarge
	}
	return out, nil
}

var (
	scientific     = regexp.MustCompile(`^\d+(\.\d+)?[eE]\+?\d+$`)
	decimalComma   = regexp.MustCompile(`^\d+,\d{1,2}$`)
	thousandsComma = regexp.MustCompile(`^\d{1,3}(,\d{3})+$`)
)

// cleanCode undoes what spreadsheets do to long numeric codes ("7.50105E+12", "123.0").
func cleanCode(v string) string {
	v = strings.TrimSpace(v)
	if scientific.MatchString(v) {
		if f, _, err := big.ParseFloat(v, 10, 128, big.ToNearestEven); err == nil && f.IsInt() {
			return f.Text('f', 0)
		}
	}
	return strings.TrimSuffix(v, ".0")
}

// cleanNumber turns "C$ 1,250.50", "15%" or "421,75" into a plain decimal string. Values it
// cannot interpret are returned as typed so validation reports them.
func cleanNumber(v string) string {
	v = strings.NewReplacer("C$", "", "US$", "", "$", "", "%", "", " ", "", " ", "").Replace(v)
	switch {
	case strings.Contains(v, ",") && strings.Contains(v, "."):
		v = strings.ReplaceAll(v, ",", "")
	case decimalComma.MatchString(v):
		v = strings.Replace(v, ",", ".", 1)
	case thousandsComma.MatchString(v):
		v = strings.ReplaceAll(v, ",", "")
	}
	// Spreadsheet float noise such as 421.75000000000006.
	if whole, frac, ok := strings.Cut(v, "."); ok && len(frac) > 4 && whole != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if r := math.Round(f*100) / 100; math.Abs(f-r) < 1e-6 {
				v = strconv.FormatFloat(r, 'f', -1, 64)
			}
		}
	}
	return v
}
