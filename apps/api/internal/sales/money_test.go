package sales

import "testing"

func TestMoneyRules(t *testing.T) {
	if got := usdToNIO(3000, 36_600_000); got != 109_800 {
		t.Errorf("US$ 30 at 36.60 = %d cents, want 109800", got)
	}
	if got := percentOf(99_999, 1500); got != 15_000 {
		t.Errorf("15%% of 999.99 = %d, want 15000", got)
	}
	shares := allocate(1000, []int64{3333, 3333, 3334})
	if shares[0]+shares[1]+shares[2] != 1000 {
		t.Errorf("allocation must add up: %v", shares)
	}
	for cents, want := range map[int64]string{
		85_000:      "Ochocientos cincuenta córdobas con 00/100",
		59_114:      "Quinientos noventa y un córdobas con 14/100",
		2_100_000:   "Veintiún mil córdobas con 00/100",
		100:         "Un córdoba con 00/100",
		150_000_000: "Un millón quinientos mil córdobas con 00/100",
	} {
		if got := AmountInWords(cents); got != want {
			t.Errorf("AmountInWords(%d) = %q, want %q", cents, got, want)
		}
	}
	if c, err := parseCents("1250.5"); err != nil || c != 125_050 || formatCents(-4950) != "-49.50" {
		t.Errorf("parse/format: %d %v %s", c, err, formatCents(-4950))
	}
}
