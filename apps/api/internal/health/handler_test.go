package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	cases := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   Status
	}{
		{"database up", nil, http.StatusOK, Status{Status: "ok", Database: "ok"}},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, Status{Status: "degraded", Database: "unreachable"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			NewHandler(fakePinger{err: tc.pingErr}).Routes(r)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			var body Status
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != tc.wantStatus || body != tc.wantBody {
				t.Errorf("got %d %+v, want %d %+v", rec.Code, body, tc.wantStatus, tc.wantBody)
			}
		})
	}
}
