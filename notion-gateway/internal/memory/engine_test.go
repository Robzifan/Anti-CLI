package memory

import (
	"context"
	"strings"
	"testing"
)

type fakeStore struct {
	conversation string
	messages     []Message
	summary      string
	records      []MemoryRecord
	completed    string
}

func (f *fakeStore) ResolveConversation(_ context.Context, _ Scope, explicit, _, _ string) (string, error) {
	if explicit != "" { return explicit, nil }
	return f.conversation, nil
}
func (f *fakeStore) AppendUser(context.Context, string, string, string) (string, error) { return "turn-1", nil }
func (f *fakeStore) CompleteTurn(_ context.Context, _ string, assistant, _, _ string) error { f.completed = assistant; return nil }
func (f *fakeStore) AbortTurn(context.Context, string, string) error { return nil }
func (f *fakeStore) LatestSummary(context.Context, string) (string, error) { return f.summary, nil }
func (f *fakeStore) RecentMessages(context.Context, string, int) ([]Message, error) { return f.messages, nil }
func (f *fakeStore) SearchMemories(context.Context, Scope, string, int) ([]MemoryRecord, error) { return f.records, nil }
func (f *fakeStore) CountTurns(context.Context, string) (int, error) { return 1, nil }
func (f *fakeStore) UpsertSummary(context.Context, string, string, int) error { return nil }

func TestStartAssemblesScopedContext(t *testing.T) {
	store := &fakeStore{conversation: "c1", summary: "Current goal: diagnose stream"}
	store.messages = append(store.messages, Message{Role: "user", Content: "Prior request"})
	store.records = append(store.records, MemoryRecord{Type: "decision", Content: "Use local continuity", Importance: .9, Confidence: .9})
	engine := New(store, Config{Enabled: true, MaxInjectedTokens: 1000})
	turn, err := engine.Start(context.Background(), Request{Scope: Scope{UserID: "u", ProjectID: "p"}, Input: "Continue decoder work", Model: "mock"})
	if err != nil { t.Fatal(err) }
	for _, want := range []string{"<conversation_summary>", "<recent_turns>", "<retrieved_memory>", "<current_user_request>"} {
		if !strings.Contains(turn.Prompt, want) { t.Fatalf("missing %s in %s", want, turn.Prompt) }
	}
}

func TestSanitizeBeforePromptAndPersistence(t *testing.T) {
	store := &fakeStore{conversation: "c1"}
	engine := New(store, Config{Enabled: true})
	turn, err := engine.Start(context.Background(), Request{Scope: Scope{UserID: "u"}, Input: "token_v2 abcdefghijklmnopqrstuvwxyz", Model: "mock"})
	if err != nil { t.Fatal(err) }
	if strings.Contains(turn.Prompt, "abcdefghijklmnopqrstuvwxyz") { t.Fatal("secret reached prompt") }
	if err := engine.Complete(context.Background(), turn, "done", "r1"); err != nil { t.Fatal(err) }
	if store.completed != "done" { t.Fatal("completion not persisted") }
}
