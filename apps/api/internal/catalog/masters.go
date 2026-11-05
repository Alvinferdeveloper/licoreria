// Package catalog manages the master tables products depend on: categories, brands and units.
package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"licoreria/api/internal/apperr"
	"licoreria/api/internal/audit"
	"licoreria/api/internal/platform/database"
)

// Master is a category, brand or unit. Fields that don't apply to a kind stay empty.
type Master struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Abbreviation string     `json:"abbreviation,omitempty"`
	ParentID     *uuid.UUID `json:"parentId,omitempty"`
	Active       bool       `json:"active"`
	ProductCount int        `json:"productCount"`
}

type MasterInput struct {
	Name         string
	Abbreviation string
	ParentID     *uuid.UUID
}

// kind describes one master table; names are constants, never user input.
type kind struct {
	table       string
	productFK   string
	hasParent   bool
	hasActive   bool
	hasAbbrev   bool
	constraints database.ConstraintFields
}

var (
	categories = kind{table: "categories", productFK: "category_id", hasParent: true, hasActive: true,
		constraints: database.ConstraintFields{"categories_name_uq": "name", "categories_parent_id_fkey": "parentId"}}
	brands = kind{table: "brands", productFK: "brand_id", hasActive: true,
		constraints: database.ConstraintFields{"brands_name_key": "name"}}
	units = kind{table: "units", productFK: "unit_id", hasAbbrev: true,
		constraints: database.ConstraintFields{"units_name_key": "name", "units_abbreviation_key": "abbreviation"}}
)

var ErrCategoryCycle = apperr.Validation(apperr.FieldError{
	Field: "parentId", Code: "cycle", Message: "Una categoría no puede quedar dentro de sí misma.",
})

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (k kind) selectSQL() string {
	cols := []string{"m.id", "m.name"}
	cols = append(cols, ternary(k.hasAbbrev, "m.abbreviation", "''"))
	cols = append(cols, ternary(k.hasParent, "m.parent_id", "NULL::uuid"))
	cols = append(cols, ternary(k.hasActive, "m.active", "true"))
	cols = append(cols, fmt.Sprintf("(SELECT count(*) FROM products p WHERE p.%s = m.id AND p.active)", k.productFK))
	return fmt.Sprintf("SELECT %s FROM %s m", strings.Join(cols, ", "), k.table)
}

func scanMaster(row pgx.Row) (Master, error) {
	var m Master
	err := row.Scan(&m.ID, &m.Name, &m.Abbreviation, &m.ParentID, &m.Active, &m.ProductCount)
	return m, err
}

func (s *Service) list(ctx context.Context, k kind) ([]Master, error) {
	order := " ORDER BY m.name"
	if k.hasActive {
		order = " ORDER BY m.active DESC, m.name"
	}
	rows, err := s.pool.Query(ctx, k.selectSQL()+order)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", k.table, err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Master, error) { return scanMaster(row) })
}

func (s *Service) get(ctx context.Context, db database.DBTX, k kind, id uuid.UUID) (Master, error) {
	m, err := scanMaster(db.QueryRow(ctx, k.selectSQL()+" WHERE m.id = $1", id))
	return m, database.MapError(err, nil)
}

func (s *Service) create(ctx context.Context, actor uuid.UUID, k kind, in MasterInput) (Master, error) {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	var created Master
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		switch {
		case k.hasParent:
			err = tx.QueryRow(ctx, `INSERT INTO categories (name, parent_id) VALUES ($1, $2) RETURNING id`, in.Name, in.ParentID).Scan(&id)
		case k.hasAbbrev:
			err = tx.QueryRow(ctx, `INSERT INTO units (name, abbreviation) VALUES ($1, $2) RETURNING id`, in.Name, strings.TrimSpace(in.Abbreviation)).Scan(&id)
		default:
			err = tx.QueryRow(ctx, `INSERT INTO brands (name) VALUES ($1) RETURNING id`, in.Name).Scan(&id)
		}
		if err != nil {
			return database.MapError(err, k.constraints)
		}
		if created, err = s.get(ctx, tx, k, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: k.table + ".created", Entity: k.table, EntityID: &id, After: created})
	})
	return created, err
}

func (s *Service) update(ctx context.Context, actor uuid.UUID, k kind, id uuid.UUID, in MasterInput) (Master, error) {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	var updated Master
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, k, id)
		if err != nil {
			return err
		}
		switch {
		case k.hasParent:
			if err := s.checkNoCycle(ctx, tx, id, in.ParentID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE categories SET name = $2, parent_id = $3 WHERE id = $1`, id, in.Name, in.ParentID)
		case k.hasAbbrev:
			_, err = tx.Exec(ctx, `UPDATE units SET name = $2, abbreviation = $3 WHERE id = $1`, id, in.Name, strings.TrimSpace(in.Abbreviation))
		default:
			_, err = tx.Exec(ctx, `UPDATE brands SET name = $2 WHERE id = $1`, id, in.Name)
		}
		if err != nil {
			return database.MapError(err, k.constraints)
		}
		if updated, err = s.get(ctx, tx, k, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: k.table + ".updated", Entity: k.table, EntityID: &id, Before: before, After: updated})
	})
	return updated, err
}

func (s *Service) setActive(ctx context.Context, actor uuid.UUID, k kind, id uuid.UUID, active bool) (Master, error) {
	var updated Master
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET active = $2 WHERE id = $1`, k.table), id, active)
		if err != nil {
			return database.MapError(err, nil)
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		if updated, err = s.get(ctx, tx, k, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{UserID: &actor, Action: k.table + ".status_changed", Entity: k.table, EntityID: &id,
			After: map[string]bool{"active": active}})
	})
	return updated, err
}

// checkNoCycle rejects a parent that is the category itself or one of its descendants.
func (s *Service) checkNoCycle(ctx context.Context, db database.DBTX, id uuid.UUID, parentID *uuid.UUID) error {
	if parentID == nil {
		return nil
	}
	var cycle bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM categories WHERE id = $2
			UNION ALL
			SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = $1)`, id, *parentID).Scan(&cycle)
	if err != nil {
		return fmt.Errorf("check category cycle: %w", err)
	}
	if cycle {
		return ErrCategoryCycle
	}
	return nil
}

func ternary(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
