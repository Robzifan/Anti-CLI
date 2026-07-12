package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	var b bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&b, &slog.HandlerOptions{ReplaceAttr: redact}))
	l.LogAttrs(context.Background(), slog.LevelInfo, "x", slog.String("cookie", "secret"), slog.String("safe", "ok"))
	got := b.String()
	if strings.Contains(got, "secret") {
		t.Fatalf("secret leaked: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("redaction missing: %s", got)
	}
}
