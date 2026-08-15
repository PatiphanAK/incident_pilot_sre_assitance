package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/akrylysov/algnhsa"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"stock_app/src/internal/observability"
	"stock_app/src/internal/router"
)

var (
	app           *fiber.App
	lambdaHandler lambda.Handler
)

func init() {
	observability.Configure()

	app = fiber.New()

	deps, err := router.NewDependencies(context.Background())
	if err != nil {
		slog.Error("build dependencies", "error", err)
		os.Exit(1)
	}
	router.Register(app, deps)

	lambdaHandler = algnhsa.New(adaptor.FiberApp(app), nil)
}

func main() {
	// Local run: `go run ./src/cmd` without AWS_LAMBDA_RUNTIME_API starts a plain HTTP server.
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		slog.Info("stock_app listening", "port", port)
		if err := app.Listen(":" + port); err != nil {
			slog.Error("listen", "error", err)
			os.Exit(1)
		}
		return
	}
	lambda.Start(lambdaHandler)
}
