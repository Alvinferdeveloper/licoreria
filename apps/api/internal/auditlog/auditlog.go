// Package auditlog exposes the audit trail for reading. Entries are written by package audit
// and can't be changed once stored.
package auditlog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
)

type Entry struct {
	ID        uuid.UUID       `json:"id"`
	UserName  *string         `json:"userName"`
	Action    string          `json:"action"`
	Entity    string          `json:"entity"`
	EntityID  *uuid.UUID      `json:"entityId"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
	IP        *string         `json:"ip"`
	UserAgent *string         `json:"userAgent"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.Require("audit", "view")).Get("/audit-log", h.list)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	where, args, err := filters(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	var total int64
	if err := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM audit_log a LEFT JOIN users u ON u.id = a.user_id WHERE `+where, args...).Scan(&total); err != nil {
		httpx.Fail(w, r, fmt.Errorf("count audit log: %w", err))
		return
	}
	limit := len(args) + 1
	rows, err := h.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT a.id, u.full_name, a.action, a.entity, a.entity_id, a.before_data, a.after_data, host(a.ip), a.user_agent, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.user_id
		WHERE %s ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d`, where, limit, limit+1), append(args, page.Size, page.Offset())...)
	if err != nil {
		httpx.Fail(w, r, fmt.Errorf("list audit log: %w", err))
		return
	}
	defer rows.Close()

	items := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.UserName, &e.Action, &e.Entity, &e.EntityID, &e.Before, &e.After, &e.IP, &e.UserAgent, &e.CreatedAt); err != nil {
			httpx.Fail(w, r, fmt.Errorf("scan audit entry: %w", err))
			return
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, httpx.NewList(items, page, total))
}

func filters(r *http.Request) (string, []any, error) {
	q := r.URL.Query()
	clauses := []string{"true"}
	var args []any
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	for name, clause := range map[string]string{
		"from": "a.created_at >= ($%d::date)::timestamp AT TIME ZONE 'America/Managua'",
		"to":   "a.created_at < ($%d::date + 1)::timestamp AT TIME ZONE 'America/Managua'",
	} {
		value := q.Get(name)
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return "", nil, apperr.Validation(apperr.FieldError{Field: name, Code: "date", Message: "Escribe una fecha válida."})
		}
		add(clause, value)
	}
	if module := q.Get("module"); module != "" {
		add("split_part(a.action, '.', 1) = $%d", module)
	}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		add("(u.full_name ILIKE '%%' || $%[1]d || '%%' OR a.action ILIKE '%%' || $%[1]d || '%%' OR host(a.ip) LIKE $%[1]d || '%%')", search)
	}
	return strings.Join(clauses, " AND "), args, nil
}
