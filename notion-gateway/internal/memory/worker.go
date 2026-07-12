package memory

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"
)

const consolidateEventType = "memory.consolidate"
const consolidateToolEventType = "tool.consolidate"

type outboxEvent struct {
    ID       string
    Type     string
    Payload  string
    Attempts int
}

// Worker processes durable derived-memory work outside the HTTP/streaming path.
type Worker struct {
    store    *SQLiteStore
    interval time.Duration
}

func NewWorker(store *SQLiteStore) *Worker {
    return &Worker{store: store, interval: 250 * time.Millisecond}
}

func (w *Worker) Run(ctx context.Context) {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()
    for {
        for {
            processed, err := w.RunOnce(ctx)
            if err != nil || !processed {
                break
            }
        }
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
        }
    }
}

// RunOnce is exported for deterministic tests and bounded shutdown draining.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
    event, err := w.store.claimOutbox(ctx)
    if err != nil || event == nil {
        return false, err
    }
    if err := w.handle(ctx, *event); err != nil {
        _ = w.store.retryOutbox(ctx, event.ID, event.Attempts, Sanitize(err.Error()))
        return true, err
    }
    if err := w.store.completeOutbox(ctx, event.ID); err != nil {
        return true, err
    }
    return true, nil
}

func (w *Worker) handle(ctx context.Context, event outboxEvent) error {
    switch event.Type {
    case consolidateEventType:
        var payload struct {
            ConversationID string `json:"conversation_id"`
        }
        if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
            return fmt.Errorf("decode outbox payload: %w", err)
        }
        if payload.ConversationID == "" {
            return errors.New("outbox payload has no conversation_id")
        }
        return w.store.consolidateConversation(ctx, payload.ConversationID)
    case consolidateToolEventType:
        var payload struct {
            ExecutionID string `json:"execution_id"`
        }
        if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
            return fmt.Errorf("decode outbox payload: %w", err)
        }
        if payload.ExecutionID == "" {
            return errors.New("outbox payload has no execution_id")
        }
        return w.store.consolidateToolExecution(ctx, payload.ExecutionID)
    default:
        return fmt.Errorf("unsupported outbox event %q", event.Type)
    }
}
