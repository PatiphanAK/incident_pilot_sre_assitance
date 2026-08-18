package cockroach

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool builds a pool from STOCK_DATABASE_URL (falling back to
// DATABASE_URL, exactly like the app's wiring in router.NewDependencies). It
// skips when unset so `go test` runs without a live database.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("STOCK_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("STOCK_DATABASE_URL/DATABASE_URL not set; skipping live CockroachDB test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	return pool
}

// TestPingSchema is the regression for the 500s: it connects to the SAME
// database the stock pool uses and asserts the products/inventory tables are
// actually present. Pointed at target_app (the misconfiguration — the pools
// fell back to DATABASE_URL) it FAILS here; pointed at stock_db it passes.
// This is exactly the check /health and startup now run.
func TestPingSchema(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	r := NewProductRepository(pool)
	if err := r.PingSchema(context.Background()); err != nil {
		t.Fatalf("PingSchema (the app would 500 on every stock request): %v", err)
	}
}
