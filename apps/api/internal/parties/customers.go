package parties

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
)

var ErrWalkInLocked = apperr.New(apperr.KindConflict, "walk_in_locked", "El cliente Consumidor final no se puede modificar.")

type Customer struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"fullName"`
	TaxID     *string   `json:"taxId"`
	Phone     *string   `json:"phone"`
	Email     *string   `json:"email"`
	BirthDate *string   `json:"birthDate"`
	Address   string    `json:"address"`
	IsWalkIn  bool      `json:"isWalkIn"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

type customerRequest struct {
	FullName  string  `json:"fullName" validate:"required,min=3,max=120"`
	TaxID     *string `json:"taxId" validate:"omitempty,max=20"`
	Phone     *string `json:"phone" validate:"omitempty,max=20"`
	Email     *string `json:"email" validate:"omitempty,email,max=254"`
	BirthDate *string `json:"birthDate" validate:"omitempty,datetime=2006-01-02"`
	Address   string  `json:"address" validate:"max=300"`
}

var customerConstraints = database.ConstraintFields{"customers_tax_id_key": "taxId", "customers_birth_date_check": "birthDate"}

const customerCols = `id, full_name, tax_id, phone, email, to_char(birth_date, 'YYYY-MM-DD'), address, is_walk_in, active, created_at`

func scanCustomer(row pgx.Row) (Customer, error) {
	var c Customer
	err := row.Scan(&c.ID, &c.FullName, &c.TaxID, &c.Phone, &c.Email, &c.BirthDate, &c.Address, &c.IsWalkIn, &c.Active, &c.CreatedAt)
	return c, err
}

type CustomerService struct {
	pool *pgxpool.Pool
}

func NewCustomerService(pool *pgxpool.Pool) *CustomerService { return &CustomerService{pool: pool} }

func (s *CustomerService) list(ctx context.Context, f Filter, page httpx.Page) ([]Customer, int64, error) {
	condition, args := whereClause(f, "full_name", "tax_id", "phone", "email")
	return listPage(ctx, s.pool, "customers", customerCols, condition, "is_walk_in DESC, active DESC, full_name", args, page, scanCustomer)
}

func (s *CustomerService) get(ctx context.Context, db database.DBTX, id uuid.UUID) (Customer, error) {
	c, err := scanCustomer(db.QueryRow(ctx, "SELECT "+customerCols+" FROM customers WHERE id = $1", id))
	return c, database.MapError(err, nil)
}

func (s *CustomerService) save(ctx context.Context, actor uuid.UUID, id *uuid.UUID, req customerRequest) (Customer, error) {
	var saved Customer
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var before *Customer
		args := []any{clean(req.FullName), cleanPtr(req.TaxID), cleanPtr(req.Phone), cleanPtr(req.Email), cleanPtr(req.BirthDate), clean(req.Address)}
		var savedID uuid.UUID
		var err error
		if id == nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO customers (full_name, tax_id, phone, email, birth_date, address)
				VALUES ($1, $2, $3, $4, $5::date, $6) RETURNING id`, args...).Scan(&savedID)
		} else {
			current, getErr := s.get(ctx, tx, *id)
			if getErr != nil {
				return getErr
			}
			if current.IsWalkIn {
				return ErrWalkInLocked
			}
			before, savedID = &current, *id
			_, err = tx.Exec(ctx, `
				UPDATE customers SET full_name = $1, tax_id = $2, phone = $3, email = $4, birth_date = $5::date, address = $6
				WHERE id = $7`, append(args, savedID)...)
		}
		if err != nil {
			return database.MapError(err, customerConstraints)
		}
		if saved, err = s.get(ctx, tx, savedID); err != nil {
			return err
		}
		action := "customers.created"
		if before != nil {
			action = "customers.updated"
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: action, Entity: "customers", EntityID: &savedID, Before: before, After: saved})
	})
	return saved, err
}

type CustomerHandler struct {
	svc *CustomerService
}

func NewCustomerHandler(svc *CustomerService) *CustomerHandler { return &CustomerHandler{svc: svc} }

func (h *CustomerHandler) Routes(r chi.Router) {
	mountCRUD(r, "/customers", "customers", crudHandlers{
		list: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (httpx.List[Customer], error) {
				page, err := httpx.ParsePage(r)
				if err != nil {
					return httpx.List[Customer]{}, err
				}
				f, err := parseFilter(r)
				if err != nil {
					return httpx.List[Customer]{}, err
				}
				items, total, err := h.svc.list(r.Context(), f, page)
				return httpx.NewList(items, page, total), err
			})
		},
		get: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Customer, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Customer{}, err
				}
				return h.svc.get(r.Context(), h.svc.pool, id)
			})
		},
		create: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusCreated, func() (Customer, error) {
				var req customerRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Customer{}, err
				}
				return h.svc.save(r.Context(), actorID(r), nil, req)
			})
		},
		update: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Customer, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Customer{}, err
				}
				var req customerRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Customer{}, err
				}
				return h.svc.save(r.Context(), actorID(r), &id, req)
			})
		},
		setStatus: func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, http.StatusOK, func() (Customer, error) {
				id, err := httpx.URLParamUUID(r, "id")
				if err != nil {
					return Customer{}, err
				}
				var req statusRequest
				if err := httpx.Bind(w, r, &req); err != nil {
					return Customer{}, err
				}
				current, err := h.svc.get(r.Context(), h.svc.pool, id)
				if err != nil {
					return Customer{}, err
				}
				if current.IsWalkIn {
					return Customer{}, ErrWalkInLocked
				}
				if err := setActive(r.Context(), h.svc.pool, actorID(r), "customers", id, *req.Active); err != nil {
					return Customer{}, err
				}
				return h.svc.get(r.Context(), h.svc.pool, id)
			})
		},
	})
}
