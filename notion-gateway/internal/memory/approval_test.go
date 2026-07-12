package memory

import (
    "context"
    "path/filepath"
    "testing"

    _ "modernc.org/sqlite"
)

func TestApprovalOneTimeResolution(t *testing.T) {
    store, err := OpenSQLite(context.Background(), "sqlite", filepath.Join(t.TempDir(), "approval.db"))
    if err != nil { t.Fatal(err) }
    defer store.Close()
    engine := New(store, Config{Enabled:true})
    approval, err := engine.CreateApproval(context.Background(), Approval{
        Scope: Scope{UserID:"local", ProjectID:"project"}, ConversationID:"conversation",
        Server:"project-filesystem", Tool:"write_file", ArgumentsJSON:`{"path":"note.txt","content":"hello"}`,
    })
    if err != nil { t.Fatal(err) }
    if approval.Status != "pending" { t.Fatalf("status=%q", approval.Status) }
    resolved, err := engine.ResolveApproval(context.Background(), approval.ID, "approved")
    if err != nil { t.Fatal(err) }
    if resolved.Status != "approved" { t.Fatalf("status=%q", resolved.Status) }
    if _, err := engine.ResolveApproval(context.Background(), approval.ID, "approved"); err == nil { t.Fatal("expected one-time approval enforcement") }
}

func TestApprovalRejectsSensitiveArguments(t *testing.T) {
    store, err := OpenSQLite(context.Background(), "sqlite", filepath.Join(t.TempDir(), "approval.db"))
    if err != nil { t.Fatal(err) }
    defer store.Close()
    engine := New(store, Config{Enabled:true})
    if _, err := engine.CreateApproval(context.Background(), Approval{Scope:Scope{UserID:"local"}, Server:"s", Tool:"write", ArgumentsJSON:"token_v2 abcdefghijklmnopqrstuvwxyz"}); err == nil {
        t.Fatal("expected sensitive arguments rejection")
    }
}
