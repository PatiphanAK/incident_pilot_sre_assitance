// Package cockroach is the outbound adapter persisting users in CockroachDB.
// The application layer only sees the domain.UserRepository port.
package cockroach

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/user/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

// Compile-time check that the adapter satisfies the port.
var _ domain.UserRepository = (*UserRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// The users table is created by the SQL files in migrations/, not by the app.
// See migrations/002_create_users.sql and migrations/003_add_password_hash.sql.

// userColumns is the canonical SELECT list for users; scanUser relies on this order.
const userColumns = "id, username, email, password_hash, created_at, updated_at"

// PingSchema reports whether the users table actually exists in the connected
// database. A connection can reach CockroachDB (Pool.Ping succeeds) while
// pointing at the wrong database — e.g. defaultdb instead of target_app — in
// which case the table is absent and every real query fails with
// "relation users does not exist" (SQLSTATE 42P01). /health and startup use
// this so that situation surfaces as a failure instead of a false "up".
func (r *UserRepository) PingSchema(ctx context.Context) error {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')`,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("schema unavailable: users table not found in the connected database")
	}
	return nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	return translateError(err)
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		user := domain.User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE users
		 SET username = $2, email = $3, updated_at = $4
		 WHERE id = $1
		 RETURNING `+userColumns,
		user.ID, user.Username, user.Email, user.UpdatedAt,
	)
	updated, err := scanUser(row)
	if err != nil {
		return err
	}
	*user = *updated
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	user := domain.User{}
	if err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// translateError maps PostgreSQL/CockroachDB error codes to domain errors.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return domain.ErrEmailTaken
	}
	return err
}
