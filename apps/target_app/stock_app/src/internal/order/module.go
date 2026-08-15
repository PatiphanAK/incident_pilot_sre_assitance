package order

import (
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
}

func NewModule(pool *pgxpool.Pool, tokens *auth.TokenService, stockPort domain.StockPort) *Module {
	repo := cockroach.NewOrderRepository(pool)
	service := application.NewOrderService(repo, stockPort)
	handler := handlers.NewHandler(service, tokens)

	return &Module{
		Handler: handler,
	}
}

// RegisterRoutes mounts the order routes on the given router.
func (m *Module) RegisterRoutes(router fiber.Router) {
	handlers.RegisterRoutes(router, m.Handler)
}
