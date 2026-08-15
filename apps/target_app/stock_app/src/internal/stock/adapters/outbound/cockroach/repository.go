// Package cockroach is the outbound adapter persisting products in CockroachDB
// (stock_db). The application layer only sees the domain.ProductRepository port.
package cockroach

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
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

// The products table is created by the SQL files in migrations/, not by the app.
// See migrations/005_create_stock_tables.sql.

// productColumns is the canonical SELECT list for products; scanProduct relies on
// this order.
const productColumns = "id, name, quantity, price, created_at, updated_at"

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO products (id, name, quantity, price, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.Name, p.Quantity, formatDecimal(p.Price), p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1`, id)
	return scanProduct(row)
}

func (r *ProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		var priceStr string
		if err := rows.Scan(&p.ID, &p.Name, &p.Quantity, &priceStr, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Price = parseDecimal(priceStr)
		products = append(products, p)
	}
	return products, rows.Err()
}

// DecrementQuantity reduces a product's quantity by `by`. It reads the current
// product first (to capture the price and the not-found case), then updates.
// Note: this is read-then-update, so it is not race-safe under concurrent
// reservations on the same product; a production version would use an atomic
// `UPDATE ... WHERE quantity >= $by RETURNING ...` or a SELECT ... FOR UPDATE.
func (r *ProductRepository) DecrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Quantity < by {
		return nil, domain.ErrInsufficientStock
	}
	if _, err := r.pool.Exec(ctx,
		`UPDATE products SET quantity = quantity - $1, updated_at = $2 WHERE id = $3`,
		by, time.Now().UTC(), id,
	); err != nil {
		return nil, err
	}
	p.Quantity -= by
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

// IncrementQuantity increases a product's quantity by `by` (the compensation for a
// reservation that must be undone).
func (r *ProductRepository) IncrementQuantity(ctx context.Context, id string, by int) (*domain.Product, error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := r.pool.Exec(ctx,
		`UPDATE products SET quantity = quantity + $1, updated_at = $2 WHERE id = $3`,
		by, time.Now().UTC(), id,
	); err != nil {
		return nil, err
	}
	p.Quantity += by
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

func scanProduct(row pgx.Row) (*domain.Product, error) {
	var p domain.Product
	var priceStr string
	if err := row.Scan(&p.ID, &p.Name, &p.Quantity, &priceStr, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	p.Price = parseDecimal(priceStr)
	return &p, nil
}

// parseDecimal parses a Cockroach DECIMAL/NUMERIC text value into a float64.
// We read price as text (rather than relying on the driver's NUMERIC decoding) so
// the behaviour is identical across pgx versions and Cockroach releases.
func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// formatDecimal renders a float64 as a 2-decimal string for INSERT into a DECIMAL
// column, avoiding any ambiguity with the driver's NUMERIC handling.
func formatDecimal(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
