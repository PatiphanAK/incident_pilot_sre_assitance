package cockroach

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool builds a pool from ORDER_DATABASE_URL (falling back to
// DATABASE_URL, exactly like the app's wiring in router.NewDependencies). It
// skips when unset so `go test` runs without a live database.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ORDER_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("ORDER_DATABASE_URL/DATABASE_URL not set; skipping live CockroachDB test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	return pool
}

// TestPingSchema is the regression for the 500s: it connects to the SAME
// database the order pool uses and asserts the orders/order_items tables are
// actually present. Pointed at target_app (the misconfiguration — the pools
// fell back to DATABASE_URL) it FAILS here; pointed at order_db it passes.
// This is exactly the check /health and startup now run.
func TestPingSchema(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	r := NewOrderRepository(pool)
	if err := r.PingSchema(context.Background()); err != nil {
		t.Fatalf("PingSchema (the app would 500 on every order request): %v", err)
	}
}
