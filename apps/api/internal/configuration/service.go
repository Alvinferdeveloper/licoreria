// Package configuration lets an administrator edit the business data, VAT, invoice
// format and logo stored through the settings package.
package configuration

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/platform/storage"
	"licoreria/api/internal/settings"
)

// MaxLogoBytes bounds the logo; it is downloaded again to draw it on the invoice.
const MaxLogoBytes = 2 << 20

var ErrInvalidLogo = apperr.Validation(apperr.FieldError{Field: "logo", Code: "invalid_image", Message: "El logo debe ser una imagen PNG o JPG de hasta 2 MB."})

// logoExtensions lists what the PDF engine can draw.
var logoExtensions = map[string]string{"image/png": "png", "image/jpeg": "jpg"}

type View struct {
	Business settings.Business `json:"business"`
	Tax      settings.Tax      `json:"tax"`
	POS      settings.POS      `json:"pos"`
	Invoice  settings.Invoice  `json:"invoice"`
	Schedule settings.Schedule `json:"schedule"`
}

// Input is everything the screen edits; the logo has its own endpoints.
type Input struct {
	Name, TaxID, Address, Phone, Email string
	TaxPercent, DiscountLimitPercent   float64
	DefaultPrintFormat, FooterNote     string
	// Schedule is nil when the caller leaves the usage hours untouched.
	Schedule *settings.Schedule
}

type Service struct {
	pool    *pgxpool.Pool
	storage *storage.Supabase
}

func NewService(pool *pgxpool.Pool, store *storage.Supabase) *Service {
	return &Service{pool: pool, storage: store}
}

func (s *Service) Get(ctx context.Context) (View, error) { return load(ctx, s.pool) }

func load(ctx context.Context, db database.DBTX) (View, error) {
	var v View
	var err error
	if v.Business, err = settings.GetBusiness(ctx, db); err != nil {
		return View{}, err
	}
	if v.Tax, err = settings.GetTax(ctx, db); err != nil {
		return View{}, err
	}
	if v.POS, err = settings.GetPOS(ctx, db); err != nil {
		return View{}, err
	}
	if v.Invoice, err = settings.GetInvoice(ctx, db); err != nil {
		return View{}, err
	}
	v.Schedule, err = settings.GetSchedule(ctx, db)
	return v, err
}

func (s *Service) Update(ctx context.Context, actor auth.Principal, in Input) (View, error) {
	if in.Schedule != nil {
		if err := checkSchedule(*in.Schedule); err != nil {
			return View{}, err
		}
	}
	var after View
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := load(ctx, tx)
		if err != nil {
			return err
		}
		after = before
		after.Business.Name = strings.TrimSpace(in.Name)
		after.Business.TaxID = strings.TrimSpace(in.TaxID)
		after.Business.Address = strings.TrimSpace(in.Address)
		after.Business.Phone = strings.TrimSpace(in.Phone)
		after.Business.Email = strings.TrimSpace(in.Email)
		after.Tax.DefaultPercent = in.TaxPercent
		after.POS.DiscountLimitPercent = in.DiscountLimitPercent
		after.POS.DefaultPrintFormat = in.DefaultPrintFormat
		after.Invoice.FooterNote = strings.TrimSpace(in.FooterNote)
		if in.Schedule != nil {
			after.Schedule = *in.Schedule
		}

		for key, value := range map[string]any{"business": after.Business, "tax": after.Tax, "pos": after.POS, "invoice": after.Invoice, "schedule": after.Schedule} {
			if err := settings.Set(ctx, tx, key, value, actor.UserID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "settings.updated", Entity: "settings", Before: before, After: after})
	})
	return after, err
}

func (s *Service) SetLogo(ctx context.Context, actor auth.Principal, data []byte) (View, error) {
	contentType := http.DetectContentType(data)
	ext, ok := logoExtensions[contentType]
	if !ok || len(data) > MaxLogoBytes {
		return View{}, ErrInvalidLogo
	}
	url, err := s.storage.Put(ctx, fmt.Sprintf("business/logo-%s.%s", uuid.NewString(), ext), contentType, data)
	if err != nil {
		return View{}, err
	}
	return s.replaceLogo(ctx, actor, &url)
}

func (s *Service) RemoveLogo(ctx context.Context, actor auth.Principal) (View, error) {
	return s.replaceLogo(ctx, actor, nil)
}

func (s *Service) replaceLogo(ctx context.Context, actor auth.Principal, url *string) (View, error) {
	var previous *string
	var view View
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		business, err := settings.GetBusiness(ctx, tx)
		if err != nil {
			return err
		}
		previous = business.LogoURL
		business.LogoURL = url
		if err := settings.Set(ctx, tx, "business", business, actor.UserID); err != nil {
			return err
		}
		if view, err = load(ctx, tx); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "settings.logo_changed", Entity: "settings", Before: previous, After: url})
	})
	if err != nil {
		return View{}, err
	}
	if previous != nil && s.storage.Enabled() {
		if key := s.storage.KeyFromURL(*previous); key != "" {
			if err := s.storage.Delete(ctx, key); err != nil {
				logx.From(ctx).Warn("delete previous logo", "error", err)
			}
		}
	}
	return view, nil
}
