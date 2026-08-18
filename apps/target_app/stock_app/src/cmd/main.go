// Command stock_app runs the target application as a plain HTTP server — the
// ECS Fargate container style. On SIGTERM/SIGINT (ECS task stop, or Ctrl-C) it
// drains in-flight requests and stops the background workers before exiting.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"stock_app/src/internal/observability"
	"stock_app/src/internal/router"
)

func main() {
	observability.Configure()

	// ECS sends SIGTERM before stopping a task; treat it (and Ctrl-C) as shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowMethods: []string{
			fiber.MethodGet,
			fiber.MethodPost,
			fiber.MethodPut,
			fiber.MethodPatch,
			fiber.MethodDelete,
			fiber.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
		},
	}))

	deps, err := router.NewDependencies(ctx)
	if err != nil {
		slog.Error("build dependencies", "error", err)
		os.Exit(1)
	}
	defer deps.Shutdown()
	router.Register(app, deps)

	// Serve until the listener fails or a shutdown signal arrives.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("stock_app listening", "port", port)
		errCh <- app.Listen(":" + port)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("listen", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		// Graceful drain: stop accepting new requests and finish in-flight ones.
		// The metrics flusher / DB sampler (both tied to ctx) stop with the
		// context; the deferred deps.Shutdown() then closes the DB pools.
		slog.Info("shutdown signal received, draining")
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			slog.Error("graceful shutdown", "error", err)
		}
	}
}
