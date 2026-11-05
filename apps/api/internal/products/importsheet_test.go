package products

import "testing"

func TestCleanCells(t *testing.T) {
	for in, want := range map[string]string{"C$ 1,250.50": "1250.50", "421,75": "421.75", "421.75000000000006": "421.75"} {
		if got := cleanNumber(in); got != want {
			t.Errorf("cleanNumber(%q) = %q, want %q", in, got, want)
		}
	}
	if got := cleanCode("7.430011001073E+12"); got != "7430011001073" {
		t.Errorf("cleanCode = %q", got)
	}
}
