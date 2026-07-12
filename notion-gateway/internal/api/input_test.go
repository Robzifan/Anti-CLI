package api

import (
    "encoding/json"
    "testing"
)

func TestParseInputUsesLatestUserMessage(t *testing.T) {
    raw := json.RawMessage(`[
        {"role":"user","content":[{"type":"input_text","text":"old user request"}]},
        {"role":"assistant","content":[{"type":"output_text","text":"old assistant answer"}]},
        {"role":"user","content":[{"type":"input_text","text":"latest Codex request"}]}
    ]`)
    got, err := parseInput(raw)
    if err != nil { t.Fatal(err) }
    if got != "latest Codex request" { t.Fatalf("got %q", got) }
}

func TestParseInputStringCompatibility(t *testing.T) {
    got, err := parseInput(json.RawMessage(`"plain input"`))
    if err != nil { t.Fatal(err) }
    if got != "plain input" { t.Fatalf("got %q", got) }
}
