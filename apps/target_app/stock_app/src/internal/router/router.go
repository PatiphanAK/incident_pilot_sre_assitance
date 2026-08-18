package router

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/apidoc"
	"stock_app/src/internal/auth"
	"stock_app/src/internal/observability"
	"stock_app/src/internal/order"
	"stock_app/src/internal/order/adapters/outbound/stock" // package stockadapter
	"stock_app/src/internal/stock"
	"stock_app/src/internal/user"
)

// Dependencies holds the shared infrastructure and the wired domain modules.
type Dependencies struct {
	Pool    *pgxpool.Pool
	Tokens  *auth.TokenService
	User    *user.Module
	Stock   *stock.Module
	Order   *order.Module
	Metrics *observability.Metrics

	stockPool *pgxpool.Pool
	orderPool *pgxpool.Pool
}

// Shutdown releases the infrastructure. The background workers (metrics flusher,
// DB sampler) are tied to the context passed to NewDependencies, so they stop on
// its cancellation (SIGTERM) without needing an explicit stop here.
func (d *Dependencies) Shutdown() {
	d.Pool.Close()
	d.stockPool.Close()
	d.orderPool.Close()
}

// NewDependencies creates the infrastructure connections (CockroachDB) and
// lets each business domain module wire its own dependencies.
func NewDependencies(ctx context.Context) (*Dependencies, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	tokens := auth.NewTokenService(jwtSecret)

	userModule := user.NewModule(pool, tokens)

	// Surface a wrong-database / missing-schema misconfiguration at boot. The app
	// still starts (so /health can report the state), but this ERROR log means the
	// SRE agent sees the problem immediately instead of only after the first 500.
	if err := userModule.Check(ctx); err != nil {
		slog.Error("startup.schema_check_failed",
			"database", "target_app",
			"error", err.Error(),
			"hint", "verify DATABASE_URL points at the database with the migrations applied (e.g. /target_app, not /defaultdb)",
		)
	}

	// Each module owns its own database. STOCK/ORDER_DATABASE_URL fall back to
	// DATABASE_URL so a single-database setup still works; set them to the
	// separate stock_db / order_db URLs for the prepared-to-split layout.
	stockPool, err := pgxpool.New(ctx, envOr("STOCK_DATABASE_URL", databaseURL))
	if err != nil {
		return nil, fmt.Errorf("create stock pool: %w", err)
	}
	orderPool, err := pgxpool.New(ctx, envOr("ORDER_DATABASE_URL", databaseURL))
	if err != nil {
		return nil, fmt.Errorf("create order pool: %w", err)
	}

	stockModule := stock.NewModule(stockPool, tokens)
	// The order module depends on the stock context in-process via its StockPort.
	// stockadapter translates the stock context's errors into the order domain's.
	orderModule := order.NewModule(orderPool, tokens, stockadapter.NewAdapter(stockModule.Service))

	// Same boot-time schema check as for users: in the deployed layout the stock
	// and order tables live in stock_db / order_db, so an unset
	// STOCK/ORDER_DATABASE_URL (falling back to DATABASE_URL = /target_app) used
	// to 500 every stock/order request while /health still reported "ok".
	if err := stockModule.Check(ctx); err != nil {
		slog.Error("startup.schema_check_failed",
			"database", "stock_db",
			"error", err.Error(),
			"hint", "verify STOCK_DATABASE_URL points at stock_db (unset = falls back to DATABASE_URL)",
		)
	}
	if err := orderModule.Check(ctx); err != nil {
		slog.Error("startup.schema_check_failed",
			"database", "order_db",
			"error", err.Error(),
			"hint", "verify ORDER_DATABASE_URL points at order_db (unset = falls back to DATABASE_URL)",
		)
	}

	// Observability: structured logging + CloudWatch metrics. A nil publisher
	// (no AWS_REGION) means metrics are emitted to the log stream instead.
	metrics := observability.NewMetrics()
	publisher, err := observability.NewPublisher(ctx, os.Getenv("AWS_REGION"), envOr("CLOUDWATCH_NAMESPACE", "stock_app"), "stock_app")
	if err != nil {
		slog.Warn("cloudwatch metrics disabled", "error", err)
	}
	interval := 60 * time.Second
	if v := os.Getenv("METRICS_FLUSH_INTERVAL"); v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			interval = d
		}
	}
	// ctx here is the process's signal context: cancelling it (SIGTERM/SIGINT)
	// stops the flusher and the DB sampler below.
	observability.StartFlusher(ctx, metrics, publisher, interval)

	// Per-database health sampling (CockroachDB): the observation bot's
	// get_metric() can watch each database, not just HTTP.
	dbInterval := 15 * time.Second
	if v := os.Getenv("DB_SAMPLE_INTERVAL"); v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			dbInterval = d
		}
	}
	observability.StartDBSampler(ctx, []observability.NamedPinger{
		{Database: "target_app", Pinger: pool},
		{Database: "stock_db", Pinger: stockPool},
		{Database: "order_db", Pinger: orderPool},
	}, metrics, dbInterval)

	return &Dependencies{
		Pool:      pool,
		Tokens:    tokens,
		User:      userModule,
		Stock:     stockModule,
		Order:     orderModule,
		Metrics:   metrics,
		stockPool: stockPool,
		orderPool: orderPool,
	}, nil
}

// envOr returns the value of the environment variable key, or def when it is empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Register mounts cross-cutting routes and delegates route registration to each module.
func Register(app *fiber.App, deps *Dependencies) {
<<<<<<< HEAD
	// เปิดทางให้ Frontend จากพอร์ต 5173 ยิง API เข้ามาได้
	app.Use(cors.New())

=======
	app.Use(observability.RequestMiddleware(deps.Metrics))
	apidoc.RegisterRoutes(app)
>>>>>>> main
	app.Get("/health", health(deps))

	// API versioned under /api/v1: every module's routes are mounted on this
	// group, so a future /api/v2 can be added alongside without moving v1.
	api := app.Group("/api/v1")
	deps.User.RegisterRoutes(api)
<<<<<<< HEAD

	// 👉 เพิ่มเส้นทางสำหรับจัดการ Order ตรงนี้ครับ
	api.Post("/orders", func(c fiber.Ctx) error {
		// จำลองการสั่งซื้อสำเร็จและตัดสต็อก
		return c.JSON(fiber.Map{
			"message": "Order placed successfully",
			"status":  "success",
		})
	})
=======
	deps.Stock.RegisterRoutes(api)
	deps.Order.RegisterRoutes(api)
}

// healthCheck pings one database and verifies its schema. Real health checks
// also feed the DatabaseLatency / DatabaseUp metrics.
func healthCheck(ctx context.Context, name string, p *pgxpool.Pool, check func(context.Context) error, m *observability.Metrics) error {
	start := time.Now()
	if err := p.Ping(ctx); err != nil {
		m.ObserveDatabase(name, time.Since(start), err)
		return err
	}
	if err := check(ctx); err != nil {
		m.ObserveDatabase(name, time.Since(start), err)
		return err
	}
	m.ObserveDatabase(name, time.Since(start), nil)
	return nil
>>>>>>> main
}

func health(deps *Dependencies) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		// A connection can reach CockroachDB while pointed at the wrong database
		// (e.g. defaultdb instead of target_app, or DATABASE_URL used for all
		// three pools while the business tables live in stock_db / order_db):
		// the ping succeeds, but the app's tables are absent and every real
		// request 500s. Verify each database the app actually depends on so
		// /health reports "down" instead of a false "up".
		var down []string
		if err := healthCheck(ctx, "target_app", deps.Pool, deps.User.Check, deps.Metrics); err != nil {
			slog.Error("health.schema_check_failed", "database", "target_app", "error", err.Error())
			down = append(down, "target_app")
		}
		if err := healthCheck(ctx, "stock_db", deps.stockPool, deps.Stock.Check, deps.Metrics); err != nil {
			slog.Error("health.schema_check_failed", "database", "stock_db", "error", err.Error())
			down = append(down, "stock_db")
		}
		if err := healthCheck(ctx, "order_db", deps.orderPool, deps.Order.Check, deps.Metrics); err != nil {
			slog.Error("health.schema_check_failed", "database", "order_db", "error", err.Error())
			down = append(down, "order_db")
		}
		if len(down) > 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":   "unhealthy",
				"database": "down",
				"reason":   "schema unavailable",
				"down":     down,
			})
		}
		return c.JSON(fiber.Map{
			"status":   "ok",
			"service":  "stock_app",
			"database": "up",
		})
	}
}
