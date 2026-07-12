package telemetry

import (
	"log/slog"
	"os"
	"strings"
)

func NewLogger(level string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l, ReplaceAttr: redact}))
}
func redact(_ []string, a slog.Attr) slog.Attr {
	switch strings.ToLower(a.Key) {
	case "authorization", "cookie", "set-cookie", "token_v2", "api_key", "auth.json":
		a.Value = slog.StringValue("[REDACTED]")
	}
	return a
}
