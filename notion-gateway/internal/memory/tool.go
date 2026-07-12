package memory

import (
    "context"
    "time"
)

// ToolExecution is evidence of an operation that actually ran; it is not an LLM claim.
type ToolExecution struct {
    ID               string
    Scope            Scope
    ConversationID   string
    TurnID           string
    ToolName         string
    ToolVersion      string
    Operation        string
    PayloadVariant   string
    InputFingerprint string
    InputSummary     string
    OutputSummary    string
    Status           string // succeeded, failed, cancelled
    ErrorCategory    string
    ErrorSummary     string
    CodeRevision     string
    ArtifactRefsJSON string
    IdempotencyKey   string
    StartedAt        time.Time
    FinishedAt       time.Time
}

// ToolStore is optional so conversation-only stores remain valid test doubles.
type ToolStore interface {
    RecordToolExecution(context.Context, ToolExecution) (ToolExecution, error)
    SearchToolExecutions(context.Context, Scope, string, int) ([]ToolExecution, error)
}
