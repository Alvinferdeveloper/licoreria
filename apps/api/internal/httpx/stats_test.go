package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatsCountsByHourAndKeepsFailures(t *testing.T) {
	now := time.Date(2026, 11, 23, 10, 30, 0, 0, time.UTC)
	stats := NewStats(func() time.Time { return now })
	status := http.StatusOK
	h := stats.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))

	for _, s := range []int{200, 404, StatusClientClosed, 500} {
		status = s
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil))
	now = now.Add(25 * time.Hour) // the old hour falls out of the window
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))

	hours, failed := stats.Snapshot()
	last := hours[len(hours)-1]
	total := 0
	for _, hour := range hours {
		total += hour.Requests
	}
	if len(hours) != 24 || total != 1 || last.ServerErrors != 1 {
		t.Errorf("window should only hold the latest hour: total %d, last %+v", total, last)
	}
	if len(failed) != 2 || failed[1].Path != "/api/v1/products" {
		t.Errorf("failures not kept newest first: %+v", failed)
	}
}
