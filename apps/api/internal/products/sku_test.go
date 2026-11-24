package products

import "testing"

func TestBaseSKU(t *testing.T) {
	cases := []struct{ brand, name, want string }{
		{"Flor de Caña", "Flor de Caña 7 años 750 ml", "FDC-7AN-750"},
		{"Ron Plata", "Ron Plata Blanco 1.75 L", "RP-BLA-1750"},
		{"Toña", "Toña lata 355 ml", "TON-LAT-355"},
		{"", "Hielo en bolsa", "HIE-ENB"},
	}
	for _, c := range cases {
		if got := BaseSKU(c.brand, c.name); got != c.want {
			t.Errorf("BaseSKU(%q, %q) = %q, want %q", c.brand, c.name, got, c.want)
		}
	}
}
