package domain

import "time"

type Capabilities struct {
	Text             bool `json:"text"`
	Vision           bool `json:"vision"`
	WebSearch        bool `json:"web_search"`
	ReasoningSummary bool `json:"reasoning_summary"`
	Tools            bool `json:"tools"`
}

type CapabilitySnapshot struct {
	SupportedReasoningEfforts []string `json:"supported_reasoning_efforts,omitempty"`
	DefaultReasoningEffort    string   `json:"default_reasoning_effort,omitempty"`
	DisplayGroup              string   `json:"display_group,omitempty"`
	Routes                    []string `json:"routes,omitempty"`
	ApproachingRateLimit      *bool    `json:"approaching_rate_limit,omitempty"`
	RestrictedAccessCodename  string   `json:"restricted_access_codename,omitempty"`
}

type Model struct {
	PublicID           string             `json:"public_id"`
	UpstreamID         string             `json:"-"`
	DisplayName        string             `json:"display_name"`
	Provider           string             `json:"provider,omitempty"`
	AccountID          string             `json:"-"`
	WorkspaceID        string             `json:"-"`
	Capabilities       Capabilities       `json:"capabilities"`
	CapabilitySnapshot CapabilitySnapshot `json:"capability_snapshot"`
	Status             string             `json:"status"`
	LastVerifiedAt     time.Time          `json:"last_verified_at"`
	DiscoverySource    string             `json:"discovery_source"`
	Diagnostic         string             `json:"diagnostic,omitempty"`
}

type InferenceRequest struct {
	Model string
	Input string
}
type EventKind string

const (
	EventDelta EventKind = "delta"
	EventTool  EventKind = "tool"
	EventDone  EventKind = "done"
	EventError EventKind = "error"
)

type ToolEvent struct {
	Name           string
	Version        string
	Operation      string
	PayloadVariant string
	InputSummary   string
	OutputSummary  string
	Status         string
	ErrorCategory  string
	ErrorSummary   string
	CodeRevision   string
	ArtifactRefsJSON string
	IdempotencyKey string
}

type InferenceEvent struct {
	Kind EventKind
	Text string
	Tool *ToolEvent
	Err  error
}
