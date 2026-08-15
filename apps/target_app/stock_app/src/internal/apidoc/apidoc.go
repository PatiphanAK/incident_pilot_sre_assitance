// Package apidoc serves the interactive API documentation: Swagger UI at
// /doc/api/v1 with the OpenAPI spec at /doc/api/v1/openapi.yaml. Both are
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
// how to get a token via /api/auth/login and use the Authorize button).
func RegisterRoutes(app *fiber.App) {
	app.Get("/doc/api/v1", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		return c.Send(page)
	})
	app.Get("/doc/api/v1/openapi.yaml", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml")
		return c.Send(spec)
	})
}
