package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/notioncodex/gateway/internal/domain"
)

const DefaultMaxFrameBytes = 256 << 10

var (
	ErrMalformedFrame     = errors.New("malformed normalized frame")
	ErrFrameTooLarge      = errors.New("normalized frame exceeds limit")
	ErrDuplicateTerminal  = errors.New("duplicate terminal frame")
	ErrFrameAfterTerminal = errors.New("frame received after terminal frame")
)

type ErrorCategory string

const (
	ErrorUnknown          ErrorCategory = "unknown"
	ErrorAuthentication   ErrorCategory = "authentication"
	ErrorPermission       ErrorCategory = "permission"
	ErrorRateLimited      ErrorCategory = "rate_limited"
	ErrorModelUnavailable ErrorCategory = "model_unavailable"
	ErrorProtocolMismatch ErrorCategory = "protocol_mismatch"
	ErrorUpstream         ErrorCategory = "upstream"
)

// UpstreamError is adapter-neutral. A concrete observed adapter is responsible
// for assigning a category; this package does not infer categories from private
// status codes or payload fields.
type UpstreamError struct {
	Category    ErrorCategory
	SafeCode    string
	SafeMessage string
}

func (e *UpstreamError) Error() string {
	if e.SafeCode == "" {
		return string(e.Category)
	}
	return string(e.Category) + ": " + e.SafeCode
}

type normalizedFrame struct {
	Kind     string        `json:"kind"`
	Text     string        `json:"text,omitempty"`
	Category ErrorCategory `json:"category,omitempty"`
	Code     string        `json:"code,omitempty"`
	Message  string        `json:"message,omitempty"`
}

// StreamDecoder decodes the project's normalized NDJSON research format.
// It is not an implementation or description of any Notion wire protocol.
type StreamDecoder struct {
	scanner  *bufio.Scanner
	terminal bool
}

func NewStreamDecoder(r io.Reader, maxFrameBytes int) *StreamDecoder {
	if maxFrameBytes <= 0 {
		maxFrameBytes = DefaultMaxFrameBytes
	}

	initialBufferBytes := 4096
	if maxFrameBytes < initialBufferBytes {
		initialBufferBytes = maxFrameBytes
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, initialBufferBytes), maxFrameBytes)

	return &StreamDecoder{scanner: scanner}
}

func (d *StreamDecoder) Next(
	ctx context.Context,
) (domain.InferenceEvent, error) {
	if err := ctx.Err(); err != nil {
		return domain.InferenceEvent{}, err
	}

	for d.scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return domain.InferenceEvent{}, err
		}

		line := strings.TrimSpace(d.scanner.Text())
		if line == "" {
			continue
		}

		var frame normalizedFrame

		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&frame); err != nil {
			return domain.InferenceEvent{},
				fmt.Errorf("%w: %v", ErrMalformedFrame, err)
		}

		if err := requireFrameEOF(decoder); err != nil {
			return domain.InferenceEvent{}, err
		}

		if d.terminal {
			if frame.Kind == "done" || frame.Kind == "error" {
				return domain.InferenceEvent{}, ErrDuplicateTerminal
			}
			return domain.InferenceEvent{}, ErrFrameAfterTerminal
		}

		switch frame.Kind {
		case "delta":
			if frame.Category != "" ||
				frame.Code != "" ||
				frame.Message != "" {
				return domain.InferenceEvent{},
					fmt.Errorf(
						"%w: delta contains error fields",
						ErrMalformedFrame,
					)
			}

			return domain.InferenceEvent{
				Kind: domain.EventDelta,
				Text: frame.Text,
			}, nil

		case "done":
			if frame.Text != "" ||
				frame.Category != "" ||
				frame.Code != "" ||
				frame.Message != "" {
				return domain.InferenceEvent{},
					fmt.Errorf(
						"%w: done contains data fields",
						ErrMalformedFrame,
					)
			}

			d.terminal = true

			return domain.InferenceEvent{
				Kind: domain.EventDone,
			}, nil

		case "error":
			if frame.Text != "" {
				return domain.InferenceEvent{},
					fmt.Errorf(
						"%w: error contains text",
						ErrMalformedFrame,
					)
			}

			category := frame.Category
			if category == "" {
				category = ErrorUnknown
			}

			if !validErrorCategory(category) {
				return domain.InferenceEvent{},
					fmt.Errorf(
						"%w: unsupported error category %q",
						ErrMalformedFrame,
						category,
					)
			}

			d.terminal = true

			return domain.InferenceEvent{
				Kind: domain.EventError,
				Err: &UpstreamError{
					Category:    category,
					SafeCode:    frame.Code,
					SafeMessage: frame.Message,
				},
			}, nil

		default:
			return domain.InferenceEvent{},
				fmt.Errorf(
					"%w: unsupported kind %q",
					ErrMalformedFrame,
					frame.Kind,
				)
		}
	}

	if err := d.scanner.Err(); err != nil {
		if strings.Contains(
			strings.ToLower(err.Error()),
			"token too long",
		) {
			return domain.InferenceEvent{}, ErrFrameTooLarge
		}

		return domain.InferenceEvent{},
			fmt.Errorf("read normalized frame: %w", err)
	}

	return domain.InferenceEvent{}, io.EOF
}

func requireFrameEOF(decoder *json.Decoder) error {
	var trailing any

	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	}

	return ErrMalformedFrame
}

func validErrorCategory(category ErrorCategory) bool {
	switch category {
	case ErrorUnknown,
		ErrorAuthentication,
		ErrorPermission,
		ErrorRateLimited,
		ErrorModelUnavailable,
		ErrorProtocolMismatch,
		ErrorUpstream:
		return true
	default:
		return false
	}
}
