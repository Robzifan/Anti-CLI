package protocol

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/notioncodex/gateway/internal/domain"
)

func TestStreamDecoder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantKinds []domain.EventKind
		wantText  string
		wantError error
	}{
		{name: "normal unicode", input: "{\"kind\":\"delta\",\"text\":\"Привет 🌍\"}\n{\"kind\":\"done\"}\n", wantKinds: []domain.EventKind{domain.EventDelta, domain.EventDone}, wantText: "Привет 🌍"},
		{name: "empty delta", input: "{\"kind\":\"delta\",\"text\":\"\"}\n{\"kind\":\"done\"}\n", wantKinds: []domain.EventKind{domain.EventDelta, domain.EventDone}},
		{name: "malformed", input: "{bad}\n", wantError: ErrMalformedFrame},
		{name: "unknown field", input: "{\"kind\":\"done\",\"private\":true}\n", wantError: ErrMalformedFrame},
		{name: "unknown error category", input: "{\"kind\":\"error\",\"category\":\"invented\"}\n", wantError: ErrMalformedFrame},
		{name: "done with text", input: "{\"kind\":\"done\",\"text\":\"unexpected\"}\n", wantError: ErrMalformedFrame},
		{name: "duplicate terminal", input: "{\"kind\":\"done\"}\n{\"kind\":\"done\"}\n", wantKinds: []domain.EventKind{domain.EventDone}, wantError: ErrDuplicateTerminal},
		{name: "delta after terminal", input: "{\"kind\":\"done\"}\n{\"kind\":\"delta\",\"text\":\"late\"}\n", wantKinds: []domain.EventKind{domain.EventDone}, wantError: ErrFrameAfterTerminal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decoder := NewStreamDecoder(strings.NewReader(test.input), 0)
			var kinds []domain.EventKind
			var text strings.Builder
			for {
				event, err := decoder.Next(context.Background())
				if err != nil {
					if test.wantError == nil && !errors.Is(err, io.EOF) {
						t.Fatalf("Next() error = %v", err)
					}
					if test.wantError != nil && !errors.Is(err, test.wantError) {
						t.Fatalf("Next() error = %v, want %v", err, test.wantError)
					}
					break
				}
				kinds = append(kinds, event.Kind)
				text.WriteString(event.Text)
			}
			if len(kinds) != len(test.wantKinds) {
				t.Fatalf("kinds = %v, want %v", kinds, test.wantKinds)
			}
			for index := range kinds {
				if kinds[index] != test.wantKinds[index] {
					t.Fatalf("kinds = %v, want %v", kinds, test.wantKinds)
				}
			}
			if text.String() != test.wantText {
				t.Fatalf("text = %q, want %q", text.String(), test.wantText)
			}
		})
	}
}

func TestStreamDecoderCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewStreamDecoder(strings.NewReader("{\"kind\":\"done\"}\n"), 0).Next(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Next() error = %v, want context.Canceled", err)
	}
}

func TestStreamDecoderErrorCategory(t *testing.T) {
	t.Parallel()
	decoder := NewStreamDecoder(strings.NewReader("{\"kind\":\"error\",\"category\":\"rate_limited\",\"code\":\"safe_code\"}\n"), 0)
	event, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var upstream *UpstreamError
	if event.Kind != domain.EventError || !errors.As(event.Err, &upstream) || upstream.Category != ErrorRateLimited {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func FuzzStreamDecoder(f *testing.F) {
	f.Add([]byte("{\"kind\":\"delta\",\"text\":\"hello\"}\n{\"kind\":\"done\"}\n"))
	f.Add([]byte("{bad}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > DefaultMaxFrameBytes*2 {
			t.Skip()
		}
		decoder := NewStreamDecoder(strings.NewReader(string(data)), DefaultMaxFrameBytes)
		for range 32 {
			_, err := decoder.Next(context.Background())
			if err != nil {
				return
			}
		}
	})
}

func TestGoldenNormalizedStreams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		wantKinds []domain.EventKind
	}{
		{name: "basic.ndjson", wantKinds: []domain.EventKind{domain.EventDelta, domain.EventDelta, domain.EventDone}},
		{name: "error.ndjson", wantKinds: []domain.EventKind{domain.EventError}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file, err := os.Open(filepath.Join("..", "..", "..", "testdata", "notion", "normalized", test.name))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			decoder := NewStreamDecoder(file, 0)
			var got []domain.EventKind
			for {
				event, err := decoder.Next(context.Background())
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, event.Kind)
			}
			if len(got) != len(test.wantKinds) {
				t.Fatalf("kinds = %v, want %v", got, test.wantKinds)
			}
			for index := range got {
				if got[index] != test.wantKinds[index] {
					t.Fatalf("kinds = %v, want %v", got, test.wantKinds)
				}
			}
		})
	}
}

func TestStreamDecoderFrameLimit(t *testing.T) {
	t.Parallel()
	input := "{\"kind\":\"delta\",\"text\":\"" + strings.Repeat("x", 1024) + "\"}\n"
	_, err := NewStreamDecoder(strings.NewReader(input), 64).Next(context.Background())
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("Next() error = %v, want %v", err, ErrFrameTooLarge)
	}
}
