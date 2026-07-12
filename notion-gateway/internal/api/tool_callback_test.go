package api_test

import (
    "bytes"
    "context"
    "io"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"
    "time"

    "github.com/notioncodex/gateway/internal/api"
    "github.com/notioncodex/gateway/internal/domain"
    "github.com/notioncodex/gateway/internal/memory"
    "github.com/notioncodex/gateway/internal/models"
    "github.com/notioncodex/gateway/internal/notion/transport/mock"

    _ "modernc.org/sqlite"
)

func TestNormalizedToolEventPersistsToMemory(t *testing.T) {
    store, err := memory.OpenSQLite(context.Background(), "sqlite", filepath.Join(t.TempDir(), "memory.db"))
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()

    tool := domain.ToolEvent{
        Name: "transcript.decoder", Operation: "decode", PayloadVariant: "minimalArrayUser",
        OutputSummary: "array stream decoded successfully", Status: "succeeded", IdempotencyKey: "callback-test-1",
    }
    transport := &mock.Transport{}
    transport.ToolEvents = append(transport.ToolEvents, tool)
    registry := models.NewRegistry(transport)
    if err := registry.Refresh(context.Background(), "a", "w"); err != nil {
        t.Fatal(err)
    }
    engine := memory.New(store, memory.Config{Enabled: true})
    handler := api.New(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, transport, time.Second, engine)

    request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"mock/echo","input":"decode transcript","stream":false,"metadata":{"ncg_conversation_id":"callback-test","ncg_project_id":"project-a"}}`))
    recorder := httptest.NewRecorder()
    handler.ServeHTTP(recorder, request)
    if recorder.Code != http.StatusOK {
        t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
    }

    processed, err := memory.NewWorker(store).RunOnce(context.Background())
    if err != nil || !processed {
        t.Fatalf("processed=%v err=%v", processed, err)
    }
    executions, err := store.SearchToolExecutions(context.Background(), memory.Scope{UserID: "local", ProjectID: "project-a"}, "decoder", 5)
    if err != nil {
        t.Fatal(err)
    }
    if len(executions) != 1 || executions[0].PayloadVariant != "minimalArrayUser" {
        t.Fatalf("executions=%#v", executions)
    }
}
