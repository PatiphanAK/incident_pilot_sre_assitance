package auth

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// RequireAuth is fiber middleware that guards routes behind a valid
// "Authorization: Bearer <token>" header. On success the user id from the
// token's subject is available via c.Locals("userID").
func RequireAuth(ts *TokenService) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid authorization header"})
		}
		claims, err := ts.Verify(strings.TrimPrefix(header, prefix))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}
		c.Locals("userID", claims.Subject)
		return c.Next()
	}
}
