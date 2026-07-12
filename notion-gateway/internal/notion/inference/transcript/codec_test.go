package transcript

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/notioncodex/gateway/internal/domain"
)

func TestBuild(t *testing.T) {
	body, err := Build(BuildOptions{SpaceID: "space", Model: "dynamic-model", Input: "hello", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}

	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if request["spaceId"] != "space" {
		t.Fatalf("request = %#v", request)
	}
	transcript, ok := request["transcript"].([]any)
	if !ok || len(transcript) != 3 {
		t.Fatalf("transcript = %#v", request["transcript"])
	}
	user, ok := transcript[2].(map[string]any)
	if !ok {
		t.Fatalf("user item = %#v", transcript[2])
	}
	value, ok := user["value"].([]any)
	if !ok || len(value) != 1 {
		t.Fatalf("user value = %#v", user["value"])
	}
	row, ok := value[0].([]any)
	if !ok || len(row) != 1 || row[0] != "hello" {
		t.Fatalf("user row = %#v", value[0])
	}
}

func TestDecoderSnapshotDeltaAndDone(t *testing.T) {
	input := strings.Join([]string{
		`{"recordMap":{"message":{"step":{"type":"agent-inference","value":[{"type":"text","content":"hel"}]}}}}`,
		`{"recordMap":{"message":{"step":{"type":"agent-inference","value":[{"type":"text","content":"hello"}]}}}}`,
		`{"type":"agent-turn-full-record-map"}`,
	}, "\n")

	decoder := NewDecoder(strings.NewReader(input))
	first, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	second, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	done, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if first.Kind != domain.EventDelta || first.Text != "hel" || second.Text != "lo" || done.Kind != domain.EventDone {
		t.Fatalf("events = %#v %#v %#v", first, second, done)
	}
	_, err = decoder.Next(context.Background())
	if !errors.Is(err, io.EOF) { t.Fatalf("error = %v", err) }
}

func TestDecoderArrayStreamDeltaAndDone(t *testing.T) {
	input := `[
		{"recordMap":{"message":{"step":{"type":"agent-inference","value":[{"type":"text","content":"sol"}]}}}},
		{"type":"agent-turn-full-record-map","step":{"type":"agent-inference","value":[{"type":"text","content":"sol-live-test"}],"finishedAt":123}}
	]`
	decoder := NewDecoder(strings.NewReader(input))
	first, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	second, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	done, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if first.Kind != domain.EventDelta || first.Text != "sol" { t.Fatalf("first = %#v", first) }
	if second.Kind != domain.EventDelta || second.Text != "-live-test" { t.Fatalf("second = %#v", second) }
	if done.Kind != domain.EventDone { t.Fatalf("done = %#v", done) }
	_, err = decoder.Next(context.Background())
	if !errors.Is(err, io.EOF) { t.Fatalf("error = %v", err) }
}

func TestDecoderTextMapContent(t *testing.T) {
	input := `[{"step":{"value":[{"type":"text","text":{"content":"sol-live-test"}}]}},{"type":"agent-turn-full-record-map"}]`
	decoder := NewDecoder(strings.NewReader(input))
	delta, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if delta.Kind != domain.EventDelta || delta.Text != "sol-live-test" {
		t.Fatalf("delta = %#v", delta)
	}
}

func TestDecoderFallbackAssistantStrings(t *testing.T) {
	input := `[{"type":"agent-inference","message":"sol-live-test","finishedAt":123}]`
	decoder := NewDecoder(strings.NewReader(input))
	delta, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if delta.Kind != domain.EventDelta || delta.Text != "sol-live-test" {
		t.Fatalf("delta = %#v", delta)
	}
	done, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if done.Kind != domain.EventDone {
		t.Fatalf("done = %#v", done)
	}
}

func TestDecoderDoesNotEndOnTurnMapPlaceholder(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"id":"1","type":"agent-turn-full-record-map"}}]}`,
		`{"type":"patch-sync","data":{"s":[{"id":"2","type":"agent-inference","value":[{"type":"text","content":"Received."}],"finishedAt":123}]}}`,
	}, "\n")
	decoder := NewDecoder(strings.NewReader(input))
	delta, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if delta.Kind != domain.EventDelta || delta.Text != "Received." {
		t.Fatalf("delta = %#v", delta)
	}
	done, err := decoder.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if done.Kind != domain.EventDone {
		t.Fatalf("done = %#v", done)
	}
}


func TestDecoderPatchStreamIgnoresToolUseAndStreamsText(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-inference","value":[{"type":"thinking","content":"hidden"},{"type":"tool_use","content":"ignored"}]}}]}`,
		`{"type":"patch","v":[{"o":"a","p":"/s/0/value/-","v":{"type":"text","content":"I"}}]}`,
		`{"type":"patch","v":[{"o":"x","p":"/s/0/value/2/content","v":" couldn't"}]}`,
		`{"type":"patch","v":[{"o":"x","p":"/s/0/value/2/content","v":" find"}]}`,
		`{"type":"patch","v":[{"o":"a","p":"/s/0/finishedAt","v":123}]}`,
	}, "\n")
	decoder := NewDecoder(strings.NewReader(input))
	first, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if first.Kind != domain.EventDelta || first.Text != "I" { t.Fatalf("first = %#v", first) }
	second, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if second.Kind != domain.EventDelta || second.Text != " couldn't" { t.Fatalf("second = %#v", second) }
	third, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if third.Kind != domain.EventDelta || third.Text != " find" { t.Fatalf("third = %#v", third) }
	done, err := decoder.Next(context.Background())
	if err != nil { t.Fatal(err) }
	if done.Kind != domain.EventDone { t.Fatalf("done = %#v", done) }
}

func TestDecoderIgnoresToolUseSnapshotOutput(t *testing.T) {
	input := `{"data":{"s":[{"type":"agent-inference","value":[{"type":"thinking","content":"hidden"},{"type":"tool_use","content":"ignored"}],"finishedAt":123}]},"type":"patch-sync"}`
	decoder := NewDecoder(strings.NewReader(input))
	_, err := decoder.Next(context.Background())
	if !errors.Is(err, io.EOF) { t.Fatalf("error = %v", err) }
}

func FuzzDecoder(f *testing.F) {
	f.Add([]byte(`{"type":"agent-turn-full-record-map"}`))
	f.Add([]byte(`[{"type":"agent-turn-full-record-map"}]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			t.Skip()
		}
		decoder := NewDecoder(strings.NewReader(string(data)))
		for range 16 {
			_, err := decoder.Next(context.Background())
			if err != nil {
				return
			}
		}
	})
}
