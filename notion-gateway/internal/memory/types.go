// Package memory implements durable, scope-isolated conversation continuity.
package memory

import "time"

type Scope struct {
	UserID    string
	ProjectID string
}

type Request struct {
	Scope
	ConversationID    string
	PreviousResponseID string
	ClientKey         string
	Input             string
	Model             string
}

type Turn struct {
	ID             string
	ConversationID string
	Scope          Scope
	Prompt         string
	Manifest       []Section
}

type Section struct {
	Kind            string `json:"kind"`
	Source          string `json:"source"`
	Reason          string `json:"reason"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

type Config struct {
	Enabled              bool
	MaxInjectedTokens    int
	RecentTurns          int
	SummaryTriggerTurns  int
	MaxMemoryRecords     int
}

func (c Config) withDefaults() Config {
	if c.MaxInjectedTokens <= 0 { c.MaxInjectedTokens = 12000 }
	if c.RecentTurns <= 0 { c.RecentTurns = 6 }
	if c.SummaryTriggerTurns <= 0 { c.SummaryTriggerTurns = 4 }
	if c.MaxMemoryRecords <= 0 { c.MaxMemoryRecords = 8 }
	return c
}

type MemoryRecord struct {
	ID           string
	Scope        Scope
	ConversationID string
	Type         string
	SubjectKey   string
	Content      string
	Importance   float64
	Confidence   float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
