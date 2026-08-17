package order

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/order/adapters/inbound/http" // package handlers
	"stock_app/src/internal/order/adapters/outbound/cockroach"
	"stock_app/src/internal/order/application"
	"stock_app/src/internal/order/domain"
)

// Module wires the order domain's own dependencies:
// CockroachDB (order_db) -> repository (outbound) -> service (application, which
// also depends on the stock StockPort) -> handler (inbound).
//
// stockPort is the in-process dependency on the stock context. It is satisfied by
// the stock context's StockService (see the router wiring); the order context
// never imports the stock context directly.
type Module struct {
	Handler *handlers.Handler
	repo    *cockroach.OrderRepository
}

func NewModule(pool *pgxpool.Pool, tokens *auth.TokenService, stockPort domain.StockPort) *Module {
	repo := cockroach.NewOrderRepository(pool)
	service := application.NewOrderService(repo, stockPort)
	handler := handlers.NewHandler(service, tokens)

	return &Module{
		Handler: handler,
		repo:    repo,
	}
}

// Check verifies the order schema exists in the connected database. It backs the
// /health endpoint and a one-time startup check, so a wrong-database
// misconfiguration (e.g. ORDER_DATABASE_URL unset and falling back to
// DATABASE_URL = target_app instead of order_db) is reported instead of
// silently 500-ing every request.
func (m *Module) Check(ctx context.Context) error {
	return m.repo.PingSchema(ctx)
}

// RegisterRoutes mounts the order routes on the given router.
func (m *Module) RegisterRoutes(router fiber.Router) {
	handlers.RegisterRoutes(router, m.Handler)
}
