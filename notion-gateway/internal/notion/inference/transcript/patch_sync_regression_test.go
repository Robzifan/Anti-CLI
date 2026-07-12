package transcript

import (
    "context"
    "strings"
    "testing"

    "github.com/notioncodex/gateway/internal/domain"
)

// Regression fixture derived from a redacted live memory response. patch-sync
// introduces /s/10 before the visible assistant text receives x patches.
func TestDecoderPatchSyncSeedsFinalInferenceOffset(t *testing.T) {
    input := strings.Join([]string{
        `{"type":"patch-start","data":{"s":[{"type":"agent-instruction-state"}]}}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-turn-full-record-map"}},{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}},{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}},{"o":"a","p":"/s/-","v":{"type":"agent-tool-result"}},{"o":"a","p":"/s/-","v":{"type":"title"}},{"o":"a","p":"/s/-","v":{"type":"agent-inference","value":[{"type":"thinking","content":"hidden"}]}}]}`,
        `{"type":"patch-sync","data":{"s":[{"type":"agent-instruction-state"},{"type":"agent-turn-full-record-map"},{"type":"agent-tool-result"},{"type":"agent-tool-result"},{"type":"agent-tool-result"},{"type":"title"},{"type":"agent-inference","value":[{"type":"thinking","content":"hidden"}]},{"type":"agent-tool-result"},{"type":"agent-inference","value":[{"type":"thinking","content":"hidden"}]},{"type":"agent-tool-result"},{"type":"agent-inference","value":[{"type":"text","content":"You"}]}]}}`,
        `{"type":"patch","v":[{"o":"x","p":"/s/10/value/0/content","v":" asked"}]}`,
        `{"type":"patch","v":[{"o":"a","p":"/s/10/finishedAt","v":123}]}`,
    }, "\n")
    decoder := NewDecoder(strings.NewReader(input))
    first, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    second, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    done, err := decoder.Next(context.Background())
    if err != nil { t.Fatal(err) }
    if first.Kind != domain.EventDelta || first.Text != "You" { t.Fatalf("first=%#v", first) }
    if second.Kind != domain.EventDelta || second.Text != " asked" { t.Fatalf("second=%#v", second) }
    if done.Kind != domain.EventDone { t.Fatalf("done=%#v", done) }
}
