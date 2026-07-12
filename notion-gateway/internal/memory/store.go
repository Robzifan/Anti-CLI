package memory

import (
	"context"
	"time"
)

// Store is deliberately small: SQL/FTS, Postgres and test doubles share these semantics.
type Store interface {
	ResolveConversation(context.Context, Scope, string, string, string) (string, error)
	AppendUser(context.Context, string, string, string) (string, error)
	CompleteTurn(context.Context, string, string, string, string) error
	AbortTurn(context.Context, string, string) error
	LatestSummary(context.Context, string) (string, error)
	RecentMessages(context.Context, string, int) ([]Message, error)
	SearchMemories(context.Context, Scope, string, int) ([]MemoryRecord, error)
	CountTurns(context.Context, string) (int, error)
	UpsertSummary(context.Context, string, string, int) error
}

type Message struct {
	Role      string
	Content   string
	CreatedAt time.Time
}
