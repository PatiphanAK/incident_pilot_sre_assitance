package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"stock_app/src/internal/user/domain"
)

// TestMapError covers the failure that produced the 500s: a raw (non-domain)
// database error must map to a generic 500 (not leak the error to the client),
// while each domain error keeps its specific status and message.
func TestMapError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		body   string // substring expected in the JSON body
	}{
		{"not found", domain.ErrUserNotFound, fiber.StatusNotFound, "user not found"},
		{"email taken", domain.ErrEmailTaken, fiber.StatusConflict, "email already taken"},
		{"invalid credentials", domain.ErrInvalidCredentials, fiber.StatusUnauthorized, "invalid email or password"},
		// A raw CockroachDB error such as the one that hit defaultdb
		// ("relation \"users\" does not exist", SQLSTATE 42P01) is NOT a domain
		// error, so it must collapse to a generic 500 — and never expose itself.
		{"raw db error", errors.New(`ERROR: relation "users" does not exist (SQLSTATE 42P01)`),
			fiber.StatusInternalServerError, "internal server error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// mapError only uses the fiber.Ctx; the service/tokens can be nil.
			h := NewHandler(nil, nil)
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return h.mapError(c, tc.err) })

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tc.body) {
				t.Errorf("body = %q, want to contain %q", string(body), tc.body)
			}
			// The raw error must never reach the client.
			if strings.Contains(string(body), `relation`) {
				t.Errorf("raw db error leaked to client: %q", string(body))
			}
		})
	}
}
