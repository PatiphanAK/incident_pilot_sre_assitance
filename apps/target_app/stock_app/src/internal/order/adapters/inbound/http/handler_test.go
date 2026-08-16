package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"stock_app/src/internal/order/domain"
)

// TestMapError covers the order handler's error mapping: each domain error keeps
// its specific status and message, while a raw (non-domain) database error must
// collapse to a generic 500 and never leak to the client.
func TestMapError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		body   string // substring expected in the JSON body
	}{
		{"not found", domain.ErrOrderNotFound, fiber.StatusNotFound, "order not found"},
		{"invalid status transition", domain.ErrInvalidStatusTransition, fiber.StatusBadRequest, "invalid status transition"},
		{"insufficient stock", domain.ErrInsufficientStock, fiber.StatusConflict, "insufficient stock"},
		{"product not found", domain.ErrProductNotFound, fiber.StatusConflict, "product not found"},
		// A raw CockroachDB error (e.g. "relation \"orders\" does not exist",
		// SQLSTATE 42P01) is NOT a domain error, so it must collapse to a generic
		// 500 — and never expose itself.
		{"raw db error", errors.New(`ERROR: relation "orders" does not exist (SQLSTATE 42P01)`),
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
