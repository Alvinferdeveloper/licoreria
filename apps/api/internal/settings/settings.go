// Package settings reads business configuration stored as JSON in the settings table.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/platform/database"
)

type Business struct {
	Name    string  `json:"name"`
	TaxID   string  `json:"taxId"`
	Address string  `json:"address"`
	Phone   string  `json:"phone"`
	Email   string  `json:"email"`
	LogoURL *string `json:"logoUrl"`
}

type Security struct {
	IdleTimeoutMinutes int `json:"idleTimeoutMinutes"`
	MaxFailedAttempts  int `json:"maxFailedAttempts"`
	LockoutMinutes     int `json:"lockoutMinutes"`
	TempPasswordHours  int `json:"tempPasswordHours"`
}

func (s Security) Lockout() time.Duration { return time.Duration(s.LockoutMinutes) * time.Minute }

func (s Security) TempPasswordTTL() time.Duration {
	return time.Duration(s.TempPasswordHours) * time.Hour
}

// Defaults apply when a key is missing, so the system keeps working on a partial configuration.
var (
	DefaultBusiness = Business{Name: "Licorería"}
	DefaultSecurity = Security{IdleTimeoutMinutes: 30, MaxFailedAttempts: 5, LockoutMinutes: 15, TempPasswordHours: 72}
)

func GetBusiness(ctx context.Context, db database.DBTX) (Business, error) {
	value := DefaultBusiness
	return value, get(ctx, db, "business", &value)
}

func GetSecurity(ctx context.Context, db database.DBTX) (Security, error) {
	value := DefaultSecurity
	return value, get(ctx, db, "security", &value)
}

// POS holds the point-of-sale rules; DiscountLimitPercent is the most a role without
// discounts:authorize may discount without an administrator's PIN.
type POS struct {
	DiscountLimitPercent float64 `json:"discountLimitPercent"`
	AllowNegativeStock   bool    `json:"allowNegativeStock"`
	DefaultPrintFormat   string  `json:"defaultPrintFormat"`
}

var DefaultPOS = POS{DiscountLimitPercent: 5, DefaultPrintFormat: "ticket"}

func GetPOS(ctx context.Context, db database.DBTX) (POS, error) {
	value := DefaultPOS
	return value, get(ctx, db, "pos", &value)
}

// get decodes the stored JSON over dst; fields absent from the JSON keep their defaults.
func get(ctx context.Context, db database.DBTX, key string, dst any) error {
	var raw []byte
	err := db.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load setting %q: %w", key, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode setting %q: %w", key, err)
	}
	return nil
}
