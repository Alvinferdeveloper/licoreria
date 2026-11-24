package products

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/auth"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/logx"
	"licoreria/api/internal/platform/storage"
	"licoreria/api/internal/settings"
)

const MaxImageBytes = 5 << 20

var (
	ErrPriceChangeForbidden = apperr.New(apperr.KindForbidden, "price_change_forbidden", "Tu rol no puede cambiar precios ni impuestos.")
	ErrInvalidImage         = apperr.Validation(apperr.FieldError{Field: "image", Code: "invalid_image", Message: "Sube una imagen JPG, PNG o WebP de hasta 5 MB."})
)

var imageExtensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

type Input struct {
	SKU                string
	Barcode            *string
	Name               string
	Description        string
	CategoryID         uuid.UUID
	BrandID            *uuid.UUID
	UnitID             uuid.UUID
	AlcoholPercent     *string
	VolumeML           *int
	Price              string
	AverageCost        string
	TaxPercent         string
	MinStock           int
	ReorderPoint       int
	AllowNegativeStock bool
	Presentations      []PresentationInput
}

type PresentationInput struct {
	ID      *uuid.UUID
	Name    string
	Factor  int
	Barcode *string
	Price   string
}

type Service struct {
	pool    *pgxpool.Pool
	repo    Repository
	storage *storage.Supabase
}

func NewService(pool *pgxpool.Pool, store *storage.Supabase) *Service {
	return &Service{pool: pool, storage: store}
}

// canSeeCosts: cashiers never see costs or margins.
func canSeeCosts(p auth.Principal) bool { return p.Can("products", "authorize") }

func present(p auth.Principal, product Product) Product {
	if !canSeeCosts(p) {
		product.AverageCost = nil
	}
	return product
}

func (s *Service) List(ctx context.Context, p auth.Principal, f Filter, page httpx.Page) ([]Product, int64, error) {
	items, total, err := s.repo.list(ctx, s.pool, f, page)
	for i := range items {
		items[i] = present(p, items[i])
	}
	return items, total, err
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Product, error) {
	product, err := s.repo.get(ctx, s.pool, id)
	return present(p, product), err
}

type LookupResult struct {
	Product        Product    `json:"product"`
	PresentationID *uuid.UUID `json:"presentationId"`
}

func (s *Service) Lookup(ctx context.Context, p auth.Principal, code string) (LookupResult, error) {
	product, presentationID, err := s.repo.findByCode(ctx, s.pool, code)
	return LookupResult{Product: present(p, product), PresentationID: presentationID}, err
}

// defaultTax is the VAT for products created without one.
func defaultTax(ctx context.Context, db database.DBTX) (string, error) {
	tax, err := settings.GetTax(ctx, db)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(tax.DefaultPercent, 'f', -1, 64), nil
}

func normalize(in Input) Input {
	in.SKU = strings.ToUpper(strings.TrimSpace(in.SKU))
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Description = strings.TrimSpace(in.Description)
	in.Barcode = trimOrNil(in.Barcode)
	in.AlcoholPercent = trimOrNil(in.AlcoholPercent)
	if in.TaxPercent == "" {
		in.TaxPercent = "15"
	}
	if in.AverageCost == "" {
		in.AverageCost = "0"
	}
	for i := range in.Presentations {
		in.Presentations[i].Name = strings.Join(strings.Fields(in.Presentations[i].Name), " ")
		in.Presentations[i].Barcode = trimOrNil(in.Presentations[i].Barcode)
	}
	return in
}

func (s *Service) Create(ctx context.Context, actor auth.Principal, in Input) (Product, error) {
	if !canSeeCosts(actor) {
		return Product{}, ErrPriceChangeForbidden
	}
	if in.TaxPercent == "" {
		var err error
		if in.TaxPercent, err = defaultTax(ctx, s.pool); err != nil {
			return Product{}, err
		}
	}
	in = normalize(in)
	var created Product
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.checkBarcodes(ctx, tx, uuid.Nil, in); err != nil {
			return err
		}
		id, err := s.repo.insert(ctx, tx, in)
		if err != nil {
			return err
		}
		if err := s.repo.syncPresentations(ctx, tx, id, in.Presentations); err != nil {
			return err
		}
		if created, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.created", Entity: "products", EntityID: &id, After: created})
	})
	return present(actor, created), err
}

func (s *Service) Update(ctx context.Context, actor auth.Principal, id uuid.UUID, in Input) (Product, error) {
	in = normalize(in)
	var updated Product
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.repo.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !canSeeCosts(actor) && pricesChanged(before, in) {
			return ErrPriceChangeForbidden
		}
		if err := s.checkBarcodes(ctx, tx, id, in); err != nil {
			return err
		}
		if err := s.repo.update(ctx, tx, writeInput{Input: in, ID: id}); err != nil {
			return err
		}
		if err := s.repo.syncPresentations(ctx, tx, id, in.Presentations); err != nil {
			return err
		}
		if updated, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.updated", Entity: "products", EntityID: &id, Before: before, After: updated})
	})
	return present(actor, updated), err
}

func (s *Service) SetActive(ctx context.Context, actor auth.Principal, id uuid.UUID, active bool) (Product, error) {
	var updated Product
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.setActive(ctx, tx, id, active); err != nil {
			return err
		}
		var err error
		if updated, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.status_changed", Entity: "products", EntityID: &id,
			After: map[string]bool{"active": active}})
	})
	return present(actor, updated), err
}

// SetPrice changes only the sale price, e.g. after a supplier raised the cost.
func (s *Service) SetPrice(ctx context.Context, actor auth.Principal, id uuid.UUID, price string) (Product, error) {
	if !canSeeCosts(actor) {
		return Product{}, ErrPriceChangeForbidden
	}
	var updated Product
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.repo.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.repo.setPrice(ctx, tx, id, price); err != nil {
			return err
		}
		if updated, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.price_changed", Entity: "products", EntityID: &id,
			Before: map[string]string{"price": before.Price}, After: map[string]string{"price": updated.Price}})
	})
	return present(actor, updated), err
}

// SetImage uploads to Supabase first and only then points the product at it; the previous
// image is removed after the change commits.
func (s *Service) SetImage(ctx context.Context, actor auth.Principal, id uuid.UUID, data []byte) (Product, error) {
	contentType := http.DetectContentType(data)
	ext, ok := imageExtensions[contentType]
	if !ok || len(data) == 0 || len(data) > MaxImageBytes {
		return Product{}, ErrInvalidImage
	}
	if _, err := s.repo.get(ctx, s.pool, id); err != nil {
		return Product{}, err
	}

	url, err := s.storage.Put(ctx, fmt.Sprintf("products/%s/%s.%s", id, randomName(), ext), contentType, data)
	if err != nil {
		return Product{}, err
	}
	return s.replaceImage(ctx, actor, id, &url)
}

func (s *Service) RemoveImage(ctx context.Context, actor auth.Principal, id uuid.UUID) (Product, error) {
	return s.replaceImage(ctx, actor, id, nil)
}

func (s *Service) replaceImage(ctx context.Context, actor auth.Principal, id uuid.UUID, url *string) (Product, error) {
	var previous *string
	var updated Product
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if previous, err = s.repo.setImage(ctx, tx, id, url); err != nil {
			return err
		}
		if updated, err = s.repo.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor.UserID, Action: "products.image_changed", Entity: "products", EntityID: &id,
			Before: previous, After: url})
	})
	if err != nil {
		return Product{}, err
	}
	if previous != nil {
		if key := s.storage.KeyFromURL(*previous); key != "" {
			if err := s.storage.Delete(ctx, key); err != nil {
				logx.From(ctx).Warn("delete previous product image", "error", err, "product_id", id)
			}
		}
	}
	return present(actor, updated), nil
}

type barcodeRef struct {
	field string
	code  *string
}

func (s *Service) checkBarcodes(ctx context.Context, db database.DBTX, productID uuid.UUID, in Input) error {
	seen := map[string]bool{}
	codes := []barcodeRef{{"barcode", in.Barcode}}
	for _, p := range in.Presentations {
		codes = append(codes, barcodeRef{"presentations", p.Barcode})
	}

	for _, c := range codes {
		if c.code == nil {
			continue
		}
		taken, err := s.repo.barcodeTaken(ctx, db, *c.code, productID)
		if err != nil {
			return err
		}
		if taken || seen[*c.code] {
			return apperr.Validation(apperr.FieldError{
				Field: c.field, Code: "barcode_taken", Message: fmt.Sprintf("El código %s ya está asignado a otro producto o presentación.", *c.code),
			})
		}
		seen[*c.code] = true
	}
	return nil
}

func pricesChanged(before Product, in Input) bool {
	if !sameAmount(before.Price, in.Price) || !sameAmount(before.TaxPercent, in.TaxPercent) {
		return true
	}
	prices := map[uuid.UUID]string{}
	for _, p := range before.Presentations {
		prices[p.ID] = p.Price
	}
	for _, p := range in.Presentations {
		if p.ID == nil || !sameAmount(prices[*p.ID], p.Price) {
			return true
		}
	}
	return false
}

// sameAmount compares decimal strings ignoring trailing zeros ("15" equals "15.00").
func sameAmount(a, b string) bool {
	trim := func(v string) string {
		if strings.Contains(v, ".") {
			v = strings.TrimRight(strings.TrimRight(v, "0"), ".")
		}
		return v
	}
	return trim(a) == trim(b)
}

func trimOrNil(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func randomName() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
