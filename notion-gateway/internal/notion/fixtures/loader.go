// Package fixtures loads locally sanitized protocol research fixtures.
package fixtures

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/notioncodex/gateway/internal/notion/protocol"
)

const MaxFixtureBytes int64 = 1 << 20

var (
	errTooLarge = errors.New("fixture exceeds 1 MiB limit")
	bearerValue = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{8,}`)
	jwtValue    = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]{5,}\b`)
)

// Load reads one strict JSON document, validates its envelope, and rejects
// credential-shaped keys or values. It does not rewrite or persist the source.
func Load(r io.Reader) (protocol.Envelope, error) {
	limited := io.LimitReader(r, MaxFixtureBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("read fixture: %w", err)
	}
	if int64(len(raw)) > MaxFixtureBytes {
		return protocol.Envelope{}, errTooLarge
	}

	var untrusted any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&untrusted); err != nil {
		return protocol.Envelope{}, fmt.Errorf("decode fixture: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return protocol.Envelope{}, err
	}
	if err := rejectSecrets(untrusted, "$"); err != nil {
		return protocol.Envelope{}, err
	}

	var envelope protocol.Envelope
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&envelope); err != nil {
		return protocol.Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if err := Validate(envelope); err != nil {
		return protocol.Envelope{}, err
	}
	return envelope, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing data: %w", err)
	}
	return errors.New("fixture must contain exactly one JSON document")
}

// Validate checks only the project-owned fixture envelope. It deliberately
// makes no assertion about private upstream payload fields.
func Validate(envelope protocol.Envelope) error {
	if envelope.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", envelope.SchemaVersion)
	}
	switch envelope.Direction {
	case "request", "response", "event":
	default:
		return fmt.Errorf("unsupported direction %q", envelope.Direction)
	}
	if strings.TrimSpace(envelope.Transport) == "" {
		return errors.New("transport is required")
	}
	if len(envelope.Payload) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Payload), []byte("null")) {
		return errors.New("payload is required")
	}
	return nil
}

func rejectSecrets(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if forbiddenKey(key) {
				return fmt.Errorf("fixture rejected: forbidden key at %s.%s", path, key)
			}
			if err := rejectSecrets(child, path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := rejectSecrets(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case string:
		if bearerValue.MatchString(typed) || jwtValue.MatchString(typed) {
			return fmt.Errorf("fixture rejected: credential-shaped value at %s", path)
		}
	}
	return nil
}

func forbiddenKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	for _, marker := range []string{"authorization", "cookie", "set_cookie", "token_v2", "api_key", "access_token", "refresh_token", "client_secret"} {
		if normalized == marker || strings.HasSuffix(normalized, "_"+marker) {
			return true
		}
	}
	return false
}
