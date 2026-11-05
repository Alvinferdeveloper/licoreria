// Package pdfx holds the shared look of generated PDFs (invoices, reports): embedded fonts,
// the paper-and-ink palette and a few text styles.
package pdfx

import (
	"fmt"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/fontrepository"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	Sans = "gosans"
	Mono = "gomono"
)

// Store is the store's zone (Nicaragua has no daylight saving time).
var Store = time.FixedZone("America/Managua", -6*60*60)

// DateTime formats an instant in the store's zone as "05/11/2026 10:21 a. m.".
func DateTime(t time.Time) string {
	return strings.NewReplacer("AM", "a. m.", "PM", "p. m.").Replace(t.In(Store).Format("02/01/2006 03:04 PM"))
}

var (
	Ink    = &props.Color{Red: 35, Green: 27, Blue: 22}
	Muted  = &props.Color{Red: 95, Green: 86, Blue: 79}
	Line   = &props.Color{Red: 226, Green: 219, Blue: 212}
	Sunken = &props.Color{Red: 243, Green: 238, Blue: 230}
	Amber  = &props.Color{Red: 173, Green: 89, Blue: 7}
	Danger = &props.Color{Red: 178, Green: 34, Blue: 34}
)

// New returns a letter-size document with the embedded fonts and the given title.
func New(title string) (core.Maroto, error) {
	fonts, err := fontrepository.New().
		AddUTF8FontFromBytes(Sans, fontstyle.Normal, goregular.TTF).
		AddUTF8FontFromBytes(Sans, fontstyle.Bold, gobold.TTF).
		AddUTF8FontFromBytes(Mono, fontstyle.Normal, gomono.TTF).
		AddUTF8FontFromBytes(Mono, fontstyle.Bold, gomonobold.TTF).
		Load()
	if err != nil {
		return nil, fmt.Errorf("load pdf fonts: %w", err)
	}
	cfg := config.NewBuilder().
		WithPageSize(pagesize.Letter).
		WithLeftMargin(15).WithRightMargin(15).WithTopMargin(15).WithBottomMargin(12).
		WithCustomFonts(fonts).
		WithDefaultFont(&props.Font{Family: Sans, Size: 9, Color: Ink}).
		WithTitle(title, true).
		WithSequentialMode().
		Build()
	return maroto.New(cfg), nil
}

// Text builds a text style; mono is for amounts and codes.
func Text(size float64, bold, mono bool, a align.Type, color *props.Color) props.Text {
	family, style := Sans, fontstyle.Normal
	if mono {
		family = Mono
	}
	if bold {
		style = fontstyle.Bold
	}
	if color == nil {
		color = Ink
	}
	return props.Text{Family: family, Style: style, Size: size, Align: a, Color: color}
}
