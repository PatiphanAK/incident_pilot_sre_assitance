// Package apidoc serves the interactive API documentation: Swagger UI at
// /docs with the OpenAPI spec at /docs/openapi.yaml. Both are
// embedded in the binary, so the distroless container needs no extra files, and
// the page is same-origin with the API — Try-it-out calls work without CORS.
package apidoc

import (
	_ "embed"

	"github.com/gofiber/fiber/v3"
)

//go:embed openapi.yaml
var spec []byte

//go:embed swagger.html
var page []byte

// RegisterRoutes mounts the documentation (public — no auth; the page documents
// how to get a token via /api/v1/auth/login and use the Authorize button).
func RegisterRoutes(app *fiber.App) {
	app.Get("/docs", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		return c.Send(page)
	})
	app.Get("/docs/openapi.yaml", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml")
		return c.Send(spec)
	})
}
