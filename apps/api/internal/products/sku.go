package products

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/textx"
)

var volumePattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(ml|cl|l)\b`)

func letters(s string) []string {
	return strings.FieldsFunc(strings.ToUpper(textx.StripAccents(s)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// BaseSKU builds BRAND-NAME-VOLUME: brand initials (or its first three letters when it's one
// word), the first three characters of the rest of the name and the volume in ml.
// "Flor de Caña 7 años 750 ml" by "Flor de Caña" → FDC-7AN-750.
func BaseSKU(brand, name string) string {
	volume := ""
	if m := volumePattern.FindStringSubmatch(name); m != nil {
		amount, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		switch strings.ToLower(m[2]) {
		case "l":
			amount *= 1000
		case "cl":
			amount *= 10
		}
		volume = strconv.Itoa(int(amount))
		name = strings.Replace(name, m[0], " ", 1)
	}

	rest := letters(name)
	brandWords := letters(brand)
	if len(brandWords) == 0 && len(rest) > 0 {
		brandWords, rest = rest[:1], rest[1:]
	}
	// The brand usually opens the product name; don't repeat it in the second part.
	for len(brandWords) > 0 && len(rest) > 0 && rest[0] == brandWords[0] {
		brandWords, rest = brandWords[1:], rest[1:]
		if len(brandWords) == 0 {
			brandWords = letters(brand)
			break
		}
	}

	var brandCode string
	if len(brandWords) == 1 {
		brandCode = first(brandWords[0], 3)
	} else {
		for _, w := range brandWords {
			brandCode += first(w, 1)
		}
		brandCode = first(brandCode, 4)
	}
	parts := []string{brandCode, first(strings.Join(rest, ""), 3), volume}
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "PRD"
	}
	return strings.Join(out, "-")
}

func first(s string, n int) string {
	r := []rune(s)
	return string(r[:min(n, len(r))])
}

// SuggestSKU returns BaseSKU, or BaseSKU-2, -3… when that code is already taken.
func (s *Service) SuggestSKU(ctx context.Context, brandID *uuid.UUID, name string) (string, error) {
	return suggestSKU(ctx, s.pool, brandID, name)
}

func suggestSKU(ctx context.Context, db database.DBTX, brandID *uuid.UUID, name string) (string, error) {
	brand := ""
	if brandID != nil {
		if err := db.QueryRow(ctx, `SELECT coalesce((SELECT name FROM brands WHERE id = $1), '')`, *brandID).Scan(&brand); err != nil {
			return "", fmt.Errorf("load brand: %w", err)
		}
	}
	base := BaseSKU(brand, name)
	rows, err := db.Query(ctx, `SELECT upper(sku::text) FROM products WHERE sku ILIKE $1 || '%'`, base)
	if err != nil {
		return "", fmt.Errorf("check sku: %w", err)
	}
	taken := map[string]bool{}
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return "", err
		}
		taken[sku] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	candidate := base
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", base, n)
	}
	return candidate, nil
}
