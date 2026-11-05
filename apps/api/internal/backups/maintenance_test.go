package backups

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGateRejectsRequestsWhileClosed(t *testing.T) {
	gate := &Gate{}
	handler := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec
	}

	if rec := call(); rec.Code != http.StatusNoContent {
		t.Fatalf("open gate: status %d", rec.Code)
	}
	gate.Close(context.Background(), 0)
	if rec := call(); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"maintenance"`) {
		t.Fatalf("closed gate: status %d, body %s", rec.Code, rec.Body)
	}
	gate.Open()
	if rec := call(); rec.Code != http.StatusNoContent {
		t.Fatalf("reopened gate: status %d", rec.Code)
	}
}
