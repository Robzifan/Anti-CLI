package transcript

import (
    "context"
    "strings"
    "testing"

    "github.com/notioncodex/gateway/internal/domain"
)

// Regression fixture derived from a redacted live stream: patch-start already
// contains /s/0, so later append operations must begin at /s/1.
func TestDecoderPatchStartSeedsStreamOffsets(t *testing.T) {
    input := strings.Join([]string{
        `{"type":"patch-start","data":{"s":[{"type":"agent-instruction-state"}]}}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-turn-full-record-map"}}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-inference","value":[{"type":"text","content":"stre"}]}}]}`,
        `{"type":"patch","v":[{"o":"x","p":"/s/5/value/0/content","v":"aming live works."}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/5/finishedAt","v":123}]}`,
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
