package inventory

import "testing"

func days(perDay int) []int {
	h := make([]int, HistoryDays)
	for i := range h {
		h[i] = perDay
	}
	return h
}

func TestPlan(t *testing.T) {
	cases := []struct {
		name      string
		f         Forecast
		want      bool
		suggested int
		urgency   string
		daysLeft  float64
	}{
		// 2 a day for 14 days = 28, minus the 10 on hand = 18, rounded up to boxes of 12.
		{"runs out within coverage, whole boxes", Forecast{Stock: 10, History: days(2), PackSize: 12}, true, 24, UrgencySoon, 5},
		{"lasts longer than coverage", Forecast{Stock: 100, History: days(2)}, false, 0, "", 50},
		{"sold out", Forecast{Stock: 0, MinStock: 3, History: days(1)}, true, 17, UrgencyOut, 0},
		{"three days or less is critical", Forecast{Stock: 5, History: days(2)}, true, 23, UrgencyCritical, 2.5},
		{"no sales and above minimum is not reordered", Forecast{Stock: 4, MinStock: 2, History: days(0)}, false, 0, "", 0},
		{"slow seller under its minimum", Forecast{Stock: 3, MinStock: 6, History: days(0)}, true, 3, UrgencyMin, 0},
	}
	for _, c := range cases {
		f := c.f
		got := plan(&f, 14)
		if got != c.want || f.Suggested != c.suggested || f.Urgency != c.urgency {
			t.Errorf("%s: got %v suggested %d urgency %q", c.name, got, f.Suggested, f.Urgency)
		}
		if c.daysLeft > 0 && (f.DaysLeft == nil || *f.DaysLeft != c.daysLeft) {
			t.Errorf("%s: days left %v, want %v", c.name, f.DaysLeft, c.daysLeft)
		}
	}

	// A week selling faster than the month raises the rate above the monthly average.
	h := days(1)
	for i := HistoryDays - 7; i < HistoryDays; i++ {
		h[i] = 4
	}
	f := Forecast{Stock: 30, History: h}
	plan(&f, 14)
	if f.DailyRate <= 1.75 {
		t.Errorf("recent week should weigh more: rate %v", f.DailyRate)
	}
}
