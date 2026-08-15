package router

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"stock_app/src/internal/auth"
	"stock_app/src/internal/user"
)

// Dependencies holds the shared infrastructure and the wired domain modules.
type Dependencies struct {
	Pool   *pgxpool.Pool
	Tokens *auth.TokenService
	User   *user.Module
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

	return &Dependencies{
		Pool:   pool,
		Tokens: tokens,
		User:   userModule,
	}, nil
}

// Register mounts cross-cutting routes and delegates route registration to each module.
func Register(app *fiber.App, deps *Dependencies) {
	// เปิดทางให้ Frontend จากพอร์ต 5173 ยิง API เข้ามาได้
	app.Use(cors.New())

	app.Get("/health", health(deps))

	api := app.Group("/api")
	deps.User.RegisterRoutes(api)

	// 👉 เพิ่มเส้นทางสำหรับจัดการ Order ตรงนี้ครับ
	api.Post("/orders", func(c fiber.Ctx) error {
		// จำลองการสั่งซื้อสำเร็จและตัดสต็อก
		return c.JSON(fiber.Map{
			"message": "Order placed successfully",
			"status":  "success",
		})
	})
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
