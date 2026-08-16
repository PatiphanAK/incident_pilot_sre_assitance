// Package cockroach is the outbound adapter persisting products in CockroachDB
// (stock_db). The application layer only sees the domain.ProductRepository port.
package cockroach

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/stock/domain"
)

type ProductRepository struct {
	pool *pgxpool.Pool
}

// Compile-time check that the adapter satisfies the port.
var _ domain.ProductRepository = (*ProductRepository)(nil)

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

// The products and inventory tables are created by the SQL files in migrations/,
// not by the app. See migrations/008_reset_stock_schema.sql.

// productSelect is the canonical SELECT list for products (joined with
// inventory); scanProduct relies on this order. A product without an inventory
// row still lists, with a quantity of 0.
const productSelect = `p.id, p.sku, p.name, p.description, COALESCE(i.quantity, 0), p.price, p.created_at, p.updated_at`

// productFrom joins the one-to-one inventory row.
const productFrom = `FROM products p LEFT JOIN inventory i ON i.product_id = p.id`

// Create inserts the product and its inventory row (initial quantity) together.
// Both tables live in stock_db, so this is a single-DB transaction.
func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO products (id, sku, name, description, price, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.SKU, p.Name, p.Description, formatDecimal(p.Price), p.CreatedAt, p.UpdatedAt,
	); err != nil {
		return translateError(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO inventory (product_id, quantity, updated_at)
		 VALUES ($1, $2, $3)`,
		p.ID, p.Quantity, p.UpdatedAt,
	); err != nil {
		return translateError(err)
	}
	return tx.Commit(ctx)
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+productSelect+` `+productFrom+` WHERE p.id = $1`, id)
	return scanProduct(row)
}

func (r *ProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+productSelect+` `+productFrom+` ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		p, err := scanProductRow(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, *p)
	}
	return products, rows.Err()
}

// Update sets name/description (each only when non-nil — COALESCE keeps the
// current value otherwise) and price, then returns the full updated product.
func (r *ProductRepository) Update(ctx context.Context, id string, name, description *string, price float64) (*domain.Product, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products
		 SET name = COALESCE($2, name),
		     description = COALESCE($3, description),
		     price = $4,
		     updated_at = now()
		 WHERE id = $1`,
		id, name, description, formatDecimal(price),
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrProductNotFound
	}
	return r.GetByID(ctx, id)
}

// Delete removes the product; the inventory row is removed by the DB-level
// ON DELETE CASCADE on inventory.product_id.
func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

// SetQuantity sets the product's quantity to the given absolute value, creating
// the inventory row if one is missing. The product's existence is checked first
// (a missing product is a not-found, not a silent new inventory row).
func (r *ProductRepository) SetQuantity(ctx context.Context, id string, quantity int) error {
	if _, err := r.GetByID(ctx, id); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO inventory (product_id, quantity, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (product_id) DO UPDATE
		   SET quantity = EXCLUDED.quantity, updated_at = EXCLUDED.updated_at`,
		id, quantity,
	)
	return err
}

// DecrementQuantity reduces a product's quantity by `by`. It reads the current
// product first (to capture the price and the not-found case), then updates.
// The UPDATE is guarded with `quantity >= $1`, so a concurrent reservation cannot
// drive the quantity negative even if the read was stale.
// Note: this is still read-then-update, so two concurrent reservations can both
// pass the guard under extreme contention; a production version would fold the
// decrement into one atomic statement.
func (r *ProductRepository) DecrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Quantity < by {
		return nil, domain.ErrInsufficientStock
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE inventory SET quantity = quantity - $1, updated_at = $2
		 WHERE product_id = $3 AND quantity >= $1`,
		by, time.Now().UTC(), id,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		// The read said there was enough, but the guarded update changed nothing:
		// a concurrent decrement won. Same failure the pre-read check would give.
		return nil, domain.ErrInsufficientStock
	}
	p.Quantity -= by
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

// IncrementQuantity increases a product's quantity by `by` (the compensation for a
// reservation that must be undone). The upsert also covers a product whose
// inventory row is missing: it is created with the released quantity.
func (r *ProductRepository) IncrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO inventory (product_id, quantity, updated_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (product_id) DO UPDATE
		   SET quantity = inventory.quantity + EXCLUDED.quantity, updated_at = EXCLUDED.updated_at`,
		id, by, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		// The product was deleted between the read and the write.
		return nil, domain.ErrProductNotFound
	}
	p.Quantity += by
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

// scanProduct reads one joined product row from a pgx.Row (GetByID).
func scanProduct(row pgx.Row) (*domain.Product, error) {
	var p domain.Product
	var desc *string
	var priceStr string
	if err := row.Scan(&p.ID, &p.SKU, &p.Name, &desc, &p.Quantity, &priceStr, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	if desc != nil {
		p.Description = *desc
	}
	p.Price = parseDecimal(priceStr)
	return &p, nil
}

// scanProductRow reads one joined product row from a pgx.Rows (List).
func scanProductRow(rows pgx.Rows) (*domain.Product, error) {
	var p domain.Product
	var desc *string
	var priceStr string
	if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &desc, &p.Quantity, &priceStr, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if desc != nil {
		p.Description = *desc
	}
	p.Price = parseDecimal(priceStr)
	return &p, nil
}

// translateError maps PostgreSQL/CockroachDB error codes to domain errors.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return domain.ErrSKUExists
	}
	return err
}

// parseDecimal parses a Cockroach DECIMAL/NUMERIC text value into a float64.
// We read price as text (rather than relying on the driver's NUMERIC decoding) so
// the behaviour is identical across pgx versions and Cockroach releases.
func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// formatDecimal renders a float64 as a 2-decimal string for INSERT/UPDATE into a
// DECIMAL column, avoiding any ambiguity with the driver's NUMERIC handling.
func formatDecimal(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
