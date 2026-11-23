package backups

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"licoreria/api/internal/platform/pdfx"
)

func at(day, hour int) time.Time { return time.Date(2026, 11, day, hour, 0, 0, 0, pdfx.Store) }

func TestIsDue(t *testing.T) {
	yesterday, today := at(4, 3), at(5, 3)
	failedRecently := at(5, 9)
	cases := []struct {
		name          string
		now           time.Time
		done, attempt *time.Time
		want          bool
	}{
		{"never ran", at(5, 10), nil, nil, true},
		{"before the slot, yesterday's still counts", at(5, 1), &yesterday, &yesterday, false},
		{"after the slot, only yesterday's", at(5, 3), &yesterday, &yesterday, true},
		{"already done today", at(5, 10), &today, &today, false},
		{"failed less than an hour ago", failedRecently.Add(30 * time.Minute), &yesterday, &failedRecently, false},
	}
	for _, c := range cases {
		if got := isDue(c.now, 2, c.done, c.attempt); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	// With an 8 p.m. slot, last night's backup covers the day until 20:00.
	lastNight := at(4, 21)
	if isDue(at(5, 10), 20, &lastNight, &lastNight) || !isDue(at(5, 21), 20, &lastNight, &lastNight) {
		t.Error("a configured hour must move the daily slot")
	}
}

func TestExpiredKeepsDailyThenOnePerWeek(t *testing.T) {
	start := time.Date(2026, 11, 29, 3, 0, 0, 0, pdfx.Store) // Sunday
	var items []retained
	for i := range 60 {
		items = append(items, retained{id: uuid.New(), createdAt: start.AddDate(0, 0, -i)})
	}
	drop := expired(items, 7, 4)
	if kept := len(items) - len(drop); kept != 11 {
		t.Fatalf("kept %d backups, want 7 daily + 4 weekly", kept)
	}
	dropped := map[uuid.UUID]bool{}
	for _, id := range drop {
		dropped[id] = true
	}
	for _, item := range items[:7] {
		if dropped[item.id] {
			t.Fatalf("dropped a daily backup from %s", item.createdAt)
		}
	}
}
