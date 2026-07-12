package api_test

import (
    "context"
    "io"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/notioncodex/gateway/internal/api"
    "github.com/notioncodex/gateway/internal/models"
    "github.com/notioncodex/gateway/internal/notion/transport/mock"
)

func TestMCPAPIKeyProtectsMCPRoutes(t *testing.T) {
    transport := &mock.Transport{}
    registry := models.NewRegistry(transport)
    if err := registry.Refresh(context.Background(), "a", "w"); err != nil { t.Fatal(err) }
    handler := api.NewWithMCPAndKey(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, transport, time.Second, nil, nil, "test-key")

    unauthenticated := httptest.NewRecorder()
    handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil))
    if unauthenticated.Code != http.StatusUnauthorized { t.Fatalf("status=%d", unauthenticated.Code) }

    authenticated := httptest.NewRecorder()
    request := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil)
    request.Header.Set("X-NotionCodex-API-Key", "test-key")
    handler.ServeHTTP(authenticated, request)
    if authenticated.Code != http.StatusNotFound { t.Fatalf("status=%d", authenticated.Code) }
}
