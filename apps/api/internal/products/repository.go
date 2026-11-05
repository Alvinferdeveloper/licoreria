// Package products manages the product catalog: products, their sale presentations
// (six-pack, box) and images. Stock is read-only here; it changes through inventory movements.
package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"licoreria/api/internal/httpx"
	"licoreria/api/internal/platform/database"
	"licoreria/api/internal/platform/textx"
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type UnitRef struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Abbreviation string    `json:"abbreviation"`
}

// Amounts are decimal strings ("421.75") to avoid float rounding.
type Product struct {
	ID                 uuid.UUID      `json:"id"`
	SKU                string         `json:"sku"`
	Barcode            *string        `json:"barcode"`
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	Category           Ref            `json:"category"`
	Brand              *Ref           `json:"brand"`
	Unit               UnitRef        `json:"unit"`
	AlcoholPercent     *string        `json:"alcoholPercent"`
	VolumeML           *int           `json:"volumeMl"`
	ImageURL           *string        `json:"imageUrl"`
	Price              string         `json:"price"`
	AverageCost        *string        `json:"averageCost,omitempty"`
	TaxPercent         string         `json:"taxPercent"`
	Stock              int            `json:"stock"`
	MinStock           int            `json:"minStock"`
	ReorderPoint       int            `json:"reorderPoint"`
	AllowNegativeStock bool           `json:"allowNegativeStock"`
	Active             bool           `json:"active"`
	Presentations      []Presentation `json:"presentations"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

type Presentation struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Factor  int       `json:"factor"`
	Barcode *string   `json:"barcode"`
	Price   string    `json:"price"`
	Active  bool      `json:"active"`
}

type Filter struct {
	Search     string
	CategoryID *uuid.UUID
	BrandID    *uuid.UUID
	Active     *bool
	LowStock   bool
}

var constraintFields = database.ConstraintFields{
	"products_sku_key":                          "sku",
	"products_barcode_key":                      "barcode",
	"products_category_id_fkey":                 "categoryId",
	"products_brand_id_fkey":                    "brandId",
	"products_unit_id_fkey":                     "unitId",
	"product_presentations_barcode_key":         "presentations",
	"product_presentations_product_id_name_key": "presentations",
}

type Repository struct{}

const selectProduct = `
	SELECT p.id, p.sku, p.barcode, p.name, p.description,
	       c.id, c.name, b.id, b.name, u.id, u.name, u.abbreviation,
	       p.alcohol_percent::text, p.volume_ml, p.image_url,
	       p.price::text, p.average_cost::text, p.tax_percent::text,
	       p.stock, p.min_stock, p.reorder_point, p.allow_negative_stock, p.active,
	       COALESCE((SELECT json_agg(json_build_object(
	           'id', pp.id, 'name', pp.name, 'factor', pp.factor, 'barcode', pp.barcode,
	           'price', pp.price::text, 'active', pp.active) ORDER BY pp.factor)
	         FROM product_presentations pp WHERE pp.product_id = p.id), '[]'),
	       p.updated_at
	FROM products p
	JOIN categories c ON c.id = p.category_id
	LEFT JOIN brands b ON b.id = p.brand_id
	JOIN units u ON u.id = p.unit_id`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	var brandID *uuid.UUID
	var brandName *string
	var cost string
	var presentations []byte
	err := row.Scan(&p.ID, &p.SKU, &p.Barcode, &p.Name, &p.Description,
		&p.Category.ID, &p.Category.Name, &brandID, &brandName, &p.Unit.ID, &p.Unit.Name, &p.Unit.Abbreviation,
		&p.AlcoholPercent, &p.VolumeML, &p.ImageURL,
		&p.Price, &cost, &p.TaxPercent,
		&p.Stock, &p.MinStock, &p.ReorderPoint, &p.AllowNegativeStock, &p.Active,
		&presentations, &p.UpdatedAt)
	if err != nil {
		return Product{}, err
	}
	if brandID != nil {
		p.Brand = &Ref{ID: *brandID, Name: *brandName}
	}
	p.AverageCost = &cost
	if err := json.Unmarshal(presentations, &p.Presentations); err != nil {
		return Product{}, fmt.Errorf("decode presentations: %w", err)
	}
	return p, nil
}

func (Repository) list(ctx context.Context, db database.DBTX, f Filter, page httpx.Page) ([]Product, int64, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	q := strings.TrimSpace(f.Search)
	queryParam := 0
	if q != "" {
		// Every word must appear in the name, accent-insensitive; exact codes and fuzzy
		// similarity (typos like "jonie walker") also match.
		var words []string
		for _, word := range strings.Fields(q) {
			args = append(args, word)
			words = append(words, fmt.Sprintf("immutable_unaccent(lower(p.name)) LIKE '%%' || immutable_unaccent(lower($%d)) || '%%'", len(args)))
		}
		add(fmt.Sprintf(`((%s) OR p.sku LIKE upper(?) || '%%' OR p.barcode = ?
		     OR immutable_unaccent(lower(p.name)) %% immutable_unaccent(lower(?))
		     OR EXISTS (SELECT 1 FROM product_presentations pp WHERE pp.product_id = p.id AND pp.barcode = ?))`,
			strings.Join(words, " AND ")), q)
		queryParam = len(args)
	}
	if f.CategoryID != nil {
		add("p.category_id = ?", *f.CategoryID)
	}
	if f.BrandID != nil {
		add("p.brand_id = ?", *f.BrandID)
	}
	if f.Active != nil {
		add("p.active = ?", *f.Active)
	}
	if f.LowStock {
		where = append(where, "p.stock <= p.min_stock")
	}
	condition := strings.Join(where, " AND ")

	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM products p WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	// Ranking parameters are only bound to the list query; the count doesn't reference them.
	order := "p.active DESC, p.name"
	if q != "" {
		var wholeWords []string
		for _, word := range strings.Fields(q) {
			args = append(args, regexp.QuoteMeta(word))
			wholeWords = append(wholeWords, fmt.Sprintf(`(immutable_unaccent(lower(p.name)) ~ ('\m' || immutable_unaccent(lower($%d)) || '\M'))::int`, len(args)))
		}
		// Exact codes, then names where the words appear whole ("7 años" over "750 ml"), then similarity.
		order = fmt.Sprintf(`(p.sku = upper($%[1]d) OR p.barcode = $%[1]d) DESC, (%[2]s) DESC,
			similarity(immutable_unaccent(lower(p.name)), immutable_unaccent(lower($%[1]d))) DESC, p.name`,
			queryParam, strings.Join(wholeWords, " + "))
	}

	args = append(args, page.Size, page.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d",
		selectProduct, condition, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Product, error) { return scanProduct(row) })
	if err != nil {
		return nil, 0, fmt.Errorf("read products: %w", err)
	}
	return items, total, nil
}

func (Repository) get(ctx context.Context, db database.DBTX, id uuid.UUID) (Product, error) {
	p, err := scanProduct(db.QueryRow(ctx, selectProduct+" WHERE p.id = $1", id))
	return p, database.MapError(err, nil)
}

// findByCode resolves a scanned code: product barcode or SKU, or a presentation barcode.
func (Repository) findByCode(ctx context.Context, db database.DBTX, code string) (Product, *uuid.UUID, error) {
	var productID uuid.UUID
	var presentationID *uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT id, NULL::uuid FROM products WHERE (barcode = $1 OR sku = upper($1)) AND active
		UNION ALL
		SELECT pp.product_id, pp.id FROM product_presentations pp
		JOIN products p ON p.id = pp.product_id
		WHERE pp.barcode = $1 AND pp.active AND p.active
		LIMIT 1`, strings.TrimSpace(code)).Scan(&productID, &presentationID)
	if err != nil {
		return Product{}, nil, database.MapError(err, nil)
	}
	p, err := Repository{}.get(ctx, db, productID)
	return p, presentationID, err
}

type writeInput struct {
	Input
	ID uuid.UUID
}

func (Repository) insert(ctx context.Context, db database.DBTX, in Input) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO products (sku, barcode, name, description, category_id, brand_id, unit_id,
		                      alcohol_percent, volume_ml, price, average_cost, tax_percent,
		                      min_stock, reorder_point, allow_negative_stock)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10::numeric, $11::numeric, $12::numeric, $13, $14, $15)
		RETURNING id`,
		in.SKU, in.Barcode, in.Name, in.Description, in.CategoryID, in.BrandID, in.UnitID,
		in.AlcoholPercent, in.VolumeML, in.Price, in.AverageCost, in.TaxPercent,
		in.MinStock, in.ReorderPoint, in.AllowNegativeStock).Scan(&id)
	return id, database.MapError(err, constraintFields)
}

func (Repository) update(ctx context.Context, db database.DBTX, w writeInput) error {
	_, err := db.Exec(ctx, `
		UPDATE products SET sku = $2, barcode = $3, name = $4, description = $5, category_id = $6, brand_id = $7,
		       unit_id = $8, alcohol_percent = $9::numeric, volume_ml = $10, price = $11::numeric,
		       tax_percent = $12::numeric, min_stock = $13, reorder_point = $14, allow_negative_stock = $15
		WHERE id = $1`,
		w.ID, w.SKU, w.Barcode, w.Name, w.Description, w.CategoryID, w.BrandID, w.UnitID,
		w.AlcoholPercent, w.VolumeML, w.Price, w.TaxPercent, w.MinStock, w.ReorderPoint, w.AllowNegativeStock)
	return database.MapError(err, constraintFields)
}

// syncPresentations upserts the given presentations and deactivates the rest; they are
// never deleted because past sales reference them.
func (Repository) syncPresentations(ctx context.Context, db database.DBTX, productID uuid.UUID, items []PresentationInput) error {
	keep := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item.ID != nil {
			tag, err := db.Exec(ctx, `
				UPDATE product_presentations SET name = $3, factor = $4, barcode = $5, price = $6::numeric, active = true
				WHERE id = $1 AND product_id = $2`, *item.ID, productID, item.Name, item.Factor, item.Barcode, item.Price)
			if err != nil {
				return database.MapError(err, constraintFields)
			}
			if tag.RowsAffected() > 0 {
				keep = append(keep, *item.ID)
				continue
			}
		}
		var id uuid.UUID
		err := db.QueryRow(ctx, `
			INSERT INTO product_presentations (product_id, name, factor, barcode, price)
			VALUES ($1, $2, $3, $4, $5::numeric) RETURNING id`, productID, item.Name, item.Factor, item.Barcode, item.Price).Scan(&id)
		if err != nil {
			return database.MapError(err, constraintFields)
		}
		keep = append(keep, id)
	}
	_, err := db.Exec(ctx, `
		UPDATE product_presentations SET active = false WHERE product_id = $1 AND NOT (id = ANY($2))`, productID, keep)
	return database.MapError(err, nil)
}

// barcodeTaken reports whether a code is already used by another product or presentation,
// since a scanned code must resolve to exactly one item.
func (Repository) barcodeTaken(ctx context.Context, db database.DBTX, code string, productID uuid.UUID) (bool, error) {
	var taken bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM products WHERE barcode = $1 AND id <> $2)
		    OR EXISTS (SELECT 1 FROM product_presentations WHERE barcode = $1 AND product_id <> $2)`, code, productID).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check barcode: %w", err)
	}
	return taken, nil
}

// importRefs are the lookups an import validates against, keyed by textx.Fold (names) or
// upper-case SKU.
type importRefs struct {
	categories map[string][]uuid.UUID
	units      map[string]uuid.UUID
	brands     map[string]uuid.UUID
	takenSKUs  map[string]bool
	takenCodes map[string]bool
}

func (Repository) importRefs(ctx context.Context, db database.DBTX, skus, codes []string) (importRefs, error) {
	refs := importRefs{categories: map[string][]uuid.UUID{}, units: map[string]uuid.UUID{}, brands: map[string]uuid.UUID{},
		takenSKUs: map[string]bool{}, takenCodes: map[string]bool{}}
	rows, err := db.Query(ctx, `
		SELECT 'category', id, name FROM categories WHERE active
		UNION ALL SELECT 'brand', id, name::text FROM brands
		UNION ALL SELECT 'unit', id, name::text FROM units
		UNION ALL SELECT 'unit', id, abbreviation::text FROM units
		UNION ALL SELECT 'sku', id, upper(sku::text) FROM products WHERE sku = ANY($1::text[]::citext[])
		UNION ALL SELECT 'code', id, barcode FROM products WHERE barcode = ANY($2)
		UNION ALL SELECT 'code', id, barcode FROM product_presentations WHERE barcode = ANY($2)`, skus, codes)
	if err != nil {
		return refs, fmt.Errorf("load import references: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, value string
		var id uuid.UUID
		if err := rows.Scan(&kind, &id, &value); err != nil {
			return refs, fmt.Errorf("read import references: %w", err)
		}
		switch kind {
		case "category":
			refs.categories[textx.Fold(value)] = append(refs.categories[textx.Fold(value)], id)
		case "brand":
			refs.brands[textx.Fold(value)] = id
		case "unit":
			refs.units[textx.Fold(value)] = id
		case "sku":
			refs.takenSKUs[value] = true
		case "code":
			refs.takenCodes[value] = true
		}
	}
	return refs, rows.Err()
}

func (Repository) ensureBrand(ctx context.Context, db database.DBTX, name string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO brands (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = brands.name RETURNING id`, name).Scan(&id)
	return id, database.MapError(err, nil)
}

// masterNames lists active category names and units as {name, abbreviation}.
func (Repository) masterNames(ctx context.Context, db database.DBTX) ([]string, [][2]string, error) {
	rows, err := db.Query(ctx, `
		SELECT name, abbreviation FROM (
			SELECT name, '' AS abbreviation FROM categories WHERE active
			UNION ALL SELECT name::text, abbreviation::text FROM units) m
		ORDER BY lower(name)`)
	if err != nil {
		return nil, nil, fmt.Errorf("list master names: %w", err)
	}
	defer rows.Close()
	var categories []string
	var units [][2]string
	for rows.Next() {
		var name, abbreviation string
		if err := rows.Scan(&name, &abbreviation); err != nil {
			return nil, nil, fmt.Errorf("read master names: %w", err)
		}
		if abbreviation == "" {
			categories = append(categories, name)
		} else {
			units = append(units, [2]string{name, abbreviation})
		}
	}
	return categories, units, rows.Err()
}

func (Repository) setActive(ctx context.Context, db database.DBTX, id uuid.UUID, active bool) error {
	tag, err := db.Exec(ctx, `UPDATE products SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return database.MapError(err, nil)
	}
	if tag.RowsAffected() == 0 {
		return database.MapError(pgx.ErrNoRows, nil)
	}
	return nil
}

func (Repository) setImage(ctx context.Context, db database.DBTX, id uuid.UUID, url *string) (*string, error) {
	var previous *string
	err := db.QueryRow(ctx, `
		UPDATE products p SET image_url = $2 FROM (SELECT image_url FROM products WHERE id = $1 FOR UPDATE) old
		WHERE p.id = $1 RETURNING old.image_url`, id, url).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, database.MapError(err, nil)
	}
	return previous, err
}
