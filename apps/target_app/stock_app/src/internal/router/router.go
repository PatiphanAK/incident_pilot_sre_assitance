package router

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/observability"
	"stock_app/src/internal/user"
)

// Dependencies holds the shared infrastructure and the wired domain modules.
type Dependencies struct {
	Pool    *pgxpool.Pool
	Tokens  *auth.TokenService
	User    *user.Module
	Metrics *observability.Metrics
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
	observability.StartFlusher(context.Background(), metrics, publisher, interval)

	return &Dependencies{
		Pool:    pool,
		Tokens:  tokens,
		User:    userModule,
		Metrics: metrics,
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
	app.Use(observability.RequestMiddleware(deps.Metrics))
	app.Get("/health", health(deps))

	api := app.Group("/api")
	deps.User.RegisterRoutes(api)
}

func health(deps *Dependencies) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := deps.Pool.Ping(c.Context()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":   "unhealthy",
				"database": "down",
			})
		}
		return c.JSON(fiber.Map{
			"status":   "ok",
			"service":  "stock_app",
			"database": "up",
		})
	}
}
