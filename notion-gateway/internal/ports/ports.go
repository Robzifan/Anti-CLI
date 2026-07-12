package ports

import (
	"context"
	"io"
	"time"

	"github.com/notioncodex/gateway/internal/domain"
)

type EventStream interface {
	Recv(context.Context) (domain.InferenceEvent, error)
	Close() error
}
type InferenceTransport interface {
	Infer(context.Context, domain.InferenceRequest) (EventStream, error)
}
type ModelDiscoverer interface {
	Discover(context.Context, string, string) ([]domain.Model, error)
}
type AccountStore interface {
	ListAccounts(context.Context) ([]Account, error)
}
type ConversationStore interface {
	SaveConversation(context.Context, Conversation) error
}
type MemoryStore interface {
	Search(context.Context, string, int) ([]Memory, error)
}
type SecretStore interface {
	Get(context.Context, string) ([]byte, error)
	Put(context.Context, string, []byte) error
}
type ContextBuilder interface {
	Build(context.Context, ContextRequest) (Context, error)
}

type Account struct {
	ID, WorkspaceID string
	CooldownUntil   time.Time
}
type Conversation struct {
	ID        string
	CreatedAt time.Time
}
type Memory struct{ ID, Scope, Content string }
type ContextRequest struct {
	ProjectPath string
	TokenBudget int
}
type Context struct {
	Text     string
	Manifest []ContextSection
}
type ContextSection struct {
	Kind, Source    string
	EstimatedTokens int
}

var _ io.Closer = (EventStream)(nil)
