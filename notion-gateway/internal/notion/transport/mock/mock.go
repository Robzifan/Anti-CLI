package mock

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
	"strings"

	"github.com/notioncodex/gateway/internal/domain"
	"github.com/notioncodex/gateway/internal/ports"
)

const ModelID = "mock/echo"

type Transport struct {
	Delay time.Duration
	ToolEvents []domain.ToolEvent
}

func (t *Transport) Discover(_ context.Context, accountID, workspaceID string) ([]domain.Model, error) {
	return []domain.Model{{PublicID: ModelID, UpstreamID: "mock-echo", DisplayName: "Mock Echo", Provider: "mock", AccountID: accountID, WorkspaceID: workspaceID, Capabilities: domain.Capabilities{Text: true}, Status: "available", LastVerifiedAt: time.Now().UTC(), DiscoverySource: "mock_transport"}}, nil
}
func (t *Transport) Infer(ctx context.Context, req domain.InferenceRequest) (ports.EventStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Model != ModelID && req.Model != "notion-auto" {
		return nil, errors.New("model not available")
	}
	events := make([]domain.InferenceEvent, 0, len(t.ToolEvents)+2)
	for index := range t.ToolEvents {
		event := t.ToolEvents[index]
		events = append(events, domain.InferenceEvent{Kind: domain.EventTool, Tool: &event})
	}
	events = append(events, domain.InferenceEvent{Kind: domain.EventDelta, Text: echoInput(req.Input)}, domain.InferenceEvent{Kind: domain.EventDone})
	return &stream{events: events, delay: t.Delay}, nil
}


// echoInput makes the mock transport exercise the same response lifecycle as a
// model without persisting the gateway's injected context as assistant output.
// A real model receives the complete assembled prompt but returns an answer;
// the mock therefore echoes only the current user request when that envelope is present.
func echoInput(input string) string {
	const open = "<current_user_request>"
	const close = "</current_user_request>"
	start := strings.Index(input, open)
	if start < 0 {
		return input
	}
	start += len(open)
	end := strings.Index(input[start:], close)
	if end < 0 {
		return input
	}
	return strings.TrimSpace(input[start : start+end])
}

type stream struct {
	mu     sync.Mutex
	events []domain.InferenceEvent
	index  int
	delay  time.Duration
	closed bool
}

func (s *stream) Recv(ctx context.Context) (domain.InferenceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.index >= len(s.events) {
		return domain.InferenceEvent{}, io.EOF
	}
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return domain.InferenceEvent{}, ctx.Err()
		case <-time.After(s.delay):
		}
	}
	e := s.events[s.index]
	s.index++
	return e, nil
}
func (s *stream) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true; return nil }
