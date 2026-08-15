package stock

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/stock/adapters/inbound/http" // package handlers
	"stock_app/src/internal/stock/adapters/outbound/cockroach"
	"stock_app/src/internal/stock/application"
)

// Module wires the stock domain's own dependencies:
// CockroachDB (stock_db) -> repository (outbound) -> service (application) -> handler (inbound).
//
// Service is exposed so other contexts (e.g. the order context) can call it
// in-process; the order context depends on it structurally via its own StockPort.
type Module struct {
	Handler *handlers.Handler
	Service *application.StockService
}

func NewModule(pool *pgxpool.Pool, tokens *auth.TokenService) *Module {
	repo := cockroach.NewProductRepository(pool)
	service := application.NewStockService(repo)
	handler := handlers.NewHandler(service, tokens)

	return &Module{
		Handler: handler,
		Service: service,
	}
}

// RegisterRoutes mounts the stock routes on the given router.
func (m *Module) RegisterRoutes(router fiber.Router) {
	handlers.RegisterRoutes(router, m.Handler)
}
