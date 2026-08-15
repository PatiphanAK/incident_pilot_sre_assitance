package observability

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// RequestMiddleware logs every request as structured JSON and records it into
// the metrics collector. It is mounted globally via app.Use, so it also covers
// the /health endpoint.
func RequestMiddleware(m *Metrics) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)
		status := c.RequestCtx().Response.StatusCode()

		m.Observe(status, latency)
		fields := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"user_id", c.Locals("userID"),
		}
		// A non-nil err from c.Next() is a server-side error; log it at ERROR
		// level so the SRE agent can surface it even when the status read above is
		// still the default (Fiber v3 sets some statuses, e.g. Not Found, after the
		// middleware chain returns). The RequestErrors metric stays status-based so
		// 4xx responses are not counted as server errors.
		if err != nil {
			slog.Error("http.request", append(fields, "error", err.Error())...)
		} else {
			slog.Info("http.request", fields...)
		}
		return err
	}
}
