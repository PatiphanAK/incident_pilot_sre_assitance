package user

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/user/adapters/inbound/http" // package handlers
	"stock_app/src/internal/user/adapters/outbound/cockroach"
	"stock_app/src/internal/user/application"
)

// Module wires the user domain's own dependencies:
// CockroachDB (infra) -> repository (outbound adapter) -> service (application) -> handler (inbound).
type Module struct {
	Handler *handlers.Handler
	repo    *cockroach.UserRepository
}

func NewModule(pool *pgxpool.Pool, tokens *auth.TokenService) *Module {
	repo := cockroach.NewUserRepository(pool)
	service := application.NewUserService(repo)
	handler := handlers.NewHandler(service, tokens)

	return &Module{
		Handler: handler,
		repo:    repo,
	}
}

// Check verifies the user schema exists in the connected database. It backs the
// /health endpoint and a one-time startup check, so a wrong-database
// misconfiguration (e.g. DATABASE_URL pointing at defaultdb instead of
// target_app) is reported instead of silently 500-ing every request.
func (m *Module) Check(ctx context.Context) error {
	return m.repo.PingSchema(ctx)
}

// RegisterRoutes mounts the user routes on the given router.
func (m *Module) RegisterRoutes(router fiber.Router) {
	handlers.RegisterRoutes(router, m.Handler)
}
