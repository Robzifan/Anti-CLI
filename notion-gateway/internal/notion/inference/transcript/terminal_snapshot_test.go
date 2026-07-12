package transcript

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/notioncodex/gateway/internal/domain"
)

func TestDecoderTerminalSnapshotWithText(t *testing.T) {
	input := `{"type":"agent-turn-full-record-map","step":{"type":"agent-inference","value":[{"type":"text","content":"sol-live-test"}],"finishedAt":123}}`
	decoder := NewDecoder(strings.NewReader(input))
	delta, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	done, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if delta.Kind != domain.EventDelta || delta.Text != "sol-live-test" || done.Kind != domain.EventDone {
		t.Fatalf("events = %#v %#v", delta, done)
	}
	_, err = decoder.Next(context.Background())
	if !errors.Is(err, io.EOF) { t.Fatalf("error = %v", err) }
}
