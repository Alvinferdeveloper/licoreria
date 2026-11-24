package httpx

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const (
	statsHours   = 24
	recentErrors = 10
)

// HourStats is the traffic of one clock hour.
type HourStats struct {
	Hour         time.Time `json:"hour"`
	Requests     int       `json:"requests"`
	ClientErrors int       `json:"clientErrors"`
	ServerErrors int       `json:"serverErrors"`
	AvgMs        float64   `json:"avgMs"`
	MaxMs        int64     `json:"maxMs"`
	totalMs      int64
}

// FailedRequest is a 5xx kept so an administrator can trace it in the logs by request id.
type FailedRequest struct {
	At        time.Time `json:"at"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	RequestID string    `json:"requestId"`
}

// Stats counts requests in memory, per hour, for the last 24 hours. It resets on restart.
type Stats struct {
	mu      sync.Mutex
	now     func() time.Time
	started time.Time
	hours   [statsHours]HourStats
	failed  []FailedRequest
}

func NewStats(now func() time.Time) *Stats {
	return &Stats{now: now, started: now()}
}

func (s *Stats) Started() time.Time { return s.started }

// Middleware records every response; it goes inside RequestID so failures carry their id.
func (s *Stats) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Probes and the status page polling itself would only drown the real traffic.
		if strings.HasSuffix(r.URL.Path, "/health") || strings.HasSuffix(r.URL.Path, "/system/status") {
			next.ServeHTTP(w, r)
			return
		}
		start := s.now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		s.record(start, s.now().Sub(start), r.Method, r.URL.Path, status, middleware.GetReqID(r.Context()))
	})
}

func (s *Stats) record(at time.Time, took time.Duration, method, path string, status int, requestID string) {
	hour := at.Truncate(time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := &s.hours[hour.Unix()/3600%statsHours]
	if !slot.Hour.Equal(hour) {
		*slot = HourStats{Hour: hour}
	}
	ms := took.Milliseconds()
	slot.Requests++
	slot.totalMs += ms
	slot.MaxMs = max(slot.MaxMs, ms)
	switch {
	case status >= http.StatusInternalServerError:
		slot.ServerErrors++
		s.failed = append([]FailedRequest{{At: at, Method: method, Path: path, Status: status, RequestID: requestID}}, s.failed...)
		if len(s.failed) > recentErrors {
			s.failed = s.failed[:recentErrors]
		}
	case status >= http.StatusBadRequest && status != StatusClientClosed:
		slot.ClientErrors++
	}
}

// Snapshot returns the last 24 hours oldest first, empty hours included, and the latest failures.
func (s *Stats) Snapshot() ([]HourStats, []FailedRequest) {
	current := s.now().Truncate(time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	hours := make([]HourStats, 0, statsHours)
	for i := statsHours - 1; i >= 0; i-- {
		hour := current.Add(-time.Duration(i) * time.Hour)
		slot := s.hours[hour.Unix()/3600%statsHours]
		if !slot.Hour.Equal(hour) {
			slot = HourStats{Hour: hour}
		}
		if slot.Requests > 0 {
			slot.AvgMs = float64(slot.totalMs) / float64(slot.Requests)
		}
		hours = append(hours, slot)
	}
	return hours, append([]FailedRequest{}, s.failed...)
}
