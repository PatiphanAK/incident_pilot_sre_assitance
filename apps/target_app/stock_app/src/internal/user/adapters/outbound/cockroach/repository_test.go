package cockroach

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/user/domain"
)

// newTestPool builds a pool from DATABASE_URL; it skips when unset so `go test`
// runs without a live database.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping live CockroachDB test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	return pool
}

// TestPingSchema is the regression for the 500s: it connects to the SAME
// DATABASE_URL the app uses and asserts the users table is actually present.
// Pointed at defaultdb (the misconfiguration) it FAILS here; pointed at
// target_app it passes. This is exactly the check /health and startup now run.
func TestPingSchema(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	r := NewUserRepository(pool)
	if err := r.PingSchema(context.Background()); err != nil {
		t.Fatalf("PingSchema (the app would 500 on every request): %v", err)
	}
}

// TestRegisterLoginRoundTrip exercises the exact SQL that 500'd: the register
// INSERT and the login SELECT. It also catches a schema/column mismatch against
// the CockroachDB schema.
func TestRegisterLoginRoundTrip(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	r := NewUserRepository(pool)
	ctx := context.Background()

	// A unique email avoids the UNIQUE constraint on repeated runs.
	email := fmt.Sprintf("itest-%d@x.com", time.Now().UnixNano())
	now := time.Now().UTC()
	u := &domain.User{
		ID:           uuid.NewString(),
		Username:     "itest",
		Email:        email,
		PasswordHash: "test-hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// register: INSERT
	if err := r.Create(ctx, u); err != nil {
		t.Fatalf("Create (register) failed: %v", err)
	}
	// login: SELECT ... WHERE email = $1 (must run before cleanup)
	got, err := r.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail (login) failed: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("GetByEmail id = %q, want %q", got.ID, u.ID)
	}
	// cleanup
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
