// Package audit records critical actions (who, what, when, from where) in audit_log.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

// Actions recorded across modules.
const (
	ActionLoginSucceeded    = "auth.login_succeeded"
	ActionLoginFailed       = "auth.login_failed"
	ActionAccountLocked     = "auth.account_locked"
	ActionLogout            = "auth.logout"
	ActionPasswordChanged   = "auth.password_changed"
	ActionPasswordReset     = "auth.password_reset"
	ActionAccountActivated  = "auth.account_activated"
	ActionTokenReuse        = "auth.refresh_token_reused"
	ActionLoginOutsideHours = "auth.login_outside_hours"
	ActionScheduleExtended  = "schedule.extended"
	ActionExtensionRemoved  = "schedule.extension_removed"
	ActionUserCreated       = "users.created"
	ActionUserUpdated       = "users.updated"
	ActionUserStatusChanged = "users.status_changed"
	ActionUserAccessResent  = "users.access_resent"
	ActionRolePermissions   = "roles.permissions_updated"
)

type Entry struct {
	UserID   *uuid.UUID
	Action   string
	Entity   string
	EntityID *uuid.UUID
	Before   any
	After    any
}

// Record writes an entry using db, which may be a transaction so the log commits with the change.
func Record(ctx context.Context, db database.DBTX, e Entry) error {
	before, err := toJSON(e.Before)
	if err != nil {
		return err
	}
	after, err := toJSON(e.After)
	if err != nil {
		return err
	}

	client := httpx.Client(ctx)
	_, err = db.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, entity, entity_id, before_data, after_data, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, NULLIF($8, ''))`,
		e.UserID, e.Action, e.Entity, e.EntityID, before, after, client.IP, client.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("record audit %s: %w", e.Action, err)
	}
	return nil
}

func toJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal audit data: %w", err)
	}
	return data, nil
}
