package transcript

import (
    "context"
    "strings"
    "testing"

    "github.com/notioncodex/gateway/internal/domain"
)

func TestDecoderDoesNotTerminateOnPartialFullRecordSnapshot(t *testing.T) {
    input := strings.Join([]string{
        `{"type":"agent-turn-full-record-map","step":{"type":"agent-inference","value":[{"type":"text","content":"stre"}]}}`,
        `{"recordMap":{"message":{"step":{"type":"agent-inference","value":[{"type":"text","content":"streaming live works."}]}}}}`,
        `{"type":"agent-turn-full-record-map"}`,
    }, "\n")
    decoder := NewDecoder(strings.NewReader(input))
    first, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    second, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    done, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    if first.Kind != domain.EventDelta || first.Text != "stre" { t.Fatalf("first=%#v", first) }
    if second.Kind != domain.EventDelta || second.Text != "aming live works." { t.Fatalf("second=%#v", second) }
    if done.Kind != domain.EventDone { t.Fatalf("done=%#v", done) }
}
