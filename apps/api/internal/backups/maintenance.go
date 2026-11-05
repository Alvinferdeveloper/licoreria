package backups

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/httpx"
)

var ErrMaintenance = apperr.New(apperr.KindUnavailable, "maintenance", "El sistema se está restaurando. Vuelve a intentar en unos minutos.")

// Gate rejects API traffic during a restore and tracks requests in flight.
type Gate struct {
	closed   atomic.Bool
	inFlight atomic.Int64
}

func (g *Gate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.inFlight.Add(1)
		defer g.inFlight.Add(-1)
		if g.closed.Load() {
			w.Header().Set("Retry-After", "60")
			httpx.Fail(w, r, ErrMaintenance)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Close stops new requests and waits up to timeout for the others to finish; the caller's
// own request is the one left in flight.
func (g *Gate) Close(ctx context.Context, timeout time.Duration) {
	g.closed.Store(true)
	deadline := time.Now().Add(timeout)
	for g.inFlight.Load() > 1 && time.Now().Before(deadline) && ctx.Err() == nil {
		time.Sleep(50 * time.Millisecond)
	}
}

func (g *Gate) Open() { g.closed.Store(false) }
