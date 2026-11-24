// Package monitor reports the health of everything the store depends on: database, backups,
// disk, email, file storage, today's exchange rate and the API's own traffic.
package monitor

import (
	"context"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/settings"
)

const (
	StatusOK      = "ok"
	StatusWarn    = "warn"
	StatusFail    = "fail"
	StatusUnknown = "unknown"
)

type API struct {
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"startedAt"`
	GoVersion  string    `json:"goVersion"`
	Revision   string    `json:"revision"`
	Goroutines int       `json:"goroutines"`
	MemoryMB   float64   `json:"memoryMb"`
}

type Database struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	SizeBytes int64  `json:"sizeBytes"`
	InUse     int32  `json:"inUse"`
	Idle      int32  `json:"idle"`
	MaxConns  int32  `json:"maxConns"`
	Version   string `json:"version"`
}

type Backups struct {
	Status       string     `json:"status"`
	LastAt       *time.Time `json:"lastAt"`
	LastFailedAt *time.Time `json:"lastFailedAt"`
	Offsite      bool       `json:"offsite"`
	ScheduleHour int        `json:"scheduleHour"`
}

type Disk struct {
	Status     string `json:"status"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

type Email struct {
	Status          string     `json:"status"`
	Sender          string     `json:"sender"`
	Pending         int        `json:"pending"`
	OldestPendingAt *time.Time `json:"oldestPendingAt"`
	Sent24h         int        `json:"sent24h"`
	Failed24h       int        `json:"failed24h"`
}

type Storage struct {
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

type ExchangeRate struct {
	Status string  `json:"status"`
	Today  *string `json:"today"`
}

type Activity struct {
	ActiveSessions int `json:"activeSessions"`
	ActiveUsers    int `json:"activeUsers"`
	OpenShifts     int `json:"openShifts"`
	OfflineSynced  int `json:"offlineSynced24h"`
}

type Traffic struct {
	Status string                `json:"status"`
	Hours  []httpx.HourStats     `json:"hours"`
	Failed []httpx.FailedRequest `json:"failed"`
}

// Report is the whole status page; Status is the worst of its checks.
type Report struct {
	Status       string       `json:"status"`
	CheckedAt    time.Time    `json:"checkedAt"`
	API          API          `json:"api"`
	Database     Database     `json:"database"`
	Backups      Backups      `json:"backups"`
	Disk         Disk         `json:"disk"`
	Email        Email        `json:"email"`
	Storage      Storage      `json:"storage"`
	ExchangeRate ExchangeRate `json:"exchangeRate"`
	Activity     Activity     `json:"activity"`
	Traffic      Traffic      `json:"traffic"`
}

// Options are the facts about the deployment the checks need.
type Options struct {
	BackupDir      string
	Offsite        bool
	StorageEnabled bool
	MailSender     string
}

type Handler struct {
	pool  *pgxpool.Pool
	stats *httpx.Stats
	opts  Options
	now   func() time.Time
}

func NewHandler(pool *pgxpool.Pool, stats *httpx.Stats, opts Options) *Handler {
	return &Handler{pool: pool, stats: stats, opts: opts, now: time.Now}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.Require("system", "view")).Get("/system/status", h.status)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	httpx.JSON(w, r, http.StatusOK, h.report(ctx))
}

func (h *Handler) report(ctx context.Context) Report {
	now := h.now()
	rep := Report{CheckedAt: now, API: h.api(), Storage: Storage{Status: StatusOK, Enabled: h.opts.StorageEnabled}}
	if !h.opts.StorageEnabled {
		rep.Storage.Status = StatusWarn
	}
	rep.Disk = diskCheck(h.opts.BackupDir)
	hours, failed := h.stats.Snapshot()
	rep.Traffic = Traffic{Status: trafficStatus(hours), Hours: hours, Failed: failed}

	rep.Database = h.database(ctx)
	if rep.Database.Status == StatusFail {
		// Everything below lives in the database; without it those checks can't say anything.
		rep.Backups.Status, rep.Email.Status, rep.ExchangeRate.Status = StatusUnknown, StatusUnknown, StatusUnknown
	} else {
		rep.Backups = h.backups(ctx, now)
		rep.Email = h.email(ctx, now)
		rep.ExchangeRate = h.exchangeRate(ctx)
		rep.Activity = h.activity(ctx)
	}
	rep.Status = worst(rep.API.Status, rep.Database.Status, rep.Backups.Status, rep.Disk.Status, rep.Email.Status, rep.Storage.Status, rep.ExchangeRate.Status, rep.Traffic.Status)
	return rep
}

func (h *Handler) api() API {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	a := API{Status: StatusOK, StartedAt: h.stats.Started(), GoVersion: runtime.Version(), Goroutines: runtime.NumGoroutine(), MemoryMB: float64(mem.Alloc) / (1 << 20)}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				a.Revision = s.Value[:7]
			}
		}
	}
	return a
}

func (h *Handler) database(ctx context.Context) Database {
	start := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		return Database{Status: StatusFail}
	}
	stat := h.pool.Stat()
	d := Database{Status: StatusOK, LatencyMs: time.Since(start).Milliseconds(), InUse: stat.AcquiredConns(), Idle: stat.IdleConns(), MaxConns: stat.MaxConns()}
	if err := h.pool.QueryRow(ctx, `SELECT pg_database_size(current_database()), current_setting('server_version')`).Scan(&d.SizeBytes, &d.Version); err != nil {
		d.Status = StatusWarn
	}
	if d.LatencyMs > 200 {
		d.Status = StatusWarn
	}
	return d
}

func (h *Handler) backups(ctx context.Context, now time.Time) Backups {
	b := Backups{Offsite: h.opts.Offsite}
	policy, err := settings.GetBackups(ctx, h.pool)
	if err != nil {
		return Backups{Status: StatusUnknown}
	}
	b.ScheduleHour = policy.Hour
	if err := h.pool.QueryRow(ctx, `
		SELECT max(created_at) FILTER (WHERE status = 'completed'), max(created_at) FILTER (WHERE status = 'failed')
		FROM backups`).Scan(&b.LastAt, &b.LastFailedAt); err != nil {
		return Backups{Status: StatusUnknown}
	}
	b.Status = backupStatus(now, b)
	return b
}

// backupStatus fails when the last day passed without a good backup or the newest attempt failed.
func backupStatus(now time.Time, b Backups) string {
	switch {
	case b.LastAt == nil || now.Sub(*b.LastAt) > 48*time.Hour:
		return StatusFail
	case b.LastFailedAt != nil && b.LastFailedAt.After(*b.LastAt):
		return StatusFail
	case now.Sub(*b.LastAt) > 26*time.Hour || !b.Offsite:
		return StatusWarn
	}
	return StatusOK
}

func (h *Handler) email(ctx context.Context, now time.Time) Email {
	e := Email{Sender: h.opts.MailSender}
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'pending'),
		       min(scheduled_at) FILTER (WHERE status = 'pending'),
		       count(*) FILTER (WHERE status = 'sent' AND sent_at > now() - interval '24 hours'),
		       count(*) FILTER (WHERE status = 'failed' AND created_at > now() - interval '24 hours')
		FROM email_queue`).Scan(&e.Pending, &e.OldestPendingAt, &e.Sent24h, &e.Failed24h); err != nil {
		return Email{Status: StatusUnknown, Sender: h.opts.MailSender}
	}
	e.Status = StatusOK
	if e.Failed24h > 0 || (e.OldestPendingAt != nil && now.Sub(*e.OldestPendingAt) > 15*time.Minute) {
		e.Status = StatusWarn
	}
	return e
}

func (h *Handler) exchangeRate(ctx context.Context) ExchangeRate {
	var rate *string
	if err := h.pool.QueryRow(ctx, `
		SELECT (SELECT rate::text FROM exchange_rates
		        WHERE currency_code = 'USD' AND effective_date = (now() AT TIME ZONE 'America/Managua')::date)`).Scan(&rate); err != nil {
		return ExchangeRate{Status: StatusUnknown}
	}
	if rate == nil {
		// The POS refuses to sell without today's rate.
		return ExchangeRate{Status: StatusWarn}
	}
	return ExchangeRate{Status: StatusOK, Today: rate}
}

func (h *Handler) activity(ctx context.Context) Activity {
	var a Activity
	_ = h.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM refresh_sessions WHERE revoked_at IS NULL AND expires_at > now()),
		       (SELECT count(DISTINCT user_id) FROM refresh_sessions WHERE revoked_at IS NULL AND expires_at > now()),
		       (SELECT count(*) FROM cash_shifts WHERE closed_at IS NULL),
		       (SELECT count(*) FROM sales WHERE offline_at IS NOT NULL AND created_at > now() - interval '24 hours')`,
	).Scan(&a.ActiveSessions, &a.ActiveUsers, &a.OpenShifts, &a.OfflineSynced)
	return a
}

func diskCheck(dir string) Disk {
	total, free, ok := diskUsage(dir)
	if !ok || total == 0 {
		return Disk{Status: StatusUnknown}
	}
	d := Disk{Status: StatusOK, TotalBytes: total, FreeBytes: free}
	switch share := float64(free) / float64(total); {
	case share < 0.05:
		d.Status = StatusFail
	case share < 0.15:
		d.Status = StatusWarn
	}
	return d
}

// trafficStatus looks at the current hour: any server error warns, more than 5% fails.
func trafficStatus(hours []httpx.HourStats) string {
	if len(hours) == 0 {
		return StatusOK
	}
	last := hours[len(hours)-1]
	switch {
	case last.Requests >= 20 && float64(last.ServerErrors)/float64(last.Requests) > 0.05:
		return StatusFail
	case last.ServerErrors > 0:
		return StatusWarn
	}
	return StatusOK
}

// An unknown check doesn't make the whole system look degraded; its own card says so.
var rank = map[string]int{StatusOK: 0, StatusUnknown: 0, StatusWarn: 1, StatusFail: 2}

func worst(statuses ...string) string {
	out := StatusOK
	for _, s := range statuses {
		if rank[s] > rank[out] {
			out = s
		}
	}
	return out
}
