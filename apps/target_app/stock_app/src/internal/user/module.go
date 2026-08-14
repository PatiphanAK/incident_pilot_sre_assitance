package user

import (
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
}

func NewModule(pool *pgxpool.Pool, tokens *auth.TokenService) *Module {
	repo := cockroach.NewUserRepository(pool)
	service := application.NewUserService(repo)
	handler := handlers.NewHandler(service, tokens)

	return &Module{
		Handler: handler,
	}
}

// RegisterRoutes mounts the user routes on the given router.
func (m *Module) RegisterRoutes(router fiber.Router) {
	handlers.RegisterRoutes(router, m.Handler)
}
