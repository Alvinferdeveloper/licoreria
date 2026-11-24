package risk

import (
	"slices"
	"testing"
)

func TestAssessComparesWithTheStore(t *testing.T) {
	cashiers := []Cashier{
		{Sales: 1000, Voids: 5, Shifts: 30, ShortShifts: 3},
		{Sales: 1000, Voids: 5, Shifts: 30, ShortShifts: 3},
		// Twice the voids of the others and short on most closes.
		{Sales: 500, Voids: 8, Shifts: 15, ShortShifts: 9, PINFailures: 4},
		// Rare but small: two voids are not a pattern.
		{Sales: 20, Voids: 2, OutsideHours: 1},
	}
	store := assess(cashiers)

	if store.Void <= 0 || store.Short <= 0 {
		t.Fatalf("store rates not computed: %+v", store)
	}
	if cashiers[0].Level != LevelLow || len(cashiers[0].Flags) != 0 {
		t.Errorf("average cashier flagged: %+v", cashiers[0])
	}
	if got := cashiers[2]; got.Level != LevelHigh || !slices.Contains(got.Flags, FlagVoids) || !slices.Contains(got.Flags, FlagShortages) || !slices.Contains(got.Flags, FlagPIN) {
		t.Errorf("outlier not flagged: %+v", got)
	}
	if got := cashiers[3]; got.Level != LevelMedium || slices.Contains(got.Flags, FlagVoids) {
		t.Errorf("few voids should not flag, outside hours should: %+v", got)
	}
}
