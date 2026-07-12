package memory

import (
    "context"
    "errors"
    "time"
)

type Approval struct {
    ID             string
    Scope          Scope
    ConversationID string
    Server         string
    Tool           string
    ArgumentsJSON  string
    Status         string // pending, approved, denied, expired, executed
    ExpiresAt      time.Time
    CreatedAt      time.Time
    ResolvedAt     time.Time
}

type ApprovalStore interface {
    CreateApproval(context.Context, Approval) (Approval, error)
    ResolveApproval(context.Context, string, string) (Approval, error)
}

func (e *Engine) CreateApproval(ctx context.Context, approval Approval) (Approval, error) {
    store, ok := e.store.(ApprovalStore)
    if !ok { return Approval{}, errors.New("approval storage is unavailable") }
    if HasSensitiveContent(approval.ArgumentsJSON) { return Approval{}, errors.New("tool arguments contain sensitive content") }
    return store.CreateApproval(ctx, approval)
}

func (e *Engine) ResolveApproval(ctx context.Context, id, action string) (Approval, error) {
    store, ok := e.store.(ApprovalStore)
    if !ok { return Approval{}, errors.New("approval storage is unavailable") }
    return store.ResolveApproval(ctx, id, action)
}
