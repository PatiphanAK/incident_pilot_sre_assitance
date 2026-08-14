package main

import (
	"context"
	"log"
	"os"

	"github.com/akrylysov/algnhsa"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"stock_app/src/internal/router"
)

var (
	app           *fiber.App
	lambdaHandler lambda.Handler
)

func init() {
	app = fiber.New()

	deps, err := router.NewDependencies(context.Background())
	if err != nil {
		log.Fatalf("build dependencies: %v", err)
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
		log.Printf("stock_app listening on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("listen: %v", err)
		}
		return
	}
	lambda.Start(lambdaHandler)
}
