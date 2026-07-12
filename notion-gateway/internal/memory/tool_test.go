package memory

import (
    "context"
    "path/filepath"
    "strings"
    "testing"

    _ "modernc.org/sqlite"
)

func TestToolExecutionIsScopedRetrievedAndConsolidated(t *testing.T) {
    store, err := OpenSQLite(context.Background(), "sqlite", filepath.Join(t.TempDir(), "tool.db"))
    if err != nil { t.Fatal(err) }
    defer store.Close()

    scope := Scope{UserID: "user-a", ProjectID: "project-a"}
    execution, err := store.RecordToolExecution(context.Background(), ToolExecution{
        Scope: scope, ConversationID: "conversation-a", ToolName: "transcript.decoder",
        ToolVersion: "v2", Operation: "decode", PayloadVariant: "minimalArrayUser",
        InputSummary: "array framed transcript", OutputSummary: "decoder accepted array framing and emitted terminal event",
        Status: "succeeded", CodeRevision: "abc123", IdempotencyKey: "decoder-array-success",
    })
    if err != nil { t.Fatal(err) }
    if execution.InputFingerprint == "" { t.Fatal("expected input fingerprint") }

    processed, err := NewWorker(store).RunOnce(context.Background())
    if err != nil || !processed { t.Fatalf("processed=%v err=%v", processed, err) }
    tools, err := store.SearchToolExecutions(context.Background(), scope, "array decoder", 5)
    if err != nil { t.Fatal(err) }
    if len(tools) != 1 || tools[0].PayloadVariant != "minimalArrayUser" { t.Fatalf("tools=%#v", tools) }
    memories, err := store.SearchMemories(context.Background(), scope, "decoder array", 5)
    if err != nil { t.Fatal(err) }
    if len(memories) != 1 || !strings.Contains(memories[0].Content, "minimalArrayUser") { t.Fatalf("memories=%#v", memories) }

    otherProject, err := store.SearchToolExecutions(context.Background(), Scope{UserID: "user-a", ProjectID: "project-b"}, "array", 5)
    if err != nil { t.Fatal(err) }
    if len(otherProject) != 0 { t.Fatalf("cross-project tool leakage: %#v", otherProject) }
}
