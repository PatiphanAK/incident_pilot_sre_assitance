// Package cockroach is the outbound adapter persisting orders in CockroachDB
// (order_db). The application layer only sees the domain.OrderRepository port.
package cockroach

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/order/domain"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

// Compile-time check that the adapter satisfies the port.
var _ domain.OrderRepository = (*OrderRepository)(nil)

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

// The orders and order_items tables are created by the SQL files in migrations/,
// not by the app. See migrations/007_create_order_tables.sql (recreated by
// migrations/008_reset_stock_schema.sql).

// PingSchema reports whether the order tables (orders, order_items) actually
// exist in the connected database. A connection can reach CockroachDB
// (Pool.Ping succeeds) while the pool points at the wrong database — e.g.
// target_app instead of order_db — in which case the tables are absent and
// every real query fails with "relation orders does not exist" (SQLSTATE
// 42P01). /health and startup use this so that situation surfaces as a failure
// instead of a false "up".
func (r *OrderRepository) PingSchema(ctx context.Context) error {
	var orders, orderItems bool
	if err := r.pool.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'orders'),
			EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'order_items')`,
	).Scan(&orders, &orderItems); err != nil {
		return err
	}
	if !orders || !orderItems {
		return errors.New("schema unavailable: orders/order_items tables not found in the connected database (expected the order_db schema)")
	}
	return nil
}

const orderColumns = "id, user_id, status, total_price, created_at"

// Create inserts the order and its items together. orders and order_items live in
// the same database (order_db), so this is a single-DB transaction (not a
// distributed one).
func (r *OrderRepository) Create(ctx context.Context, o *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO orders (id, user_id, status, total_price, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		o.ID, o.UserID, o.Status, formatDecimal(o.TotalPrice), o.CreatedAt,
	); err != nil {
		return err
	}
	for _, item := range o.Items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity, price_at_purchase)
			 VALUES ($1, $2, $3, $4)`,
			o.ID, item.ProductID, item.Quantity, formatDecimal(item.PriceAtPurchase),
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id)
	o, err := scanOrderRow(row)
	if err != nil {
		return nil, err
	}
	items, err := r.items(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *OrderRepository) List(ctx context.Context) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+orderColumns+` FROM orders ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var o domain.Order
		var totalStr string
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &totalStr, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.TotalPrice = parseDecimal(totalStr)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// UpdateStatus sets the order's status. Transition rules are enforced by the
// application layer (OrderService.UpdateOrderStatus), not here.
func (r *OrderRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE orders SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

// Delete removes the order and its items. order_items references orders within
// order_db, so the items must go first — and in one transaction, so a failure
// cannot leave an order with orphaned items.
func (r *OrderRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM orders WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}
	return tx.Commit(ctx)
}

// items loads the line items of one order.
func (r *OrderRepository) items(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, order_id, product_id, quantity, price_at_purchase FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var it domain.OrderItem
		var priceStr string
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.Quantity, &priceStr); err != nil {
			return nil, err
		}
		it.PriceAtPurchase = parseDecimal(priceStr)
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanOrderRow(row pgx.Row) (*domain.Order, error) {
	var o domain.Order
	var totalStr string
	if err := row.Scan(&o.ID, &o.UserID, &o.Status, &totalStr, &o.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	o.TotalPrice = parseDecimal(totalStr)
	return &o, nil
}

// parseDecimal parses a Cockroach DECIMAL/NUMERIC text value into a float64.
func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// formatDecimal renders a float64 as a 2-decimal string for INSERT into a DECIMAL
// column, avoiding any ambiguity with the driver's NUMERIC handling.
func formatDecimal(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
