package memory

import (
    "context"
    "path/filepath"
    "testing"

    _ "modernc.org/sqlite"
)

func TestWorkerConsolidatesOutboxAndCreatesMemory(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "nested", "memory.db")
    store, err := OpenSQLite(context.Background(), "sqlite", dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()

    scope := Scope{UserID: "u", ProjectID: "p"}
    conversationID, err := store.ResolveConversation(context.Background(), scope, "", "", "worker-test")
    if err != nil {
        t.Fatal(err)
    }
    turnID, err := store.AppendUser(context.Background(), conversationID, "Remember the decision", "mock/echo")
    if err != nil {
        t.Fatal(err)
    }
    if err := store.CompleteTurn(context.Background(), turnID, "Decision: use an outbox worker for derived memory.", "resp-worker", "completed"); err != nil {
        t.Fatal(err)
    }

    processed, err := NewWorker(store).RunOnce(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if !processed {
        t.Fatal("expected pending outbox event")
    }
    records, err := store.SearchMemories(context.Background(), scope, "what decision", 5)
    if err != nil {
        t.Fatal(err)
    }
    if len(records) != 1 || records[0].Type != "decision" {
        t.Fatalf("records = %#v", records)
    }
    summary, err := store.LatestSummary(context.Background(), conversationID)
    if err != nil {
        t.Fatal(err)
    }
    if summary == "" {
        t.Fatal("expected async summary")
    }
}

func TestOpenSQLiteCreatesParentDirectory(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "missing", "parent", "memory.db")
    store, err := OpenSQLite(context.Background(), "sqlite", dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()
}
