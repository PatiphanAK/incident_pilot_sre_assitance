// Package observability instruments the target app so the SRE agent can observe
// it: structured JSON logs (get_log), CloudWatch metrics (get_metric), and the
// /health endpoint (get_health). It has no dependency on the business domains.
package observability

import (
	"log/slog"
	"os"
)

// Configure installs a structured JSON logger to stdout as the slog default.
// CloudWatch captures the container's stdout into CloudWatch Logs, which the SRE
// agent reads via get_log(). Logging has no AWS dependency, so it always works,
// including local dev.
func Configure() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
}
