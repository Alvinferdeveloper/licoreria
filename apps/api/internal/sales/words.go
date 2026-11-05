package sales

import (
	"fmt"
	"strings"
)

var (
	units    = []string{"", "uno", "dos", "tres", "cuatro", "cinco", "seis", "siete", "ocho", "nueve", "diez", "once", "doce", "trece", "catorce", "quince", "dieciséis", "diecisiete", "dieciocho", "diecinueve", "veinte", "veintiuno", "veintidós", "veintitrés", "veinticuatro", "veinticinco", "veintiséis", "veintisiete", "veintiocho", "veintinueve"}
	tens     = []string{"", "", "", "treinta", "cuarenta", "cincuenta", "sesenta", "setenta", "ochenta", "noventa"}
	hundreds = []string{"", "ciento", "doscientos", "trescientos", "cuatrocientos", "quinientos", "seiscientos", "setecientos", "ochocientos", "novecientos"}
)

// AmountInWords spells córdobas the way invoices do: "Ochocientos cincuenta córdobas con 00/100".
func AmountInWords(cents int64) string {
	whole := cents / 100
	words := "cero"
	if whole > 0 {
		words = spell(whole)
	}
	// "uno" becomes "un" before a noun: "veintiún córdobas", "un millón".
	if strings.HasSuffix(words, "uno") {
		words = strings.TrimSuffix(words, "uno") + "un"
	}
	if strings.HasSuffix(words, "veintiun") {
		words = strings.TrimSuffix(words, "veintiun") + "veintiún"
	}
	currency := "córdobas"
	if whole == 1 {
		currency = "córdoba"
	}
	text := fmt.Sprintf("%s %s con %02d/100", words, currency, cents%100)
	return strings.ToUpper(text[:1]) + text[1:]
}

func spell(n int64) string {
	switch {
	case n >= 1_000_000:
		millions, rest := n/1_000_000, n%1_000_000
		head := "un millón"
		if millions > 1 {
			head = apocope(spell(millions)) + " millones"
		}
		return join(head, rest)
	case n >= 1000:
		thousands, rest := n/1000, n%1000
		head := "mil"
		if thousands > 1 {
			head = apocope(spell(thousands)) + " mil"
		}
		return join(head, rest)
	case n == 100:
		return "cien"
	case n >= 100:
		return join(hundreds[n/100], n%100)
	case n >= 30:
		if n%10 == 0 {
			return tens[n/10]
		}
		return tens[n/10] + " y " + units[n%10]
	default:
		return units[n]
	}
}

func join(head string, rest int64) string {
	if rest == 0 {
		return head
	}
	return head + " " + spell(rest)
}

// apocope shortens a trailing "uno" before "mil"/"millones": "veintiún mil", "treinta y un mil".
func apocope(s string) string {
	if strings.HasSuffix(s, "veintiuno") {
		return strings.TrimSuffix(s, "veintiuno") + "veintiún"
	}
	if strings.HasSuffix(s, "uno") {
		return strings.TrimSuffix(s, "uno") + "un"
	}
	return s
}
