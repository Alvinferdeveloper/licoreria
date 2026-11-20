package schedule

import (
	"testing"
	"time"

	"licoreria/api/internal/settings"
)

func at(day, hour, minute int) time.Time {
	// November 2026: the 16th is a Monday, the 22nd a Sunday.
	return time.Date(2026, time.November, day, hour, minute, 0, 0, Zone)
}

func TestStatus(t *testing.T) {
	s := settings.DefaultSchedule
	s.Enabled = true
	s.Days[time.Friday] = settings.ScheduleDay{Open: "16:00", Close: "02:00"}
	s.Days[time.Sunday] = settings.ScheduleDay{Closed: true}

	cases := []struct {
		name string
		t    time.Time
		open bool
		next time.Time
	}{
		{"inside", at(16, 12, 0), true, time.Time{}},
		{"tolerance before opening", at(16, 7, 30), true, time.Time{}},
		{"tolerance after closing", at(16, 22, 29), true, time.Time{}},
		{"after tolerance", at(16, 22, 30), false, at(17, 7, 30)},
		{"friday window past midnight", at(21, 1, 0), true, time.Time{}},
		{"closed sunday", at(22, 12, 0), false, at(23, 7, 30)},
	}
	for _, c := range cases {
		open, next := Status(s, c.t)
		if open != c.open || !next.Equal(c.next) {
			t.Errorf("%s: got open=%v next=%v, want open=%v next=%v", c.name, open, next, c.open, c.next)
		}
	}

	s.Enabled = false
	if open, _ := Status(s, at(22, 3, 0)); !open {
		t.Error("disabled schedule must always be open")
	}
}

func TestNextOccurrence(t *testing.T) {
	now := at(16, 22, 0)
	if got, _ := NextOccurrence("23:30", now); !got.Equal(at(16, 23, 30)) {
		t.Errorf("later today: got %v", got)
	}
	if got, _ := NextOccurrence("01:00", now); !got.Equal(at(17, 1, 0)) {
		t.Errorf("after midnight: got %v", got)
	}
	if _, ok := NextOccurrence("25:00", now); ok {
		t.Error("invalid clock accepted")
	}
}
