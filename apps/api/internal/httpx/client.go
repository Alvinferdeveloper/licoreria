package httpx

import (
	"context"
	"net"
	"net/http"
)

// ClientInfo identifies who made a request, for the audit log and refresh sessions.
type ClientInfo struct {
	IP        string
	UserAgent string
}

type clientKey struct{}

func withClientInfo(r *http.Request) context.Context {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	return context.WithValue(r.Context(), clientKey{}, ClientInfo{IP: ip, UserAgent: r.UserAgent()})
}

func Client(ctx context.Context) ClientInfo {
	info, _ := ctx.Value(clientKey{}).(ClientInfo)
	return info
}
