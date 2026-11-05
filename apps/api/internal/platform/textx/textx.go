// Package textx holds small text helpers shared across domains.
package textx

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func StripAccents(s string) string {
	result, _, _ := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	return result
}

// Fold returns a comparison key: lowercase, without accents and with single spaces.
func Fold(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(StripAccents(s))), " ")
}
