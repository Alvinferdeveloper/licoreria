package monitor

import (
	"testing"
	"time"
)

func TestBackupStatus(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	ago := func(h int) *time.Time { at := now.Add(-time.Duration(h) * time.Hour); return &at }
	cases := []struct {
		name string
		b    Backups
		want string
	}{
		{"recent with offsite copy", Backups{LastAt: ago(10), Offsite: true}, StatusOK},
		{"recent but only on this server", Backups{LastAt: ago(10)}, StatusWarn},
		{"skipped a day", Backups{LastAt: ago(30), Offsite: true}, StatusWarn},
		{"never ran", Backups{Offsite: true}, StatusFail},
		{"newest attempt failed", Backups{LastAt: ago(20), LastFailedAt: ago(2), Offsite: true}, StatusFail},
	}
	for _, c := range cases {
		if got := backupStatus(now, c.b); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
