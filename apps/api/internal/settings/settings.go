// Package settings reads business configuration stored as JSON in the settings table.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// Backups sets how many scheduled backups are kept.
type Backups struct {
	DailyRetention  int `json:"dailyRetention"`
	WeeklyRetention int `json:"weeklyRetention"`
}

var DefaultBackups = Backups{DailyRetention: 7, WeeklyRetention: 4}

func GetBackups(ctx context.Context, db database.DBTX) (Backups, error) {
	value := DefaultBackups
	return value, get(ctx, db, "backups", &value)
}

func GetPOS(ctx context.Context, db database.DBTX) (POS, error) {
	value := DefaultPOS
	return value, get(ctx, db, "pos", &value)
}

// Tax holds the VAT applied to new products; existing products and past sales keep their own.
type Tax struct {
	DefaultPercent float64 `json:"defaultPercent"`
}

var DefaultTax = Tax{DefaultPercent: 15}

func GetTax(ctx context.Context, db database.DBTX) (Tax, error) {
	value := DefaultTax
	return value, get(ctx, db, "tax", &value)
}

// Invoice holds the text the business chooses for its printed documents.
type Invoice struct {
	FooterNote string `json:"footerNote"`
}

var DefaultInvoice = Invoice{FooterNote: "Gracias por su compra. Conserve este comprobante."}

func GetInvoice(ctx context.Context, db database.DBTX) (Invoice, error) {
	value := DefaultInvoice
	return value, get(ctx, db, "invoice", &value)
}

// ScheduleDay is one weekday's usage window in "HH:MM"; a close at or before the open
// means the window ends the next day.
type ScheduleDay struct {
	Closed bool   `json:"closed"`
	Open   string `json:"open"`
	Close  string `json:"close"`
}

// Schedule limits when users without schedule:authorize may sign in and sell.
// Days is indexed by time.Weekday (Sunday first).
type Schedule struct {
	Enabled          bool           `json:"enabled"`
	ToleranceMinutes int            `json:"toleranceMinutes"`
	Days             [7]ScheduleDay `json:"days"`
}

var DefaultSchedule = func() Schedule {
	s := Schedule{ToleranceMinutes: 30}
	for i := range s.Days {
		s.Days[i] = ScheduleDay{Open: "08:00", Close: "22:00"}
	}
	return s
}()

func GetSchedule(ctx context.Context, db database.DBTX) (Schedule, error) {
	value := DefaultSchedule
	return value, get(ctx, db, "schedule", &value)
}

// Set merges value into the stored JSON, so keys this version doesn't know are kept.
func Set(ctx context.Context, db database.DBTX, key string, value any, by uuid.UUID) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting %q: %w", key, err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO settings (key, value, updated_by) VALUES ($1, $2::jsonb, $3)
		ON CONFLICT (key) DO UPDATE SET value = settings.value || EXCLUDED.value, updated_by = EXCLUDED.updated_by`,
		key, string(raw), by)
	if err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
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
