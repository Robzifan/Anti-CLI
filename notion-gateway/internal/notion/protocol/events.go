// Package protocol contains normalized envelopes only. Concrete, observed
// upstream mappings must remain in a versioned adapter and fixture tests.
package protocol

import "encoding/json"

// Envelope preserves unknown sanitized payloads until an observed protocol
// version can be mapped without inventing fields.
type Envelope struct {
	SchemaVersion int               `json:"schema_version"`
	Direction     string            `json:"direction"`
	Transport     string            `json:"transport"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Payload       json.RawMessage   `json:"payload"`
}
