package parties

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/audit"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

type Supplier struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	TaxID       *string   `json:"taxId"`
	ContactName string    `json:"contactName"`
	Phone       *string   `json:"phone"`
	Email       *string   `json:"email"`
	Address     string    `json:"address"`
	CreditDays  int       `json:"creditDays"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
}

type supplierRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=120"`
	TaxID       *string `json:"taxId" validate:"omitempty,max=20"`
	ContactName string  `json:"contactName" validate:"max=120"`
	Phone       *string `json:"phone" validate:"omitempty,max=20"`
	Email       *string `json:"email" validate:"omitempty,email,max=254"`
	Address     string  `json:"address" validate:"max=300"`
	CreditDays  int     `json:"creditDays" validate:"gte=0,lte=365"`
}

var supplierConstraints = database.ConstraintFields{"suppliers_tax_id_key": "taxId"}

const supplierCols = `id, name, tax_id, contact_name, phone, email, address, credit_days, active, created_at`

func scanSupplier(row pgx.Row) (Supplier, error) {
	var s Supplier
	err := row.Scan(&s.ID, &s.Name, &s.TaxID, &s.ContactName, &s.Phone, &s.Email, &s.Address, &s.CreditDays, &s.Active, &s.CreatedAt)
	return s, err
}

type SupplierService struct {
	pool *pgxpool.Pool
}

func NewSupplierService(pool *pgxpool.Pool) *SupplierService { return &SupplierService{pool: pool} }

func (s *SupplierService) list(ctx context.Context, f Filter, page httpx.Page) ([]Supplier, int64, error) {
	condition, args := whereClause(f, "name", "tax_id", "contact_name", "phone", "email")
	return listPage(ctx, s.pool, "suppliers", supplierCols, condition, "active DESC, name", args, page, scanSupplier)
}

func (s *SupplierService) get(ctx context.Context, db database.DBTX, id uuid.UUID) (Supplier, error) {
	sup, err := scanSupplier(db.QueryRow(ctx, "SELECT "+supplierCols+" FROM suppliers WHERE id = $1", id))
	return sup, database.MapError(err, nil)
}

func (s *SupplierService) save(ctx context.Context, actor uuid.UUID, id *uuid.UUID, req supplierRequest) (Supplier, error) {
	var saved Supplier
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		args := []any{clean(req.Name), cleanPtr(req.TaxID), clean(req.ContactName), cleanPtr(req.Phone), cleanPtr(req.Email), clean(req.Address), req.CreditDays}
		var before *Supplier
		var savedID uuid.UUID
		var err error
		if id == nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO suppliers (name, tax_id, contact_name, phone, email, address, credit_days)
				VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`, args...).Scan(&savedID)
		} else {
			current, getErr := s.get(ctx, tx, *id)
			if getErr != nil {
				return getErr
			}
			before, savedID = &current, *id
			_, err = tx.Exec(ctx, `
				UPDATE suppliers SET name = $1, tax_id = $2, contact_name = $3, phone = $4, email = $5, address = $6, credit_days = $7
				WHERE id = $8`, append(args, savedID)...)
		}
		if err != nil {
			return database.MapError(err, supplierConstraints)
		}
		if saved, err = s.get(ctx, tx, savedID); err != nil {
			return err
		}
		action := "suppliers.created"
		if before != nil {
			action = "suppliers.updated"
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: action, Entity: "suppliers", EntityID: &savedID, Before: before, After: saved})
	})
	return saved, err
}

type SupplierHandler struct {
	svc *SupplierService
}

func NewSupplierHandler(svc *SupplierService) *SupplierHandler { return &SupplierHandler{svc: svc} }

func (h *SupplierHandler) Routes(r chi.Router) {
	mountCRUD(r, "/suppliers", "suppliers", crudHandlers{
		list: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (httpx.List[Supplier], error) {
				page, err := httpx.ParsePage(r)
				if err != nil {
					return httpx.List[Supplier]{}, err
				}
				f, err := parseFilter(r)
				if err != nil {
					return httpx.List[Supplier]{}, err
				}
				items, total, err := h.svc.list(r.Context(), f, page)
				return httpx.NewList(items, page, total), err
			})
		},
		get: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Supplier, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Supplier{}, err
				}
				return h.svc.get(r.Context(), h.svc.pool, id)
			})
		},
		create: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusCreated, func() (Supplier, error) {
				var req supplierRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Supplier{}, err
				}
				return h.svc.save(r.Context(), actorID(r), nil, req)
			})
		},
		update: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Supplier, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Supplier{}, err
				}
				var req supplierRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Supplier{}, err
				}
				return h.svc.save(r.Context(), actorID(r), &id, req)
			})
		},
		setStatus: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Supplier, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Supplier{}, err
				}
				var req statusRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Supplier{}, err
				}
				if err := setActive(r.Context(), h.svc.pool, actorID(r), "suppliers", id, *req.Active); err != nil {
					return Supplier{}, err
				}
				return h.svc.get(r.Context(), h.svc.pool, id)
			})
		},
	})
}
