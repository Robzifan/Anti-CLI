package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Engine struct { store Store; cfg Config }

func New(store Store, cfg Config) *Engine { return &Engine{store: store, cfg: cfg.withDefaults()} }

func (e *Engine) Start(ctx context.Context, req Request) (Turn, error) {
	if !e.cfg.Enabled { return Turn{Prompt: req.Input}, nil }
	input := Sanitize(req.Input)
	conversationID, err := e.store.ResolveConversation(ctx, req.Scope, req.ConversationID, req.PreviousResponseID, req.ClientKey)
	if err != nil { return Turn{}, fmt.Errorf("resolve conversation: %w", err) }
	turnID, err := e.store.AppendUser(ctx, conversationID, input, req.Model)
	if err != nil { return Turn{}, fmt.Errorf("append user turn: %w", err) }

	budget := e.cfg.MaxInjectedTokens
	sections := make([]Section, 0, 4)
	parts := make([]string, 0, 5)
	if summary, err := e.store.LatestSummary(ctx, conversationID); err == nil && summary != "" {
		chunk := trimTokens(summary, budget/6)
		parts = append(parts, "<conversation_summary>\n"+chunk+"\n</conversation_summary>")
		sections = append(sections, Section{Kind: "summary", Source: "conversation", Reason: "continuity", EstimatedTokens: estimateTokens(chunk)})
		budget -= estimateTokens(chunk)
	}
	if messages, err := e.store.RecentMessages(ctx, conversationID, e.cfg.RecentTurns*2); err == nil {
		chunk := formatMessages(messages, budget/3)
		if chunk != "" {
			parts = append(parts, "<recent_turns>\n"+chunk+"\n</recent_turns>")
			sections = append(sections, Section{Kind: "recent_turns", Source: "conversation", Reason: "latest verified turns", EstimatedTokens: estimateTokens(chunk)})
			budget -= estimateTokens(chunk)
		}
	}
	if records, err := e.store.SearchMemories(ctx, req.Scope, input, e.cfg.MaxMemoryRecords); err == nil {
		chunk := formatMemories(records, budget/3)
		if chunk != "" {
			parts = append(parts, "<retrieved_memory>\n"+chunk+"\n</retrieved_memory>")
			sections = append(sections, Section{Kind: "memory", Source: "scoped_retrieval", Reason: "lexical relevance + scope", EstimatedTokens: estimateTokens(chunk)})
			budget -= estimateTokens(chunk)
		}
	}
	if tools, ok := e.store.(ToolStore); ok {
		if executions, err := tools.SearchToolExecutions(ctx, req.Scope, input, 4); err == nil {
			chunk := formatToolHistory(executions, budget/5)
			if chunk != "" {
				parts = append(parts, "<verified_tool_history>\n"+chunk+"\n</verified_tool_history>")
				sections = append(sections, Section{Kind: "tool_history", Source: "tool_executions", Reason: "verified scoped tool outcome", EstimatedTokens: estimateTokens(chunk)})
			}
		}
	}
	parts = append(parts, "<current_user_request>\n"+input+"\n</current_user_request>")
	sections = append(sections, Section{Kind: "current_request", Source: "request", Reason: "always include", EstimatedTokens: estimateTokens(input)})
	return Turn{ID: turnID, ConversationID: conversationID, Scope: req.Scope, Prompt: strings.Join(parts, "\n\n"), Manifest: sections}, nil
}

func (e *Engine) Complete(ctx context.Context, turn Turn, assistant, responseID string) error {
	if !e.cfg.Enabled || turn.ID == "" { return nil }
	assistant = Sanitize(assistant)
	// Completion is intentionally small and transactional: it persists the turn
	// and queues derived work. Summaries, extraction, and FTS indexing run later.
	return e.store.CompleteTurn(ctx, turn.ID, assistant, responseID, "completed")
}

func (e *Engine) Abort(ctx context.Context, turn Turn, reason string) { if e.cfg.Enabled && turn.ID != "" { _ = e.store.AbortTurn(ctx, turn.ID, Sanitize(reason)) } }

func extractCandidates(conversationID, assistant string) []MemoryRecord {
	if containsSecret(assistant) { return nil }
	var out []MemoryRecord
	for _, line := range strings.Split(assistant, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		lower := strings.ToLower(line)
		kind := ""
		switch {
		case strings.HasPrefix(lower, "decision:"), strings.HasPrefix(lower, "решение:"):
			kind = "decision"
		case strings.HasPrefix(lower, "root cause:"), strings.HasPrefix(lower, "причина:"):
			kind = "bug"
		case strings.HasPrefix(lower, "fixed by:"), strings.HasPrefix(lower, "исправлено:"):
			kind = "tool_outcome"
		case strings.HasPrefix(lower, "open loop:"), strings.HasPrefix(lower, "открытый вопрос:"):
			kind = "open_loop"
		}
		if kind == "" || len([]rune(line)) < 16 || len([]rune(line)) > 800 { continue }
		out = append(out, MemoryRecord{ID: newID(), ConversationID: conversationID, Type: kind, SubjectKey: kind+":"+strings.ToLower(line[:min(80, len(line))]), Content: line, Importance: 0.8, Confidence: 0.75, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
	}
	return out
}

func deterministicSummary(messages []Message) string {
	if len(messages) == 0 { return "" }
	var items []string
	for _, message := range messages {
		text := strings.TrimSpace(message.Content)
		if text == "" { continue }
		items = append(items, strings.ToUpper(message.Role[:1])+message.Role[1:]+": "+trimTokens(text, 110))
	}
	if len(items) == 0 { return "" }
	return "Current conversation state (sanitized):\n- " + strings.Join(items, "\n- ")
}

func formatMessages(messages []Message, max int) string {
	if len(messages) == 0 { return "" }
	var lines []string
	for _, m := range messages { lines = append(lines, strings.ToUpper(m.Role[:1])+m.Role[1:]+": "+m.Content) }
	return trimTokens(strings.Join(lines, "\n"), max)
}
func formatMemories(records []MemoryRecord, max int) string {
	sort.SliceStable(records, func(i, j int) bool { return records[i].Importance*records[i].Confidence > records[j].Importance*records[j].Confidence })
	var lines []string
	for _, r := range records { lines = append(lines, fmt.Sprintf("- [%s; confidence %.2f] %s", r.Type, r.Confidence, r.Content)) }
	return trimTokens(strings.Join(lines, "\n"), max)
}
func estimateTokens(s string) int { return (len([]rune(s))+3)/4 }
func trimTokens(s string, max int) string { if estimateTokens(s) <= max { return s }; r := []rune(s); n := max*4; if n > len(r) { n = len(r) }; return string(r[:n])+"…" }
func min(a,b int) int { if a < b { return a }; return b }
func newID() string { var b [16]byte; if _, err := rand.Read(b[:]); err != nil { return fmt.Sprintf("fallback-%d", time.Now().UnixNano()) }; return hex.EncodeToString(b[:]) }

// RecordToolExecution persists only sanitized, evidence-backed tool outcomes.
func (e *Engine) RecordToolExecution(ctx context.Context, execution ToolExecution) (ToolExecution, error) {
	tools, ok := e.store.(ToolStore)
	if !ok { return ToolExecution{}, fmt.Errorf("tool-aware memory is unavailable for this store") }
	execution.InputSummary = Sanitize(execution.InputSummary)
	execution.OutputSummary = Sanitize(execution.OutputSummary)
	execution.ErrorSummary = Sanitize(execution.ErrorSummary)
	return tools.RecordToolExecution(ctx, execution)
}

func formatToolHistory(executions []ToolExecution, max int) string {
	var lines []string
	for _, execution := range executions {
		detail := execution.OutputSummary
		if execution.Status != "succeeded" { detail = execution.ErrorSummary }
		variant := ""
		if execution.PayloadVariant != "" { variant = "; variant=" + execution.PayloadVariant }
		lines = append(lines, fmt.Sprintf("- [%s%s; %s] %s", execution.ToolName, variant, execution.Status, detail))
	}
	return trimTokens(strings.Join(lines, "\n"), max)
}
